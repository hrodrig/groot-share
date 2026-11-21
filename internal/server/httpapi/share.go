package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

// shareCreateRequest is the JSON body for POST /v1/shares/{id...}.
// Exactly one of ExpiresAt (RFC3339) or ExpiresIn (Go duration) is allowed.
type shareCreateRequest struct {
	ExpiresAt string `json:"expires_at"`
	ExpiresIn string `json:"expires_in"`
	Label     string `json:"label"`
	MaxUses   int    `json:"max_uses"`
}

func (srv *API) HandleCreateShare(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(r.PathValue("id"), "/")
	if id == "" {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	// Resolve the archive first so a 404 is honest for unknown ids.
	if _, err := srv.ResolveArchive(r.Context(), id); err != nil {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	var req shareCreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	now := time.Now().UTC()
	hasAt, hasIn := req.ExpiresAt != "", req.ExpiresIn != ""
	if hasAt == hasIn {
		core.WriteJSONError(w, http.StatusBadRequest, "expires_at or expires_in (exactly one)")
		return
	}
	var expires time.Time
	switch {
	case hasAt:
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			core.WriteJSONError(w, http.StatusBadRequest, "invalid expires_at")
			return
		}
		expires = t.UTC()
	case hasIn:
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil || d <= 0 {
			core.WriteJSONError(w, http.StatusBadRequest, "invalid expires_in")
			return
		}
		expires = now.Add(d)
	}
	if !expires.After(now) {
		core.WriteJSONError(w, http.StatusBadRequest, "expiry already passed")
		return
	}
	if req.MaxUses < 0 {
		core.WriteJSONError(w, http.StatusBadRequest, "max_uses must be >= 0")
		return
	}
	ac := core.ActorFrom(r.Context())
	raw, hash, err := auth.NewShareToken()
	if err != nil {
		slog.Error("share token", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	link, err := srv.Store.CreateShareLink(r.Context(), id, hash, ac.User.ID, req.Label, req.MaxUses, expires)
	if err != nil {
		slog.Error("create share link", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	// Audit the create (admin actor); raw token is never logged or stored.
	srv.RecordUserAudit(r, "share_create", strconv.FormatInt(link.ID, 10), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         link.ID,
		"url":        srv.RequestBaseURL(r) + "/s/" + raw,
		"expires_at": link.ExpiresAt.UTC().Format(time.RFC3339),
		"max_uses":   link.MaxUses,
		"label":      link.Label,
	})
}

func (srv *API) HandleListShares(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(r.PathValue("id"), "/")
	if id == "" {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	if _, err := srv.ResolveArchive(r.Context(), id); err != nil {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	links, err := srv.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(links))
	now := time.Now().UTC()
	for _, l := range links {
		out = append(out, shareJSON(l, now))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (srv *API) HandleRevokeShare(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(r.PathValue("share_id"), "/")
	shareID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || shareID <= 0 {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	if err := srv.Store.RevokeShareLink(r.Context(), shareID, time.Now().UTC()); err != nil {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	srv.RecordUserAudit(r, "share_revoke", raw, "")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"revoked": true})
}

// handleShareDownload serves the public GET /s/{token}.
func (srv *API) HandleShareDownload(w http.ResponseWriter, r *http.Request) {
	token := strings.Trim(r.PathValue("token"), "/")
	if token == "" {
		srv.NotFound(w, r)
		return
	}
	link, err := srv.Store.ShareByTokenHash(r.Context(), auth.HashSecret(token))
	if err != nil {
		// Unknown token (or DB error): 404 either way, no oracle.
		srv.NotFound(w, r)
		return
	}
	now := time.Now().UTC()
	if reason := shareInactiveReason(link, now); reason != "" {
		srv.ShareGone(w, r, reason)
		return
	}
	rc, a, err := srv.OpenDownload(r.Context(), link.ArchiveID)
	if err != nil {
		srv.NotFound(w, r)
		return
	}
	// Increment the use atomically BEFORE serving, draining the cap so a
	// max_uses=N link cannot oversell under concurrency. Fail closed: if the
	// atomic bump reports exhausted/revoked/expired, do not stream the bytes.
	if _, err := srv.Store.IncrementShareUse(r.Context(), link.ID, now); err != nil {
		switch {
		case errors.Is(err, store.ErrShareExhausted):
			_ = rc.Close()
			srv.ShareGone(w, r, "exhausted")
			return
		case errors.Is(err, store.ErrShareNotFound):
			_ = rc.Close()
			srv.NotFound(w, r)
			return
		default:
			// DB hiccup mid-request: log but serve (the link already passed
			// the Active check; failing closed on a transient error would
			// break legitimate downloads). Matches prior lenient behavior.
			slog.Warn("share use increment", "error", err, "share", link.ID)
		}
	}
	srv.recordShareDownload(r, link, a)
	serveBlob(w, r, a, rc)
}

func (srv *API) recordShareDownload(r *http.Request, link store.ShareLink, a store.Archive) {
	if srv.Store == nil {
		return
	}
	ev := store.Audit{
		Actor:     "share",
		Action:    "share_download",
		ObjectID:  a.ID,
		ObjectKey: a.Key,
		RemoteIP:  core.RemoteIP(r.RemoteAddr),
	}
	if err := srv.Store.InsertAudit(r.Context(), ev); err != nil {
		slog.Error("audit share download", "error", err)
	}
}

func shareJSON(l store.ShareLink, now time.Time) map[string]any {
	out := map[string]any{
		"id":         l.ID,
		"label":      l.Label,
		"max_uses":   l.MaxUses,
		"use_count":  l.UseCount,
		"created_at": l.CreatedAt.UTC().Format(time.RFC3339),
		"active":     l.Active(now),
	}
	if !l.ExpiresAt.IsZero() {
		out["expires_at"] = l.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if !l.RevokedAt.IsZero() {
		out["revoked_at"] = l.RevokedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// shareInactiveReason returns a stable machine string explaining why a link is
// no longer servable, or "" when it is still active. Order matches store.
// ShareLink.Active: revoked (explicit admin action) wins over expiry/exhaust.
func shareInactiveReason(l store.ShareLink, now time.Time) string {
	switch {
	case !l.RevokedAt.IsZero():
		return "revoked"
	case !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt):
		return "expired"
	case l.MaxUses > 0 && l.UseCount >= l.MaxUses:
		return "exhausted"
	default:
		return ""
	}
}

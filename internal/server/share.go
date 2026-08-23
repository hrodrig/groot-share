package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

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

func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(r.PathValue("id"), "/")
	if id == "" {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	// Resolve the archive first so a 404 is honest for unknown ids.
	if _, err := s.resolveArchive(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	var req shareCreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	now := time.Now().UTC()
	hasAt, hasIn := req.ExpiresAt != "", req.ExpiresIn != ""
	if hasAt == hasIn {
		writeJSONError(w, http.StatusBadRequest, "expires_at or expires_in (exactly one)")
		return
	}
	var expires time.Time
	switch {
	case hasAt:
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid expires_at")
			return
		}
		expires = t.UTC()
	case hasIn:
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil || d <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid expires_in")
			return
		}
		expires = now.Add(d)
	}
	if !expires.After(now) {
		writeJSONError(w, http.StatusBadRequest, "expiry already passed")
		return
	}
	if req.MaxUses < 0 {
		writeJSONError(w, http.StatusBadRequest, "max_uses must be >= 0")
		return
	}
	ac := actorFrom(r.Context())
	raw, hash, err := auth.NewShareToken()
	if err != nil {
		slog.Error("share token", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	link, err := s.Store.CreateShareLink(r.Context(), id, hash, ac.User.ID, req.Label, req.MaxUses, expires)
	if err != nil {
		slog.Error("create share link", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	// Audit the create (admin actor); raw token is never logged or stored.
	s.recordUserAudit(r, "share_create", strconv.FormatInt(link.ID, 10), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         link.ID,
		"url":        s.requestBaseURL(r) + "/s/" + raw,
		"expires_at": link.ExpiresAt.UTC().Format(time.RFC3339),
		"max_uses":   link.MaxUses,
		"label":      link.Label,
	})
}

func (s *Server) handleListShares(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(r.PathValue("id"), "/")
	if id == "" {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	if _, err := s.resolveArchive(r.Context(), id); err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	links, err := s.Store.ListShareLinks(r.Context(), id)
	if err != nil {
		slog.Error("list share links", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
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

func (s *Server) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	raw := strings.Trim(r.PathValue("share_id"), "/")
	shareID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || shareID <= 0 {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	if err := s.Store.RevokeShareLink(r.Context(), shareID, time.Now().UTC()); err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	s.recordUserAudit(r, "share_revoke", raw, "")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"revoked": true})
}

// handleShareDownload serves the public GET /s/{token}.
func (s *Server) handleShareDownload(w http.ResponseWriter, r *http.Request) {
	token := strings.Trim(r.PathValue("token"), "/")
	if token == "" {
		s.handleNotFound(w, r)
		return
	}
	link, err := s.Store.ShareByTokenHash(r.Context(), auth.HashSecret(token))
	if err != nil {
		// Unknown token (or DB error): 404 either way, no oracle.
		s.handleNotFound(w, r)
		return
	}
	now := time.Now().UTC()
	if reason := shareInactiveReason(link, now); reason != "" {
		s.writeShareGone(w, r, reason)
		return
	}
	rc, a, err := s.openDownload(r.Context(), link.ArchiveID)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	defer func() { _ = rc.Close() }()
	// Increment use after a successful open (one-shot links consume once).
	if _, err := s.Store.IncrementShareUse(r.Context(), link.ID); err != nil {
		slog.Warn("share use increment", "error", err, "share", link.ID)
	}
	s.recordShareDownload(r, link, a)
	serveBlob(w, r, a, rc)
}

func (s *Server) recordShareDownload(r *http.Request, link store.ShareLink, a store.Archive) {
	if s.Store == nil {
		return
	}
	ev := store.Audit{
		Actor:     "share",
		Action:    "share_download",
		ObjectID:  a.ID,
		ObjectKey: a.Key,
		RemoteIP:  remoteIP(r.RemoteAddr),
	}
	if err := s.Store.InsertAudit(r.Context(), ev); err != nil {
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

// writeShareGone renders a 410 Gone HTML page with a clear, honest reason for
// a share link that once existed but is no longer servable (revoked, expired,
// or use-limit reached). Distinct from 404, which is reserved for tokens that
// do not resolve, so token-guessing gets no oracle. Unlike the API-facing
// handleNotFound, this always renders HTML: /s/{token} is a public download
// opened in a browser by a human, not an API client.
func (s *Server) writeShareGone(w http.ResponseWriter, r *http.Request, reason string) {
	var title, body string
	switch reason {
	case "revoked":
		title = "Share link revoked"
		body = "This share link was revoked and no longer works."
	case "expired":
		title = "Share link expired"
		body = "This share link has expired."
	case "exhausted":
		title = "Share link used up"
		body = "This share link reached its download limit and no longer works."
	default:
		title = "Share link unavailable"
		body = "This share link is no longer available."
	}
	data := s.pageShell()
	data["GoneTitle"] = title
	data["GoneBody"] = body
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusGone)
	_ = shareGoneTmpl.Execute(w, data)
}

var shareGoneTmpl = template.Must(template.New("share-gone").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Share link</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
    </div>
  </div>
</header>
<main id="main" class="wrap">
  <section class="card" style="max-width:460px;margin:48px auto;">
    <div class="card-head">
      <h2>410 — {{.GoneTitle}}</h2>
      <p class="hint">{{.GoneBody}}</p>
    </div>
    <div style="padding:16px 24px 20px;">
      <a class="btn" href="/">Go to Home</a>
    </div>
  </section>
</main>
{{.ThemeToggleScript}}
</body></html>
`))

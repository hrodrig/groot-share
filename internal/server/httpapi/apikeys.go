package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *API) HandleListMyAPIKeys(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	items, err := srv.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if err != nil {
		slog.Error("list api keys", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, k := range items {
		out = append(out, apiKeyJSON(k))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (srv *API) HandleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, err := core.ParseUserID(strings.Trim(r.PathValue("id"), "/"))
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := srv.Store.DeleteAPIKey(r.Context(), id, ac.User.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.WriteJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := srv.RequireUserAudit(r, "apikey.revoke", strconv.FormatInt(id, 10), ac.User.Username); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func apiKeyJSON(k store.APIKeyRecord) map[string]any {
	out := map[string]any{
		"id":     k.ID,
		"prefix": k.Prefix,
		"scope":  k.Scope,
	}
	if !k.CreatedAt.IsZero() {
		out["created_at"] = k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if !k.LastUsedAt.IsZero() {
		out["last_used_at"] = k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return out
}

func (srv *API) createAPIKeyForUser(w http.ResponseWriter, r *http.Request, userID int64, role auth.Role) (raw, prefix string, scope auth.KeyScope, err error) {
	scope = auth.KeyScopeUpload
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Scope string `json:"scope"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if decErr := dec.Decode(&body); decErr == nil && body.Scope != "" {
			scope = auth.KeyScope(body.Scope)
		}
	} else if err = r.ParseForm(); err == nil {
		if v := strings.TrimSpace(r.Form.Get("scope")); v != "" {
			scope = auth.KeyScope(v)
		}
	}
	if scope != auth.KeyScopeUpload && scope != auth.KeyScopeRead {
		return "", "", "", ErrBadAPIKeyScope
	}
	if role == auth.RoleViewer {
		return "", "", "", ErrForbiddenAPIKeyScope
	}
	if role == auth.RoleUploader && scope == auth.KeyScopeRead {
		return "", "", "", ErrForbiddenAPIKeyScope
	}
	raw, hash, prefix, err := auth.NewAPIKey()
	if err != nil {
		return "", "", "", err
	}
	if err := srv.Store.CreateAPIKey(r.Context(), userID, hash, prefix, scope); err != nil {
		return "", "", "", err
	}
	return raw, prefix, scope, nil
}

var (
	ErrBadAPIKeyScope       = errors.New("bad scope")
	ErrForbiddenAPIKeyScope = errors.New("forbidden scope")
)

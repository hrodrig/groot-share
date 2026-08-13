package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) handleListMyAPIKeys(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	items, err := s.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if err != nil {
		slog.Error("list api keys", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, k := range items {
		out = append(out, apiKeyJSON(k))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	id, err := parseUserID(strings.Trim(r.PathValue("id"), "/"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.Store.DeleteAPIKey(r.Context(), id, ac.User.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.recordUserAudit(r, "apikey.revoke", strconv.FormatInt(id, 10), ac.User.Username)
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
	return out
}

func (s *Server) createAPIKeyForUser(w http.ResponseWriter, r *http.Request, userID int64, role auth.Role) (raw, prefix string, scope auth.KeyScope, err error) {
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
		return "", "", "", errBadScope
	}
	if role == auth.RoleViewer {
		return "", "", "", errForbiddenScope
	}
	if role == auth.RoleUploader && scope == auth.KeyScopeRead {
		return "", "", "", errForbiddenScope
	}
	raw, hash, prefix, err := auth.NewAPIKey()
	if err != nil {
		return "", "", "", err
	}
	if err := s.Store.CreateAPIKey(r.Context(), userID, hash, prefix, scope); err != nil {
		return "", "", "", err
	}
	return raw, prefix, scope, nil
}

var (
	errBadScope       = errors.New("bad scope")
	errForbiddenScope = errors.New("forbidden scope")
)

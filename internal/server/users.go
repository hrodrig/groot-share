package server

import (
	"context"
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

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListUsers(r.Context())
	if err != nil {
		slog.Error("list users", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		out = append(out, userJSON(u))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	rec, err := s.userRecord(r.Context(), u)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(userJSON(rec))
}

func (s *Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	id, u, ok := s.loadUserForPatch(w, r)
	if !ok {
		return
	}
	var body patchUserBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !s.applyPatchUserFields(w, r, id, u, body) {
		return
	}
	s.writeUserJSON(w, r, id)
}

type patchUserBody struct {
	Role     *string `json:"role"`
	Active   *bool   `json:"active"`
	Password string  `json:"password"`
	Name     *string `json:"name"`
	Username *string `json:"username"`
}

func (s *Server) loadUserForPatch(w http.ResponseWriter, r *http.Request) (int64, store.User, bool) {
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return 0, store.User{}, false
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return 0, store.User{}, false
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return 0, store.User{}, false
	}
	return id, u, true
}

func (s *Server) applyPatchUserFields(w http.ResponseWriter, r *http.Request, id int64, u store.User, body patchUserBody) bool {
	if !s.patchUserRoleActive(w, r, id, u, body) {
		return false
	}
	if !s.patchUserPassword(w, r, id, body.Password) {
		return false
	}
	if body.Name != nil && !s.patchUserName(w, r, id, *body.Name) {
		return false
	}
	if body.Username != nil && !s.patchUserUsername(w, r, id, *body.Username) {
		return false
	}
	return true
}

func (s *Server) patchUserRoleActive(w http.ResponseWriter, r *http.Request, id int64, u store.User, body patchUserBody) bool {
	newRole := u.Role
	if body.Role != nil {
		newRole = auth.Role(*body.Role)
		if !auth.ValidRole(newRole) {
			writeJSONError(w, http.StatusBadRequest, "bad_request")
			return false
		}
	}
	newActive := u.Active
	if body.Active != nil {
		newActive = *body.Active
	}
	if body.Role == nil && body.Active == nil {
		return true
	}
	if err := s.Store.GuardLastAdmin(r.Context(), id, newRole, newActive); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			writeJSONError(w, http.StatusConflict, "last_admin")
			return false
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if err := s.Store.UpdateUser(r.Context(), id, newRole, newActive); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	s.recordUserAudit(r, "user.update", strconv.FormatInt(id, 10), u.Username)
	return true
}

func (s *Server) patchUserPassword(w http.ResponseWriter, r *http.Request, id int64, password string) bool {
	if password == "" {
		return true
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return false
	}
	if err := s.Store.SetPassword(r.Context(), id, hash); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	return true
}

func (s *Server) patchUserName(w http.ResponseWriter, r *http.Request, id int64, name string) bool {
	if err := s.Store.SetName(r.Context(), id, name); err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			writeJSONError(w, http.StatusBadRequest, "bad_request")
			return false
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	return true
}

func (s *Server) patchUserUsername(w http.ResponseWriter, r *http.Request, id int64, username string) bool {
	if err := s.Store.SetUsername(r.Context(), id, username); err != nil {
		if errors.Is(err, store.ErrUsernameRequired) || errors.Is(err, store.ErrUsernameTooLong) {
			writeJSONError(w, http.StatusBadRequest, "bad_request")
			return false
		}
		if isUniqueViolation(err) {
			writeJSONError(w, http.StatusConflict, "conflict")
			return false
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	s.recordUserAudit(r, "user.update", strconv.FormatInt(id, 10), strings.TrimSpace(username))
	return true
}

func (s *Server) writeUserJSON(w http.ResponseWriter, r *http.Request, id int64) {
	updated, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	rec, err := s.userRecord(r.Context(), updated)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(userJSON(rec))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.Store.GuardLastAdmin(r.Context(), id, u.Role, false); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			writeJSONError(w, http.StatusConflict, "last_admin")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.Store.UpdateUser(r.Context(), id, u.Role, false); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.recordUserAudit(r, "user.deactivate", strconv.FormatInt(id, 10), u.Username)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ac.Method != auth.AuthSession {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.Store.SetPassword(r.Context(), ac.User.ID, hash); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) userRecord(ctx context.Context, u store.User) (store.UserRecord, error) {
	items, err := s.Store.ListUsers(ctx)
	if err != nil {
		return store.UserRecord{}, err
	}
	for _, rec := range items {
		if rec.ID == u.ID {
			return rec, nil
		}
	}
	return store.UserRecord{User: u}, nil
}

func userJSON(u store.UserRecord) map[string]any {
	out := map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"name":     u.Name,
		"role":     u.Role,
		"active":   u.Active,
	}
	if !u.CreatedAt.IsZero() {
		out["created_at"] = u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return out
}

func parseUserID(raw string) (int64, error) {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return 0, errors.New("empty id")
	}
	return strconv.ParseInt(raw, 10, 64)
}

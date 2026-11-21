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

func (srv *API) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := srv.Store.ListUsers(r.Context())
	if err != nil {
		slog.Error("list users", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		out = append(out, core.UserJSON(u))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
}

func (srv *API) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := core.ParseUserID(r.PathValue("id"))
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := srv.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.WriteJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	rec, err := srv.UserRecord(r.Context(), u)
	if err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(core.UserJSON(rec))
}

func (srv *API) HandlePatchUser(w http.ResponseWriter, r *http.Request) {
	id, u, ok := srv.loadUserForPatch(w, r)
	if !ok {
		return
	}
	var body patchUserBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !srv.applyPatchUserFields(w, r, id, u, body) {
		return
	}
	srv.writeUserJSON(w, r, id)
}

type patchUserBody struct {
	Role     *string `json:"role"`
	Active   *bool   `json:"active"`
	Password string  `json:"password"`
	Name     *string `json:"name"`
	Username *string `json:"username"`
}

func (srv *API) loadUserForPatch(w http.ResponseWriter, r *http.Request) (int64, store.User, bool) {
	id, err := core.ParseUserID(r.PathValue("id"))
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return 0, store.User{}, false
	}
	u, err := srv.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.WriteJSONError(w, http.StatusNotFound, "not_found")
			return 0, store.User{}, false
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return 0, store.User{}, false
	}
	return id, u, true
}

func (srv *API) applyPatchUserFields(w http.ResponseWriter, r *http.Request, id int64, u store.User, body patchUserBody) bool {
	if !srv.patchUserRoleActive(w, r, id, u, body) {
		return false
	}
	if !srv.patchUserPassword(w, r, id, body.Password) {
		return false
	}
	if body.Name != nil && !srv.patchUserName(w, r, id, *body.Name) {
		return false
	}
	if body.Username != nil && !srv.patchUserUsername(w, r, id, *body.Username) {
		return false
	}
	return true
}

func (srv *API) patchUserRoleActive(w http.ResponseWriter, r *http.Request, id int64, u store.User, body patchUserBody) bool {
	newRole := u.Role
	if body.Role != nil {
		newRole = auth.Role(*body.Role)
		if !auth.ValidRole(newRole) {
			core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
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
	if err := srv.Store.GuardLastAdmin(r.Context(), id, newRole, newActive); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			core.WriteJSONError(w, http.StatusConflict, "last_admin")
			return false
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if err := srv.Store.UpdateUser(r.Context(), id, newRole, newActive); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if err := srv.RequireUserAudit(r, "user.update", strconv.FormatInt(id, 10), u.Username); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return false
	}
	return true
}

func (srv *API) patchUserPassword(w http.ResponseWriter, r *http.Request, id int64, password string) bool {
	if password == "" {
		return true
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return false
	}
	if err := srv.Store.SetPassword(r.Context(), id, hash); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	return true
}

func (srv *API) patchUserName(w http.ResponseWriter, r *http.Request, id int64, name string) bool {
	if err := srv.Store.SetName(r.Context(), id, name); err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
			return false
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	return true
}

func (srv *API) patchUserUsername(w http.ResponseWriter, r *http.Request, id int64, username string) bool {
	if err := srv.Store.SetUsername(r.Context(), id, username); err != nil {
		if errors.Is(err, store.ErrUsernameRequired) || errors.Is(err, store.ErrUsernameTooLong) {
			core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
			return false
		}
		if core.IsUniqueViolation(err) {
			core.WriteJSONError(w, http.StatusConflict, "conflict")
			return false
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if err := srv.RequireUserAudit(r, "user.update", strconv.FormatInt(id, 10), strings.TrimSpace(username)); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return false
	}
	return true
}

func (srv *API) writeUserJSON(w http.ResponseWriter, r *http.Request, id int64) {
	updated, err := srv.Store.UserByID(r.Context(), id)
	if err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	rec, err := srv.UserRecord(r.Context(), updated)
	if err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(core.UserJSON(rec))
}

func (srv *API) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := core.ParseUserID(r.PathValue("id"))
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	u, err := srv.Store.UserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			core.WriteJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := srv.Store.GuardLastAdmin(r.Context(), id, u.Role, false); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			core.WriteJSONError(w, http.StatusConflict, "last_admin")
			return
		}
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := srv.Store.UpdateUser(r.Context(), id, u.Role, false); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := srv.RequireUserAudit(r, "user.deactivate", strconv.FormatInt(id, 10), u.Username); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

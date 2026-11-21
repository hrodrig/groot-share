package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *API) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Admin    bool   `json:"admin"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	role := auth.RoleUploader
	if body.Role != "" {
		role = auth.Role(body.Role)
	} else if body.Admin {
		role = auth.RoleAdmin
	}
	if !auth.ValidRole(role) {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	created, err := srv.Store.CreateUser(r.Context(), body.Username, name, hash, role)
	if err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
			return
		}
		core.WriteJSONError(w, http.StatusConflict, "conflict")
		return
	}
	if err := srv.RequireUserAudit(r, "user.create", strconv.FormatInt(created.ID, 10), created.Username); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return
	}
	rec, _ := srv.UserRecord(r.Context(), created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(core.UserJSON(rec))
}

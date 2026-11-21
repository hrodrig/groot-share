package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
)

func (srv *API) HandlePatchMe(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil {
		core.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ac.Method != auth.AuthSession {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Password == "" {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := srv.Store.SetPassword(r.Context(), ac.User.ID, hash); err != nil {
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	srv.SetSessionCookie(w, "", -1)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

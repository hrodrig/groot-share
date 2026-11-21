package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
)

func (srv *API) HandleMe(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil {
		core.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"username": ac.User.Username, "name": ac.User.Name, "role": ac.User.Role})
}

package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
)

func (srv *API) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ac := core.ActorFrom(r.Context())
	if ac == nil {
		core.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ac.Method == auth.AuthAPIKey {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	raw, prefix, scope, err := srv.CreateAPIKeyForUser(w, r, ac.User.ID, ac.User.Role)
	if errors.Is(err, core.ErrBadAPIKeyScope) {
		core.WriteJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if errors.Is(err, core.ErrForbiddenAPIKeyScope) {
		core.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err != nil {
		slog.Error("api key create failed", "error", err)
		core.WriteJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"api_key": raw, "prefix": prefix, "scope": scope})
}

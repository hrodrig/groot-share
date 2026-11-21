package server

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/auth"
)

func (s *Server) pinRoutes(mux *http.ServeMux) {
	mux.Handle("POST /v1/pin/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.api.HandlePin))
	mux.Handle("DELETE /v1/pin/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.api.HandlePin))
}

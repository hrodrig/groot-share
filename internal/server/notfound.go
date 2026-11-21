// Package server: friendly 404 surface for browser clients, JSON for API.
package server

import (
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if core.WantsJSON(r) ||
		strings.HasPrefix(r.URL.Path, "/v1/") ||
		strings.HasPrefix(r.URL.Path, "/s/") {
		core.WriteJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	data := s.ui.PageShell()
	data["Path"] = r.URL.Path
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = tmpl.NotFound.Execute(w, data)
}

// writeShareGone is injected into httpapi for expired/revoked share links (HTML).
func (s *Server) writeShareGone(w http.ResponseWriter, r *http.Request, reason string) {
	if core.WantsJSON(r) {
		core.WriteJSONError(w, http.StatusGone, reason)
		return
	}
	data := s.ui.PageShell()
	data["Reason"] = reason
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusGone)
	_ = tmpl.ShareGone.Execute(w, data)
}

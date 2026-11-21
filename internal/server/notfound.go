// Package server: friendly 404 surface for browser clients, JSON for API.
package server

import (
	"github.com/hrodrig/groot-share/internal/server/tmpl"
	"net/http"
	"strings"
)

// notFoundTmpl renders the 404 page reusing the page shell (brand, theme,
// favicon, layout CSS). It intentionally omits the app nav: a 404 is
// reached for unmatched routes which may not have an authenticated actor,
// and the primary CTA ("Back to Captures") is enough to recover.

// handleNotFound renders a 404 response. Browser requests (HTML accept, or
// any path that is not the JSON API surface) get the branded page; API
// requests keep the structured JSON error.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) ||
		strings.HasPrefix(r.URL.Path, "/v1/") ||
		strings.HasPrefix(r.URL.Path, "/s/") {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	data := s.pageShell()
	data["Path"] = r.URL.Path
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = tmpl.NotFound.Execute(w, data)
}

package server

import (
	"net/http"
)

// defaultCSP is the built-in Content-Security-Policy emitted on HTML pages
// when GFS_CSP is unset. script-src/style-src must allow 'unsafe-inline':
// the whole front end is inline <style> ({{.CSS}}) and a handful of inline
// <script> blocks (theme, password toggle, filters). There are no external
// scripts, styles, or frames. img-src 'self' data: covers the favicons and
// any inline data-URI marks; connect-src 'self' allows the fetch() calls the
// UI makes to /v1/*. A stricter policy (nonce/hash) needs the CSS/JS moved out
// of the templates first.
const defaultCSP = "default-src 'self'; base-uri 'none'; object-src 'none'; " +
	"frame-ancestors 'none'; form-action 'self'; " +
	"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self'"

// securityHeaders applies defensive response headers to every route. It runs
// outermost so headers are set regardless of which handler (or the access
// log) writes the response.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	csp := s.Cfg.CSP
	if csp == "" {
		csp = defaultCSP
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Strict-Transport-Security only makes sense over TLS. Emit it when the
		// connection is TLS or a trusted proxy forwarded https; never on plain
		// HTTP (an incidental HSTS header on :8080 would poison the host).
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if csp != "-" {
			h.Set("Content-Security-Policy", csp)
		}
		next.ServeHTTP(w, r)
	})
}

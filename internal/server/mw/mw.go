// Package mw provides HTTP middleware for the gfs server (access log, security headers).
package mw

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

// DefaultCSP is the built-in Content-Security-Policy emitted on HTML pages
// when GFS_CSP is unset. script-src/style-src must allow 'unsafe-inline':
// the whole front end is inline <style> and a handful of inline <script>
// blocks. There are no external scripts, styles, or frames.
const DefaultCSP = "default-src 'self'; base-uri 'none'; object-src 'none'; " +
	"frame-ancestors 'none'; form-action 'self'; " +
	"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self'"

// SecurityHeaders applies defensive response headers to every route. It should
// wrap outermost so headers are set regardless of which handler writes.
// Empty csp uses DefaultCSP; csp "-" disables the CSP header.
func SecurityHeaders(csp string, next http.Handler) http.Handler {
	if csp == "" {
		csp = DefaultCSP
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if csp != "-" {
			h.Set("Content-Security-Policy", csp)
		}
		next.ServeHTTP(w, r)
	})
}

// AccessLog logs method/path/status/ip/duration for non-probe requests.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}
		slog.Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"ip", RemoteIP(r.RemoteAddr),
			"dur", time.Since(start).Round(time.Millisecond),
		)
	})
}

// RemoteIP returns the host portion of remoteAddr (or the whole string).
func RemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

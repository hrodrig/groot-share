// Package server implements the HTTP API (probes + identity).
package server

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

// Server is the HTTP front-end.
type Server struct {
	Cfg   config.Config
	Store *store.Store
	Ready func() bool
}

// Handler returns the root mux with middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /login", s.handleLoginGET)
	mux.HandleFunc("POST /login", s.handleLoginPOST)
	mux.HandleFunc("POST /logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /{$}", s.requireAuth(s.handleHome))
	mux.HandleFunc("GET /v1/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("POST /v1/api-keys", s.requireAuth(s.handleCreateAPIKey))
	mux.HandleFunc("POST /v1/users", s.requireAuth(s.handleCreateUser))
	return s.accessLog(mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.Ready != nil && !s.Ready() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (s *Server) accessLog(next http.Handler) http.Handler {
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
			"ip", remoteIP(r.RemoteAddr),
			"dur", time.Since(start).Round(time.Millisecond),
		)
	})
}

func remoteIP(remoteAddr string) string {
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

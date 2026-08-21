// Package server implements the HTTP API (probes + identity).
package server

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/ratelimit"
	"github.com/hrodrig/groot-share/internal/store"
)

// Server is the HTTP front-end.
type Server struct {
	Cfg     config.Config
	Store   *store.Store
	Blobs   blob.Store
	Ready   func() bool
	Version string
	// LoginLimit gates POST /login (nil = unlimited; production sets from config).
	LoginLimit *ratelimit.Limiter
	// listCache memoizes vps-s3 listings (zero-value = cold, safe).
	listCache listCache
}

// Handler returns the root mux with middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /login", s.handleLoginGET)
	mux.HandleFunc("POST /login", s.handleLoginPOST)
	mux.HandleFunc("POST /logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /{$}", s.requirePermission(auth.PermArchivesRead, s.handleHome))
	mux.HandleFunc("GET /upload", s.requirePermission(auth.PermArchivesWrite, s.handleUploadGET))
	mux.HandleFunc("GET /activity", s.requirePermission(auth.PermAuditRead, s.handleActivityGET))
	mux.HandleFunc("GET /settings", s.requireAuth(s.handleSettingsGET))
	mux.HandleFunc("POST /settings/password", s.requireAuth(s.handleSettingsPasswordPOST))
	mux.HandleFunc("POST /settings/name", s.requireAuth(s.handleSettingsNamePOST))
	mux.HandleFunc("POST /settings/api-keys", s.requirePermission(auth.PermAPIKeysManage, s.handleSettingsAPIKeyPOST))
	mux.HandleFunc("POST /settings/api-keys/{id}/revoke", s.requireAuth(s.handleSettingsAPIKeyRevokePOST))
	mux.HandleFunc("GET /admin/users", s.requirePermission(auth.PermUsersManage, s.handleAdminUsersGET))
	mux.HandleFunc("POST /admin/users/create", s.requirePermission(auth.PermUsersManage, s.handleAdminUsersCreatePOST))
	mux.HandleFunc("POST /admin/users/{id}/role", s.requirePermission(auth.PermUsersManage, s.handleAdminUserRolePOST))
	mux.HandleFunc("POST /admin/users/{id}/username", s.requirePermission(auth.PermUsersManage, s.handleAdminUserUsernamePOST))
	mux.HandleFunc("POST /admin/users/{id}/deactivate", s.requirePermission(auth.PermUsersManage, s.handleAdminUserDeactivatePOST))
	mux.HandleFunc("POST /admin/users/{id}/activate", s.requirePermission(auth.PermUsersManage, s.handleAdminUserActivatePOST))
	mux.HandleFunc("POST /admin/users/{id}/remove", s.requirePermission(auth.PermUsersManage, s.handleAdminUserRemovePOST))
	mux.HandleFunc("GET /v1/me", s.requirePermission(auth.PermArchivesRead, s.handleMe))
	mux.HandleFunc("PATCH /v1/me", s.requireAuth(s.handlePatchMe))
	mux.HandleFunc("GET /v1/me/api-keys", s.requireAuth(s.handleListMyAPIKeys))
	mux.HandleFunc("DELETE /v1/me/api-keys/{id}", s.requireAuth(s.handleDeleteAPIKey))
	mux.HandleFunc("POST /v1/api-keys", s.requirePermission(auth.PermAPIKeysManage, s.handleCreateAPIKey))
	mux.HandleFunc("GET /v1/users", s.requirePermission(auth.PermUsersManage, s.handleListUsers))
	mux.HandleFunc("POST /v1/users", s.requirePermission(auth.PermUsersManage, s.handleCreateUser))
	mux.HandleFunc("GET /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.handleGetUser))
	mux.HandleFunc("PATCH /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.handlePatchUser))
	mux.HandleFunc("DELETE /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.handleDeleteUser))
	mux.HandleFunc("GET /v1/archives", s.requirePermission(auth.PermArchivesRead, s.handleListArchives))
	mux.HandleFunc("POST /v1/archives", s.requirePermission(auth.PermArchivesWrite, s.handleUpload))
	mux.HandleFunc("GET /v1/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.handleDownload))
	mux.HandleFunc("DELETE /v1/archives/{id...}", s.requirePermission(auth.PermArchivesDelete, s.handleDelete))
	mux.HandleFunc("POST /v1/archives/{id...}", s.requirePermission(auth.PermArchivesDelete, s.handleDelete))
	mux.HandleFunc("GET /v1/audit", s.requirePermission(auth.PermAuditRead, s.handleListAudit))
	mux.HandleFunc("POST /v1/archives/{id}/shares", s.requirePermission(auth.PermSharesManage, s.handleCreateShare))
	mux.HandleFunc("GET /v1/archives/{id}/shares", s.requirePermission(auth.PermSharesManage, s.handleListShares))
	mux.HandleFunc("DELETE /v1/archives/{id}/shares/{share_id}", s.requirePermission(auth.PermSharesManage, s.handleRevokeShare))
	mux.HandleFunc("GET /s/{token}", s.handleShareDownload)
	s.pinRoutes(mux)
	mountFaviconRoutes(mux)
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

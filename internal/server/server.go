// Package server implements the HTTP API (probes + identity).
package server

import (
	"io"
	"net/http"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/httpapi"
	"github.com/hrodrig/groot-share/internal/server/mw"
	"github.com/hrodrig/groot-share/internal/server/web"
)

// Server is the HTTP front-end.
type Server struct {
	*core.Core
	api *httpapi.API
	ui  *web.Web
}

// Handler returns the root mux with middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /login", s.ui.HandleLoginGET)
	mux.HandleFunc("POST /login", s.ui.HandleLoginPOST)
	mux.HandleFunc("POST /logout", s.requireAuth(s.ui.HandleLogout))
	mux.HandleFunc("GET /{$}", s.requirePermission(auth.PermArchivesRead, s.ui.HandleHome))
	mux.HandleFunc("GET /upload", s.requirePermission(auth.PermArchivesWrite, s.ui.HandleUploadGET))
	mux.HandleFunc("GET /activity", s.requirePermission(auth.PermAuditRead, s.ui.HandleActivityGET))
	mux.HandleFunc("GET /settings", s.requireAuth(s.ui.HandleSettingsGET))
	mux.HandleFunc("POST /settings/password", s.requireAuth(s.ui.HandleSettingsPasswordPOST))
	mux.HandleFunc("POST /settings/name", s.requireAuth(s.ui.HandleSettingsNamePOST))
	mux.HandleFunc("POST /settings/api-keys", s.requirePermission(auth.PermAPIKeysManage, s.ui.HandleSettingsAPIKeyPOST))
	mux.HandleFunc("POST /settings/api-keys/{id}/revoke", s.requireAuth(s.ui.HandleSettingsAPIKeyRevokePOST))
	mux.HandleFunc("GET /admin/users", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUsersGET))
	mux.HandleFunc("POST /admin/users/create", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUsersCreatePOST))
	mux.HandleFunc("POST /admin/users/{id}/role", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUserRolePOST))
	mux.HandleFunc("POST /admin/users/{id}/username", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUserUsernamePOST))
	mux.HandleFunc("POST /admin/users/{id}/deactivate", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUserDeactivatePOST))
	mux.HandleFunc("POST /admin/users/{id}/activate", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUserActivatePOST))
	mux.HandleFunc("POST /admin/users/{id}/remove", s.requirePermission(auth.PermUsersManage, s.ui.HandleAdminUserRemovePOST))
	mux.HandleFunc("GET /v1/me", s.requirePermission(auth.PermArchivesRead, s.api.HandleMe))
	mux.HandleFunc("PATCH /v1/me", s.requireAuth(s.api.HandlePatchMe))
	mux.HandleFunc("GET /v1/me/api-keys", s.requireAuth(s.api.HandleListMyAPIKeys))
	mux.HandleFunc("DELETE /v1/me/api-keys/{id}", s.requireAuth(s.api.HandleDeleteAPIKey))
	mux.HandleFunc("POST /v1/api-keys", s.requirePermission(auth.PermAPIKeysManage, s.api.HandleCreateAPIKey))
	mux.HandleFunc("GET /v1/users", s.requirePermission(auth.PermUsersManage, s.api.HandleListUsers))
	mux.HandleFunc("POST /v1/users", s.requirePermission(auth.PermUsersManage, s.api.HandleCreateUser))
	mux.HandleFunc("GET /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.api.HandleGetUser))
	mux.HandleFunc("PATCH /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.api.HandlePatchUser))
	mux.HandleFunc("DELETE /v1/users/{id}", s.requirePermission(auth.PermUsersManage, s.api.HandleDeleteUser))
	mux.HandleFunc("GET /v1/archives", s.requirePermission(auth.PermArchivesRead, s.api.HandleListArchives))
	mux.HandleFunc("POST /v1/archives", s.requirePermission(auth.PermArchivesWrite, s.api.HandleUpload))
	mux.HandleFunc("GET /v1/archives/{id...}", s.requirePermission(auth.PermArchivesRead, s.api.HandleDownload))
	mux.HandleFunc("DELETE /v1/archives/{id...}", s.requirePermission(auth.PermArchivesDelete, s.api.HandleDelete))
	mux.HandleFunc("POST /v1/archives/delete/{id...}", s.requirePermission(auth.PermArchivesDelete, s.api.HandleDelete))
	mux.HandleFunc("GET /v1/audit", s.requirePermission(auth.PermAuditRead, s.api.HandleListAudit))
	mux.HandleFunc("GET /v1/activity/export", s.requireAuth(s.api.HandleActivityExport))
	mux.HandleFunc("GET /v1/shares/{id...}", s.requirePermission(auth.PermSharesManage, s.api.HandleListShares))
	mux.HandleFunc("POST /v1/shares/{id...}", s.requirePermission(auth.PermSharesManage, s.api.HandleCreateShare))
	mux.HandleFunc("DELETE /v1/shares/{share_id}", s.requirePermission(auth.PermSharesManage, s.api.HandleRevokeShare))
	mux.HandleFunc("GET /s/{token}", s.api.HandleShareDownload)
	mux.HandleFunc("GET /shares/{id...}", s.requirePermission(auth.PermSharesManage, s.ui.HandleSharesPage))
	mux.HandleFunc("POST /shares/{id...}", s.requirePermission(auth.PermSharesManage, s.ui.HandleSharesCreate))
	mux.HandleFunc("POST /shares/{share_id}/revoke", s.requirePermission(auth.PermSharesManage, s.ui.HandleSharesRevoke))
	s.pinRoutes(mux)
	mountFaviconRoutes(mux)
	mux.HandleFunc("/", s.handleNotFound)
	return mw.SecurityHeaders(s.Cfg.CSP, mw.AccessLog(mux))
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

func remoteIP(remoteAddr string) string { return core.RemoteIP(remoteAddr) }

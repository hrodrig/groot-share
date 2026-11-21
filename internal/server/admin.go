package server

import (
	"errors"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

// requestBaseURL returns the base URL used to build share links.
// If Config.BaseURL is set (operator-pinned via GFS_BASE_URL), it
// wins and request headers are ignored — this is the fail-closed
// mode. Otherwise the URL is derived from the request, which
// requires running behind a trusted proxy that overwrites Host and
// X-Forwarded-Proto; see SECURITY.md.
func (s *Server) requestBaseURL(r *http.Request) string {
	if s.Cfg.BaseURL != "" {
		return s.Cfg.BaseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	return scheme + "://" + r.Host
}

func (s *Server) handleAdminUsersGET(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	users, err := s.Store.ListUsers(r.Context())
	if err != nil {
		slog.Error("admin list users", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["BaseURL"] = s.requestBaseURL(r)
	data["Nav"] = "admin"
	data["Users"] = users
	data["NoticeKind"], data["NoticeText"] = adminNotice(r.URL.Query().Get("notice"))
	_ = tmpl.AdminUsers.Execute(w, data)
}

func (s *Server) handleAdminUsersCreatePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	if strings.TrimSpace(r.Form.Get("username")) == "" {
		http.Redirect(w, r, "/admin/users?notice=username", http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		http.Redirect(w, r, "/admin/users?notice=name", http.StatusSeeOther)
		return
	}
	role := auth.RoleUploader
	if v := strings.TrimSpace(r.Form.Get("role")); v != "" {
		role = auth.Role(v)
	}
	if !auth.ValidRole(role) {
		http.Redirect(w, r, "/admin/users?notice=role", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(r.Form.Get("password"))
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			http.Redirect(w, r, "/admin/users?notice=pw_short", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	if _, err := s.Store.CreateUser(r.Context(), r.Form.Get("username"), name, hash, role); err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			http.Redirect(w, r, "/admin/users?notice=name", http.StatusSeeOther)
			return
		}
		if isUniqueViolation(err) {
			http.Redirect(w, r, "/admin/users?notice=taken", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=created", http.StatusSeeOther)
}

func (s *Server) handleAdminUserRolePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	newRole := auth.Role(strings.TrimSpace(r.Form.Get("role")))
	if !auth.ValidRole(newRole) {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if err := s.Store.GuardLastAdmin(r.Context(), id, newRole, u.Active); err != nil {
		http.Redirect(w, r, "/admin/users?notice=last_admin", http.StatusSeeOther)
		return
	}
	if err := s.Store.UpdateUser(r.Context(), id, newRole, u.Active); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=updated", http.StatusSeeOther)
}

func (s *Server) handleAdminUserUsernamePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	if err := s.Store.SetUsername(r.Context(), id, r.Form.Get("username")); err != nil {
		if errors.Is(err, store.ErrUsernameRequired) || errors.Is(err, store.ErrUsernameTooLong) {
			http.Redirect(w, r, "/admin/users?notice=username", http.StatusSeeOther)
			return
		}
		if isUniqueViolation(err) {
			http.Redirect(w, r, "/admin/users?notice=taken", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=updated", http.StatusSeeOther)
}

func (s *Server) handleAdminUserDeactivatePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if err := s.Store.GuardLastAdmin(r.Context(), id, u.Role, false); err != nil {
		http.Redirect(w, r, "/admin/users?notice=last_admin", http.StatusSeeOther)
		return
	}
	if err := s.Store.UpdateUser(r.Context(), id, u.Role, false); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=deactivated", http.StatusSeeOther)
}

func (s *Server) handleAdminUserActivatePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if err := s.Store.UpdateUser(r.Context(), id, u.Role, true); err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=activated", http.StatusSeeOther)
}

func (s *Server) handleAdminUserRemovePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if id == ac.User.ID {
		http.Redirect(w, r, "/admin/users?notice=self", http.StatusSeeOther)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	if u.Active {
		http.Redirect(w, r, "/admin/users?notice=active", http.StatusSeeOther)
		return
	}
	if err := s.Store.RemoveUser(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrLastAdmin) {
			http.Redirect(w, r, "/admin/users?notice=last_admin", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?notice=removed", http.StatusSeeOther)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "constraint failed")
}

func adminNotice(token string) (kind, text string) {
	n, ok := adminNoticeCopy[token]
	if !ok {
		return "", ""
	}
	return n[0], n[1]
}

var adminNoticeCopy = map[string][2]string{
	"created":     {"ok", "User created."},
	"updated":     {"ok", "User updated."},
	"deactivated": {"ok", "User deactivated."},
	"activated":   {"ok", "User activated."},
	"removed":     {"ok", "User removed."},
	"self":        {"err", "You cannot remove your own account."},
	"active":      {"err", "Deactivate the user before removing them."},
	"last_admin":  {"err", "Cannot change the last active admin."},
	"pw_short":    {"err", "Password must be at least 8 characters."},
	"username":    {"err", "Username is required."},
	"name":        {"err", "Name is required."},
	"taken":       {"err", "That username is already in use."},
	"role":        {"err", "Choose a valid role: viewer, uploader, or admin."},
	"error":       {"err", "That action failed. Check the form and try again."},
}

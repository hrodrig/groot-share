package server

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
)

func requestBaseURL(r *http.Request) string {
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
	data["BaseURL"] = requestBaseURL(r)
	data["Nav"] = "admin"
	data["Users"] = users
	data["NoticeKind"], data["NoticeText"] = adminNotice(r.URL.Query().Get("notice"))
	_ = adminUsersTmpl.Execute(w, data)
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
	role := auth.RoleUploader
	if v := strings.TrimSpace(r.Form.Get("role")); v != "" {
		role = auth.Role(v)
	}
	if !auth.ValidRole(role) {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(r.Form.Get("password"))
	if err != nil {
		http.Redirect(w, r, "/admin/users?notice=error", http.StatusSeeOther)
		return
	}
	if _, err := s.Store.CreateUser(r.Context(), r.Form.Get("username"), hash, role); err != nil {
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
		http.NotFound(w, r)
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
		http.NotFound(w, r)
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

func (s *Server) handleAdminUserDeactivatePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
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

func adminNotice(token string) (kind, text string) {
	switch token {
	case "created":
		return "ok", "User created."
	case "updated":
		return "ok", "User updated."
	case "deactivated":
		return "ok", "User deactivated."
	case "last_admin":
		return "err", "Cannot change the last active admin."
	case "error":
		return "err", "That action failed. Check the form and try again."
	default:
		return "", ""
	}
}

var adminUsersTmpl = template.Must(template.New("admin").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Users</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        {{if .BrandSub}}<span class="brand-sub">{{.BrandSub}}</span>{{end}}
      </a>
` + appNavTmpl + `
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
      <span class="who">{{.Username}} <span class="role">{{.Role}}</span></span>
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<div class="page-head">
  <div><h1>Users</h1><p class="sub">Manage accounts and roles.</p></div>
</div>
<section class="card" aria-labelledby="create-h">
  <div class="card-head"><h2 id="create-h">Create user</h2></div>
  <form method="post" action="/admin/users/create" class="stack-form">
    <label class="field"><span>Username</span><input name="username" required autocomplete="off"></label>
    <label class="field"><span>Password</span><input name="password" type="password" required autocomplete="new-password"></label>
    <label class="field"><span>Role</span>
      <select name="role">
        <option value="viewer">viewer</option>
        <option value="uploader" selected>uploader</option>
        <option value="admin">admin</option>
      </select>
    </label>
    <button class="btn" type="submit">Create user</button>
  </form>
</section>
<section class="card" aria-labelledby="users-h">
  <div class="card-head"><h2 id="users-h">Accounts</h2></div>
  {{if .Users}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Username</th>
      <th scope="col">Role</th>
      <th scope="col">Active</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Users}}
    <tr>
      <td>{{.Username}}</td>
      <td>{{.Role}}</td>
      <td>{{if .Active}}yes{{else}}no{{end}}</td>
      <td class="muted tabular">{{if .CreatedAt.IsZero}}—{{else}}{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="actions">
        {{if .Active}}
        <form method="post" action="/admin/users/{{.ID}}/role" class="inline-form">
          <select name="role" aria-label="Role for {{.Username}}">
            <option value="viewer"{{if eq .Role "viewer"}} selected{{end}}>viewer</option>
            <option value="uploader"{{if eq .Role "uploader"}} selected{{end}}>uploader</option>
            <option value="admin"{{if eq .Role "admin"}} selected{{end}}>admin</option>
          </select>
          <button class="btn btn-quiet btn-sm" type="submit">Set role</button>
        </form>
        <form method="post" action="/admin/users/{{.ID}}/deactivate">
          <button class="btn btn-danger-quiet btn-sm" type="submit">Deactivate</button>
        </form>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else}}
  <div class="empty"><p class="empty-title">No users</p></div>
  {{end}}
</section>
</main>
{{.AppFoot}}
{{.ThemeToggleScript}}
</body></html>
`))

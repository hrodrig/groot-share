package server

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
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

func (s *Server) handleAdminUserUsernamePOST(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleAdminUserActivatePOST(w http.ResponseWriter, r *http.Request) {
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
		http.NotFound(w, r)
		return
	}
	if id == ac.User.ID {
		http.Redirect(w, r, "/admin/users?notice=self", http.StatusSeeOther)
		return
	}
	u, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
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
` + appWhoTmpl + `
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
{{if .NoticeText}}<div class="notice notice-{{.NoticeKind}}" role="status">{{.NoticeText}}</div>{{end}}
<div class="page-head">
  <div><h1>Users</h1><p class="sub">Manage accounts, logins, and roles.</p></div>
</div>
<section class="card" aria-labelledby="create-h">
  <div class="card-head"><h2 id="create-h">Create user</h2></div>
  <div class="card-body">
  <form method="post" action="/admin/users/create" class="stack-form">
    <label class="field"><span>Name</span><input name="name" required autocomplete="name" maxlength="80"><small class="field-hint">Required. Shown in the header. Long names shorten (Juan ...egro).</small></label>
    <label class="field"><span>Username</span><input name="username" required autocomplete="off" maxlength="64"><small class="field-hint">Required. Login id. Must be unique.</small></label>
    <label class="field"><span>Password</span><input name="password" type="password" required autocomplete="new-password" minlength="8"><small class="field-hint">At least 8 characters.</small></label>
    <label class="field"><span>Role</span>
      <select name="role">
        <option value="viewer">viewer</option>
        <option value="uploader" selected>uploader</option>
        <option value="admin">admin</option>
      </select>
      <small class="field-hint">viewer reads; uploader also writes; admin manages users.</small>
    </label>
    <button class="btn" type="submit">Create user</button>
  </form>
  </div>
</section>
<section class="card" aria-labelledby="users-h">
  <div class="card-head"><h2 id="users-h">Accounts</h2></div>
  {{if .Users}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Name</th>
      <th scope="col">Username</th>
      <th scope="col">Role</th>
      <th scope="col">Active</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Users}}
    <tr>
      <td title="{{.Name}}">{{.DisplayName}}</td>
      <td>
        <form method="post" action="/admin/users/{{.ID}}/username" class="inline-form">
          <input name="username" value="{{.Username}}" required maxlength="64" autocomplete="off" aria-label="Login for {{.Name}}">
          <button class="btn btn-quiet btn-sm" type="submit">Set login</button>
        </form>
      </td>
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
        {{else}}
        <form method="post" action="/admin/users/{{.ID}}/activate">
          <button class="btn btn-quiet btn-sm" type="submit">Activate</button>
        </form>
        {{if ne .ID $.ActorID}}
        <form method="post" action="/admin/users/{{.ID}}/remove" data-confirm="This deletes {{.Username}} and their sessions and API keys. It cannot be undone.">
          <button class="btn btn-danger-quiet btn-sm" type="submit">Remove</button>
        </form>
        {{end}}
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
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Remove user</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok">Remove</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        dlg.showModal();
      });
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

package server

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (s *Server) handleSettingsGET(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	keys, err := s.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if err != nil {
		slog.Error("settings list keys", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["Nav"] = "settings"
	data["APIKeys"] = keys
	data["CanCreateReadKey"] = ac.User.Role == auth.RoleAdmin
	data["NoticeKind"], data["NoticeText"] = settingsNotice(r.URL.Query().Get("notice"))
	_ = settingsTmpl.Execute(w, data)
}

func (s *Server) handleSettingsPasswordPOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	hash, err := auth.HashPassword(r.Form.Get("password"))
	if err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			http.Redirect(w, r, "/settings?notice=pw_short", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err := s.Store.SetPassword(r.Context(), ac.User.ID, hash); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	s.setSessionCookie(w, "", -1)
	http.Redirect(w, r, "/login?notice=password", http.StatusSeeOther)
}

func (s *Server) handleSettingsNamePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err := s.Store.SetName(r.Context(), ac.User.ID, r.Form.Get("name")); err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			http.Redirect(w, r, "/settings?notice=name", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings?notice=named", http.StatusSeeOther)
}

func (s *Server) handleSettingsAPIKeyPOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermAPIKeysManage) || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	raw, prefix, scope, err := s.createAPIKeyForUser(w, r, ac.User.ID, ac.User.Role)
	if errors.Is(err, errBadScope) || errors.Is(err, errForbiddenScope) {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	if err != nil {
		slog.Error("settings create key", "error", err)
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	keys, listErr := s.Store.ListAPIKeysByUser(r.Context(), ac.User.ID)
	if listErr != nil {
		slog.Error("settings list keys after create", "error", listErr)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := s.pageShell()
	mergeActorData(data, ac)
	data["Nav"] = "settings"
	data["APIKeys"] = keys
	data["CanCreateReadKey"] = ac.User.Role == auth.RoleAdmin
	data["NewAPIKey"] = raw
	data["NewAPIKeyPrefix"] = prefix
	data["NewAPIKeyScope"] = scope
	data["NoticeKind"] = "ok"
	data["NoticeText"] = "API key created. Copy it now — it will not be shown again."
	_ = settingsTmpl.Execute(w, data)
}

func (s *Server) handleSettingsAPIKeyRevokePOST(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || ac.Method != auth.AuthSession {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseUserID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Store.DeleteAPIKey(r.Context(), id, ac.User.ID); err != nil {
		http.Redirect(w, r, "/settings?notice=error", http.StatusSeeOther)
		return
	}
	s.recordUserAudit(r, "apikey.revoke", strconv.FormatInt(id, 10), ac.User.Username)
	http.Redirect(w, r, "/settings?notice=revoked", http.StatusSeeOther)
}

func settingsNotice(token string) (kind, text string) {
	switch token {
	case "password":
		return "ok", "Password updated."
	case "named":
		return "ok", "Name updated."
	case "name":
		return "err", "Name is required (max 80 characters)."
	case "revoked":
		return "ok", "API key deleted."
	case "pw_short":
		return "err", "Password must be at least 8 characters."
	case "error":
		return "err", "That action failed. Check the form and try again."
	default:
		return "", ""
	}
}

var settingsTmpl = template.Must(template.New("settings").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Settings</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
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
  <div><h1>Settings</h1><p class="sub">Name, password, and API keys.</p></div>
</div>
<section class="card" aria-labelledby="name-h">
  <div class="card-head"><h2 id="name-h">Name</h2></div>
  <div class="card-body">
  <form method="post" action="/settings/name" class="stack-form">
    <label class="field"><span>Username</span><input value="{{.Username}}" readonly autocomplete="username"><small class="field-hint">Login id. Only an admin can change it.</small></label>
    <label class="field"><span>Name</span><input name="name" required autocomplete="name" maxlength="80" value="{{.Name}}"><small class="field-hint">Shown in the header. Long names shorten (Juan ...egro).</small></label>
    <button class="btn" type="submit">Update name</button>
  </form>
  </div>
</section>
<section class="card" aria-labelledby="pw-h">
  <div class="card-head"><h2 id="pw-h">Password</h2></div>
  <div class="card-body">
  <form method="post" action="/settings/password" class="stack-form">
    <label class="field"><span>New password</span><input name="password" type="password" required autocomplete="new-password" minlength="8"><small class="field-hint">At least 8 characters.</small></label>
    <button class="btn" type="submit">Update password</button>
  </form>
  </div>
</section>
{{if .CanManageKeys}}
<section class="card" aria-labelledby="keys-h">
  <div class="card-head"><h2 id="keys-h">API keys</h2></div>
  <div class="card-body card-stack">
  <div class="notice notice-warn" role="note">The secret is shown once. Copy it now — it cannot be recovered.</div>
  {{if .NewAPIKey}}
  <div class="key-reveal">
    <p class="field-hint">Prefix <span class="mono">{{.NewAPIKeyPrefix}}</span> · scope {{.NewAPIKeyScope}}</p>
    <div class="key-once" id="new-key">{{.NewAPIKey}}</div>
    <button class="btn" type="button" id="copy-new-key">Copy key</button>
  </div>
  {{end}}
  {{if .APIKeys}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">Prefix</th>
      <th scope="col">Scope</th>
      <th scope="col">Created (UTC)</th>
      <th scope="col">Last used (UTC)</th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .APIKeys}}
    <tr>
      <td class="mono">{{.Prefix}}</td>
      <td>{{.Scope}}</td>
      <td class="muted tabular">{{if .CreatedAt.IsZero}}—{{else}}{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="muted tabular">{{if .LastUsedAt.IsZero}}never{{else}}{{.LastUsedAt.UTC.Format "2006-01-02 15:04"}}{{end}}</td>
      <td class="actions">
        <form method="post" action="/settings/api-keys/{{.ID}}/revoke" data-confirm="Delete API key {{.Prefix}}? Anything using it will fail immediately.">
          <button class="btn btn-danger btn-sm" type="submit">Delete</button>
        </form>
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{else if not .NewAPIKey}}
  <div class="empty"><p class="empty-title">No API keys</p></div>
  {{end}}
  <form method="post" action="/settings/api-keys" class="stack-form">
    <label class="field"><span>Scope</span>
      <select name="scope">
        <option value="upload">upload — POST /v1/archives only</option>
        {{if .CanCreateReadKey}}<option value="read">read — list, download, audit</option>{{end}}
      </select>
    </label>
    <button class="btn" type="submit">Create API key</button>
  </form>
  </div>
</section>
{{end}}
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Delete API key</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok">Delete</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var btn = document.getElementById('copy-new-key');
  var el = document.getElementById('new-key');
  if (btn && el && navigator.clipboard) {
    btn.addEventListener('click', function () {
      navigator.clipboard.writeText(el.textContent.trim()).then(function () {
        btn.textContent = 'Copied';
        setTimeout(function () { btn.textContent = 'Copy key'; }, 2000);
      });
    });
  }
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

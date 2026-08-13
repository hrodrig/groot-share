package server

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

const (
	sessionCookie = "gfs_session"
	sessionTTL    = 24 * time.Hour
)

type ctxKey int

const actorKey ctxKey = 1

func (s *Server) handleLoginGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := pageShellData(s.Version)
	data["Error"] = ""
	_ = loginTmpl.Execute(w, data)
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	username, password, asJSON, err := parseLogin(r)
	if err != nil {
		s.loginFail(w, r, asJSON, http.StatusBadRequest, "bad_request")
		return
	}
	if s.Store == nil {
		s.loginFail(w, r, asJSON, http.StatusServiceUnavailable, "not_ready")
		return
	}
	u, err := s.Store.UserByUsername(r.Context(), username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("login lookup failed", "error", err)
		s.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, password) || !u.Active {
		s.loginFail(w, r, asJSON, http.StatusUnauthorized, "unauthorized")
		return
	}
	raw, hash, err := auth.NewSessionToken()
	if err != nil {
		slog.Error("session token failed", "error", err)
		s.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.Store.CreateSession(r.Context(), u.ID, hash, time.Now().Add(sessionTTL)); err != nil {
		slog.Error("session store failed", "error", err)
		s.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	s.setSessionCookie(w, raw, int(sessionTTL.Seconds()))
	if asJSON {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"username": u.Username, "role": u.Role})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" && s.Store != nil {
		_ = s.Store.DeleteSession(r.Context(), auth.HashSecret(c.Value))
	}
	s.setSessionCookie(w, "", -1)
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`+"\n")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"username": ac.User.Username, "role": ac.User.Role})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	scope := auth.KeyScopeUpload
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Scope string `json:"scope"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&body); err == nil && body.Scope != "" {
			scope = auth.KeyScope(body.Scope)
		}
	}
	if scope != auth.KeyScopeUpload && scope != auth.KeyScopeRead {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if ac.Method == auth.AuthAPIKey {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if ac.User.Role == auth.RoleViewer {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if ac.User.Role == auth.RoleUploader && scope == auth.KeyScopeRead {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	raw, hash, prefix, err := auth.NewAPIKey()
	if err != nil {
		slog.Error("api key generate failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.Store.CreateAPIKey(r.Context(), ac.User.ID, hash, prefix, scope); err != nil {
		slog.Error("api key store failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"api_key": raw, "prefix": prefix, "scope": scope})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil || !ac.Can(auth.PermUsersManage) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Admin    bool   `json:"admin"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	role := auth.RoleUploader
	if body.Role != "" {
		role = auth.Role(body.Role)
	} else if body.Admin {
		role = auth.RoleAdmin
	}
	if !auth.ValidRole(role) {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	created, err := s.Store.CreateUser(r.Context(), body.Username, hash, role)
	if err != nil {
		writeJSONError(w, http.StatusConflict, "conflict")
		return
	}
	s.recordUserAudit(r, "user.create", strconv.FormatInt(created.ID, 10), created.Username)
	rec, _ := s.userRecord(r.Context(), created)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(userJSON(rec))
}

func (s *Server) actorFromRequest(r *http.Request) *Actor {
	if s.Store == nil {
		return nil
	}
	if key := auth.ExtractKey(r); key != "" {
		ka, err := s.Store.AuthByAPIKeyHash(r.Context(), auth.HashSecret(key))
		if err != nil {
			return nil
		}
		return &Actor{User: ka.User, Method: auth.AuthAPIKey, KeyScope: ka.Scope}
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	u, err := s.Store.UserBySessionHash(r.Context(), auth.HashSecret(c.Value))
	if err != nil {
		return nil
	}
	return &Actor{User: u, Method: auth.AuthSession}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.Cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func parseLogin(r *http.Request) (username, password string, asJSON bool, err error) {
	asJSON = wantsJSON(r) || strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if asJSON && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err = dec.Decode(&body); err != nil {
			return "", "", true, err
		}
		return strings.TrimSpace(body.Username), body.Password, true, nil
	}
	if err = r.ParseForm(); err != nil {
		return "", "", asJSON, err
	}
	return strings.TrimSpace(r.Form.Get("username")), r.Form.Get("password"), asJSON, nil
}

func (s *Server) loginFail(w http.ResponseWriter, r *http.Request, asJSON bool, code int, msg string) {
	if asJSON {
		writeJSONError(w, code, msg)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	data := pageShellData(s.Version)
	data["Error"] = loginErrorCopy(msg)
	_ = loginTmpl.Execute(w, data)
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func wantsJSON(r *http.Request) bool {
	a := r.Header.Get("Accept")
	return strings.Contains(a, "application/json")
}

// isBrowserForm reports whether the request is the browser's multipart upload
// form, which expects redirect-and-notice UX instead of JSON error bodies.
func isBrowserForm(r *http.Request) bool {
	return !wantsJSON(r) && strings.Contains(r.Header.Get("Content-Type"), "multipart/")
}

var pageFuncs = template.FuncMap{
	"humansize": humanSize,
	"pagerurl":  pagerURL,
	"sortlink":  sortURL,
}

var loginTmpl = template.Must(template.New("login").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Sign in</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body class="gate">
<a class="skip" href="#main">Skip to content</a>
<div class="gate-tools">{{.ThemeToggle}}</div>
<main id="main" class="gate-wrap">
<div class="gate-brand"><span class="crate crate-lg" aria-hidden="true"></span><span class="wordmark-lg">gfs</span></div>
<p class="gate-sub">Sign in to the archive door</p>
<div class="card gate-card">
{{if .Error}}<div class="alert" role="alert">{{.Error}}</div>{{end}}
<form method="post" action="/login">
<label class="field"><span>Username</span><input name="username" autocomplete="username" required autofocus></label>
<label class="field field-pw"><span>Password</span><span class="input-group"><input name="password" id="login-password" type="password" autocomplete="current-password" required><button type="button" class="pw-toggle" id="pw-toggle" aria-label="Show password" aria-controls="login-password"><svg class="icon-show" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg><svg class="icon-hide is-hidden" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-10-8-10-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 10 8 10 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg></button></span></label>
<button class="btn btn-block" type="submit">Sign in</button>
</form>
</div>
<p class="gate-foot">gfs — groot files share</p>
</main>
{{.ThemeToggleScript}}
` + passwordToggleScript + `
</body></html>
`))

var homeTmpl = template.Must(template.New("home").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Captures</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        <span class="brand-sub">archive door</span>
      </a>
      <nav class="appnav" aria-label="Primary">
        <a href="/" {{if eq .Nav "captures"}}aria-current="page"{{end}}>Captures</a>
        {{if .CanUpload}}<a href="/upload" {{if eq .Nav "upload"}}aria-current="page"{{end}}>Upload</a>{{end}}
        <a href="/activity" {{if eq .Nav "activity"}}aria-current="page"{{end}}>Activity</a>
      </nav>
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
  <div>
    <h1>Captures</h1>
    <p class="sub">{{.StatsLine}}</p>
  </div>
</div>
<section class="card" aria-labelledby="ar-h">
  <div class="card-head"><h2 id="ar-h">Archives</h2></div>
  {{if .Items}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "key"}} is-active{{end}}"><a href="{{sortlink $.Pager "key"}}"{{if eq $.Pager.SortField "key"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Name<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "key"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "source"}} is-active{{end}}"><a href="{{sortlink $.Pager "source"}}"{{if eq $.Pager.SortField "source"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Source<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "source"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "storage"}} is-active{{end}}"><a href="{{sortlink $.Pager "storage"}}"{{if eq $.Pager.SortField "storage"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Storage<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "storage"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable num{{if eq $.Pager.SortField "size"}} is-active{{end}}"><a href="{{sortlink $.Pager "size"}}"{{if eq $.Pager.SortField "size"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Size<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "size"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col" class="sortable{{if eq $.Pager.SortField "uploaded"}} is-active{{end}}"><a href="{{sortlink $.Pager "uploaded"}}"{{if eq $.Pager.SortField "uploaded"}} aria-sort="{{if $.Pager.SortAsc}}ascending{{else}}descending{{end}}"{{end}}>Uploaded (UTC)<span class="sort-ind" aria-hidden="true">{{if eq $.Pager.SortField "uploaded"}}{{if $.Pager.SortAsc}}▲{{else}}▼{{end}}{{else}}↕{{end}}</span></a></th>
      <th scope="col"><span class="visually-hidden">Actions</span></th>
    </tr></thead>
    <tbody>
    {{range .Items}}
    <tr>
      <td class="key">{{.Key}}</td>
      <td><span class="pill pill-{{.Source}}">{{.Source}}</span></td>
      <td>{{if .Storage}}<span class="pill pill-{{.Storage}}">{{.Storage}}</span>{{end}}</td>
      <td class="num tabular">{{humansize .Size}}</td>
      <td class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
      <td class="actions">
        <a class="btn btn-quiet btn-sm btn-icon" href="/v1/archives/{{.ID}}/file" title="Download" aria-label="Download {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 3v10"/><path d="M8 11l4 4 4-4"/><path d="M4 20h16"/></svg></a>
        {{if $.CanDelete}}
        <form method="post" action="/v1/archives/{{.ID}}/delete" data-confirm="Delete {{.Key}}? This cannot be undone.">
          <button class="btn btn-danger-quiet btn-sm btn-icon" type="submit" title="Delete" aria-label="Delete {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/></svg></button>
        </form>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{if gt .Pager.Total 0}}
  <nav class="pager" aria-label="Archives pagination">
    {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.PrevPage .Pager}}">Previous</a>{{else}}<span></span>{{end}}
    <div class="pager-center">
      <p class="pager-meta">Page {{.Pager.Page}} of {{.Pager.TotalPages}} · {{.Pager.Total}} captures</p>
      <form class="pager-size" method="get">
        {{if .Pager.HiddenSort}}<input type="hidden" name="sort" value="{{.Pager.HiddenSort}}">{{end}}
        {{if .Pager.HiddenOrder}}<input type="hidden" name="order" value="{{.Pager.HiddenOrder}}">{{end}}
        <label for="arch-per-page">Per page</label>
        <select id="arch-per-page" name="per_page" onchange="this.form.submit()">
        {{range .PagerSizes}}<option value="{{.}}"{{if eq $.Pager.PageSize .}} selected{{end}}>{{.}}</option>{{end}}
        </select>
      </form>
    </div>
    {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.NextPage .Pager}}">Next</a>{{else}}<span></span>{{end}}
  </nav>
  {{end}}
  {{else}}
  <div class="empty">
    <p class="empty-title">No captures yet</p>
    <p class="empty-sub">Browse will stay empty until the first archive lands. <a href="/upload">Upload a capture</a> or use the HTTP API.</p>
  </div>
  {{end}}
</section>
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Delete capture</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok">Delete</button>
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

var uploadTmpl = template.Must(template.New("upload").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Upload</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        <span class="brand-sub">archive door</span>
      </a>
      <nav class="appnav" aria-label="Primary">
        <a href="/" {{if eq .Nav "captures"}}aria-current="page"{{end}}>Captures</a>
        {{if .CanUpload}}<a href="/upload" {{if eq .Nav "upload"}}aria-current="page"{{end}}>Upload</a>{{end}}
        <a href="/activity" {{if eq .Nav "activity"}}aria-current="page"{{end}}>Activity</a>
      </nav>
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
  <div>
    <h1>Upload capture</h1>
    <p class="sub">Send a groot .tar.gz from your browser. For automation, use <span class="mono">POST /v1/archives</span> with an API key.</p>
  </div>
  <a class="btn btn-quiet" href="/">Back to captures</a>
</div>
<section class="card" aria-labelledby="up-h">
  <div class="card-head">
    <h2 id="up-h">Choose file</h2>
    <p class="hint">groot .tar.gz, up to {{humansize .MaxUpload}} per file</p>
  </div>
  <form method="post" action="/v1/archives" enctype="multipart/form-data" class="upload">
    <label class="dropzone" id="dropzone">
      <input type="file" name="file" id="file" accept=".tar.gz,.tgz,application/gzip" required>
      <span class="dz-text" id="dz-text">Choose a file or drop it here</span>
    </label>
    <button class="btn" type="submit">Upload capture</button>
  </form>
</section>
</main>
{{.AppFoot}}
<script>
(function () {
  var dz = document.getElementById('dropzone');
  var input = document.getElementById('file');
  var label = document.getElementById('dz-text');
  if (dz && input) {
    var show = function () {
      if (input.files && input.files.length) {
        label.textContent = input.files[0].name;
        label.classList.add('has-file');
      }
    };
    input.addEventListener('change', show);
    ['dragenter', 'dragover'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add('drag'); });
    });
    ['dragleave', 'drop'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove('drag'); });
    });
    dz.addEventListener('drop', function (e) {
      if (e.dataTransfer && e.dataTransfer.files.length) {
        input.files = e.dataTransfer.files;
        show();
      }
    });
  }
})();
</script>
{{.ThemeToggleScript}}
</body></html>
`))

var activityTmpl = template.Must(template.New("activity").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs — Activity</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="appbar">
  <div class="appbar-in">
    <div class="appbar-start">
      <a class="brand" href="/">
        <span class="crate" aria-hidden="true"></span>
        <span class="wordmark">gfs</span>
        <span class="brand-sub">archive door</span>
      </a>
      <nav class="appnav" aria-label="Primary">
        <a href="/" {{if eq .Nav "captures"}}aria-current="page"{{end}}>Captures</a>
        {{if .CanUpload}}<a href="/upload" {{if eq .Nav "upload"}}aria-current="page"{{end}}>Upload</a>{{end}}
        <a href="/activity" {{if eq .Nav "activity"}}aria-current="page"{{end}}>Activity</a>
      </nav>
    </div>
    <div class="appbar-side">
      {{.ThemeToggle}}
      <span class="who">{{.Username}} <span class="role">{{.Role}}</span></span>
      <form method="post" action="/logout"><button class="btn btn-quiet btn-sm" type="submit">Sign out</button></form>
    </div>
  </div>
</header>
<main id="main" class="wrap">
<div class="page-head">
  <div>
    <h1>Activity</h1>
    <p class="sub">Audit log for uploads, downloads, and deletions. JSON at <span class="mono">GET /v1/audit</span>.</p>
  </div>
  <a class="btn btn-quiet" href="/">Back to captures</a>
</div>
<section class="card" aria-labelledby="ac-h">
  <div class="card-head">
    <h2 id="ac-h">Recent events</h2>
    <p class="hint">{{.Pager.Total}} total</p>
  </div>
  {{if .Audit}}
  <div class="table-wrap">
  <table class="grid">
    <thead><tr>
      <th scope="col">When (UTC)</th>
      <th scope="col">Who</th>
      <th scope="col">Action</th>
      <th scope="col">Object</th>
      <th scope="col">IP</th>
    </tr></thead>
    <tbody>
    {{range .Audit}}
    <tr>
      <td class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
      <td>{{.Actor}}</td>
      <td>{{.Action}}</td>
      <td class="muted mono" style="word-break: break-all;">{{.ObjectKey}}</td>
      <td class="muted tabular">{{.RemoteIP}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  {{if gt .Pager.Total 0}}
  <nav class="pager" aria-label="Activity pagination">
    {{if .Pager.HasPrev}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.PrevPage .Pager}}">Previous</a>{{else}}<span></span>{{end}}
    <div class="pager-center">
      <p class="pager-meta">Page {{.Pager.Page}} of {{.Pager.TotalPages}} · {{.Pager.Total}} events</p>
      <form class="pager-size" method="get">
        <label for="act-per-page">Per page</label>
        <select id="act-per-page" name="per_page" onchange="this.form.submit()">
        {{range .PagerSizes}}<option value="{{.}}"{{if eq $.Pager.PageSize .}} selected{{end}}>{{.}}</option>{{end}}
        </select>
      </form>
    </div>
    {{if .Pager.HasNext}}<a class="btn btn-quiet btn-sm" href="{{pagerurl .Pager.NextPage .Pager}}">Next</a>{{else}}<span></span>{{end}}
  </nav>
  {{end}}
  {{else}}
  <div class="empty">
    <p class="empty-title">No activity yet</p>
    <p class="empty-sub">Uploads, downloads, and deletions will appear here.</p>
  </div>
  {{end}}
</section>
</main>
{{.AppFoot}}
{{.ThemeToggleScript}}
</body></html>
`))

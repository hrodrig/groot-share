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
	data := s.pageShell()
	data["Error"] = ""
	data["Notice"] = ""
	if r.URL.Query().Get("notice") == "password" {
		data["Notice"] = "Password updated. Sign in again."
	}
	_ = loginTmpl.Execute(w, data)
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	username, password, asJSON, err := parseLogin(r)
	if err != nil {
		s.loginFail(w, r, asJSON, http.StatusBadRequest, "bad_request")
		return
	}
	if !s.allowLoginAttempt(r, username) {
		if secs := int(s.Cfg.LoginRateLimit.Window.Seconds()); secs > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(secs))
		}
		s.loginFail(w, r, asJSON, http.StatusTooManyRequests, "rate_limited")
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
		_ = json.NewEncoder(w).Encode(map[string]any{"username": u.Username, "name": u.Name, "role": u.Role})
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
	_ = json.NewEncoder(w).Encode(map[string]any{"username": ac.User.Username, "name": ac.User.Name, "role": ac.User.Role})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ac := actorFrom(r.Context())
	if ac == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if ac.Method == auth.AuthAPIKey {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	raw, prefix, scope, err := s.createAPIKeyForUser(w, r, ac.User.ID, ac.User.Role)
	if errors.Is(err, errBadScope) {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if errors.Is(err, errForbiddenScope) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err != nil {
		slog.Error("api key create failed", "error", err)
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
		Name     string `json:"name"`
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
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	created, err := s.Store.CreateUser(r.Context(), body.Username, name, hash, role)
	if err != nil {
		if errors.Is(err, store.ErrNameRequired) || errors.Is(err, store.ErrNameTooLong) {
			writeJSONError(w, http.StatusBadRequest, "bad_request")
			return
		}
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
		if err := s.Store.TouchAPIKeyLastUsed(r.Context(), ka.KeyID); err != nil {
			slog.Debug("touch api key last used", "error", err)
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
	data := s.pageShell()
	data["Error"] = loginErrorCopy(msg)
	_ = loginTmpl.Execute(w, data)
}

// allowLoginAttempt enforces per-IP and per-username caps on POST /login.
func (s *Server) allowLoginAttempt(r *http.Request, username string) bool {
	if s.LoginLimit == nil {
		return true
	}
	ip := remoteIP(r.RemoteAddr)
	if !s.LoginLimit.Allow("ip:" + ip) {
		return false
	}
	userKey := strings.ToLower(strings.TrimSpace(username))
	if userKey == "" {
		userKey = "empty"
	}
	return s.LoginLimit.Allow("user:" + userKey)
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
	"qswith":    func(b FilterURLBuilder, k, v string) template.URL { return b.With(k, v) },
	"qswithout": func(b FilterURLBuilder, k string) template.URL { return b.Without(k) },
}

var loginTmpl = template.Must(template.New("login").Funcs(pageFuncs).Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.LoginTitle}}</title>{{.FaviconHead}}{{.ThemeHead}}<style>{{.CSS}}</style></head>
<body class="{{.GateClass}}">
<a class="skip" href="#main">Skip to content</a>
<div class="gate-tools">{{.ThemeToggle}}</div>
<main id="main" class="gate-wrap">
<h1 class="visually-hidden">Sign in</h1>
<div class="card gate-card">
{{if .Notice}}<div class="notice notice-ok" role="status">{{.Notice}}</div>{{end}}
{{if .Error}}<div class="alert" role="alert">{{.Error}}</div>{{end}}
<form method="post" action="/login">
<label class="field"><span>Username</span><input name="username" autocomplete="username" required autofocus></label>
<label class="field field-pw"><span>Password</span><span class="input-group"><input name="password" id="login-password" type="password" autocomplete="current-password" required><button type="button" class="pw-toggle" id="pw-toggle" aria-label="Show password" aria-controls="login-password"><svg class="icon-show" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg><svg class="icon-hide is-hidden" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-10-8-10-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 10 8 10 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg></button></span></label>
<button class="btn btn-block" type="submit">Sign in</button>
</form>
</div>
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
<section class="card summary" aria-label="Inventory summary">
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.Count}}</span>
    <span class="summary-lbl">captures</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{humansize .Summary.Bytes}}</span>
    <span class="summary-lbl">on disk</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.ClusterCount}}</span>
    <span class="summary-lbl">clusters</span>
  </div>
  <div class="summary-cell">
    <span class="summary-num tabular">{{.Summary.IncompleteCount}}</span>
    <span class="summary-lbl">in transit</span>
  </div>
  <div class="summary-cell summary-topo">
    <span class="pill{{if eq .Summary.StorageTopology "vps"}} pill-local{{else if eq .Summary.StorageTopology "vps-s3"}} pill-s3{{end}}">{{.Summary.StorageTopology}}</span>
    <span class="summary-lbl">topology</span>
  </div>
</section>
{{if .CanUpload}}
<section class="card upload-cta" aria-labelledby="up-cta-h">
  <div class="upload-cta-head">
    <h2 id="up-cta-h">Upload archive</h2>
    <p class="hint">Drop or pick a groot <code>.tar.gz</code>. Up to <span class="mono">{{humansize .MaxUpload}}</span> per file.</p>
  </div>
  <form class="upload-inline" id="upload-inline" method="post" action="/v1/archives" enctype="multipart/form-data" novalidate data-max-upload="{{.MaxUpload}}">
    <label class="dropzone" id="inline-dropzone">
      <input type="file" name="file" id="inline-file" accept=".tar.gz,.tgz,application/gzip" required>
      <span class="dz-text" id="inline-dz-text">Choose a file or drop it here</span>
    </label>
    <div class="upload-meta" id="inline-meta" hidden></div>
    <progress class="upload-progress" id="inline-progress" max="1" value="0" aria-hidden="true" hidden></progress>
    <div class="upload-status" id="inline-status" role="status" hidden></div>
    <div class="upload-actions" id="inline-actions">
      <button class="btn" type="submit" id="inline-send" disabled>Upload capture</button>
      <button class="btn btn-quiet" type="button" id="inline-cancel" hidden>Cancel</button>
    </div>
  </form>
</section>
{{end}}
{{if .Pins}}
<section class="card pin-strip" aria-label="Pinned captures">
  <div class="card-head">
    <h2>Pinned</h2>
    <span class="hint">Your quick-access captures</span>
  </div>
  <ul class="pin-list">
    {{range .Pins}}
    <li class="pin">
      <a class="pin-key mono" href="/v1/archives/{{.ArchiveID}}/file" title="Download {{.ArchiveKey}}">{{.ArchiveKey}}</a>
      <span class="pin-size muted tabular">{{humansize .Size}}</span>
      <form method="post" action="/v1/pin/archives/{{.ArchiveID}}/delete" data-confirm="Unpin {{.ArchiveKey}}?">
        <button class="btn btn-quiet btn-sm" type="submit" title="Unpin" aria-label="Unpin {{.ArchiveKey}}">Unpin</button>
      </form>
    </li>
    {{end}}
  </ul>
</section>
{{end}}
{{if gt .Summary.Count 0}}
<form class="filter-bar" method="get" action="/" id="captures-filter">
  <div class="filter-row">
    <div class="filter-chips" role="group" aria-label="Clusters">
      <a class="chip{{if not .Filter.Cluster}} is-active{{end}}" href="/?{{qswithout .FilterURL "cluster"}}">All <span class="chip-count">{{.Summary.Count}}</span></a>
      {{range .ClusterChips}}
      <a class="chip{{if eq .Slug $.Filter.Cluster}} is-active{{end}}" href="/?{{qswith $.FilterURL "cluster" .Slug}}">{{.Slug}} <span class="chip-count">{{.Count}}</span></a>
      {{end}}
    </div>
    <label class="filter-search">
      <span class="visually-hidden">Search</span>
      <input type="search" name="q" value="{{.Filter.Query}}" placeholder="filename, since, message" maxlength="80">
    </label>
    <div class="filter-window" role="group" aria-label="Time window">
      <a class="chip chip-sm{{if eq .Filter.Window ""}} is-active{{end}}" href="/?{{qswithout .FilterURL "window"}}">All time</a>
      <a class="chip chip-sm{{if eq .Filter.Window "24h"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "24h"}}">24h</a>
      <a class="chip chip-sm{{if eq .Filter.Window "7d"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "7d"}}">7d</a>
      <a class="chip chip-sm{{if eq .Filter.Window "30d"}} is-active{{end}}" href="/?{{qswith .FilterURL "window" "30d"}}">30d</a>
    </div>
    <button class="btn btn-quiet btn-sm filter-apply" type="submit">Apply</button>
  </div>
</form>
{{end}}
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
        <button class="btn btn-quiet btn-sm btn-icon copy-link" type="button" data-copy-url="{{$.BaseURL}}/v1/archives/{{.ID}}/file" title="Copy download link" aria-label="Copy download link for {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M10 13a5 5 0 007.54.54l3-3a5 5 0 00-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 00-7.54-.54l-3 3a5 5 0 007.07 7.07l1.71-1.71"/></svg></button>
        {{if $.CanDelete}}
        <form method="post" action="/v1/archives/{{.ID}}/delete" data-confirm="Delete {{.Key}}? This cannot be undone." data-confirm-require="{{.Key}}">
          <button class="btn btn-danger-quiet btn-sm btn-icon" type="submit" title="Delete" aria-label="Delete {{.Key}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/></svg></button>
        </form>
        {{end}}
      </td>
    </tr>
    {{end}}
    </tbody>
  </table>
  </div>
  <ul class="archive-cards">
    {{range .Items}}
    <li class="archive-card">
      <div class="card-title mono" title="{{.Key}}">{{.Key}}</div>
      <div class="card-meta">
        <span class="pill pill-{{.Source}}">{{.Source}}</span>
        {{if .Storage}}<span class="pill pill-{{.Storage}}">{{.Storage}}</span>{{end}}
        <span class="muted tabular">{{humansize .Size}}</span>
        <span class="muted tabular">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</span>
      </div>
      <div class="card-actions">
        <a class="btn" href="/v1/archives/{{.ID}}/file" title="Download {{.Key}}">Download</a>
        <button class="btn btn-quiet copy-link" type="button" data-copy-url="{{$.BaseURL}}/v1/archives/{{.ID}}/file" title="Copy download link" aria-label="Copy download link for {{.Key}}">Copy link</button>
        {{if $.CanDelete}}
        <form method="post" action="/v1/archives/{{.ID}}/delete" data-confirm="Delete {{.Key}}? This cannot be undone." data-confirm-require="{{.Key}}">
          <button class="btn btn-danger-quiet" type="submit" title="Delete" aria-label="Delete {{.Key}}">Delete</button>
        </form>
        {{end}}
      </div>
    </li>
    {{end}}
  </ul>
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
    {{if .Filter.IsZero}}
    <p class="empty-title">No captures yet</p>
    <p class="empty-sub">Browse will stay empty until the first archive lands. <a href="/upload">Upload a capture</a> or use the HTTP API.</p>
    {{else}}
    <p class="empty-title">No matches</p>
    <p class="empty-sub">No captures match the current filters. <a href="/" class="empty-clear">Clear filters</a> to see all captures.</p>
    {{end}}
  </div>
  {{end}}
</section>
</main>
{{.AppFoot}}
<dialog id="confirm-dialog" aria-labelledby="confirm-title">
  <form method="dialog" class="dialog-card">
    <p class="dialog-title" id="confirm-title">Delete capture</p>
    <p class="dialog-text" id="confirm-text"></p>
    <div class="dialog-typed is-hidden" id="confirm-typed">
      <label class="field"><span id="confirm-typed-hint">Type the name to confirm</span><input id="confirm-input" autocomplete="off" spellcheck="false"></label>
    </div>
    <div class="dialog-actions">
      <button class="btn btn-quiet" value="cancel">Cancel</button>
      <button class="btn btn-danger" value="ok" id="confirm-ok">Delete</button>
    </div>
  </form>
</dialog>
<script>
(function () {
  var dlg = document.getElementById('confirm-dialog');
  var txt = document.getElementById('confirm-text');
  var typed = document.getElementById('confirm-typed');
  var input = document.getElementById('confirm-input');
  var hint = document.getElementById('confirm-typed-hint');
  var ok = document.getElementById('confirm-ok');
  var pending = null;
  if (dlg && dlg.showModal) {
    document.querySelectorAll('form[data-confirm]').forEach(function (f) {
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        var requireVal = f.getAttribute('data-confirm-require');
        pending = f;
        txt.textContent = f.getAttribute('data-confirm');
        if (requireVal !== null && requireVal !== undefined) {
          typed.classList.remove('is-hidden');
          hint.textContent = 'Type ' + requireVal + ' to confirm';
          input.value = '';
          input.dataset.require = requireVal;
          ok.disabled = true;
        } else {
          typed.classList.add('is-hidden');
          input.dataset.require = '';
          ok.disabled = false;
        }
        dlg.showModal();
      });
    });
    input.addEventListener('input', function () {
      ok.disabled = input.value !== (input.dataset.require || '');
    });
    dlg.addEventListener('close', function () {
      if (dlg.returnValue === 'ok' && pending) { pending.submit(); }
      pending = null;
    });
  }
  document.querySelectorAll('.copy-link').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var url = btn.getAttribute('data-copy-url');
      if (!url || !navigator.clipboard) return;
      navigator.clipboard.writeText(url).then(function () {
        btn.setAttribute('title', 'Copied');
        setTimeout(function () { btn.setAttribute('title', 'Copy download link'); }, 2000);
      });
    });
  });
  // Inline dropzone upload (XHR — fetch has no upload progress).
  var form = document.getElementById('upload-inline');
  var dz = document.getElementById('inline-dropzone');
  var fileInput = document.getElementById('inline-file');
  var dzText = document.getElementById('inline-dz-text');
  var meta = document.getElementById('inline-meta');
  var bar = document.getElementById('inline-progress');
  var status = document.getElementById('inline-status');
  var sendBtn = document.getElementById('inline-send');
  var cancelBtn = document.getElementById('inline-cancel');
  var xhr = null;
  if (form && dz && fileInput && dzText) {
    var maxUpload = parseInt(form.getAttribute('data-max-upload'), 10) || 0;
    function humanSize(n) {
      if (!n) return '0 B';
      var units = ['B', 'KB', 'MB', 'GB', 'TB'];
      var i = 0; while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
      return (i === 0 ? n : n.toFixed(1)) + ' ' + units[i];
    }
    function setStatus(kind, html) {
      status.className = 'upload-status' + (kind ? ' ' + kind : '');
      status.innerHTML = html;
      status.hidden = !html;
    }
    function resetUI() {
      bar.hidden = true; bar.value = 0;
      cancelBtn.hidden = true;
    }
    function refresh(selected) {
      // name + size before send
      var f = selected[0];
      if (!f) { dzText.textContent = 'Choose a file or drop it here'; dzText.classList.remove('has-file'); meta.hidden = true; sendBtn.disabled = true; return; }
      dzText.textContent = f.name;
      dzText.classList.add('has-file');
      meta.textContent = f.name + ' · ' + humanSize(f.size);
      meta.hidden = false;
      if (maxUpload > 0 && f.size > maxUpload) {
        meta.textContent += ' — exceeds the ' + humanSize(maxUpload) + ' limit';
        sendBtn.disabled = true;
        setStatus('err', 'File exceeds the ' + humanSize(maxUpload) + ' size limit.');
        return;
      }
      setStatus('', '');
      sendBtn.disabled = false;
    }
    fileInput.addEventListener('change', function () { if (fileInput.files) refresh(fileInput.files); });
    ['dragenter', 'dragover'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add('drag'); });
    });
    ['dragleave', 'drop'].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove('drag'); });
    });
    dz.addEventListener('drop', function (e) {
      if (e.dataTransfer && e.dataTransfer.files.length) {
        fileInput.files = e.dataTransfer.files;
        refresh(e.dataTransfer.files);
      }
    });
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var f = fileInput.files && fileInput.files.length ? fileInput.files[0] : null;
      if (!f) return;
      var fd = new FormData();
      fd.append('file', f);
      xhr = new XMLHttpRequest();
      xhr.open('POST', '/v1/archives');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.upload.onprogress = function (ev) {
        if (ev.lengthComputable) {
          bar.hidden = false;
          bar.value = ev.loaded / ev.total;
        }
      };
      xhr.onload = function () {
        var body = {};
        try { body = JSON.parse(xhr.responseText || '{}'); } catch (err) { body = {}; }
        resetUI();
        if (xhr.status === 201) {
          if (body.storage === 'transit') {
            setStatus('transit', 'Uploaded — in transit, will appear in Captures once the bucket copy completes.');
          } else {
            setStatus('ok', 'Capture uploaded.');
          }
          setTimeout(function () { window.location.assign(window.location.pathname + window.location.search); }, 800);
        } else if (xhr.status === 409) {
          var existing = body.existing && body.existing.key ? body.existing.key : f.name;
          setStatus('err', 'Already uploaded (same content): <span class="mono">' + existing + '</span>. <a href="/">View in Captures</a>.');
        } else if (xhr.status === 413) {
          setStatus('err', 'File exceeds the size limit.');
        } else {
          setStatus('err', 'Upload failed. Check the file and try again.');
        }
        sendBtn.disabled = false;
      };
      xhr.onerror = function () {
        resetUI();
        setStatus('err', 'Upload failed (network). Check the file and try again.');
        sendBtn.disabled = false;
      };
      xhr.onabort = function () {
        resetUI();
        setStatus('err', 'Upload canceled.');
        sendBtn.disabled = false;
        dzText.textContent = 'Choose a file or drop it here';
        dzText.classList.remove('has-file');
        meta.hidden = true;
      };
      setStatus('', '');
      sendBtn.disabled = true;
      cancelBtn.hidden = false;
      xhr.send(fd);
    });
    cancelBtn.addEventListener('click', function () {
      if (xhr) { xhr.abort(); xhr = null; }
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
<div class="page-head">
  <div>
    <h1>Activity</h1>
    <p class="sub">Audit log for uploads, downloads, and deletions. JSON at <span class="mono">GET /v1/audit</span>.</p>
  </div>
  <div class="page-actions">
    <a class="btn btn-quiet" href="/">Back to captures</a>
    {{if .CanExport}}
    <span class="export-group">
      <a class="btn btn-sm" href="/v1/activity/export?format=csv{{if .FilterActor}}&amp;actor={{.FilterActor}}{{end}}{{if .FilterAction}}&amp;action={{.FilterAction}}{{end}}{{if .FilterWindow}}&amp;window={{.FilterWindow}}{{end}}">Export CSV</a>
      <a class="btn btn-quiet btn-sm" href="/v1/activity/export?format=json{{if .FilterActor}}&amp;actor={{.FilterActor}}{{end}}{{if .FilterAction}}&amp;action={{.FilterAction}}{{end}}{{if .FilterWindow}}&amp;window={{.FilterWindow}}{{end}}">Export JSON</a>
    </span>
    {{end}}
  </div>
</div>
<section class="card" aria-labelledby="ac-h">
  <div class="card-head">
    <h2 id="ac-h">Recent events</h2>
    <p class="hint">{{.Pager.Total}} total</p>
  </div>
  <form class="filter-row" method="get" action="/activity">
    <label class="field-inline">
      <span class="visually-hidden">Action</span>
      <select name="action">
        <option value="">All actions</option>
        {{range .AuditActions}}<option value="{{.}}"{{if eq $.FilterAction .}} selected{{end}}>{{.}}</option>{{end}}
      </select>
    </label>
    <label class="field-inline">
      <span class="visually-hidden">Actor</span>
      <input type="search" name="actor" value="{{.FilterActor}}" placeholder="Actor" maxlength="80">
    </label>
    <label class="field-inline">
      <span class="visually-hidden">Window</span>
      <select name="window">
        <option value="">All time</option>
        <option value="24h"{{if eq .FilterWindow "24h"}} selected{{end}}>24h</option>
        <option value="7d"{{if eq .FilterWindow "7d"}} selected{{end}}>7d</option>
        <option value="30d"{{if eq .FilterWindow "30d"}} selected{{end}}>30d</option>
      </select>
    </label>
    <button class="btn btn-quiet btn-sm filter-apply" type="submit">Filter</button>
  </form>
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

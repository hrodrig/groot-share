package server

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
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
	_ = loginTmpl.Execute(w, map[string]any{"CSS": template.CSS(layoutCSS), "Error": ""})
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
	if !auth.CheckPassword(u.PasswordHash, password) {
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
		_ = json.NewEncoder(w).Encode(map[string]any{"username": u.Username, "admin": u.Admin})
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
	u := actorFrom(r.Context())
	if u == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"username": u.Username, "admin": u.Admin})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	u := actorFrom(r.Context())
	if u == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	raw, hash, prefix, err := auth.NewAPIKey()
	if err != nil {
		slog.Error("api key generate failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.Store.CreateAPIKey(r.Context(), u.ID, hash, prefix); err != nil {
		slog.Error("api key store failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"api_key": raw, "prefix": prefix})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	u := actorFrom(r.Context())
	if u == nil || !u.Admin {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request")
		return
	}
	created, err := s.Store.CreateUser(r.Context(), body.Username, hash, body.Admin)
	if err != nil {
		writeJSONError(w, http.StatusConflict, "conflict")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"username": created.Username, "admin": created.Admin})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.actorFromRequest(r)
		if u == nil {
			if wantsJSON(r) || strings.HasPrefix(r.URL.Path, "/v1/") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey, u)))
	}
}

func (s *Server) actorFromRequest(r *http.Request) *store.User {
	if s.Store == nil {
		return nil
	}
	if key := auth.ExtractKey(r); key != "" {
		u, err := s.Store.UserByAPIKeyHash(r.Context(), auth.HashSecret(key))
		if err != nil {
			return nil
		}
		return &u
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	u, err := s.Store.UserBySessionHash(r.Context(), auth.HashSecret(c.Value))
	if err != nil {
		return nil
	}
	return &u
}

func actorFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(actorKey).(*store.User)
	return u
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
	_ = loginTmpl.Execute(w, map[string]any{"CSS": template.CSS(layoutCSS), "Error": msg})
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

var loginTmpl = template.Must(template.New("login").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs login</title><style>{{.CSS}}</style></head>
<body><main>
<h1>gfs</h1>
<p class="muted">Sign in to list and download groot archives.</p>
{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
<form method="post" action="/login">
<label>Username <input name="username" autocomplete="username" required></label>
<label>Password <input name="password" type="password" autocomplete="current-password" required></label>
<button type="submit">Sign in</button>
</form>
</main></body></html>
`))

var homeTmpl = template.Must(template.New("home").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>gfs</title><style>{{.CSS}}</style></head>
<body><main>
<header class="bar">
  <div>
    <h1>gfs</h1>
    <p class="muted">Signed in as {{.Username}}{{if .Admin}} (admin){{end}}</p>
  </div>
  <form method="post" action="/logout"><button class="ghost" type="submit">Sign out</button></form>
</header>
<section>
<h2>Upload</h2>
<form method="post" action="/v1/archives" enctype="multipart/form-data">
<label>groot .tar.gz <input type="file" name="file" accept=".tar.gz,.tgz,application/gzip" required></label>
<button type="submit">Upload</button>
</form>
</section>
<section>
<h2>Archives</h2>
{{if .Items}}
<table>
<thead><tr><th>Name</th><th>Source</th><th>Size</th><th>When</th><th></th></tr></thead>
<tbody>
{{range .Items}}
<tr>
  <td>{{.Key}}</td>
  <td class="muted">{{.Source}}</td>
  <td>{{.Size}}</td>
  <td class="muted">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
  <td><a href="/v1/archives/{{.ID}}/file">Download</a></td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No archives yet.</p>
{{end}}
</section>
<section>
<h2>Audit</h2>
{{if .Audit}}
<table>
<thead><tr><th>When</th><th>Who</th><th>Action</th><th>Object</th></tr></thead>
<tbody>
{{range .Audit}}
<tr>
  <td class="muted">{{.CreatedAt.UTC.Format "2006-01-02 15:04"}}</td>
  <td>{{.Actor}}</td>
  <td>{{.Action}}</td>
  <td class="muted">{{.ObjectKey}}</td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No audit rows yet.</p>
{{end}}
</section>
</main></body></html>
`))

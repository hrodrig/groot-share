package server

import (
	"encoding/json"
	"errors"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
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
	_ = tmpl.Login.Execute(w, data)
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
	// Opportunistic purge so the sessions table doesn't grow monotonically
	// between sweep cycles. Best-effort: a failure here must not fail login.
	if n, err := s.Store.PurgeExpiredSessions(r.Context(), time.Now()); err != nil {
		slog.Warn("purge sessions", "error", err)
	} else if n > 0 {
		slog.Info("purged expired sessions", "count", n)
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
	if err := s.requireUserAudit(r, "user.create", strconv.FormatInt(created.ID, 10), created.Username); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "audit unavailable")
		return
	}
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
	// A JSON Content-Type marks this as an API login: both the body parse and
	// the response shape (JSON error/body vs HTML redirect) key off the same
	// single check, so they can't disagree. The browser leaves Content-Type as
	// application/x-www-form-urlencoded (ParseForm) despite sending the form.
	asJSON = strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if asJSON {
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
		return "", "", false, err
	}
	return strings.TrimSpace(r.Form.Get("username")), r.Form.Get("password"), false, nil
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
	_ = tmpl.Login.Execute(w, data)
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

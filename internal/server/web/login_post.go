package web

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/hrodrig/groot-share/internal/server/core"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func (srv *Web) HandleLoginPOST(w http.ResponseWriter, r *http.Request) {
	username, password, asJSON, err := parseLogin(r)
	if err != nil {
		srv.loginFail(w, r, asJSON, http.StatusBadRequest, "bad_request")
		return
	}
	if !srv.AllowLoginAttempt(r, username) {
		if secs := int(srv.Cfg.LoginRateLimit.Window.Seconds()); secs > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(secs))
		}
		srv.loginFail(w, r, asJSON, http.StatusTooManyRequests, "rate_limited")
		return
	}
	if srv.Store == nil {
		srv.loginFail(w, r, asJSON, http.StatusServiceUnavailable, "not_ready")
		return
	}
	u, err := srv.Store.UserByUsername(r.Context(), username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("login lookup failed", "error", err)
		srv.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, password) || !u.Active {
		srv.loginFail(w, r, asJSON, http.StatusUnauthorized, "unauthorized")
		return
	}
	raw, hash, err := auth.NewSessionToken()
	if err != nil {
		slog.Error("session token failed", "error", err)
		srv.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	if err := srv.Store.CreateSession(r.Context(), u.ID, hash, time.Now().Add(core.SessionTTL)); err != nil {
		slog.Error("session store failed", "error", err)
		srv.loginFail(w, r, asJSON, http.StatusInternalServerError, "internal")
		return
	}
	// Opportunistic purge so the sessions table doesn't grow monotonically
	// between sweep cycles. Best-effort: a failure here must not fail login.
	if n, err := srv.Store.PurgeExpiredSessions(r.Context(), time.Now()); err != nil {
		slog.Warn("purge sessions", "error", err)
	} else if n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}
	srv.SetSessionCookie(w, raw, int(core.SessionTTL.Seconds()))
	if asJSON {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"username": u.Username, "name": u.Name, "role": u.Role})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (srv *Web) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(core.SessionCookie); err == nil && c.Value != "" && srv.Store != nil {
		_ = srv.Store.DeleteSession(r.Context(), auth.HashSecret(c.Value))
	}
	srv.SetSessionCookie(w, "", -1)
	if core.WantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`+"\n")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

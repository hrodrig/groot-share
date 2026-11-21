package core

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
)

const (
	SessionCookie = "gfs_session"
	SessionTTL    = 24 * time.Hour
)

func (c *Core) ActorFromRequest(r *http.Request) *Actor {
	if c.Store == nil {
		return nil
	}
	if key := auth.ExtractKey(r); key != "" {
		ka, err := c.Store.AuthByAPIKeyHash(r.Context(), auth.HashSecret(key))
		if err != nil {
			return nil
		}
		if err := c.Store.TouchAPIKeyLastUsed(r.Context(), ka.KeyID); err != nil {
			slog.Debug("touch api key last used", "error", err)
		}
		return &Actor{User: ka.User, Method: auth.AuthAPIKey, KeyScope: ka.Scope}
	}
	co, err := r.Cookie(SessionCookie)
	if err != nil || co.Value == "" {
		return nil
	}
	u, err := c.Store.UserBySessionHash(r.Context(), auth.HashSecret(co.Value))
	if err != nil {
		return nil
	}
	return &Actor{User: u, Method: auth.AuthSession}
}

func (c *Core) SetSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.Cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c *Core) AllowLoginAttempt(r *http.Request, username string) bool {
	if c.LoginLimit == nil {
		return true
	}
	ip := RemoteIP(r.RemoteAddr)
	if !c.LoginLimit.Allow("ip:" + ip) {
		return false
	}
	userKey := strings.ToLower(strings.TrimSpace(username))
	if userKey == "" {
		userKey = "empty"
	}
	return c.LoginLimit.Allow("user:" + userKey)
}

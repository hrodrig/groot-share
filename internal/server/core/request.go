package core

import (
	"net/http"
	"strings"
)

// RequestBaseURL returns the base URL used to build share links.
func (c *Core) RequestBaseURL(r *http.Request) string {
	if c.Cfg.BaseURL != "" {
		return c.Cfg.BaseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	return scheme + "://" + r.Host
}

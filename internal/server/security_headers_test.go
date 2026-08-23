package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s.Handler().ServeHTTP(rr, req)

	h := rr.Header()
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
	} {
		if got := h.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if csp := h.Get("Content-Security-Policy"); csp == "" {
		t.Errorf("Content-Security-Policy missing")
	} else {
		for _, dir := range []string{"frame-ancestors 'none'", "object-src 'none'", "script-src 'self' 'unsafe-inline'"} {
			if !strings.Contains(csp, dir) {
				t.Errorf("CSP %q missing %q", csp, dir)
			}
		}
	}
	// Plain HTTP request → no HSTS.
	if got := h.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("Strict-Transport-Security = %q on plain HTTP, want empty", got)
	}
}

func TestSecurityHeadersHSTSOverTLS(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	// Simulate a trusted proxy forwarding https.
	req.Header.Set("X-Forwarded-Proto", "https")
	s.Handler().ServeHTTP(rr, req)
	if got := rr.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("Strict-Transport-Security = %q, want max-age with includeSubDomains", got)
	}
}

func TestSecurityHeadersCSPOverride(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.CSP = "default-src 'none'"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Content-Security-Policy"); got != "default-src 'none'" {
		t.Fatalf("CSP override = %q", got)
	}
}

func TestSecurityHeadersCSPDisabled(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.CSP = "-"
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("CSP should be disabled, got %q", got)
	}
}

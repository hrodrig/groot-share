package server

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

// TestRequestBaseURLPinnedIgnoresRequest verifies the fail-closed
// path: when GFS_BASE_URL is set, the helper returns it verbatim
// and ignores X-Forwarded-Proto/Host headers that an untrusted
// client could otherwise poison.
func TestRequestBaseURLPinnedIgnoresRequest(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.BaseURL = "https://share.example.com"

	cases := []struct {
		name  string
		host  string
		proto string
		tls   bool
	}{
		{"plain http headers", "attacker.example", "https", false},
		{"tls set", "attacker.example", "", true},
		{"empty host", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tc.host != "" {
				r.Host = tc.host
			}
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := s.requestBaseURL(r); got != "https://share.example.com" {
				t.Fatalf("got %q, want pinned", got)
			}
		})
	}
}

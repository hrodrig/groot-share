package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFaviconStatic(t *testing.T) {
	s := &Server{Version: "0.1.0-test"}
	paths := []string{
		"/static/favicon.ico",
		"/static/favicon.svg",
		"/static/favicon-16x16.png",
		"/static/favicon-32x32.png",
		"/static/apple-touch-icon.png",
		"/static/manifest.json",
	}
	for _, path := range paths {
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s code %d", path, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/favicon.svg", nil))
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "svg") {
		t.Fatalf("svg content-type %q", ct)
	}
	if !strings.Contains(rr.Body.String(), `<svg`) {
		t.Fatal("expected svg body")
	}
}

func TestLoginPageFaviconHead(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))
	body := rr.Body.String()
	if !strings.Contains(body, `/static/favicon.svg?v=`) || !strings.Contains(body, `theme-color`) {
		t.Fatalf("favicon head missing: %s", body[:min(500, len(body))])
	}
}

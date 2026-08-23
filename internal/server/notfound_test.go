package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestNotFoundBrowserRendersPage(t *testing.T) {
	s := newNotFoundServer(t)
	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type: %q want text/html", ct)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`<title>gfs — Not found</title>`,
		"Not found",
		"Back to Captures",
		`href="/"`,
		"/no-such-route",
		`id="theme-toggle"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("404 page missing %q in body:\n%s", want, body)
		}
	}
}

func TestNotFoundBrowserDefaultAcceptIsHTML(t *testing.T) {
	s := newNotFoundServer(t)
	req := httptest.NewRequest(http.MethodGet, "/no-such", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("default accept should be HTML; got %q", ct)
	}
}

func TestNotFoundAPIJSON(t *testing.T) {
	s := newNotFoundServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/no-such", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type: %q want JSON", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}
	if body["error"] != "not_found" {
		t.Fatalf("error: %v want not_found", body["error"])
	}
}

func TestNotFoundAPIPathNoMoreSegmentsJSON(t *testing.T) {
	// A path under /v1/ that matches no registered pattern (e.g. a
	// multi-segment URL the single-segment {id} patterns do not catch)
	// must fall through to the catch-all and return JSON, not HTML.
	s := newNotFoundServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/no-such/extra/segment", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type: %q want JSON", ct)
	}
}

func TestNotFoundShareTokenExtraSegmentJSON(t *testing.T) {
	// The /s/{token} pattern is single-segment, so an extra segment
	// falls through to the catch-all. JSON must be returned (no info
	// leak via the HTML 404 path for any /s/* catch).
	s := newNotFoundServer(t)
	req := httptest.NewRequest(http.MethodGet, "/s/abcdef-not-a-real-token/extra", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type: %q want JSON", ct)
	}
}

func TestNotFoundSpecificRouteStillMatches(t *testing.T) {
	// Sanity: registering "/" as a catch-all must not shadow the home
	// route. Specific patterns take priority in Go 1.22+ ServeMux.
	s := newNotFoundServer(t)
	login := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	if rr.Code != http.StatusOK {
		t.Fatalf("login page broken by catch-all: %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "id=\"login-password\"") {
		t.Fatal("login page rendered as 404")
	}
}

func newNotFoundServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureAdmin(context.Background(), "root", "correct-horse", ""); err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: config.Config{Topology: config.TopologyVPS}, Store: st, Ready: func() bool { return true }, Version: "0.1.0-test"}
}

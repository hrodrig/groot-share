package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/logging"
)

func TestHealthz(t *testing.T) {
	s := &Server{Cfg: config.Config{Topology: config.TopologyVPS}}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
	if rr.Body.String() != "ok\n" {
		t.Fatalf("body %q", rr.Body.String())
	}
}

func TestReadyzOK(t *testing.T) {
	s := &Server{Ready: func() bool { return true }}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
}

func TestReadyzNotReady(t *testing.T) {
	s := &Server{Ready: func() bool { return false }}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatal(rr.Code)
	}
}

func TestUnknownPath404AccessLog(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	s := &Server{Ready: func() bool { return true }}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatal(rr.Code)
	}
	out := buf.String()
	if !strings.Contains(out, `"msg":"http"`) && !strings.Contains(out, "http") {
		t.Fatalf("access log missing: %q", out)
	}
	if !strings.Contains(out, "/nope") {
		t.Fatalf("path missing: %q", out)
	}
}

func TestNotFoundEscapesUserPath(t *testing.T) {
	s := &Server{}
	rr := httptest.NewRecorder()
	p := "/<script>alert('xss')</script>"
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
	body := rr.Body.String()
	// The raw, unescaped <script> must never reach the HTML body.
	if strings.Contains(body, "<script>alert('xss')</script>") {
		t.Fatalf("unescaped user path rendered in HTML: %s", body)
	}
	// The escaped form must be present — angle brackets turned into entities
	// so the browser never parses it as markup.
	if !strings.Contains(body, "/&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;") {
		t.Fatalf("expected escaped path, got: %s", body)
	}
}

func TestHealthzSkipsAccessLog(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	s := &Server{}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
	if strings.Contains(buf.String(), `"msg":"http"`) {
		t.Fatalf("healthz should skip access log: %q", buf.String())
	}
}

func TestRemoteIP(t *testing.T) {
	if got := remoteIP("127.0.0.1:1234"); got != "127.0.0.1" {
		t.Fatal(got)
	}
	if got := remoteIP("no-port"); got != "no-port" {
		t.Fatal(got)
	}
}

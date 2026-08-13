package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("code %d", code)
	}
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("--version %d", code)
	}
	if code := run([]string{"-V"}); code != 0 {
		t.Fatalf("-V %d", code)
	}
}

func TestRunMissingTopology(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	if code := run(nil); code != 1 {
		t.Fatalf("want 1 got %d", code)
	}
}

func TestNewHTTPServerProbes(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	cfg := config.Config{
		ListenAddr: "127.0.0.1:0",
		Topology:   config.TopologyVPS,
		DataDir:    dir,
	}
	srv := newHTTPServer(cfg, st)

	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz %d", rr.Code)
	}
}

func TestNewHTTPServerReadyzVPSS3MissingCreds(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	cfg := config.Config{
		ListenAddr: "127.0.0.1:0",
		Topology:   config.TopologyVPSS3,
		DataDir:    dir,
		S3Bucket:   "lab",
	}
	srv := newHTTPServer(cfg, st)
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz %d", rr.Code)
	}
}

func TestListenAndServeShutdown(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux()}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = srv.Shutdown(context.Background())
	}()
	if code := listenAndServe(srv); code != 0 {
		t.Fatalf("code %d", code)
	}
}

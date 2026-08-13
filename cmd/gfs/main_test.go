package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
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

func TestRunInvalidTopology(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "nope")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	if code := run(nil); code != 1 {
		t.Fatalf("want 1 got %d", code)
	}
}

func TestRunMissingBootstrap(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", t.TempDir())
	t.Setenv("GFS_BOOTSTRAP_ADMIN", "")
	t.Setenv("GFS_BOOTSTRAP_PASSWORD", "")
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

func TestOpenBlobsVPSOnly(t *testing.T) {
	blobs, err := openBlobs(config.Config{Topology: config.TopologyVPS})
	if err != nil || blobs != nil {
		t.Fatalf("vps blobs %+v %v", blobs, err)
	}
}

func TestOpenBlobsVPSS3(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ak")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "sk")
	blobs, err := openBlobs(config.Config{
		Topology: config.TopologyVPSS3,
		S3Bucket: "lab",
		S3Region: "us-east-1",
	})
	if err != nil || blobs == nil {
		t.Fatalf("vps-s3 blobs %v %v", blobs, err)
	}
}

func TestNewAppReadyWithMemoryBlob(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	mem := blob.NewMemory()
	cfg := config.Config{
		Topology: config.TopologyVPSS3,
		DataDir:  dir,
		S3Bucket: "lab",
	}
	app := newApp(cfg, st, mem, "test")
	if !app.Ready() {
		t.Fatal("ready with memory blob")
	}
}

func TestListenAndServeBadAddr(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0\n", Handler: http.NewServeMux()}
	if code := listenAndServe(srv); code != 1 {
		t.Fatalf("code %d", code)
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

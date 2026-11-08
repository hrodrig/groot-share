package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
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

func TestRunMissingDataDir(t *testing.T) {
	t.Setenv("GFS_TOPOLOGY", "vps")
	t.Setenv("GFS_DATA_DIR", "")
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
	if code := listenAndServe(context.Background(), srv); code != 1 {
		t.Fatalf("code %d", code)
	}
}

// TestListenConfigDisablesMPTCP guards the CVE-2026-98432 mitigation: Go 1.24+
// enables MPTCP on Linux listeners by default and the public exploit targets
// hosts exposing an MPTCP listener.
func TestListenConfigDisablesMPTCP(t *testing.T) {
	if listenConfig().MultipathTCP() {
		t.Fatal("listen config must not use MPTCP")
	}
}

// TestListenerDoesNotUseMPTCP verifies the production listener never negotiates
// MPTCP. The check is differential: a control listener with MPTCP explicitly
// enabled has to report MPTCP on the connection, otherwise the host kernel has
// no MPTCP support and the assertion below would be vacuous.
func TestListenerDoesNotUseMPTCP(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("MPTCP listeners exist only on Linux")
	}
	ctx := context.Background()

	var controlLC net.ListenConfig
	controlLC.SetMultipathTCP(true)
	control, err := controlLC.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("MPTCP control listener unavailable: %v", err)
	}
	defer func() { _ = control.Close() }()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	dialer.SetMultipathTCP(true)

	controlConn, err := dialer.DialContext(ctx, "tcp", control.Addr().String())
	if err != nil {
		t.Skipf("MPTCP control dial failed: %v", err)
	}
	controlMPTCP := connUsesMPTCP(t, controlConn)
	_ = controlConn.Close()

	if !controlMPTCP {
		t.Skip("control connection did not negotiate MPTCP; differential check not applicable on this host")
	}

	l, err := listenConfig().Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	conn, err := dialer.DialContext(ctx, "tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	if connUsesMPTCP(t, conn) {
		t.Fatal("production listener negotiated MPTCP; want plain TCP")
	}
}

func connUsesMPTCP(t *testing.T, c net.Conn) bool {
	t.Helper()
	tc, ok := c.(*net.TCPConn)
	if !ok {
		t.Fatalf("want *net.TCPConn, got %T", c)
	}
	use, err := tc.MultipathTCP()
	if err != nil {
		t.Fatalf("MultipathTCP: %v", err)
	}
	return use
}

func TestListenAndServeShutdown(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = srv.Shutdown(context.Background())
	}()
	if code := listenAndServe(ctx, srv); code != 0 {
		t.Fatalf("code %d", code)
	}
}

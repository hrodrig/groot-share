// Package main is the gfs entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/logging"
	"github.com/hrodrig/groot-share/internal/ratelimit"
	"github.com/hrodrig/groot-share/internal/server"
	"github.com/hrodrig/groot-share/internal/store"
)

// Set via -ldflags at build time (Makefile / GoReleaser).
var (
	version   = "dev"
	commit    = "unknown"
	branch    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-V":
			fmt.Printf("gfs %s commit=%s branch=%s date=%s\n", version, commit, branch, buildDate)
			return 0
		}
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gfs: %v\n", err)
		return 1
	}
	logging.Setup(cfg.LogFormat, cfg.LogLevel)

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		slog.Error("store open failed", "error", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	if err := st.EnsureAdmin(context.Background(), cfg.BootstrapAdmin, cfg.BootstrapPassword, cfg.BootstrapAdminName); err != nil {
		fmt.Fprintf(os.Stderr, "gfs: %v\n", err)
		return 1
	}
	if n, err := st.UserCount(context.Background()); err == nil && n > 0 {
		slog.Info("identity ready", "users", n)
	}

	blobs, err := openBlobs(cfg)
	if err != nil {
		slog.Error("s3 client", "error", err)
		return 1
	}
	app := newApp(cfg, st, blobs, version)
	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if blobs != nil {
		go app.RetryLoop(ctx, 30*time.Second)
	}
	go app.SweepLoop(ctx, cfg.RetentionEvery)
	if cfg.SFTPInbox != "" {
		go app.WatchLoop(ctx, cfg.SFTPPoll)
		slog.Info("sftp inbox watcher", "path", cfg.SFTPInbox, "poll", cfg.SFTPPoll.String())
	}
	slog.Info("starting",
		"version", version,
		"listen", cfg.ListenAddr,
		"topology", string(cfg.Topology),
		"data_dir", cfg.DataDir,
	)
	return listenAndServe(ctx, httpSrv)
}

func openBlobs(cfg config.Config) (blob.Store, error) {
	if cfg.Topology != config.TopologyVPSS3 {
		return nil, nil
	}
	return blob.NewS3(context.Background(), blob.S3Config{
		Bucket:    cfg.S3Bucket,
		Region:    cfg.S3Region,
		Endpoint:  cfg.S3Endpoint,
		PathStyle: cfg.S3PathStyle,
	})
}

func newApp(cfg config.Config, st *store.Store, blobs blob.Store, version string) *server.Server {
	return &server.Server{
		Cfg:        cfg,
		Store:      st,
		Blobs:      blobs,
		Version:    version,
		LoginLimit: ratelimit.New(cfg.LoginRateLimit.Requests, cfg.LoginRateLimit.Window),
		Ready: func() bool {
			if !st.Ping(context.Background()) {
				return false
			}
			if cfg.Topology != config.TopologyVPSS3 {
				return true
			}
			if blobs == nil {
				return cfg.S3Bucket != "" && config.S3CredsPresent()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return blobs.HeadBucket(ctx) == nil
		},
	}
}

func newHTTPServer(cfg config.Config, st *store.Store) *http.Server {
	app := newApp(cfg, st, nil, version)
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func listenAndServe(ctx context.Context, httpSrv *http.Server) int {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listen", "addr", httpSrv.Addr)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		return 0
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
			return 1
		}
		return 0
	}
}

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

	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/logging"
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

	if err := st.EnsureAdmin(context.Background(), cfg.BootstrapAdmin, cfg.BootstrapPassword); err != nil {
		fmt.Fprintf(os.Stderr, "gfs: %v\n", err)
		return 1
	}
	if n, err := st.UserCount(context.Background()); err == nil && n > 0 {
		slog.Info("identity ready", "users", n)
	}

	httpSrv := newHTTPServer(cfg, st)
	slog.Info("starting",
		"version", version,
		"listen", cfg.ListenAddr,
		"topology", string(cfg.Topology),
		"data_dir", cfg.DataDir,
	)
	return listenAndServe(httpSrv)
}

func newHTTPServer(cfg config.Config, st *store.Store) *http.Server {
	srv := &server.Server{
		Cfg:   cfg,
		Store: st,
		Ready: func() bool {
			if !st.Ping(context.Background()) {
				return false
			}
			if cfg.Topology == config.TopologyVPSS3 {
				return cfg.S3Bucket != "" && config.S3CredsPresent()
			}
			return true
		},
	}
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func listenAndServe(httpSrv *http.Server) int {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("listen", "addr", httpSrv.Addr)
		errCh <- httpSrv.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

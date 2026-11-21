package server

import (
	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

// newTestServer builds a mux-wired Server for package tests without rate limits.
func newTestServer(cfg config.Config, st *store.Store, blobs blob.Store, version string) *Server {
	if version == "" {
		version = "0.1.0-test"
	}
	return New(cfg, st, blobs, version, nil)
}

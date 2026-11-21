package server

import (
	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/ratelimit"
	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/httpapi"
	"github.com/hrodrig/groot-share/internal/server/web"
	"github.com/hrodrig/groot-share/internal/store"
)

// New builds a Server with core, API, and Web wired for tests and cmd/gfs.
func New(cfg config.Config, st *store.Store, blobs blob.Store, version string, loginLimit *ratelimit.Limiter) *Server {
	c := core.NewCore(cfg, st, blobs, version, loginLimit)
	s := &Server{Core: c}
	s.api = httpapi.New(c, s.handleNotFound, s.writeShareGone)
	s.ui = web.New(c)
	return s
}

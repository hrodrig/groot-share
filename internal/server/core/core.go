// Package core holds shared server state and domain helpers.
package core

import (
	"context"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/ratelimit"
	"github.com/hrodrig/groot-share/internal/store"
)

// Core is shared mutable state and domain logic for HTTP handlers.
type Core struct {
	Cfg     config.Config
	Store   *store.Store
	Blobs   blob.Store
	Ready   func() bool
	Version string
	// LoginLimit gates POST /login (nil = unlimited; production sets from config).
	LoginLimit        *ratelimit.Limiter
	listCache         listCache
	completenessCache completenessCache
	// AuditInsert overrides Store.InsertAudit when non-nil (tests only).
	AuditInsert func(ctx context.Context, ev store.Audit) error
}

// NewCore builds a Core value. Callers typically wrap it in server.New.
func NewCore(cfg config.Config, st *store.Store, blobs blob.Store, version string, loginLimit *ratelimit.Limiter) *Core {
	return &Core{
		Cfg:        cfg,
		Store:      st,
		Blobs:      blobs,
		Version:    version,
		LoginLimit: loginLimit,
	}
}

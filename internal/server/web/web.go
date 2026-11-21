package web

import "github.com/hrodrig/groot-share/internal/server/core"

// Web serves HTML operator pages.
type Web struct {
	*core.Core
}

// New builds a Web handler bundle.
func New(c *core.Core) *Web {
	return &Web{Core: c}
}

package httpapi

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/core"
)

// API serves JSON /v1/* handlers (and hybrid upload/delete/pin).
type API struct {
	*core.Core
	NotFound  http.HandlerFunc
	ShareGone func(w http.ResponseWriter, r *http.Request, reason string)
}

// New builds an API handler bundle.
func New(c *core.Core, notFound http.HandlerFunc, shareGone func(w http.ResponseWriter, r *http.Request, reason string)) *API {
	return &API{Core: c, NotFound: notFound, ShareGone: shareGone}
}

package httpapi

import (
	"net/http"

	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

func parsePage(r *http.Request) int     { return tmpl.ParsePage(r) }
func parsePageSize(r *http.Request) int { return tmpl.ParsePageSize(r) }
func pageViewFor(total, page, pageSize int) tmpl.PageView {
	return tmpl.PageViewFor(total, page, pageSize)
}

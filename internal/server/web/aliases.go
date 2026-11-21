package web

import (
	"net/http"
	"net/url"

	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

// Type / symbol aliases keep handler call sites compact while view helpers live in tmpl.

type pageView = tmpl.PageView
type FilterURLBuilder = tmpl.FilterURLBuilder

func parsePage(r *http.Request) int     { return tmpl.ParsePage(r) }
func parsePageSize(r *http.Request) int { return tmpl.ParsePageSize(r) }
func pageViewFor(total, page, pageSize int) pageView {
	return tmpl.PageViewFor(total, page, pageSize)
}
func applySortQuery(pv *pageView, field string, asc bool) {
	tmpl.ApplySortQuery(pv, field, asc)
}
func paginateSlice[T any](items []T, page, pageSize int) ([]T, pageView) {
	return tmpl.PaginateSlice(items, page, pageSize)
}
func noticeFromQuery(q url.Values) (string, string) { return tmpl.NoticeFromQuery(q) }
func loginErrorCopy(code string) string             { return tmpl.LoginErrorCopy(code) }
func statsLine(count int, total int64) string       { return tmpl.StatsLine(count, total) }

package server

import (
	"net/http"
	"net/url"

	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

// Type / symbol aliases keep handler call sites compact while view helpers live in tmpl.

type pageView = tmpl.PageView
type pageRef = tmpl.PageRef
type FilterURLBuilder = tmpl.FilterURLBuilder

var HTMLPageSizes = tmpl.HTMLPageSizes

const defaultSortField = tmpl.DefaultSortField

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
func firstSortAsc(field string) bool { return tmpl.FirstSortAsc(field) }
func sortURL(pv pageView, field string) string { return tmpl.SortURL(pv, field) }
func pagerURL(page int, pv pageView) string    { return tmpl.PagerURL(page, pv) }
func QueryStringWith(r *http.Request, key, value string) string {
	return tmpl.QueryStringWith(r, key, value)
}
func QueryStringWithout(r *http.Request, key string) string {
	return tmpl.QueryStringWithout(r, key)
}
func humanSize(n int64) string                      { return tmpl.HumanSize(n) }
func statsLine(count int, total int64) string       { return tmpl.StatsLine(count, total) }
func noticeFromQuery(q url.Values) (string, string) { return tmpl.NoticeFromQuery(q) }
func noticeCopy(token string) (string, string)      { return tmpl.NoticeCopy(token) }
func loginErrorCopy(code string) string             { return tmpl.LoginErrorCopy(code) }
func displayVersion(v string) string                { return tmpl.DisplayVersion(v) }
func pageWindow(current, totalPages int) []pageRef  { return tmpl.PageWindow(current, totalPages) }

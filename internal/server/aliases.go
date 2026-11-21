package server

import (
	"net/http"
	"net/url"

	"github.com/hrodrig/groot-share/internal/server/core"
	"github.com/hrodrig/groot-share/internal/server/tmpl"
)

type pageView = tmpl.PageView
type pageRef = tmpl.PageRef
type FilterURLBuilder = tmpl.FilterURLBuilder

var HTMLPageSizes = tmpl.HTMLPageSizes

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
func sortURL(pv pageView, field string) string { return tmpl.SortURL(pv, field) }
func pagerURL(page int, pv pageView) string    { return tmpl.PagerURL(page, pv) }
func QueryStringWith(r *http.Request, key, value string) string {
	return tmpl.QueryStringWith(r, key, value)
}
func QueryStringWithout(r *http.Request, key string) string {
	return tmpl.QueryStringWithout(r, key)
}
func humanSize(n int64) string                      { return tmpl.HumanSize(n) }
func noticeFromQuery(q url.Values) (string, string) { return tmpl.NoticeFromQuery(q) }
func noticeCopy(token string) (string, string)      { return tmpl.NoticeCopy(token) }
func loginErrorCopy(code string) string             { return tmpl.LoginErrorCopy(code) }
func displayVersion(v string) string                { return tmpl.DisplayVersion(v) }
func pageWindow(current, totalPages int) []pageRef  { return tmpl.PageWindow(current, totalPages) }

func parseUserID(raw string) (int64, error) {
	return core.ParseUserID(raw)
}

func adminNotice(token string) (kind, text string) {
	n, ok := adminNoticeCopy[token]
	if !ok {
		return "", ""
	}
	return n[0], n[1]
}

var adminNoticeCopy = map[string][2]string{
	"created":     {"ok", "User created."},
	"updated":     {"ok", "User updated."},
	"deactivated": {"ok", "User deactivated."},
	"activated":   {"ok", "User activated."},
	"removed":     {"ok", "User removed."},
	"self":        {"err", "You cannot remove your own account."},
	"active":      {"err", "Deactivate the user before removing them."},
	"last_admin":  {"err", "Cannot change the last active admin."},
	"pw_short":    {"err", "Password must be at least 8 characters."},
	"username":    {"err", "Username is required."},
	"name":        {"err", "Name is required."},
	"taken":       {"err", "That username is already in use."},
	"role":        {"err", "Choose a valid role: viewer, uploader, or admin."},
	"error":       {"err", "That action failed. Check the form and try again."},
}

func settingsNotice(token string) (kind, text string) {
	switch token {
	case "password":
		return "ok", "Password updated."
	case "named":
		return "ok", "Name updated."
	case "name":
		return "err", "Name is required (max 80 characters)."
	case "revoked":
		return "ok", "API key deleted."
	case "pw_short":
		return "err", "Password must be at least 8 characters."
	case "error":
		return "err", "That action failed. Check the form and try again."
	default:
		return "", ""
	}
}

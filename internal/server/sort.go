package server

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/hrodrig/groot-share/internal/store"
)

const defaultSortField = "uploaded"

func parseSort(r *http.Request) (field string, asc bool) {
	field = r.URL.Query().Get("sort")
	switch field {
	case "key", "name":
		field = "key"
	case "source", "storage", "size", "uploaded":
	default:
		field = defaultSortField
	}
	switch r.URL.Query().Get("order") {
	case "asc":
		return field, true
	case "desc":
		return field, false
	default:
		return field, firstSortAsc(field)
	}
}

func firstSortAsc(field string) bool {
	switch field {
	case "size", "uploaded":
		return false
	default:
		return true
	}
}

func sortArchives(items []store.Archive, field string, asc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		less := archiveLess(items[i], items[j], field)
		if asc {
			return less
		}
		return !less
	})
}

func archiveLess(a, b store.Archive, field string) bool {
	switch field {
	case "key":
		return a.Key < b.Key
	case "source":
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Key < b.Key
	case "storage":
		sa, sb := a.Storage, b.Storage
		if sa == "" {
			sa = "local"
		}
		if sb == "" {
			sb = "local"
		}
		if sa != sb {
			return sa < sb
		}
		return a.Key < b.Key
	case "size":
		if a.Size != b.Size {
			return a.Size < b.Size
		}
		return a.Key < b.Key
	default:
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.Key < b.Key
	}
}

func listURL(page, pageSize int, sortField string, sortAsc bool) string {
	if sortField == "" {
		sortField = defaultSortField
	}
	v := url.Values{}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if pageSize != htmlPageSize {
		v.Set("per_page", strconv.Itoa(pageSize))
	}
	if sortField != defaultSortField {
		v.Set("sort", sortField)
	}
	if sortAsc != firstSortAsc(sortField) {
		if sortAsc {
			v.Set("order", "asc")
		} else {
			v.Set("order", "desc")
		}
	}
	q := v.Encode()
	if q == "" {
		return "?"
	}
	return "?" + q
}

func sortURL(pv pageView, field string) string {
	asc := firstSortAsc(field)
	if pv.SortField == field {
		asc = !pv.SortAsc
	}
	return listURL(1, pv.PageSize, field, asc)
}

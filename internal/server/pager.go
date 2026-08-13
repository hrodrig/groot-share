package server

import (
	"net/http"
	"strconv"
)

const htmlPageSize = 25

// HTMLPageSizes are allowed ?per_page= values for list views.
var HTMLPageSizes = []int{25, 50, 100}

// pageView is HTML pagination state (1-based page index).
type pageView struct {
	Page        int
	PageSize    int
	TotalPages  int
	Total       int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
	SortField   string
	SortAsc     bool
	HiddenSort  string
	HiddenOrder string
}

func parsePage(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		return 1
	}
	return p
}

func parsePageSize(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	for _, allowed := range HTMLPageSizes {
		if n == allowed {
			return n
		}
	}
	return htmlPageSize
}

func pagerURL(page int, pv pageView) string {
	return listURL(page, pv.PageSize, pv.SortField, pv.SortAsc)
}

func applySortQuery(pv *pageView, field string, asc bool) {
	if field == "" {
		field = defaultSortField
	}
	pv.SortField = field
	pv.SortAsc = asc
	if field != defaultSortField {
		pv.HiddenSort = field
	}
	if asc != firstSortAsc(field) {
		if asc {
			pv.HiddenOrder = "asc"
		} else {
			pv.HiddenOrder = "desc"
		}
	}
}

func pageViewFor(total, page, pageSize int) pageView {
	if pageSize <= 0 {
		pageSize = htmlPageSize
	}
	if page < 1 {
		page = 1
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	return pageView{
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		Total:      total,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
	}
}

func paginateSlice[T any](items []T, page, pageSize int) ([]T, pageView) {
	pv := pageViewFor(len(items), page, pageSize)
	if len(items) == 0 {
		return nil, pv
	}
	start := (pv.Page - 1) * pageSize
	if start >= len(items) {
		return nil, pv
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], pv
}

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
	FirstPage   int
	LastPage    int
	Pages       []pageRef
	SortField   string
	SortAsc     bool
	HiddenSort  string
	HiddenOrder string
}

// pageRef is one clickable slot in the numeric page window. Kind is "page"
// for a concrete page number or "ellipsis" for a non-clickable gap marker.
type pageRef struct {
	Kind  string // "page" | "ellipsis"
	Page  int
	Label string
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
		FirstPage:  1,
		LastPage:   totalPages,
		Pages:      pageWindow(page, totalPages),
	}
}

// pageWindow builds the numeric page window (with ellipsis gaps) to render in
// the pager. It always includes the first and last pages; around the current
// page it shows a tight window of `window` pages on each side. When the total
// is small enough every page is listed with no gap markers.
//
// Short ranges (<= 7) list every page. Longer ranges show:
//
//	1 … (4) 5 [6] 7 (8) … 543
//
// where [current] is highlighted and the ellipses are non-clickable gaps.
func pageWindow(current, totalPages int) []pageRef {
	if totalPages <= 0 {
		totalPages = 1
	}
	if current < 1 {
		current = 1
	}
	if current > totalPages {
		current = totalPages
	}
	if totalPages <= 7 {
		refs := make([]pageRef, 0, totalPages)
		for p := 1; p <= totalPages; p++ {
			refs = append(refs, pageRef{Kind: "page", Page: p, Label: strconv.Itoa(p)})
		}
		return refs
	}

	const window = 1
	var refs []pageRef
	add := func(p int) { refs = append(refs, pageRef{Kind: "page", Page: p, Label: strconv.Itoa(p)}) }
	addGap := func() { refs = append(refs, pageRef{Kind: "ellipsis", Label: "…"}) }

	// Left edge: page 1, then a gap unless the window reaches it.
	refs = append(refs, pageRef{Kind: "page", Page: 1, Label: "1"})
	if current-window > 2 {
		addGap()
	}

	// Middle window around current (clamped to interior pages).
	lo := current - window
	if lo < 2 {
		lo = 2
	}
	hi := current + window
	if hi > totalPages-1 {
		hi = totalPages - 1
	}
	for p := lo; p <= hi; p++ {
		add(p)
	}

	// Right edge: gap then last page, unless the window reaches it.
	if current+window < totalPages-1 {
		addGap()
	}
	refs = append(refs, pageRef{Kind: "page", Page: totalPages, Label: strconv.Itoa(totalPages)})

	return refs
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

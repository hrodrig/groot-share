package tmpl

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
)

const htmlPageSize = 25

// HTMLPageSizes are allowed ?per_page= values for list views.
var HTMLPageSizes = []int{25, 50, 100}

// PageView is HTML pagination state (1-based page index).
type PageView struct {
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
	Pages       []PageRef
	SortField   string
	SortAsc     bool
	HiddenSort  string
	HiddenOrder string
}

// PageRef is one clickable slot in the numeric page window.
type PageRef struct {
	Kind  string // "page" | "ellipsis"
	Page  int
	Label string
}

// ParsePage reads ?page= (minimum 1).
func ParsePage(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		return 1
	}
	return p
}

// ParsePageSize reads ?per_page= against HTMLPageSizes.
func ParsePageSize(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	for _, allowed := range HTMLPageSizes {
		if n == allowed {
			return n
		}
	}
	return htmlPageSize
}

// PagerURL builds a list URL for the given page using PageView sort/size.
func PagerURL(page int, pv PageView) string {
	return ListURL(page, pv.PageSize, pv.SortField, pv.SortAsc)
}

// ApplySortQuery fills HiddenSort/HiddenOrder for form persistence.
func ApplySortQuery(pv *PageView, field string, asc bool) {
	if field == "" {
		field = DefaultSortField
	}
	pv.SortField = field
	pv.SortAsc = asc
	if field != DefaultSortField {
		pv.HiddenSort = field
	}
	if asc != FirstSortAsc(field) {
		if asc {
			pv.HiddenOrder = "asc"
		} else {
			pv.HiddenOrder = "desc"
		}
	}
}

// PageViewFor builds pagination state for total/page/pageSize.
func PageViewFor(total, page, pageSize int) PageView {
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
	return PageView{
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
		Pages:      PageWindow(page, totalPages),
	}
}

// PageWindow builds the numeric page window (with ellipsis gaps) for the pager.
func PageWindow(current, totalPages int) []PageRef {
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
		refs := make([]PageRef, 0, totalPages)
		for p := 1; p <= totalPages; p++ {
			refs = append(refs, PageRef{Kind: "page", Page: p, Label: strconv.Itoa(p)})
		}
		return refs
	}

	const window = 1
	var refs []PageRef
	add := func(p int) { refs = append(refs, PageRef{Kind: "page", Page: p, Label: strconv.Itoa(p)}) }
	addGap := func() { refs = append(refs, PageRef{Kind: "ellipsis", Label: "…"}) }

	refs = append(refs, PageRef{Kind: "page", Page: 1, Label: "1"})
	if current-window > 2 {
		addGap()
	}

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

	if current+window < totalPages-1 {
		addGap()
	}
	refs = append(refs, PageRef{Kind: "page", Page: totalPages, Label: strconv.Itoa(totalPages)})
	return refs
}

// PaginateSlice returns one page of items plus PageView.
func PaginateSlice[T any](items []T, page, pageSize int) ([]T, PageView) {
	pv := PageViewFor(len(items), page, pageSize)
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

// DefaultSortField is the Captures default sort.
const DefaultSortField = "uploaded"

// FirstSortAsc is the default ascending direction for a sort field.
func FirstSortAsc(field string) bool {
	switch field {
	case "size", "uploaded":
		return false
	default:
		return true
	}
}

// ListURL builds a query string for page/size/sort.
func ListURL(page, pageSize int, sortField string, sortAsc bool) string {
	if sortField == "" {
		sortField = DefaultSortField
	}
	v := url.Values{}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if pageSize != htmlPageSize {
		v.Set("per_page", strconv.Itoa(pageSize))
	}
	if sortField != DefaultSortField {
		v.Set("sort", sortField)
	}
	if sortAsc != FirstSortAsc(sortField) {
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

// SortURL toggles sort for a column header.
func SortURL(pv PageView, field string) string {
	asc := FirstSortAsc(field)
	if pv.SortField == field {
		asc = !pv.SortAsc
	}
	return ListURL(1, pv.PageSize, field, asc)
}

// FilterURLBuilder renders facet-chip hrefs.
type FilterURLBuilder struct {
	Base string
}

// With returns the base with key=value as template.URL.
func (b FilterURLBuilder) With(key, value string) template.URL {
	q, err := url.ParseQuery(b.Base)
	if err != nil {
		q = url.Values{}
	}
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return template.URL(q.Encode())
}

// Without returns the base with key removed.
func (b FilterURLBuilder) Without(key string) template.URL {
	return b.With(key, "")
}

// QueryStringWith sets or clears a query param on the request query string.
func QueryStringWith(r *http.Request, key, value string) string {
	q := r.URL.Query()
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return q.Encode()
}

// QueryStringWithout removes a query param.
func QueryStringWithout(r *http.Request, key string) string {
	return QueryStringWith(r, key, "")
}

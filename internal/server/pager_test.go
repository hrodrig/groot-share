package server

import (
	"net/http/httptest"
	"testing"
)

func TestPageViewFor(t *testing.T) {
	pv := pageViewFor(60, 2, 25)
	if pv.Page != 2 || pv.PageSize != 25 || pv.TotalPages != 3 || !pv.HasPrev || !pv.HasNext || pv.PrevPage != 1 || pv.NextPage != 3 {
		t.Fatalf("%+v", pv)
	}
}

func TestParsePageSize(t *testing.T) {
	for _, tc := range []struct {
		q    string
		want int
	}{
		{"", 25},
		{"per_page=50", 50},
		{"per_page=100", 100},
		{"per_page=99", 25},
	} {
		r := httptest.NewRequest("GET", "/?"+tc.q, nil)
		if got := parsePageSize(r); got != tc.want {
			t.Fatalf("?%s: got %d want %d", tc.q, got, tc.want)
		}
	}
}

func TestPagerURL(t *testing.T) {
	pv := pageView{PageSize: 25, SortField: "uploaded", SortAsc: false}
	if pagerURL(1, pv) != "?" {
		t.Fatal(pagerURL(1, pv))
	}
	if pagerURL(2, pv) != "?page=2" {
		t.Fatal(pagerURL(2, pv))
	}
	pv.PageSize = 50
	if pagerURL(1, pv) != "?per_page=50" {
		t.Fatal(pagerURL(1, pv))
	}
	pv.PageSize = 100
	if pagerURL(3, pv) != "?page=3&per_page=100" {
		t.Fatal(pagerURL(3, pv))
	}
}

func TestApplySortQuery(t *testing.T) {
	pv := pageView{PageSize: 25}
	applySortQuery(&pv, "key", true)
	if pv.SortField != "key" || pv.HiddenSort != "key" || pv.HiddenOrder != "" {
		t.Fatalf("%+v", pv)
	}
	applySortQuery(&pv, "uploaded", true)
	if pv.HiddenOrder != "asc" {
		t.Fatalf("uploaded asc %+v", pv)
	}
}

func TestPageViewForEmpty(t *testing.T) {
	pv := pageViewFor(0, 1, 25)
	if pv.TotalPages != 1 || pv.Total != 0 {
		t.Fatalf("%+v", pv)
	}
}

func TestPaginateSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	slice, pv := paginateSlice(items, 2, 2)
	if len(slice) != 2 || slice[0] != 3 || slice[1] != 4 {
		t.Fatalf("slice %v", slice)
	}
	if pv.Page != 2 || pv.TotalPages != 3 {
		t.Fatalf("%+v", pv)
	}
}

func TestPageWindow(t *testing.T) {
	labels := func(refs []pageRef) (kind []string, pages []int) {
		for _, r := range refs {
			kind = append(kind, r.Kind)
			pages = append(pages, r.Page)
		}
		return kind, pages
	}
	pageNums := func(refs []pageRef) []int {
		var out []int
		for _, r := range refs {
			if r.Kind == "page" {
				out = append(out, r.Page)
			}
		}
		return out
	}

	// Short range: every page listed, no gaps.
	if refs := pageWindow(3, 5); len(refs) != 5 || len(pageNums(refs)) != 5 {
		t.Fatalf("short range: %+v", refs)
	}

	// Large inventory (543 pages, current 20): must expose first, last, and
	// the window around 20, with ellipsis gaps on both sides.
	refs := pageWindow(20, 543)
	kinds, pages := labels(refs)
	if kinds[0] != "page" || pages[0] != 1 {
		t.Fatalf("first slot should be page 1: %+v", refs)
	}
	if kinds[len(kinds)-1] != "page" || pages[len(pages)-1] != 543 {
		t.Fatalf("last slot should be page 543: %+v", refs)
	}
	// The current page 20 must be present and (window=1) 19 and 21 too.
	nums := map[int]bool{}
	for _, p := range pageNums(refs) {
		nums[p] = true
	}
	for _, want := range []int{1, 19, 20, 21, 543} {
		if !nums[want] {
			t.Fatalf("page %d missing from window %+v", want, refs)
		}
	}
	// At least two ellipsis gaps (left + right).
	gaps := 0
	for _, k := range kinds {
		if k == "ellipsis" {
			gaps++
		}
	}
	if gaps < 2 {
		t.Fatalf("expected >=2 gaps, got %d in %+v", gaps, refs)
	}

	// First page: no left gap before page 1.
	refs = pageWindow(1, 543)
	kinds, pages = labels(refs)
	if kinds[0] != "page" || pages[0] != 1 {
		t.Fatalf("first page edge: %+v", refs)
	}
	if kinds[1] == "ellipsis" {
		t.Fatalf("no gap expected right after page 1 when on page 1: %+v", refs)
	}

	// Last page: no right gap after the last page.
	refs = pageWindow(543, 543)
	kinds, pages = labels(refs)
	if kinds[len(kinds)-1] != "page" || pages[len(pages)-1] != 543 {
		t.Fatalf("last page edge: %+v", refs)
	}
}

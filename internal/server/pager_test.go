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

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

func pageWindowNums(current, total int) (kinds []string, nums []int, gaps int) {
	for _, r := range pageWindow(current, total) {
		kinds = append(kinds, r.Kind)
		if r.Kind == "ellipsis" {
			gaps++
			continue
		}
		nums = append(nums, r.Page)
	}
	return kinds, nums, gaps
}

func TestPageWindow(t *testing.T) {
	cases := []struct {
		name      string
		current   int
		total     int
		wantFirst int
		wantLast  int
		wantPages []int
		wantGaps  int
	}{
		{name: "short range lists every page", current: 3, total: 5, wantFirst: 1, wantLast: 5, wantPages: []int{1, 2, 3, 4, 5}, wantGaps: 0},
		{name: "large range shows window and both gaps", current: 20, total: 543, wantFirst: 1, wantLast: 543, wantPages: []int{1, 19, 20, 21, 543}, wantGaps: 2},
		{name: "on first page no left gap", current: 1, total: 543, wantFirst: 1, wantLast: 543, wantGaps: 1},
		{name: "on last page no right gap", current: 543, total: 543, wantFirst: 1, wantLast: 543, wantGaps: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kinds, nums, gaps := pageWindowNums(tc.current, tc.total)
			if kinds[0] != "page" || nums[0] != tc.wantFirst {
				t.Fatalf("first slot = %q/%d, want page %d", kinds[0], nums[0], tc.wantFirst)
			}
			if kinds[len(kinds)-1] != "page" || nums[len(nums)-1] != tc.wantLast {
				t.Fatalf("last slot = %q/%d, want page %d", kinds[len(kinds)-1], nums[len(nums)-1], tc.wantLast)
			}
			if gaps != tc.wantGaps {
				t.Fatalf("gaps = %d, want %d", gaps, tc.wantGaps)
			}
			if tc.wantPages != nil {
				have := map[int]bool{}
				for _, p := range nums {
					have[p] = true
				}
				for _, p := range tc.wantPages {
					if !have[p] {
						t.Fatalf("page %d missing from window %v", p, nums)
					}
				}
			}
		})
	}
}

package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

func TestParseSort(t *testing.T) {
	for _, tc := range []struct {
		q         string
		wantField string
		wantAsc   bool
	}{
		{"", "uploaded", false},
		{"sort=key", "key", true},
		{"sort=key&order=desc", "key", false},
		{"sort=size&order=asc", "size", true},
		{"sort=bad", "uploaded", false},
	} {
		r := httptest.NewRequest("GET", "/?"+tc.q, nil)
		field, asc := parseSort(r)
		if field != tc.wantField || asc != tc.wantAsc {
			t.Fatalf("?%s: got %s asc=%v want %s asc=%v", tc.q, field, asc, tc.wantField, tc.wantAsc)
		}
	}
}

func TestSortArchives(t *testing.T) {
	now := time.Now().UTC()
	items := []store.Archive{
		{Key: "b.tar.gz", Size: 200, CreatedAt: now.Add(-time.Hour)},
		{Key: "a.tar.gz", Size: 100, CreatedAt: now},
	}
	sortArchives(items, "key", true)
	if items[0].Key != "a.tar.gz" || items[1].Key != "b.tar.gz" {
		t.Fatalf("key sort %v", items)
	}
	sortArchives(items, "size", false)
	if items[0].Size != 200 {
		t.Fatalf("size desc %v", items)
	}
}

func TestListURL(t *testing.T) {
	pv := pageView{PageSize: 25, SortField: "uploaded", SortAsc: false}
	if pagerURL(1, pv) != "?" {
		t.Fatal(pagerURL(1, pv))
	}
	pv.SortField = "key"
	pv.SortAsc = true
	if got := pagerURL(2, pv); got != "?page=2&sort=key" {
		t.Fatal(got)
	}
}

func TestSortURL(t *testing.T) {
	pv := pageView{PageSize: 50, SortField: "uploaded", SortAsc: false}
	if got := sortURL(pv, "key"); got != "?per_page=50&sort=key" {
		t.Fatal(got)
	}
	pv.SortField = "key"
	pv.SortAsc = true
	if got := sortURL(pv, "key"); !strings.Contains(got, "sort=key") || !strings.Contains(got, "order=desc") {
		t.Fatal(got)
	}
}

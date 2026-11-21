package core

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

func TestSortArchivesCore(t *testing.T) {
	now := time.Now()
	items := []store.Archive{
		{Key: "b", CreatedAt: now},
		{Key: "a", CreatedAt: now},
	}
	SortArchives(items, "key", true)
	if items[0].Key != "a" || items[1].Key != "b" {
		t.Fatalf("order %+v", items)
	}
}

func TestParseSortCore(t *testing.T) {
	req := httptest.NewRequest("GET", "/?sort=size&order=desc", nil)
	field, asc := ParseSort(req)
	if field != "size" || asc {
		t.Fatalf("field=%s asc=%v", field, asc)
	}
}

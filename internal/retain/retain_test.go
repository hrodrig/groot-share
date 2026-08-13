package retain

import (
	"fmt"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

func TestPickKeepLastOnly(t *testing.T) {
	now := time.Now().UTC()
	items := []store.Archive{
		{ID: "1", CreatedAt: now},
		{ID: "2", CreatedAt: now},
		{ID: "3", CreatedAt: now},
	}
	got := Pick(items, 2, 90, now)
	if len(got) != 1 || got[0].ID != "3" {
		t.Fatalf("%+v", got)
	}
}

func TestPickAgeDeletesUnderKeepLast(t *testing.T) {
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	items := []store.Archive{
		{ID: "a", CreatedAt: now.Add(-200 * 24 * time.Hour)},
		{ID: "b", CreatedAt: now.Add(-180 * 24 * time.Hour)},
	}
	got := Pick(items, 20, 90, now)
	if len(got) != 2 {
		t.Fatalf("age should delete both: %+v", got)
	}
}

func TestPickUnion(t *testing.T) {
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	items := []store.Archive{
		{ID: "new", CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "mid", CreatedAt: now.Add(-48 * time.Hour)},
		{ID: "old", CreatedAt: now.Add(-200 * 24 * time.Hour)},
	}
	got := Pick(items, 2, 90, now)
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("%+v", got)
	}
}

func TestPickDefaultLimits(t *testing.T) {
	now := time.Now().UTC()
	items := make([]store.Archive, 22)
	for i := range items {
		items[i] = store.Archive{ID: fmt.Sprintf("id-%d", i), CreatedAt: now}
	}
	got := Pick(items, 0, 0, now)
	if len(got) != 2 {
		t.Fatalf("default keep_last 20: got %+v", got)
	}
}

func TestPickSkipsEmptyID(t *testing.T) {
	now := time.Now().UTC()
	got := Pick([]store.Archive{{ID: "", CreatedAt: now.Add(-200 * 24 * time.Hour)}}, 20, 90, now)
	if len(got) != 0 {
		t.Fatalf("empty id skipped: %+v", got)
	}
}

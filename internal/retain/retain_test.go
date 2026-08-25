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

func TestPickKeepLastZeroKeepsAllByCount(t *testing.T) {
	now := time.Now().UTC()
	items := make([]store.Archive, 30)
	for i := range items {
		items[i] = store.Archive{ID: fmt.Sprintf("id-%d", i), CreatedAt: now}
	}
	// keepLast == 0 → no count limit; all items are "new", so age deletes none.
	got := Pick(items, 0, 90, now)
	if len(got) != 0 {
		t.Fatalf("keepLast=0 must not delete by rank: got %+v", got)
	}
}

func TestPickNegativeKeepLastDefaultsTo20(t *testing.T) {
	now := time.Now().UTC()
	items := make([]store.Archive, 22)
	for i := range items {
		items[i] = store.Archive{ID: fmt.Sprintf("id-%d", i), CreatedAt: now}
	}
	got := Pick(items, -1, 0, now)
	if len(got) != 2 {
		t.Fatalf("negative keepLast should default to 20: got %+v", got)
	}
}

func TestPickZeroKeepsAgeDeletesOld(t *testing.T) {
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	items := []store.Archive{
		{ID: "new", CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "old", CreatedAt: now.Add(-200 * 24 * time.Hour)},
	}
	// keepLast == 0 disables rank; age still deletes the old one.
	got := Pick(items, 0, 90, now)
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("keepLast=0 still deletes by age: got %+v", got)
	}
}

func TestPickZeroAgeDisablesAge(t *testing.T) {
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	items := []store.Archive{
		{ID: "new", CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "old", CreatedAt: now.Add(-200 * 24 * time.Hour)},
	}
	// maxAgeDays == 0 disables age, but keep_last (20) still applies; both fit.
	got := Pick(items, 20, 0, now)
	if len(got) != 0 {
		t.Fatalf("maxAgeDays=0 must not delete by age: got %+v", got)
	}
}

func TestPickAllZeroKeepsEverything(t *testing.T) {
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	items := []store.Archive{
		{ID: "new", CreatedAt: now.Add(-24 * time.Hour)},
		{ID: "old", CreatedAt: now.Add(-200 * 24 * time.Hour)},
	}
	// Both disabled → retention deletes nothing.
	got := Pick(items, 0, 0, now)
	if len(got) != 0 {
		t.Fatalf("keep_last=0 + max_age=0 must keep everything: got %+v", got)
	}
}

func TestPickSkipsEmptyID(t *testing.T) {
	now := time.Now().UTC()
	got := Pick([]store.Archive{{ID: "", CreatedAt: now.Add(-200 * 24 * time.Hour)}}, 20, 90, now)
	if len(got) != 0 {
		t.Fatalf("empty id skipped: %+v", got)
	}
}

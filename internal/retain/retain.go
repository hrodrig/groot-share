package retain

import (
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// Pick returns archives that violate keep_last or max_age_days (union).
// items must be newest-first.
//
// keepLast semantics:
//   - keepLast < 0  → default 20 (defensive; config never emits negative)
//   - keepLast == 0 → no count limit: keep everything by rank, age cap only
//   - keepLast > 0  → delete everything beyond the newest keepLast archives
//
// maxAgeDays semantics:
//   - maxAgeDays < 0 → default 90 (defensive; config never emits negative)
//   - maxAgeDays == 0 → no age limit: keep everything regardless of age
//   - maxAgeDays > 0 → delete archives older than maxAgeDays
func Pick(items []store.Archive, keepLast, maxAgeDays int, now time.Time) []store.Archive {
	if keepLast < 0 {
		keepLast = 20
	}
	if maxAgeDays < 0 {
		maxAgeDays = 90
	}
	cutoff := now.UTC().AddDate(0, 0, -maxAgeDays)
	seen := map[string]struct{}{}
	out := []store.Archive{}
	add := func(a store.Archive) {
		if a.ID == "" {
			return
		}
		if _, ok := seen[a.ID]; ok {
			return
		}
		seen[a.ID] = struct{}{}
		out = append(out, a)
	}
	for i, a := range items {
		if keepLast > 0 && i >= keepLast {
			add(a)
		}
		if maxAgeDays > 0 && !a.CreatedAt.IsZero() && a.CreatedAt.Before(cutoff) {
			add(a)
		}
	}
	return out
}

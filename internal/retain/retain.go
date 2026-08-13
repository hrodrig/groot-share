package retain

import (
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// Pick returns archives that violate keep_last or max_age_days (union).
// items must be newest-first.
func Pick(items []store.Archive, keepLast, maxAgeDays int, now time.Time) []store.Archive {
	if keepLast <= 0 {
		keepLast = 20
	}
	if maxAgeDays <= 0 {
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
		if i >= keepLast {
			add(a)
		}
		if !a.CreatedAt.IsZero() && a.CreatedAt.Before(cutoff) {
			add(a)
		}
	}
	return out
}

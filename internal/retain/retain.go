package retain

import (
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// Pick returns archives that violate keep_last or max_age_days (union).
// items must be newest-first overall.
//
// keepLast semantics (per cluster slug from ParseClusterSlug on Key):
//   - keepLast < 0  → default 20 (defensive; config never emits negative)
//   - keepLast == 0 → no count limit: keep everything by rank, age cap only
//   - keepLast > 0  → within each cluster bucket, delete beyond the newest keepLast
//     Unparseable keys share the empty-string bucket.
//
// maxAgeDays semantics (global):
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

	if keepLast > 0 {
		// Rank within each cluster, preserving newest-first relative order.
		rank := map[string]int{}
		for _, a := range items {
			slug, _ := store.ParseClusterSlug(a.Key)
			i := rank[slug]
			rank[slug] = i + 1
			if i >= keepLast {
				add(a)
			}
		}
	}

	if maxAgeDays > 0 {
		for _, a := range items {
			if !a.CreatedAt.IsZero() && a.CreatedAt.Before(cutoff) {
				add(a)
			}
		}
	}
	return out
}

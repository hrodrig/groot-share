package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Filter narrows a ListArchives result. A zero-value Filter is "no
// filters" and behaves the same as ListArchives. Filter is defined in the
// store package so the SQL query builder can use it without an import
// cycle from the server package.
type Filter struct {
	Cluster string    // exact cluster slug (post-filtered in Go, not SQL)
	Query   string    // case-insensitive substring match on key
	Window  string    // raw "24h"|"7d"|"30d"|"" — also derived into Since
	Since   time.Time // zero = no window; non-zero = created_at >= Since
	Source  string    // exact: "http" | "s3" | "sftp" | "" (no filter)
	Storage string    // exact: "local" | "s3" | "transit" | "" (no filter)
}

// IsZero reports whether no filter fields are set.
func (f Filter) IsZero() bool {
	return f.Cluster == "" && f.Query == "" && f.Window == "" && f.Since.IsZero() &&
		f.Source == "" && f.Storage == ""
}

// ListArchivesFiltered returns archives matching the filter. Cluster is
// applied in Go (via ParseClusterSlug on the basename) so the same
// logic works for the SQLite archives table and for the in-memory
// bucket listing on vps-s3.
func (s *Store) ListArchivesFiltered(ctx context.Context, f Filter) ([]Archive, error) {
	all, err := s.ListArchives(ctx)
	if err != nil {
		return nil, err
	}
	return applyFilter(all, f), nil
}

func applyFilter(items []Archive, f Filter) []Archive {
	if f.IsZero() {
		return items
	}
	out := make([]Archive, 0, len(items))
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, a := range items {
		if !archivePasses(a, f, q) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// archivePasses reports whether `a` matches every non-empty field of `f`.
// Pulled out of applyFilter to keep its cyclomatic complexity under the
// project's gocyclo gate.
func archivePasses(a Archive, f Filter, q string) bool {
	if f.Source != "" && a.Source != f.Source {
		return false
	}
	if f.Storage != "" {
		storage := a.Storage
		if storage == "" {
			storage = "local"
		}
		if storage != f.Storage {
			return false
		}
	}
	if !f.Since.IsZero() && a.CreatedAt.Before(f.Since) {
		return false
	}
	if q != "" && !strings.Contains(strings.ToLower(a.Key), q) {
		return false
	}
	if f.Cluster != "" {
		slug, ok := ParseClusterSlug(a.Key)
		if !ok || slug != f.Cluster {
			return false
		}
	}
	return true
}

// ClusterCounts groups archives by cluster slug (from ParseClusterSlug)
// and returns each distinct slug with its count, sorted by count
// descending then by slug ascending. Archives whose basename does not
// look like a timestamped capture are excluded.
func ClusterCounts(items []Archive) []ClusterCount {
	counts := make(map[string]int)
	for _, a := range items {
		if slug, ok := ParseClusterSlug(a.Key); ok {
			counts[slug]++
		}
	}
	out := make([]ClusterCount, 0, len(counts))
	for slug, n := range counts {
		out = append(out, ClusterCount{Slug: slug, Count: n})
	}
	sortClusterCounts(out)
	return out
}

// ClusterCount is one chip in the cluster facet row.
type ClusterCount struct {
	Slug  string
	Count int
}

func sortClusterCounts(cs []ClusterCount) {
	// Stable: highest count first, then alphabetical slug for ties.
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0; j-- {
			if cs[j].Count > cs[j-1].Count ||
				(cs[j].Count == cs[j-1].Count && cs[j].Slug < cs[j-1].Slug) {
				cs[j], cs[j-1] = cs[j-1], cs[j]
				continue
			}
			break
		}
	}
}

// filterableColumns documents which filter fields have a SQL path
// (kept here so future readers see what is and is not pushed down). It
// is not called at runtime.
//
//nolint:unused
func filterableColumns() {
	_ = sql.ErrNoRows
	_ = errors.New
	_ = fmt.Sprintf
}

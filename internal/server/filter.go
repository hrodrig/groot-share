package server

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// allowedWindows is the set of accepted time-window chips. Anything else
// in the URL is silently dropped (treated as "all") so a stale bookmark
// or a typo does not 400 the page.
var allowedWindows = map[string]bool{
	"":    true, // all
	"24h": true,
	"7d":  true,
	"30d": true,
}

// allowedSources is the same idea for the source pill filter.
var allowedSources = map[string]bool{
	"":     true,
	"http": true,
	"s3":   true,
	"sftp": true,
}

// allowedStorages is the same idea for the storage pill filter.
var allowedStorages = map[string]bool{
	"":        true,
	"local":   true,
	"s3":      true,
	"transit": true,
}

// ParseFilter reads the Captures filter query params off the request and
// returns a store.Filter. Unknown values are silently dropped so a stale
// URL never 400s the page.
func ParseFilter(r *http.Request) store.Filter {
	q := r.URL.Query()
	window := strings.TrimSpace(q.Get("window"))
	if !allowedWindows[window] {
		window = ""
	}
	source := strings.TrimSpace(q.Get("source"))
	if !allowedSources[source] {
		source = ""
	}
	storage := strings.TrimSpace(q.Get("storage"))
	if !allowedStorages[storage] {
		storage = ""
	}
	return store.Filter{
		Cluster: strings.TrimSpace(q.Get("cluster")),
		Query:   strings.TrimSpace(q.Get("q")),
		Window:  window,
		Since:   windowSince(window, time.Now().UTC()),
		Source:  source,
		Storage: storage,
	}
}

func windowSince(window string, now time.Time) time.Time {
	switch window {
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	default:
		return time.Time{}
	}
}

// queryStringWith returns a copy of the request query string with the
// named param set to value (empty string removes it). The base query
// string comes from r.URL.RawQuery so existing sort/pager/page params
// are preserved.
func queryStringWith(r *http.Request, key, value string) string {
	q := r.URL.Query()
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return q.Encode()
}

// QueryStringWith is the exported form for use from templates.
func QueryStringWith(r *http.Request, key, value string) string {
	return queryStringWith(r, key, value)
}

// QueryStringWithout returns a copy of the request query string with
// the named param removed entirely.
func QueryStringWithout(r *http.Request, key string) string {
	return queryStringWith(r, key, "")
}

// FilterURLBuilder is a tiny wrapper used by the template to render
// facet-chip hrefs without re-parsing the request each time.
type FilterURLBuilder struct {
	Base string // encoded base query string
}

// With returns the base with key=value, marked as template.URL so the
// html/template package does not escape `=` to `%3d` inside href attrs.
func (b FilterURLBuilder) With(key, value string) template.URL {
	q, err := url.ParseQuery(b.Base)
	if err != nil {
		q = url.Values{}
	}
	if value == "" {
		q.Del(key)
	} else {
		q.Set(key, value)
	}
	return template.URL(q.Encode())
}

// Without returns the base with key removed.
func (b FilterURLBuilder) Without(key string) template.URL {
	return b.With(key, "")
}

// applyFilterInMemory runs the filter on an in-memory list, treating
// empty Storage as "local" (so the filter "storage=local" matches both
// the literal "local" rows and the historical "" rows). This mirrors
// the storage normalization handleHome does before counting.
func applyFilterInMemory(items []store.Archive, f store.Filter) []store.Archive {
	if f.IsZero() {
		return items
	}
	out := make([]store.Archive, 0, len(items))
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, a := range items {
		if !serverArchivePasses(a, f, q) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func serverArchivePasses(a store.Archive, f store.Filter, q string) bool {
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
		slug, ok := store.ParseClusterSlug(a.Key)
		if !ok || slug != f.Cluster {
			return false
		}
	}
	return true
}

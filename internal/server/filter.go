package server

import (
	"net/http"
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

// applyFilterInMemory delegates to store.FilterInMemory so the in-memory
// predicate stays identical to the SQL path (no duplicate copy to drift).
func applyFilterInMemory(items []store.Archive, f store.Filter) []store.Archive {
	return store.FilterInMemory(items, f)
}

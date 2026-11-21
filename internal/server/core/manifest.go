package core

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

// manifestPeekCap bounds how much of the manifest member we are willing to
// decode. The real extras/manifest.json is a few hundred bytes; a cap is a
// hard safety bound so a hostile or corrupt member can never balloon memory.
const manifestPeekCap = 64 << 10 // 64 KiB

// manifestJobs is the smallest slice of groot's extras/manifest.json that the
// completeness badge reads. We decode only "jobs"; the rest of the object is
// ignored. Field names mirror groot's archive_layout_version 1 schema.
type manifestJobs struct {
	Jobs struct {
		Total   int `json:"total"`
		Success int `json:"success"`
		Failed  int `json:"failed"`
	} `json:"jobs"`
}

// CompletenessBadge is the per-row view-model fragment rendered next to the
// capture key. It is absent (false) unless a local archive exposes a valid
// manifest with job counters.
type CompletenessBadge struct {
	Label string // "Complete" | "N of M jobs failed" | "Failed"
	Tone  string // ok | warn | err
}

// peekManifest scans a groot .tar.gz stream for the extras/manifest.json
// member (by name, accept the "*/extras/manifest.json" suffix form) and
// returns the job counters when the member is present and well-formed.
//
// It is strictly fail-closed: any of a non-gzip stream, an invalid tar
// header, a missing member, an oversized or malformed JSON body, or
// nonsensical counters (negative total/failed, failed > total) yield ok=false
// with no error — callers simply leave the row unmarked. It never decompresses
// the whole archive: tar.Next skips bodies, and the manifest body is read once
// through io.LimitReader.
func PeekManifest(r io.Reader) (total, success, failed int, ok bool) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return 0, 0, 0, false
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return 0, 0, 0, false
		}
		if err != nil {
			return 0, 0, 0, false
		}
		if !isManifestMember(hdr.Name) {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(tr, manifestPeekCap))
		if err != nil {
			return 0, 0, 0, false
		}
		var m manifestJobs
		if err := json.Unmarshal(raw, &m); err != nil {
			return 0, 0, 0, false
		}
		j := m.Jobs
		// total == 0 yields no badge: an empty/no-count jobs block is
		// indistinguishable from a manifest that never tracked jobs, so we
		// fail closed rather than label a capture "Complete" on no evidence.
		if j.Total <= 0 || j.Failed < 0 || j.Failed > j.Total {
			return 0, 0, 0, false
		}
		return j.Total, j.Success, j.Failed, true
	}
}

// isManifestMember reports whether a tar entry name is the groot manifest,
// accepting both "extras/manifest.json" and the "*/extras/manifest.json" form
// groot itself tolerates (LookupSuffix).
func isManifestMember(name string) bool {
	return name == "extras/manifest.json" || strings.HasSuffix(name, "/extras/manifest.json")
}

// completenessBadgeOf peeks the manifest of a local (vps) archive and returns
// its badge. Non-local rows (s3, transit) and any peek failure return ok=false
// so the row stays unmarked — never an error, never log spam. Results are
// memoized per archive id for completenessCacheTTL so a page of N local
// archives doesn't open N .tar.gz files on every render.
func (c *Core) CompletenessBadgeOf(a store.Archive) (CompletenessBadge, bool) {
	if a.Storage != "local" {
		return CompletenessBadge{}, false
	}
	now := time.Now()
	if badge, ok, found := c.completenessCache.get(a.ID, now); found {
		return badge, ok
	}
	badge, ok := c.computeBadge(a)
	c.completenessCache.set(a.ID, badge, ok, now)
	return badge, ok
}

// computeBadge does the uncached manifest peek. Split out so the cache wraps
// only the expensive file open + gzip/tar scan.
func (c *Core) computeBadge(a store.Archive) (CompletenessBadge, bool) {
	p, err := c.Store.BlobPath(a.ID)
	if err != nil {
		return CompletenessBadge{}, false
	}
	f, err := os.Open(p)
	if err != nil {
		return CompletenessBadge{}, false
	}
	defer f.Close()
	total, success, failed, ok := PeekManifest(f)
	if !ok {
		return CompletenessBadge{}, false
	}
	return BadgeFromJobs(total, success, failed), true
}

// completenessCache memoizes completeness badges per archive id to avoid
// reopening and re-scanning the .tar.gz on every Captures render. The zero
// value is a cold, thread-safe cache. TTL bounds staleness so a badge
// converges within a minute if the underlying file changes.
type completenessCache struct {
	mu      sync.Mutex
	entries map[string]completenessEntry
}

type completenessEntry struct {
	badge  CompletenessBadge
	ok     bool
	filled time.Time
}

// CompletenessCacheTTL bounds staleness of a cached badge.
const CompletenessCacheTTL = 60 * time.Second

func (c *completenessCache) get(id string, now time.Time) (CompletenessBadge, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		return CompletenessBadge{}, false, false
	}
	e, ok := c.entries[id]
	if !ok || now.Sub(e.filled) >= CompletenessCacheTTL {
		return CompletenessBadge{}, false, false
	}
	return e.badge, e.ok, true
}

func (c *completenessCache) set(id string, badge CompletenessBadge, ok bool, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]completenessEntry)
	}
	c.entries[id] = completenessEntry{badge: badge, ok: ok, filled: now}
}

// SetCompletenessCacheForTest seeds or ages a cached badge (server package tests).
func (c *Core) SetCompletenessCacheForTest(id string, badge CompletenessBadge, ok bool, filled time.Time) {
	c.completenessCache.mu.Lock()
	defer c.completenessCache.mu.Unlock()
	if c.completenessCache.entries == nil {
		c.completenessCache.entries = make(map[string]completenessEntry)
	}
	c.completenessCache.entries[id] = completenessEntry{badge: badge, ok: ok, filled: filled}
}

func (c *completenessCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}

func BadgeFromJobs(total, success, failed int) CompletenessBadge {
	switch {
	case failed == 0:
		return CompletenessBadge{Label: "Complete", Tone: "ok"}
	case failed >= total:
		return CompletenessBadge{Label: "Failed", Tone: "err"}
	default:
		return CompletenessBadge{
			Label: fmt.Sprintf("%d of %d jobs failed", failed, total),
			Tone:  "warn",
		}
	}
}

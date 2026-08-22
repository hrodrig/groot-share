package server

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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

// completenessBadge is the per-row view-model fragment rendered next to the
// capture key. It is absent (false) unless a local archive exposes a valid
// manifest with job counters.
type completenessBadge struct {
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
func peekManifest(r io.Reader) (total, success, failed int, ok bool) {
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
// so the row stays unmarked — never an error, never log spam.
func (s *Server) completenessBadgeOf(a store.Archive) (completenessBadge, bool) {
	if a.Storage != "local" {
		return completenessBadge{}, false
	}
	p, err := s.Store.BlobPath(a.ID)
	if err != nil {
		return completenessBadge{}, false
	}
	f, err := os.Open(p)
	if err != nil {
		return completenessBadge{}, false
	}
	defer f.Close()
	total, success, failed, ok := peekManifest(f)
	if !ok {
		return completenessBadge{}, false
	}
	return badgeFromJobs(total, success, failed), true
}

func badgeFromJobs(total, success, failed int) completenessBadge {
	switch {
	case failed == 0:
		return completenessBadge{Label: "Complete", Tone: "ok"}
	case failed >= total:
		return completenessBadge{Label: "Failed", Tone: "err"}
	default:
		return completenessBadge{
			Label: fmt.Sprintf("%d of %d jobs failed", failed, total),
			Tone:  "warn",
		}
	}
}

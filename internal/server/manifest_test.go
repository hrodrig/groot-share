package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// grootArchive builds a .tar.gz whose only member is extras/manifest.json with
// the given raw JSON body. Placeholder other members can be appended via extra
// calls, but the manifest is the first member (as groot writes it early).
func grootArchive(t *testing.T, manifestJSON string, extra map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarMember(t, tw, "extras/manifest.json", []byte(manifestJSON))
	for name, body := range extra {
		writeTarMember(t, tw, name, []byte(body))
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeTarMember(t *testing.T, tw *tar.Writer, name string, body []byte) {
	t.Helper()
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
}

func TestPeekManifest(t *testing.T) {
	cases := []struct {
		name    string
		archive func() []byte
		total   int
		success int
		failed  int
		ok      bool
	}{
		{
			name: "well-formed partial",
			archive: func() []byte {
				return grootArchive(t, `{"groot_version":"1.0.0","archive_layout_version":1,"jobs":{"total":3,"success":2,"failed":1}}`, nil)
			},
			total: 3, success: 2, failed: 1, ok: true,
		},
		{
			name: "all failed",
			archive: func() []byte {
				return grootArchive(t, `{"jobs":{"total":1,"success":0,"failed":1}}`, nil)
			},
			total: 1, success: 0, failed: 1, ok: true,
		},
		{
			name: "suffix member name",
			archive: func() []byte {
				return rootArchiveWithManifestAt(t, "sub/extras/manifest.json", `{"jobs":{"total":2,"success":2,"failed":0}}`)
			},
			total: 2, success: 2, failed: 0, ok: true,
		},
		{
			name: "missing member",
			archive: func() []byte {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gz)
				writeTarMember(t, tw, "nodes/n1.log", []byte("hello"))
				_ = tw.Close()
				_ = gz.Close()
				return buf.Bytes()
			},
			ok: false,
		},
		{
			name: "empty jobs",
			archive: func() []byte {
				return grootArchive(t, `{"jobs":{}}`, nil)
			},
			ok: false,
		},
		{
			name: "failed exceeds total",
			archive: func() []byte {
				return grootArchive(t, `{"jobs":{"total":1,"success":0,"failed":2}}`, nil)
			},
			ok: false,
		},
		{
			name: "malformed json",
			archive: func() []byte {
				return grootArchive(t, `{"jobs":{"total":`, nil)
			},
			ok: false,
		},
		{
			name: "not gzip",
			archive: func() []byte {
				return []byte("this is not a gzip stream at all")
			},
			ok: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total, success, failed, ok := peekManifest(bytes.NewReader(tc.archive()))
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (total=%d success=%d failed=%d)", ok, tc.ok, total, success, failed)
			}
			if !ok {
				return
			}
			if total != tc.total || success != tc.success || failed != tc.failed {
				t.Fatalf("got (%d,%d,%d) want (%d,%d,%d)", total, success, failed, tc.total, tc.success, tc.failed)
			}
		})
	}
}

// rootArchiveWithManifestAt writes a single tar entry at an arbitrary path so
// the suffix match ("*/extras/manifest.json") is exercised end-to-end.
func rootArchiveWithManifestAt(t *testing.T, manifestPath, manifestJSON string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	writeTarMember(t, tw, manifestPath, []byte(manifestJSON))
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func TestBadgeFromJobs(t *testing.T) {
	cases := []struct {
		total, success, failed int
		label, tone            string
	}{
		{3, 3, 0, "Complete", "ok"},
		{1, 0, 1, "Failed", "err"},
		{3, 2, 1, "1 of 3 jobs failed", "warn"},
		{5, 1, 4, "4 of 5 jobs failed", "warn"},
	}
	for _, tc := range cases {
		b := badgeFromJobs(tc.total, tc.success, tc.failed)
		if b.Label != tc.label || b.Tone != tc.tone {
			t.Fatalf("badgeFromJobs(%d,%d,%d) = (%q,%q) want (%q,%q)",
				tc.total, tc.success, tc.failed, b.Label, b.Tone, tc.label, tc.tone)
		}
	}
}

func TestCompletenessBadgeOnHome(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)

	// A groot-shaped archive (partial failure) → badge should render.
	partial := grootArchive(t, `{"groot_version":"1.0.0","archive_layout_version":1,"jobs":{"total":4,"success":3,"failed":1}}`, nil)
	if _, err := s.Store.Ingest(context.Background(), bytes.NewReader(partial), "groot-partial.tar.gz", 1); err != nil {
		t.Fatal(err)
	}
	// A non-groot gzip (no tar manifest) → unmarked.
	plain := gzipOnly(t, []byte("just a gzip, not a groot archive"))
	if _, err := s.Store.Ingest(context.Background(), bytes.NewReader(plain), "plain-gzip.tar.gz", 1); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("home: %d", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, `1 of 4 jobs failed`) {
		t.Fatalf("badge missing for partial archive:\n%s", body)
	}
	if !strings.Contains(body, `tone-warn`) {
		t.Fatalf("warn tone missing:\n%s", body)
	}
	// The plain gzip row must not carry a badge next to its key.
	if strings.Contains(body, `plain-gzip.tar.gz <span class="completeness-badge`) {
		t.Fatalf("non-groot archive should be unmarked:\n%s", body)
	}
}

// gzipOnly returns gzip-compressed bytes with no tar wrapper — a file that is
// a valid gzip stream but not a groot archive.
func gzipOnly(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

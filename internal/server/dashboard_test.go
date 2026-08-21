package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/blob"
)

// dashboardArchive uploads a single archive with the given filename and
// returns the response. Body bytes are arbitrary; the gzip magic header
// passes the upload validation.
func dashboardArchive(t *testing.T, s *Server, ck *http.Cookie, name string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	gw := gzip.NewWriter(&body)
	_, _ = gw.Write([]byte("dashboard-test-" + name))
	_ = gw.Close()
	post := httptest.NewRequest(http.MethodPost, "/v1/archives", &body)
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("X-Gfs-Filename", name)
	post.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", name, rr.Code, rr.Body.String())
	}
	return rr
}

func TestHomeSummaryStripEmpty(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("home: %d", rr.Code)
	}
	body := rr.Body.String()
	// Strip must still render even with zero data so the operator sees
	// "0 captures · 0 B on disk" instead of an empty page.
	if !strings.Contains(body, `class="card summary"`) {
		t.Fatalf("summary strip missing: %s", body)
	}
	if !strings.Contains(body, `<span class="summary-num tabular">0</span>`) {
		t.Fatalf("zero count not rendered: %s", body)
	}
	if !strings.Contains(body, "captures</span>") {
		t.Fatalf("captures label missing: %s", body)
	}
	if !strings.Contains(body, `class="summary-lbl">clusters</span>`) {
		t.Fatalf("clusters label missing: %s", body)
	}
	if !strings.Contains(body, `class="summary-lbl">in transit</span>`) {
		t.Fatalf("transit label missing: %s", body)
	}
}

func TestHomeSummaryStripWithItems(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	// Three archives: two from one cluster, one from another.
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")
	dashboardArchive(t, s, admin, "groot-stage-20260823.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("home: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `>3</span>`) {
		t.Fatalf("count 3 not in body: %s", body)
	}
	// 2 distinct clusters: prod-eks-1 and stage.
	if !strings.Contains(body, `>2</span>`) {
		t.Fatalf("cluster count 2 not in body: %s", body)
	}
	// 0 in transit (all local).
	if !strings.Contains(body, `>0</span>`) {
		t.Fatalf("transit 0 not in body: %s", body)
	}
	// Topology pill: vps → pill-local.
	if !strings.Contains(body, `class="pill pill-local">vps`) {
		t.Fatalf("vps topology pill missing: %s", body)
	}
}

func TestHomeSummaryStripVPSS3Topology(t *testing.T) {
	s, _ := identServer(t)
	// Switch the server topology to vps-s3 and stub the blob store so
	// useBucket() reports true.
	s.Cfg.Topology = "vps-s3"
	s.Blobs = blob.NewMemory()
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-stage-20260821.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="pill pill-s3">vps-s3`) {
		t.Fatalf("vps-s3 topology pill missing: %s", body)
	}
}

func TestHomeSummaryStripUnparsedKeysCountZero(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	// Not a timestamped name → ParseClusterSlug returns false → 0 clusters.
	dashboardArchive(t, s, admin, "manual-upload.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	// Count = 1 but cluster count = 0.
	if !strings.Contains(body, `>1</span>`) {
		t.Fatalf("count 1 not in body: %s", body)
	}
	if !strings.Contains(body, `>0</span>`) {
		t.Fatalf("cluster count 0 not in body: %s", body)
	}
}

// keep the linter happy about unused imports if a future refactor removes
// the helper above; context and compress/gzip are used by dashboardArchive.
var _ = context.Background

func TestHomeUploadCTAVisibleToUploader(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="card upload-cta"`) {
		t.Fatalf("upload CTA card missing for uploader: %s", body)
	}
	if !strings.Contains(body, `href="/upload">Open upload form</a>`) {
		t.Fatalf("upload CTA link missing: %s", body)
	}
	// Size limit should be visible too.
	if !strings.Contains(body, "Up to") || !strings.Contains(body, "32.0 GiB") {
		t.Fatalf("upload size limit not visible: %s", body)
	}
}

func TestHomeUploadCTAHiddenFromViewer(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	ck := loginAs(t, s, "view", "view-secret-1")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, `class="card upload-cta"`) {
		t.Fatalf("upload CTA must be hidden from viewer: %s", body)
	}
}

package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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
	if !strings.Contains(body, `id="upload-inline"`) {
		t.Fatalf("inline upload form missing: %s", body)
	}
	if !strings.Contains(body, `id="inline-file"`) {
		t.Fatalf("inline upload file input missing: %s", body)
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

func TestHomePinStripHiddenWhenEmpty(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, `class="card pin-strip"`) {
		t.Fatalf("pin strip must be hidden when user has no pins: %s", body)
	}
}

func TestHomePinStripVisibleWhenPinned(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	// Upload two archives, pin both.
	rr1 := dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	rr2 := dashboardArchive(t, s, admin, "groot-stage-20260822.tar.gz")
	pinFromResp(t, s, admin, decodeArchiveID(t, rr1))
	pinFromResp(t, s, admin, decodeArchiveID(t, rr2))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="card pin-strip"`) {
		t.Fatalf("pin strip missing: %s", body)
	}
	if !strings.Contains(body, "groot-prod-eks-1-20260821.tar.gz") {
		t.Fatalf("first pinned key missing: %s", body)
	}
	if !strings.Contains(body, "groot-stage-20260822.tar.gz") {
		t.Fatalf("second pinned key missing: %s", body)
	}
	// Unpin form posts to the right path.
	if !strings.Contains(body, `action="/v1/pin/archives/`) {
		t.Fatalf("unpin form action wrong: %s", body)
	}
}

func TestHomePinStripUnpinFormRedirects(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	rr1 := dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	id := decodeArchiveID(t, rr1)
	pinFromResp(t, s, admin, id)

	// Confirm pin strip is present on Captures
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), `class="card pin-strip"`) {
		t.Fatalf("pin strip should be on the page after pinning")
	}

	// Unpin via the form-alias route
	del := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id+"/delete", nil)
	del.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("unpin form: %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("unpin form redirect: %q", loc)
	}

	// Confirm pin strip is gone from Captures
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if strings.Contains(rr.Body.String(), `class="card pin-strip"`) {
		t.Fatalf("pin strip should not be on the page after unpin")
	}
}

func decodeArchiveID(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var created struct {
		ID string `json:"id"`
	}
	if err := jsonDecode(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatalf("no id in upload response: %s", rr.Body.String())
	}
	return created.ID
}

func pinFromResp(t *testing.T, s *Server, ck *http.Cookie, id string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
	req.AddCookie(ck)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("pin: %d %s", rr.Code, rr.Body.String())
	}
}

func jsonDecode(b []byte, v any) error {
	return json.Unmarshal(b, v)
}

func TestHomeEmptyStateNoArchives(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "No captures yet") {
		t.Fatalf("missing 'No captures yet' empty state: %s", body)
	}
	if strings.Contains(body, "No matches") {
		t.Fatalf("'No matches' must not show when no archives exist: %s", body)
	}
	if strings.Contains(body, "Clear filters") {
		t.Fatalf("'Clear filters' must not show when no archives exist: %s", body)
	}
}

func TestHomeEmptyStateNoMatchWithFilters(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/?cluster=does-not-exist", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "No matches") {
		t.Fatalf("missing 'No matches' empty state with filters: %s", body)
	}
	if !strings.Contains(body, "Clear filters") {
		t.Fatalf("missing 'Clear filters' link with filters: %s", body)
	}
	if strings.Contains(body, "No captures yet") {
		t.Fatalf("'No captures yet' must not show when archives exist: %s", body)
	}
}

func TestHomeClearFiltersLinkGoesToRoot(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/?cluster=foo", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	// The "Clear filters" link is a plain <a href="/">; the link text is
	// "Clear filters" inside the .empty-sub paragraph.
	if !strings.Contains(body, "Clear filters") {
		t.Fatalf("Clear filters text missing: %s", body)
	}
	if !strings.Contains(body, `<a href="/">Clear filters</a>`) &&
		!strings.Contains(body, `<a href="/"`+` class="empty-clear">Clear filters</a>`) {
		t.Fatalf("Clear filters link does not point to /: %s", body)
	}
}

func TestHomeFilterBarHiddenWhenNoArchives(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, `class="filter-bar"`) {
		t.Fatalf("filter bar must be hidden when 0 archives: %s", body)
	}
}

func TestHomeFilterBarVisibleWithArchives(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")
	dashboardArchive(t, s, admin, "groot-stage-20260823.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="filter-bar"`) {
		t.Fatalf("filter bar must be visible with archives: %s", body)
	}
	// All-cluster chip is active by default
	if !strings.Contains(body, `chip is-active" href="/?">All `) {
		t.Fatalf("All-cluster chip should be active by default: %s", body)
	}
	// Cluster chips for the two distinct clusters
	if !strings.Contains(body, `>prod-eks-1 <span class="chip-count">2</span>`) {
		t.Fatalf("prod-eks-1 chip with count 2 missing: %s", body)
	}
	if !strings.Contains(body, `>stage <span class="chip-count">1</span>`) {
		t.Fatalf("stage chip with count 1 missing: %s", body)
	}
}

func TestHomeClusterFilterAppliesToList(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")
	dashboardArchive(t, s, admin, "groot-stage-20260823.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/?cluster=prod-eks-1", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "groot-prod-eks-1-20260821.tar.gz") {
		t.Fatalf("prod 0821 missing: %s", body)
	}
	if !strings.Contains(body, "groot-prod-eks-1-20260822.tar.gz") {
		t.Fatalf("prod 0822 missing: %s", body)
	}
	if strings.Contains(body, "groot-stage-20260823.tar.gz") {
		t.Fatalf("stage must be filtered out: %s", body)
	}
}

func TestHomeQueryFilterAppliesToList(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")
	dashboardArchive(t, s, admin, "groot-stage-20260823.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/?q=20260822", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "groot-prod-eks-1-20260822.tar.gz") {
		t.Fatalf("q=20260822 should match 0822: %s", body)
	}
	if strings.Contains(body, "groot-prod-eks-1-20260821.tar.gz") {
		t.Fatalf("q=20260822 must not match 0821: %s", body)
	}
}

func TestHomeWindowFilterAppliesToList(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	// Two archives, but the test only cares that the filter does not
	// blank the page when both are within 24h.
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260821.tar.gz")
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/?window=24h", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	// Both archives have CreatedAt=now, so both should appear under 24h.
	if !strings.Contains(body, "groot-prod-eks-1-20260821.tar.gz") {
		t.Fatalf("window=24h should include now's archive: %s", body)
	}
	// Window chip should be marked active.
	if !strings.Contains(body, `class="chip chip-sm is-active" href="/?window=24h"`) {
		t.Fatalf("24h window chip should be active: %s", body)
	}
}

func TestHomeArchiveCardsVisibleToUploader(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="archive-cards"`) {
		t.Fatalf("archive cards list missing: %s", body)
	}
	if !strings.Contains(body, `class="archive-card"`) {
		t.Fatalf("archive card missing: %s", body)
	}
	// Download is a primary button on the card.
	if !strings.Contains(body, `class="btn" href="/v1/archives/`) ||
		!strings.Contains(body, `>Download</a>`) {
		t.Fatalf("card Download primary action missing: %s", body)
	}
	// Copy-link action is preserved on the card.
	if !strings.Contains(body, `data-copy-url=`) {
		t.Fatalf("card copy-link action missing: %s", body)
	}
	// Admin (CanDelete) gets a delete form on the card.
	if !strings.Contains(body, `data-confirm="Delete groot-prod-eks-1-20260822.tar.gz? This cannot be undone."`) {
		t.Fatalf("card delete action missing for admin: %s", body)
	}
}

func TestHomeArchiveCardsNoDeleteForViewer(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	// A viewer cannot upload, so seed an archive as admin first.
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "groot-prod-eks-1-20260822.tar.gz")

	ck := loginAs(t, s, "view", "view-secret-1")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `class="archive-cards"`) {
		t.Fatalf("viewer should still see archive cards: %s", body)
	}
	if strings.Contains(body, `data-confirm="Delete groot-prod-eks-1-20260822.tar.gz? This cannot be undone."`) {
		t.Fatalf("viewer must not see a card delete action: %s", body)
	}
}

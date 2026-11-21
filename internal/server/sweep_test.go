package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestDeleteArchiveAPI(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "gone.tar.gz", "delete-me")
	req := httptest.NewRequest(http.MethodDelete, "/v1/archives/"+created.ID, nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.String())
	}
	list := authedGET(t, s, ck, "/v1/archives")
	if strings.Contains(list.Body.String(), created.ID) {
		t.Fatalf("still listed: %s", list.Body.String())
	}
	audit := authedGET(t, s, ck, "/v1/audit")
	if !strings.Contains(audit.Body.String(), `"action":"delete"`) {
		t.Fatalf("audit %s", audit.Body.String())
	}
}

func TestRetentionSkipsPinnedArchive(t *testing.T) {
	// #45: an archive with a pin must survive retention regardless of its age
	// or position relative to keep_last.
	s, _ := identServer(t)
	s.Cfg.KeepLast = 1
	s.Cfg.MaxAgeDays = 0
	ck := loginCookie(t, s)
	pinned := postArchive(t, s, ck, "a.tar.gz", "oldest")
	postArchive(t, s, ck, "b.tar.gz", "two")
	postArchive(t, s, ck, "c.tar.gz", "three")

	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+pinned.ID, nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("pin %d %s", rr.Code, rr.Body.String())
	}

	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	if !strings.Contains(list.Body.String(), pinned.ID) {
		t.Fatalf("pinned archive must be retained: %s", list.Body.String())
	}
}

func TestRetentionSkipsSharedArchive(t *testing.T) {
	// #45: an archive with an active share link must survive retention.
	s, _ := identServer(t)
	s.Cfg.KeepLast = 1
	s.Cfg.MaxAgeDays = 0
	ck := loginCookie(t, s)
	shared := postArchive(t, s, ck, "a.tar.gz", "oldest")
	postArchive(t, s, ck, "b.tar.gz", "two")
	postArchive(t, s, ck, "c.tar.gz", "three")

	createShare(t, s, ck, shared.ID, `{"expires_in":"24h"}`)

	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	if !strings.Contains(list.Body.String(), shared.ID) {
		t.Fatalf("shared archive must be retained: %s", list.Body.String())
	}
}

func TestRetentionKeepLast(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.KeepLast = 2
	s.Cfg.MaxAgeDays = 90
	ck := loginCookie(t, s)
	postArchive(t, s, ck, "a.tar.gz", "one")
	postArchive(t, s, ck, "b.tar.gz", "two")
	postArchive(t, s, ck, "c.tar.gz", "three")
	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	var body struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("want 2 got %d %s", len(body.Items), list.Body.String())
	}
	audit := authedGET(t, s, ck, "/v1/audit")
	if !strings.Contains(audit.Body.String(), `"actor":"retention"`) {
		t.Fatalf("audit %s", audit.Body.String())
	}
}

func TestRetentionKeepLastPerCluster(t *testing.T) {
	// #53: keep_last applies per ParseClusterSlug, not over the flat list.
	s, _ := identServer(t)
	s.Cfg.KeepLast = 1
	s.Cfg.MaxAgeDays = 0
	ck := loginCookie(t, s)
	// Upload oldest first so CreatedAt rank matches filename intent.
	pOld := postArchive(t, s, ck, "groot-capture-ccccccc-20260101-150405-prod.tar.gz", "p-old")
	sOld := postArchive(t, s, ck, "groot-capture-ddddddd-20260101-140405-staging.tar.gz", "s-old")
	pNew := postArchive(t, s, ck, "groot-capture-aaaaaaa-20260102-150405-prod.tar.gz", "p-new")
	sNew := postArchive(t, s, ck, "groot-capture-bbbbbbb-20260102-140405-staging.tar.gz", "s-new")
	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	body := list.Body.String()
	if !strings.Contains(body, pNew.ID) || !strings.Contains(body, sNew.ID) {
		t.Fatalf("newest per cluster must remain: %s", body)
	}
	if strings.Contains(body, pOld.ID) || strings.Contains(body, sOld.ID) {
		t.Fatalf("older per cluster must be deleted: %s", body)
	}
}

func TestPinOnIngestSurvivesRetention(t *testing.T) {
	// #53: X-GFS-Pin on HTTP upload pins for the uploader (fail-open path tested elsewhere).
	s, st := identServer(t)
	s.Cfg.KeepLast = 1
	s.Cfg.MaxAgeDays = 0
	ck := loginCookie(t, s)

	req := httptest.NewRequest(http.MethodPost, "/v1/archives", gzipPayload(t, []byte("evidence")))
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Gfs-Filename", "evidence.tar.gz")
	req.Header.Set("X-GFS-Pin", "1")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var pinned createdArchive
	if err := json.Unmarshal(rr.Body.Bytes(), &pinned); err != nil || pinned.ID == "" {
		t.Fatalf("body %s", rr.Body.String())
	}
	postArchive(t, s, ck, "newer.tar.gz", "cron-noise")
	postArchive(t, s, ck, "newest.tar.gz", "more-noise")

	u, err := st.UserByUsername(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}
	pins, err := st.ListPins(context.Background(), u.ID, 10)
	if err != nil || len(pins) == 0 || pins[0].ArchiveID != pinned.ID {
		t.Fatalf("expected pin for %s: %+v %v", pinned.ID, pins, err)
	}

	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	if !strings.Contains(list.Body.String(), pinned.ID) {
		t.Fatalf("pin-on-ingest archive must survive: %s", list.Body.String())
	}
}

func TestRetentionMaxAgeVPSS3(t *testing.T) {
	s, mem := vpsS3Server(t)
	s.Cfg.KeepLast = 20
	s.Cfg.MaxAgeDays = 90
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "old.tar.gz", "aged")
	mem.Touch(created.ID, time.Now().UTC().Add(-200*24*time.Hour))
	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := mem.Head(context.Background(), created.ID); err == nil {
		t.Fatal("expected bucket object gone")
	}
}

func TestSweepPurgesExpiredSessions(t *testing.T) {
	s, st := identServer(t)
	ctx := context.Background()
	u, err := st.UserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}
	_, dead, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, u.ID, dead, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserBySessionHash(ctx, dead); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired session still present: %v", err)
	}
}

func TestStagingGraceSweep(t *testing.T) {
	s, mem := vpsS3Server(t)
	mem.FailPuts = true
	s.Cfg.StagingGrace = time.Millisecond
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "stuck.tar.gz", "pending")
	if created.Storage != "transit" {
		t.Fatalf("%+v", created)
	}
	time.Sleep(5 * time.Millisecond)
	if err := s.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertStagingEmpty(t, s)
}

func TestGetDeletePathNotFound(t *testing.T) {
	// B-14c: a GET to ".../delete" must not serve archive bytes.
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "secret.tar.gz", "secret-bytes")

	for _, path := range []string{
		"/v1/archives/delete/" + created.ID,
		"/v1/archives/" + created.ID + "/file/delete",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("GET %s: want 404, got %d (body %q)", path, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), "secret-bytes") {
			t.Fatalf("GET %s leaked archive bytes", path)
		}
	}
}

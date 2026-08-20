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
		"/v1/archives/" + created.ID + "/delete",
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

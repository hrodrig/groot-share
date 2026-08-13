package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestDeleteBucketArchive(t *testing.T) {
	s, mem := vpsS3Server(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "gone.tar.gz", "bucket-delete")
	req := httptest.NewRequest(http.MethodDelete, "/v1/archives/"+created.ID, nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.String())
	}
	if _, err := mem.Head(context.Background(), created.ID); err == nil {
		t.Fatal("object still in bucket")
	}
}

func TestDeleteTransitArchiveHTML(t *testing.T) {
	s, mem := vpsS3Server(t)
	mem.FailPuts = true
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "transit.tar.gz", "in-transit")
	req := httptest.NewRequest(http.MethodPost, "/v1/archives/"+created.ID+"/delete", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=deleted") {
		t.Fatalf("html delete %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestUsersAPIEdgeCases(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	viewer := loginAs(t, s, "view", "view-secret-1")

	get := httptest.NewRequest(http.MethodGet, "/v1/users/99999", nil)
	get.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, get)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get missing %d", rr.Code)
	}

	del := httptest.NewRequest(http.MethodDelete, "/v1/users/99999", nil)
	del.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("delete missing %d", rr.Code)
	}

	patch := httptest.NewRequest(http.MethodPatch, "/v1/users/1", strings.NewReader(`{"role":"root"}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, patch)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad role %d", rr.Code)
	}

	listUsers := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	listUsers.AddCookie(viewer)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, listUsers)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer list users %d", rr.Code)
	}
}

func TestRequestBaseURLForwardedProto(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "gfs.example.com"
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := requestBaseURL(r); got != "https://gfs.example.com" {
		t.Fatalf("base url %q", got)
	}
}

func TestRequestBaseURLHTTP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "localhost:8080"
	if got := requestBaseURL(r); got != "http://localhost:8080" {
		t.Fatalf("base url %q", got)
	}
}

func TestRequestBaseURLTLS(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "gfs.example.com"
	r.TLS = &tls.ConnectionState{}
	if got := requestBaseURL(r); got != "https://gfs.example.com" {
		t.Fatalf("base url %q", got)
	}
}

func TestAdminAndSettingsNotices(t *testing.T) {
	for token, want := range map[string]string{
		"created":     "User created.",
		"updated":     "User updated.",
		"deactivated": "User deactivated.",
		"last_admin":  "last active admin",
		"error":       "That action failed",
	} {
		kind, text := adminNotice(token)
		if kind == "" || !strings.Contains(text, want) {
			t.Fatalf("admin %q: %q %q", token, kind, text)
		}
	}
	for token, want := range map[string]string{
		"password": "Password updated.",
		"revoked":  "API key revoked.",
		"error":    "That action failed",
	} {
		kind, text := settingsNotice(token)
		if kind == "" || !strings.Contains(text, want) {
			t.Fatalf("settings %q: %q %q", token, kind, text)
		}
	}
}

func TestNoticeCopyAndLoginErrors(t *testing.T) {
	for token, want := range map[string]string{
		"deleted":      "deleted",
		"upload_error": "Upload failed",
		"too_large":    "size limit",
	} {
		kind, text := noticeCopy(token)
		if kind == "" || !strings.Contains(text, want) {
			t.Fatalf("notice %q: %q %q", token, kind, text)
		}
	}
	for code, want := range map[string]string{
		"bad_request": "Enter your username",
		"not_ready":   "not ready",
		"other":       "Sign-in failed",
	} {
		if got := loginErrorCopy(code); !strings.Contains(got, want) {
			t.Fatalf("login %q: %q", code, got)
		}
	}
}

func TestSortArchivesBySourceAndStorage(t *testing.T) {
	now := time.Now().UTC()
	items := []store.Archive{
		{Key: "b.tar.gz", Source: "s3", Storage: "s3", CreatedAt: now},
		{Key: "a.tar.gz", Source: "http", Storage: "local", CreatedAt: now.Add(-time.Hour)},
	}
	sortArchives(items, "source", true)
	if items[0].Source != "http" {
		t.Fatalf("source sort %v", items)
	}
	sortArchives(items, "storage", true)
	if items[0].Storage != "local" {
		t.Fatalf("storage sort %v", items)
	}
	sortArchives(items, "uploaded", false)
	if !items[0].CreatedAt.After(items[1].CreatedAt) {
		t.Fatalf("uploaded sort %v", items)
	}
}

func TestIsMaxBytes(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.MaxUploadBytes = 4
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader([]byte("12345")))
	req.Header.Set("Content-Type", "application/gzip")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large %d", rr.Code)
	}
}

func TestActivityAndAuditPages(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	postArchive(t, s, ck, "audit.tar.gz", "audit-body")
	act := httptest.NewRequest(http.MethodGet, "/activity", nil)
	act.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, act)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Activity") {
		t.Fatalf("activity %d", rr.Code)
	}
	audit := httptest.NewRequest(http.MethodGet, "/v1/audit?page=1&per_page=10", nil)
	audit.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, audit)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"action":"upload"`) {
		t.Fatalf("audit api %d %s", rr.Code, rr.Body.String())
	}
}

func TestHomePageSortAndPager(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	postArchive(t, s, ck, "a.tar.gz", "aaa")
	postArchive(t, s, ck, "b.tar.gz", "bbb")
	home := httptest.NewRequest(http.MethodGet, "/?sort=key&order=asc&page=1&per_page=50", nil)
	home.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "b.tar.gz") {
		t.Fatalf("home sort %d", rr.Code)
	}
}

func TestUploadPageWithNotice(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/upload?notice=deleted", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "deleted") {
		t.Fatalf("upload notice %d", rr.Code)
	}
}

func TestAdminCreateUserInvalidRoleHTML(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	form := strings.NewReader("username=bob&password=bob-secret-1&role=root")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/create", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=error") {
		t.Fatalf("invalid role %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestParseUserIDBad(t *testing.T) {
	if _, err := parseUserID(""); err == nil {
		t.Fatal("empty id")
	}
	if _, err := parseUserID("nope"); err == nil {
		t.Fatal("non-numeric id")
	}
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/store"
)

func TestResolveArchiveVPSLocalID(t *testing.T) {
	s, _ := identServer(t)
	// Local vps topology, id is 32-hex. Seed an archive via the upload
	// path so the store has it.
	ck := sessionCookieFor(t, s)
	_ = postArchive(t, s, ck, "local.tar.gz", "hello")

	// Look up via the captured id.
	items := listArchivesJSON(t, s, ck)
	if len(items) == 0 {
		t.Fatal("no archives")
	}
	id := items[0].ID
	got, err := s.resolveArchive(context.Background(), id)
	if err != nil {
		t.Fatalf("resolveArchive: %v", err)
	}
	if got.ID != id {
		t.Fatalf("ID: %q want %q", got.ID, id)
	}
}

func TestResolveArchiveVPSRejectsNonHex(t *testing.T) {
	s, _ := identServer(t)
	_, err := s.resolveArchive(context.Background(), "not-32-hex")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err: %v want ErrNotFound", err)
	}
}

func TestResolveArchiveVPSRejectsTraversal(t *testing.T) {
	s, _ := identServer(t)
	_, err := s.resolveArchive(context.Background(), "../etc/passwd")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err: %v want ErrNotFound", err)
	}
}

func TestResolveArchiveVPSS3HeadOK(t *testing.T) {
	s, mem := vpsS3Server(t)
	key := "captures/2026-01-01-foo.tar.gz"
	if err := mem.Put(context.Background(), key, strings.NewReader("payload")); err != nil {
		t.Fatal(err)
	}
	got, err := s.resolveArchive(context.Background(), key)
	if err != nil {
		t.Fatalf("resolveArchive: %v", err)
	}
	// objectArchive sets ID = the S3 key, Key = filepath.Base(key).
	if got.ID != key {
		t.Fatalf("ID: %q want %q (S3 key)", got.ID, key)
	}
	if got.Key != "2026-01-01-foo.tar.gz" {
		t.Fatalf("Key: %q want basename", got.Key)
	}
	if got.Storage == "" {
		t.Fatal("Storage unset")
	}
}

func TestResolveArchiveVPSS3OutOfPrefix(t *testing.T) {
	s, mem := vpsS3Server(t)
	// Put the object in the bucket but outside the configured prefix.
	if err := mem.Put(context.Background(), "other/2026.tar.gz", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	_, err := s.resolveArchive(context.Background(), "other/2026.tar.gz")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err: %v want ErrNotFound", err)
	}
}

func TestResolveArchiveEmptyID(t *testing.T) {
	s, _ := identServer(t)
	_, err := s.resolveArchive(context.Background(), "")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err: %v want ErrNotFound", err)
	}
}

// listArchivesJSON returns the items from GET /v1/archives.
type listArchiveItem struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func listArchivesJSON(t *testing.T, s *Server, ck *http.Cookie) []listArchiveItem {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/archives", nil)
	if ck != nil {
		req.AddCookie(ck)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code < 200 || rr.Code >= 300 {
		t.Fatalf("list archives: %d %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Items []listArchiveItem `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Items
}

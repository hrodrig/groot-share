package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/blob"
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/store"
)

func vpsS3Server(t *testing.T) (*Server, *blob.Memory) {
	t.Helper()
	s, _ := identServer(t)
	mem := blob.NewMemory()
	s.Cfg.Topology = config.TopologyVPSS3
	s.Cfg.S3Prefix = "captures/"
	s.Blobs = mem
	s.Ready = func() bool { return mem.HeadBucket(context.Background()) == nil }
	return s, mem
}

type createdArchive struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Source  string `json:"source"`
	Storage string `json:"storage"`
}

func postArchive(t *testing.T, s *Server, ck *http.Cookie, name, body string) createdArchive {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Gfs-Filename", name)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var created createdArchive
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("body %s", rr.Body.String())
	}
	return created
}

func authedGET(t *testing.T, s *Server, ck *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func assertStagingEmpty(t *testing.T, s *Server) {
	t.Helper()
	entries, err := os.ReadDir(s.Store.StagingDir())
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging leftover %+v %v", entries, err)
	}
}

func TestVPSS3UploadListsFromPrefix(t *testing.T) {
	s, mem := vpsS3Server(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "run.tar.gz", "bucket-home-bytes")
	if created.Storage != "s3" || created.Source != "http" || created.Key != "run.tar.gz" {
		t.Fatalf("%+v", created)
	}
	if blob.SourceForKey(created.ID) != "http" {
		t.Fatalf("id %s", created.ID)
	}
	assertStagingEmpty(t, s)
	list := authedGET(t, s, ck, "/v1/archives")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), created.ID) {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	dl := authedGET(t, s, ck, "/v1/archives/"+created.ID+"/file")
	if dl.Code != http.StatusOK || dl.Body.String() != "bucket-home-bytes" {
		t.Fatalf("dl %d %q", dl.Code, dl.Body.String())
	}
	objs, err := mem.List(context.Background(), "captures/")
	if err != nil || len(objs) != 1 {
		t.Fatalf("mem list %+v %v", objs, err)
	}
}

func TestVPSS3TransitRetry(t *testing.T) {
	s, mem := vpsS3Server(t)
	mem.FailPuts = true
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "stuck.tar.gz", "stuck-in-transit")
	if created.Storage != "transit" {
		t.Fatalf("%+v", created)
	}
	list := authedGET(t, s, ck, "/v1/archives")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), created.ID) {
		t.Fatalf("staging must not be listed: %s", list.Body.String())
	}
	dl := authedGET(t, s, ck, "/v1/archives/"+created.ID+"/file")
	if dl.Code != http.StatusOK || dl.Body.String() != "stuck-in-transit" {
		t.Fatalf("transit dl %d %q", dl.Code, dl.Body.String())
	}
	mem.FailPuts = false
	if err := s.RetryOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	list = authedGET(t, s, ck, "/v1/archives")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), created.ID) {
		t.Fatalf("after retry list %s", list.Body.String())
	}
	assertStagingEmpty(t, s)
}

func TestVPSS3ForeignKeyList(t *testing.T) {
	s, mem := vpsS3Server(t)
	if err := mem.Put(context.Background(), "captures/cluster-run.tar.gz", strings.NewReader("from-groot")); err != nil {
		t.Fatal(err)
	}
	ck := loginCookie(t, s)
	list := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	list.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `"source":"s3"`) || !strings.Contains(body, "cluster-run.tar.gz") {
		t.Fatalf("list %d %s", rr.Code, body)
	}

	dl := httptest.NewRequest(http.MethodGet, "/v1/archives/captures/cluster-run.tar.gz/file", nil)
	dl.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusOK || rr.Body.String() != "from-groot" {
		t.Fatalf("dl %d %q", rr.Code, rr.Body.String())
	}

	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "cluster-run.tar.gz") {
		t.Fatalf("home %d %s", rr.Code, rr.Body.String())
	}
}

func TestReadyzHeadBucket(t *testing.T) {
	s, mem := vpsS3Server(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("ready %d", rr.Code)
	}
	mem.FailHeadBucket = true
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("not ready %d", rr.Code)
	}
}

func TestOpenDownloadRejectsDotDot(t *testing.T) {
	s, _ := vpsS3Server(t)
	_, _, err := s.openDownload(context.Background(), "foo/../secrets")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestVPSS3UploadDuplicate(t *testing.T) {
	s, _ := vpsS3Server(t)
	ck := loginCookie(t, s)
	payload := "bucket-dup-bytes"
	postArchive(t, s, ck, "run.tar.gz", payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Gfs-Filename", "run.tar.gz")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate code %d %s", rr.Code, rr.Body.String())
	}
	assertStagingEmpty(t, s)
}

func TestStagingDirUnusedOnVPSSuccess(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", strings.NewReader("local-home"))
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated || !strings.Contains(rr.Body.String(), `"storage":"local"`) {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
}

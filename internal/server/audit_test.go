package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuditUploadDownload(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", gzipPayload(t, []byte("audit-bytes")))
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Gfs-Filename", "run.tar.gz")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	dl := httptest.NewRequest(http.MethodGet, "/v1/archives/"+created.ID+"/file", nil)
	dl.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusOK {
		t.Fatalf("dl %d", rr.Code)
	}

	list := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	list.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("audit %d %s", rr.Code, body)
	}
	if !strings.Contains(body, `"action":"upload"`) || !strings.Contains(body, `"action":"download"`) {
		t.Fatalf("missing actions: %s", body)
	}
	if !strings.Contains(body, `"total":`) || !strings.Contains(body, `"page":`) {
		t.Fatalf("pagination fields missing: %s", body)
	}
	if strings.Contains(body, "correct-horse") || strings.Contains(body, `"gfs_`) {
		t.Fatalf("secrets in audit: %s", body)
	}
}

func TestAuditUnauthorized(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/audit", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rr.Code)
	}
}

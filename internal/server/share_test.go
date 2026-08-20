package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func createShare(t *testing.T, s *Server, ck *http.Cookie, archiveID, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/archives/"+archiveID+"/shares", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create share %d %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %s", rr.Body.String())
	}
	return out
}

func tokenFromURL(u string) string {
	i := strings.LastIndex(u, "/s/")
	if i < 0 {
		return ""
	}
	return u[i+3:]
}

func TestShareCreateAndPublicDownload(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	out := createShare(t, s, ck, created.ID, `{"expires_in":"24h"}`)
	url, _ := out["url"].(string)
	token := tokenFromURL(url)
	if token == "" {
		t.Fatalf("url %q", url)
	}

	// Public download: no cookie, must succeed and stream the gzip bytes.
	req := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("download %d %s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != string(gzipBytes(t, []byte("vendor-bytes"))) {
		t.Fatalf("body %q", rr.Body.String())
	}
}

func TestShareListNeverLeaksToken(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	createShare(t, s, ck, created.ID, `{"expires_in":"24h","label":"acme"}`)

	req := httptest.NewRequest(http.MethodGet, "/v1/archives/"+created.ID+"/shares", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "token") {
		t.Fatalf("list leaked token: %s", rr.Body.String())
	}
}

func TestShareUnknownToken404(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodGet, "/s/deadbeef", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown token %d", rr.Code)
	}
}

func TestSharePastExpiryRejected(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	req := httptest.NewRequest(http.MethodPost, "/v1/archives/"+created.ID+"/shares",
		strings.NewReader(`{"expires_at":"2020-01-01T00:00:00Z"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("past expiry should 400, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestShareRevokeThen404(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	out := createShare(t, s, ck, created.ID, `{"expires_in":"24h"}`)
	shareID := out["id"].(float64)
	token := tokenFromURL(out["url"].(string))

	req := httptest.NewRequest(http.MethodDelete, "/v1/archives/"+created.ID+"/shares/"+formatID(shareID), nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke %d %s", rr.Code, rr.Body.String())
	}

	dl := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("revoked download %d", rr.Code)
	}
}

func TestShareOneShotExhausts(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	out := createShare(t, s, ck, created.ID, `{"expires_in":"24h","max_uses":1}`)
	token := tokenFromURL(out["url"].(string))

	dl := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusOK {
		t.Fatalf("first download %d", rr.Code)
	}
	dl2 := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl2)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("second download should 404, got %d", rr.Code)
	}
}

func TestShareNonAdminForbidden(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	created := postArchive(t, s, admin, "vendor.tar.gz", "vendor-bytes")

	createUserWithRole(t, st, "up", "up-secret-12", auth.RoleUploader)
	upler := loginAs(t, s, "up", "up-secret-12")

	// Uploader cannot create.
	req := httptest.NewRequest(http.MethodPost, "/v1/archives/"+created.ID+"/shares",
		strings.NewReader(`{"expires_in":"24h"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(upler)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader create share %d %s", rr.Code, rr.Body.String())
	}

	// Uploader cannot list.
	req = httptest.NewRequest(http.MethodGet, "/v1/archives/"+created.ID+"/shares", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(upler)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader list share %d", rr.Code)
	}
}

func formatID(id float64) string {
	return strconv.FormatInt(int64(id), 10)
}

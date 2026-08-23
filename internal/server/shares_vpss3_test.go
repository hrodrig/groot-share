package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// TestSharesVPSS3EndToEnd exercises the vps-s3 share flows with an S3
// key containing '/'. Before the route restructure the S3 key never
// matched the share routes (single-segment {id}); the page returned
// 404 and the API fell through to the catch-all.
func TestSharesVPSS3EndToEnd(t *testing.T) {
	s, mem := vpsS3Server(t)
	s3Key := "captures/2026-01-01-foo.tar.gz"
	if err := mem.Put(t.Context(), s3Key, strings.NewReader("payload")); err != nil {
		t.Fatal(err)
	}
	ck := loginCookie(t, s)

	// 1. List on a fresh archive: empty list, 200.
	listReq := httptest.NewRequest(http.MethodGet, "/v1/shares/"+s3Key, nil)
	listReq.Header.Set("Accept", "application/json")
	listReq.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, listReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("list empty: %d %s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) != 0 {
		t.Fatalf("expected empty list, got %d", len(listResp.Items))
	}

	// 2. Create a share: 201 with token URL.
	createReq := httptest.NewRequest(http.MethodPost, "/v1/shares/"+s3Key,
		strings.NewReader(`{"expires_in":"24h","label":"acme"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Accept", "application/json")
	createReq.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, createReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	shareID := int64(created["id"].(float64))
	if shareID <= 0 {
		t.Fatalf("share id: %v", created["id"])
	}
	urlStr, _ := created["url"].(string)
	if !strings.Contains(urlStr, "/s/") {
		t.Fatalf("url missing /s/: %q", urlStr)
	}

	// 3. List again: exactly 1 link, no token leak.
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, listReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("list after create: %d", rr.Code)
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("list size: %d body=%q", len(listResp.Items), rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "token") {
		t.Fatalf("list leaked token: %s", rr.Body.String())
	}

	// 4. Revoke by share id alone (no archive id in the path).
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/shares/"+strconv.FormatInt(shareID, 10), nil)
	delReq.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, delReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rr.Code, rr.Body.String())
	}

	// 5. Public download via /s/{token} now yields 410 Gone (revoked link).
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	dlReq := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dlReq)
	if rr.Code != http.StatusGone {
		t.Fatalf("revoked download: %d %s", rr.Code, rr.Body.String())
	}
}

// TestSharesVPSS3HTMLPage covers the HTML page route for an S3 key
// containing '/'. The browser-facing path uses url.PathEscape on the
// id so the slash in the S3 key is sent as %2F.
func TestSharesVPSS3HTMLPage(t *testing.T) {
	s, mem := vpsS3Server(t)
	s3Key := "captures/2026-01-01-bar.tar.gz"
	if err := mem.Put(t.Context(), s3Key, strings.NewReader("payload")); err != nil {
		t.Fatal(err)
	}
	ck := loginCookie(t, s)

	page := httptest.NewRequest(http.MethodGet, "/shares/"+url.PathEscape(s3Key), nil)
	page.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, page)
	if rr.Code != http.StatusOK {
		t.Fatalf("html page: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Share links", "Create share link", s3Key} {
		if !strings.Contains(body, want) {
			t.Fatalf("html page missing %q", want)
		}
	}
}

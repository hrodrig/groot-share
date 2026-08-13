package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func loginCookie(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	body := strings.NewReader(`{"username":"root","password":"correct-horse"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login %d %s", rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no cookie")
	return nil
}

func TestUploadListDownload(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.MaxUploadBytes = 1 << 20
	ck := loginCookie(t, s)
	payload := []byte("groot-tar-bytes")
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader(payload))
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
		ID     string `json:"id"`
		Key    string `json:"key"`
		Source string `json:"source"`
		Size   int64  `json:"size"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.ID == "" || created.Key != "run.tar.gz" || created.Source != "http" {
		t.Fatalf("body %s", rr.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	list.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), created.ID) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}

	dl := httptest.NewRequest(http.MethodGet, "/v1/archives/"+created.ID+"/file", nil)
	dl.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusOK || rr.Body.String() != string(payload) {
		t.Fatalf("dl %d %q", rr.Code, rr.Body.String())
	}

	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "run.tar.gz") {
		t.Fatalf("home %d %s", rr.Code, rr.Body.String())
	}
}

func TestUploadMultipart(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "job.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(fw, "part-bytes"); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated || !strings.Contains(rr.Body.String(), "job.tar.gz") {
		t.Fatalf("mp %d %s", rr.Code, rr.Body.String())
	}
}

func TestUploadUnauthorized(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", strings.NewReader("x"))
	req.Header.Set("Content-Type", "application/gzip")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rr.Code)
	}
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
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
	gz := gzipBytes(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader(gz))
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
	if rr.Code != http.StatusOK || rr.Body.String() != string(gz) {
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

func TestUploadBrowserFormRedirect(t *testing.T) {
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
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("code %d want 303", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/upload?notice=uploaded&name=job.tar.gz" {
		t.Fatalf("location %q", loc)
	}
}

func TestUploadDuplicateJSON(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	payload := []byte("dup-bytes")
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/archives", gzipPayload(t, payload))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Gfs-Filename", "run.tar.gz")
		req.Header.Set("Accept", "application/json")
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr
	}
	if rr := post(); rr.Code != http.StatusCreated {
		t.Fatalf("first upload %d %s", rr.Code, rr.Body.String())
	}
	rr := post()
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate code %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"error":"duplicate"`) {
		t.Fatalf("body %s", rr.Body.String())
	}
	list := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	list.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("list %d items=%d err=%v", rr.Code, len(out.Items), err)
	}
}

func TestUploadDuplicateBrowserForm(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	post := func() *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", "job.tar.gz")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, "same-part"); err != nil {
			t.Fatal(err)
		}
		_ = mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/v1/archives", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr
	}
	if rr := post(); rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/upload?notice=uploaded&name=job.tar.gz" {
		t.Fatalf("first %d loc=%q", rr.Code, rr.Header().Get("Location"))
	}
	rr := post()
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/upload?notice=duplicate&name=job.tar.gz" {
		t.Fatalf("dup %d loc=%q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestUploadDuplicateMultipartJSON(t *testing.T) {
	// The inline dropzone sends multipart + Accept: application/json (XHR),
	// which must hit the JSON branch (409 + {"error":"duplicate"}) — not the
	// browser-form redirect — so the inline UI can render an inline notice.
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	post := func() *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", "inline.tar.gz")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, "same-content"); err != nil {
			t.Fatal(err)
		}
		_ = mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/v1/archives", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Accept", "application/json")
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		return rr
	}
	if rr := post(); rr.Code != http.StatusCreated {
		t.Fatalf("first upload %d %s", rr.Code, rr.Body.String())
	}
	rr := post()
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate code %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"error":"duplicate"`) || !strings.Contains(rr.Body.String(), `"existing"`) {
		t.Fatalf("expected JSON duplicate body, got %s", rr.Body.String())
	}
}

func TestIsBrowserForm(t *testing.T) {
	cases := []struct {
		name   string
		accept string
		ctype  string
		isForm bool
	}{
		{"multipart no accept", "", "multipart/form-data; boundary=x", true},
		{"multipart accept json", "application/json", "multipart/form-data; boundary=x", false},
		{"multipart accept json with charset", "application/json; charset=utf-8", "multipart/form-data; boundary=x", false},
		{"multipart accepts html text", "text/html", "multipart/form-data; boundary=x", true},
		{"raw gzip accepts json", "application/json", "application/gzip", false},
		{"raw gzip no accept", "", "application/gzip", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/archives", nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			if tc.ctype != "" {
				req.Header.Set("Content-Type", tc.ctype)
			}
			if got := isBrowserForm(req); got != tc.isForm {
				t.Fatalf("isBrowserForm() = %v, want %v", got, tc.isForm)
			}
		})
	}
}

func TestDownloadNotFound(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/v1/archives/00000000000000000000000000000000/file", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("download missing %d", rr.Code)
	}
}

func TestPatchUserPasswordByAdmin(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	createUserWithRole(t, st, "bob", "bob-secret-1", auth.RoleViewer)
	u, err := st.UserByUsername(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	patch := httptest.NewRequest(http.MethodPatch, "/v1/users/"+itoa(u.ID), strings.NewReader(`{"password":"new-bob-secret"}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, patch)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch password %d %s", rr.Code, rr.Body.String())
	}
}

func TestListArchivesEmptyJSON(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
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

func TestUploadRejectsNonGzipBody(t *testing.T) {
	// B-6: a raw (non-multipart) upload body must start with gzip magic bytes.
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	for _, body := range []string{"not-a-gzip", "PK\x03\x04zip-bytes", ""} {
		req := httptest.NewRequest(http.MethodPost, "/v1/archives", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Gfs-Filename", "run.tar.gz")
		req.Header.Set("Accept", "application/json")
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("non-gzip body %q: want 400, got %d (%s)", body, rr.Code, rr.Body.String())
		}
	}
}

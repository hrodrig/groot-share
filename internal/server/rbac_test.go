package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func createUserWithRole(t *testing.T, st *store.Store, username, password string, role auth.Role) {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(context.Background(), username, hash, role); err != nil {
		t.Fatal(err)
	}
}

func loginAs(t *testing.T, s *Server, username, password string) *http.Cookie {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login %s %d %s", username, rr.Code, rr.Body.String())
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func createAPIKey(t *testing.T, s *Server, ck *http.Cookie, scope auth.KeyScope) string {
	t.Helper()
	body := `{}`
	if scope == auth.KeyScopeRead {
		body = `{"scope":"read"}`
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create key %d %s", rr.Code, rr.Body.String())
	}
	var out struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.APIKey == "" {
		t.Fatalf("key body %s", rr.Body.String())
	}
	return out.APIKey
}

func TestViewerCannotUpload(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	ck := loginAs(t, s, "view", "view-secret-1")
	req := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader([]byte("x")))
	req.Header.Set("Content-Type", "application/gzip")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer upload %d", rr.Code)
	}
}

func TestUploaderCannotDelete(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "up", "up-secret-12", auth.RoleUploader)
	admin := loginCookie(t, s)
	payload := []byte("del-test")
	post := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader(payload))
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("X-Gfs-Filename", "run.tar.gz")
	post.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d", rr.Code)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	ck := loginAs(t, s, "up", "up-secret-12")
	del := httptest.NewRequest(http.MethodDelete, "/v1/archives/"+created.ID, nil)
	del.AddCookie(ck)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader delete %d", rr.Code)
	}
}

func TestAPIKeyUploadOnlyCannotDownload(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	upKey := createAPIKey(t, s, admin, auth.KeyScopeUpload)
	payload := []byte("key-dl-test")
	post := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader(payload))
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("Authorization", "Bearer "+upKey)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	dl := httptest.NewRequest(http.MethodGet, "/v1/archives/"+created.ID+"/file", nil)
	dl.Header.Set("Authorization", "Bearer "+upKey)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, dl)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("upload key download %d", rr.Code)
	}
}

func TestAPIKeyUploadCannotAccessMe(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	upKey := createAPIKey(t, s, admin, auth.KeyScopeUpload)
	me := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me.Header.Set("Authorization", "Bearer "+upKey)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, me)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("upload key me %d", rr.Code)
	}
}

func TestAPIKeyReadCanAccessMe(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	readKey := createAPIKey(t, s, admin, auth.KeyScopeRead)
	me := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me.Header.Set("Authorization", "Bearer "+readKey)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, me)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"root"`) {
		t.Fatalf("read key me %d %s", rr.Code, rr.Body.String())
	}
}

func TestCreateUserForbiddenForUploader(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "up", "up-secret-12", auth.RoleUploader)
	ck := loginAs(t, s, "up", "up-secret-12")
	body := strings.NewReader(`{"username":"x","password":"x-secret-12"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader create user %d", rr.Code)
	}
}

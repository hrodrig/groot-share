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
	"github.com/hrodrig/groot-share/internal/config"
	"github.com/hrodrig/groot-share/internal/logging"
	"github.com/hrodrig/groot-share/internal/store"
)

func identServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureAdmin(context.Background(), "root", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: config.Config{Topology: config.TopologyVPS}, Store: st, Ready: func() bool { return true }, Version: "0.1.0-test"}, st
}

func TestLoginWrongPassword(t *testing.T) {
	s, _ := identServer(t)
	body := strings.NewReader(`{"username":"root","password":"wrong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
	}
}

func TestLoginSetsCookieAndMe(t *testing.T) {
	s, _ := identServer(t)
	body := strings.NewReader(`{"username":"root","password":"correct-horse"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login %d %s", rr.Code, rr.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("cookie %+v", cookie)
	}

	me := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, me)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"root"`) {
		t.Fatalf("me %d %s", rr.Code, rr.Body.String())
	}
}

func TestAPIKeyShownOnceAndBearerMe(t *testing.T) {
	s, st := identServer(t)
	body := strings.NewReader(`{"username":"root","password":"correct-horse"}`)
	req := httptest.NewRequest(http.MethodPost, "/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	create := httptest.NewRequest(http.MethodPost, "/v1/api-keys", nil)
	create.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, create)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create key %d %s", rr.Code, rr.Body.String())
	}
	var payload struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil || !strings.HasPrefix(payload.APIKey, "gfs_") {
		t.Fatalf("payload %s", rr.Body.String())
	}
	ok, err := st.APIKeyHashStored(context.Background(), auth.HashSecret(payload.APIKey))
	if err != nil || !ok {
		t.Fatal("hash not stored")
	}

	me := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me.Header.Set("Authorization", "Bearer "+payload.APIKey)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, me)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("upload-scope bearer me %d want 403", rr.Code)
	}

	readKey := createAPIKey(t, s, cookie, auth.KeyScopeRead)
	me2 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me2.Header.Set("Authorization", "Bearer "+readKey)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, me2)
	if rr.Code != http.StatusOK {
		t.Fatalf("read-scope bearer me %d %s", rr.Code, rr.Body.String())
	}

	meQ := httptest.NewRequest(http.MethodGet, "/v1/me?api_key="+payload.APIKey, nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, meQ)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("query key must 401, got %d", rr.Code)
	}
}

func TestLoginDoesNotLogPassword(t *testing.T) {
	var buf bytes.Buffer
	logging.SetupWriter(&buf, "json", "info")
	s, _ := identServer(t)
	secret := "correct-horse"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=root&password="+secret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if strings.Contains(buf.String(), secret) {
		t.Fatalf("password leaked into logs: %s", buf.String())
	}
}

func TestGetLoginForm(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `id="login-password"`) {
		t.Fatalf("%d %s", rr.Code, body)
	}
	if !strings.Contains(body, `class="input-group"`) || !strings.Contains(body, `id="pw-toggle"`) || !strings.Contains(body, `id="theme-toggle"`) {
		t.Fatalf("login polish missing: %s", body)
	}
}

func TestHomeRequiresAuth(t *testing.T) {
	s, _ := identServer(t)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("code %d", rr.Code)
	}
}

func TestHomeShowsVersionFooter(t *testing.T) {
	s, _ := identServer(t)
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"root","password":"correct-horse"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "gfs v0.1.0-test") {
		t.Fatalf("home footer %d %s", rr.Code, body)
	}
	if strings.Contains(body, `id="dropzone"`) {
		t.Fatalf("home should not embed upload form")
	}
	if !strings.Contains(body, `href="/upload"`) || !strings.Contains(body, `icon-moon`) {
		t.Fatalf("home nav/upload link missing")
	}
}

func TestUploadPage(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `id="dropzone"`) {
		t.Fatalf("upload page %d %s", rr.Code, body)
	}
	if !strings.Contains(body, `aria-current="page">Upload`) {
		t.Fatalf("upload nav active missing")
	}
}

func TestActivityPage(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `id="ac-h"`) {
		t.Fatalf("activity page %d %s", rr.Code, body)
	}
	if !strings.Contains(body, `aria-current="page">Activity`) {
		t.Fatalf("activity nav missing")
	}
}

func TestHomeHasNoActivitySection(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, `id="ac-h"`) {
		t.Fatalf("home should not include activity section")
	}
	if !strings.Contains(body, `class="brand" href="/"`) {
		t.Fatalf("brand home link missing on home")
	}
}

func TestCreateUserAdmin(t *testing.T) {
	s, _ := identServer(t)
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"root","password":"correct-horse"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	body := strings.NewReader(`{"username":"bob","password":"bob-secret-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user %d %s", rr.Code, rr.Body.String())
	}
}

func TestLogoutHTMLRedirect(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("logout %d", rr.Code)
	}
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("home after logout should redirect %d", rr.Code)
	}
}

func TestLogoutJSON(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("logout json %d %s", rr.Code, rr.Body.String())
	}
}

func TestLoginFormWrongPasswordShowsError(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=root&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "Incorrect username or password") {
		t.Fatalf("form login fail %d body=%q", rr.Code, rr.Body.String())
	}
}

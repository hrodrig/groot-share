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
	return &Server{Cfg: config.Config{Topology: config.TopologyVPS}, Store: st, Ready: func() bool { return true }}, st
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
	if rr.Code != http.StatusOK {
		t.Fatalf("bearer me %d %s", rr.Code, rr.Body.String())
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
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `name="password"`) {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
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

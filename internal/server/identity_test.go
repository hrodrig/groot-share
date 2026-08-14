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
	if err := st.EnsureAdmin(context.Background(), "root", "correct-horse", ""); err != nil {
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
	if !strings.Contains(body, `/static/login-hero.jpg`) {
		t.Fatal("login hero background missing")
	}
	if strings.Contains(body, `wordmark-lg`) || strings.Contains(body, `gate-brand`) {
		t.Fatal("duplicate login wordmark still present")
	}
	if !strings.Contains(body, `gfs — Sign in`) || !strings.Contains(body, `/static/favicon.svg?v=`) {
		t.Fatal("default login chrome missing")
	}
}

func TestGetLoginSimpleHidesProductChrome(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.LoginSimple = true
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/login", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `id="login-password"`) || !strings.Contains(body, `id="theme-toggle"`) {
		t.Fatalf("%d %s", rr.Code, body)
	}
	if !strings.Contains(body, `gate-simple`) {
		t.Fatal("simple gate class missing")
	}
	if strings.Contains(body, `gfs — Sign in`) || strings.Contains(body, `/static/favicon`) || strings.Contains(body, `theme-color`) {
		t.Fatal("simple login still shows product chrome")
	}
	if !strings.Contains(body, `<title>Sign in</title>`) {
		t.Fatal("simple login title")
	}
}

func sessionCookieFor(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"root","password":"correct-horse"}`))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestHomeBrandSubAndFooter(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.BrandSub = "ACME CORP"
	s.Cfg.Footer = "Internal archive"
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(sessionCookieFor(t, s))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("code %d", rr.Code)
	}
	if !strings.Contains(body, "ACME CORP") {
		t.Fatal("brand sub missing")
	}
	if strings.Contains(body, "archive door") || strings.Contains(body, "groot-share") {
		t.Fatal("default chrome still present")
	}
	if !strings.Contains(body, "Internal archive") {
		t.Fatal("custom footer missing")
	}
}

func TestHomeHideBrandSubAndFooter(t *testing.T) {
	s, _ := identServer(t)
	s.Cfg.BrandSub = "-"
	s.Cfg.Footer = "-"
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(sessionCookieFor(t, s))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	body := rr.Body.String()
	if strings.Contains(body, "archive door") || strings.Contains(body, `class="app-foot"`) {
		t.Fatal("hidden chrome still present")
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
	body := strings.NewReader(`{"username":"bob","name":"Bob","password":"bob-secret-1"}`)
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

func TestLoginPageAndHTMLSuccess(t *testing.T) {
	s, _ := identServer(t)
	get := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, get)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Sign in") {
		t.Fatalf("login page %d", rr.Code)
	}

	form := strings.NewReader("username=root&password=correct-horse")
	post := httptest.NewRequest(http.MethodPost, "/login", form)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("html login %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestUploadPageRequiresAuth(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("upload redirect %d", rr.Code)
	}
}

func TestCreateUserJSONValidation(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"username":"x","password":"short"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("short password %d", rr.Code)
	}
}

func TestAPIKeyCannotCreateAnotherKey(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	upKey := createAPIKey(t, s, admin, auth.KeyScopeUpload)
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+upKey)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("api key create key %d", rr.Code)
	}
}

func TestViewerForbiddenAdminUsers(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	ck := loginAs(t, s, "view", "view-secret-1")
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer admin %d", rr.Code)
	}
}

func TestMeUnauthorized(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("me %d", rr.Code)
	}
}

func TestCreateUserMissingNameJSON(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"username":"noname","password":"noname-secret-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing name %d %s", rr.Code, rr.Body.String())
	}
	long := strings.NewReader(`{"username":"longname","name":"` + strings.Repeat("n", 81) + `","password":"long-secret-1"}`)
	req = httptest.NewRequest(http.MethodPost, "/v1/users", long)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("long name %d %s", rr.Code, rr.Body.String())
	}
}

func TestCreateUserMissingPasswordJSON(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"username":"only-name"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing password %d", rr.Code)
	}
}

func TestListMyAPIKeysEmpty(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/v1/me/api-keys", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items":[]`) {
		t.Fatalf("empty keys %d %s", rr.Code, rr.Body.String())
	}
}

func TestLoginNotReadyWithoutStore(t *testing.T) {
	s := &Server{Cfg: config.Config{Topology: config.TopologyVPS}, Ready: func() bool { return true }}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"root","password":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("not ready %d", rr.Code)
	}
}

func TestLoginBadRequestMalformedJSON(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed login %d", rr.Code)
	}
}

func TestCreateUserInvalidRoleJSON(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"username":"bad","password":"bad-secret-1","role":"superuser"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid role %d", rr.Code)
	}
}

func TestCreateUserDuplicateUsername(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"username":"root","name":"Root","password":"other-secret-1","role":"viewer"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate %d %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsPasswordEmptyForm(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPost, "/settings/password", strings.NewReader("password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=pw_short") {
		t.Fatalf("empty password %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestSettingsNameForm(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	form := strings.NewReader("name=Ada+Lovelace")
	req := httptest.NewRequest(http.MethodPost, "/settings/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=named") {
		t.Fatalf("name %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	u, err := st.UserByUsername(context.Background(), "root")
	if err != nil || u.Name != "Ada Lovelace" {
		t.Fatalf("stored %+v %v", u, err)
	}
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if !strings.Contains(rr.Body.String(), "Ada Lovelace") {
		t.Fatal("header missing updated name")
	}
}

func TestSettingsNameErrors(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	empty := httptest.NewRequest(http.MethodPost, "/settings/name", strings.NewReader("name="))
	empty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	empty.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, empty)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=name") {
		t.Fatalf("empty name %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	long := httptest.NewRequest(http.MethodPost, "/settings/name", strings.NewReader("name="+strings.Repeat("x", 81)))
	long.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	long.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, long)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=name") {
		t.Fatalf("long name %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	page := httptest.NewRequest(http.MethodGet, "/settings?notice=named", nil)
	page.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, page)
	if !strings.Contains(rr.Body.String(), "Name updated.") {
		t.Fatal("named notice missing")
	}
	key := createAPIKey(t, s, admin, auth.KeyScopeUpload)
	viaKey := httptest.NewRequest(http.MethodPost, "/settings/name", strings.NewReader("name=Nope"))
	viaKey.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	viaKey.Header.Set("Authorization", "Bearer "+key)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, viaKey)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("api key name %d", rr.Code)
	}
}

func TestMergeActorData(t *testing.T) {
	data := map[string]any{"Nav": "home"}
	ac := &Actor{User: store.User{Username: "root", Role: auth.RoleAdmin}, Method: auth.AuthSession}
	mergeActorData(data, ac)
	if data["Username"] != "root" || !data["CanDelete"].(bool) {
		t.Fatalf("merged %+v", data)
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

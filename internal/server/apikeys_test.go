package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestCreateAPIKeyBadScope(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"scope":"admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad scope %d", rr.Code)
	}
}

func TestUploaderCannotCreateReadScopeKey(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "up", "up-secret-12", auth.RoleUploader)
	ck := loginAs(t, s, "up", "up-secret-12")
	body := strings.NewReader(`{"scope":"read"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader read key %d %s", rr.Code, rr.Body.String())
	}
}

func TestRevokedKeyUnauthorized(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	raw := createAPIKey(t, s, admin, auth.KeyScopeUpload)

	list := httptest.NewRequest(http.MethodGet, "/v1/me/api-keys", nil)
	list.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	if rr.Code != http.StatusOK {
		t.Fatalf("list keys %d", rr.Code)
	}
	var keys struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil || len(keys.Items) != 1 {
		t.Fatalf("keys body %s", rr.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/v1/me/api-keys/"+strconv.FormatInt(keys.Items[0].ID, 10), nil)
	del.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("revoke %d %s", rr.Code, rr.Body.String())
	}

	post := httptest.NewRequest(http.MethodPost, "/v1/archives", bytes.NewReader([]byte("x")))
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("Authorization", "Bearer "+raw)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("revoked upload %d", rr.Code)
	}
}

func TestAdminUsersPageSmoke(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin users %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Create user") || !strings.Contains(body, "root") {
		t.Fatalf("admin page missing content")
	}
	if !strings.Contains(body, "At least 8 characters.") || !strings.Contains(body, `class="card-body"`) {
		t.Fatal("create-user hints or padding missing")
	}
	if !strings.Contains(body, `name="name"`) {
		t.Fatal("name field missing")
	}
	if strings.Contains(body, `autocomplete="name" maxlength="80" value=`) {
		t.Fatal("create-user name must not be prefilled")
	}
}

func TestSettingsPageSmoke(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("settings %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "API keys") {
		t.Fatalf("settings missing keys section")
	}
	if !strings.Contains(body, "At least 8 characters.") {
		t.Fatal("password hint missing")
	}
	if !strings.Contains(body, "Update name") {
		t.Fatal("name form missing")
	}
	if !strings.Contains(body, "Login id") || !strings.Contains(body, `value="root"`) {
		t.Fatal("username (login) missing")
	}
	if !strings.Contains(body, "notice-warn") || !strings.Contains(body, "cannot be recovered") {
		t.Fatal("api key warning missing")
	}
}

func TestAdminUserHTMLFlow(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)

	form := strings.NewReader("name=Alice&username=alice&password=alice-secret-1&role=viewer")
	req := httptest.NewRequest(http.MethodPost, "/admin/users/create", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create user %d", rr.Code)
	}

	users, err := st.ListUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var aliceID int64
	for _, u := range users {
		if u.Username == "alice" {
			aliceID = u.ID
		}
	}
	if aliceID == 0 {
		t.Fatal("alice not created")
	}

	roleForm := strings.NewReader("role=uploader")
	roleReq := httptest.NewRequest(http.MethodPost, "/admin/users/"+strconv.FormatInt(aliceID, 10)+"/role", roleForm)
	roleReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	roleReq.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, roleReq)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("set role %d", rr.Code)
	}

	deactReq := httptest.NewRequest(http.MethodPost, "/admin/users/"+strconv.FormatInt(aliceID, 10)+"/deactivate", nil)
	deactReq.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, deactReq)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("deactivate %d", rr.Code)
	}

	page := httptest.NewRequest(http.MethodGet, "/admin/users?notice=deactivated", nil)
	page.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, page)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "alice") {
		t.Fatalf("admin page after deactivate")
	}
	if !strings.Contains(body, "/activate") || !strings.Contains(body, "/remove") {
		t.Fatal("inactive actions missing")
	}
}

func TestAdminUserHTMLActivateRemove(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	u := createUserViaAPI(t, s, admin, "alice", "alice-secret-1", "viewer")
	id := strconv.FormatInt(u.ID, 10)
	deact := httptest.NewRequest(http.MethodPost, "/admin/users/"+id+"/deactivate", nil)
	deact.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, deact)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("deactivate %d", rr.Code)
	}
	act := httptest.NewRequest(http.MethodPost, "/admin/users/"+id+"/activate", nil)
	act.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, act)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=activated") {
		t.Fatalf("activate %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	deact = httptest.NewRequest(http.MethodPost, "/admin/users/"+id+"/deactivate", nil)
	deact.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, deact)
	rm := httptest.NewRequest(http.MethodPost, "/admin/users/"+id+"/remove", nil)
	rm.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, rm)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "notice=removed") {
		t.Fatalf("remove %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
	if _, err := st.UserByID(context.Background(), u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("alice still present: %v", err)
	}
}

func TestSettingsPasswordAndKeyFlow(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)

	pwForm := strings.NewReader("password=new-secret-99")
	pwReq := httptest.NewRequest(http.MethodPost, "/settings/password", pwForm)
	pwReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pwReq.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, pwReq)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "/login?notice=password") {
		t.Fatalf("password change %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}

	admin = loginAs(t, s, "root", "new-secret-99")

	keyForm := strings.NewReader("scope=upload")
	keyReq := httptest.NewRequest(http.MethodPost, "/settings/api-keys", keyForm)
	keyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	keyReq.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, keyReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("create key html %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "gfs_") || !strings.Contains(body, "Copy key") {
		t.Fatalf("missing one-time key display")
	}
	if !strings.Contains(body, "Last used") || !strings.Contains(body, "never") {
		t.Fatal("last used column missing")
	}

	list := httptest.NewRequest(http.MethodGet, "/v1/me/api-keys", nil)
	list.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, list)
	var keys struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil || len(keys.Items) == 0 {
		t.Fatalf("list keys %s", rr.Body.String())
	}

	revoke := httptest.NewRequest(http.MethodPost, "/settings/api-keys/"+strconv.FormatInt(keys.Items[0].ID, 10)+"/revoke", nil)
	revoke.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, revoke)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("revoke html %d", rr.Code)
	}

	settings := httptest.NewRequest(http.MethodGet, "/settings?notice=revoked", nil)
	settings.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, settings)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "API key deleted") {
		t.Fatalf("settings notice")
	}
}

func TestViewerSettingsNoAPIKeys(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	ck := loginAs(t, s, "view", "view-secret-1")
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("settings %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "Create API key") {
		t.Fatalf("viewer should not see key create")
	}
}

func TestAdminLastAdminGuardHTML(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	usersReq := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	usersReq.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, usersReq)
	var users struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &users); err != nil || len(users.Items) == 0 {
		t.Fatal("no users")
	}
	rootID := users.Items[0].ID

	deact := httptest.NewRequest(http.MethodPost, "/admin/users/"+strconv.FormatInt(rootID, 10)+"/deactivate", nil)
	deact.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, deact)
	if rr.Code != http.StatusSeeOther || !strings.Contains(rr.Header().Get("Location"), "last_admin") {
		t.Fatalf("last admin guard %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}

	page := httptest.NewRequest(http.MethodGet, "/admin/users?notice=last_admin", nil)
	page.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, page)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "last active admin") {
		t.Fatalf("last admin notice")
	}
}

func TestDeleteAPIKeyNotFound(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodDelete, "/v1/me/api-keys/99999", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("delete missing %d", rr.Code)
	}
}

func TestAdminCanCreateReadScopeKey(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	body := strings.NewReader(`{"scope":"read"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("admin read key %d %s", rr.Code, rr.Body.String())
	}
}

func TestCapturesPageHasCopyLink(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	payload := []byte("copy-link-test")
	post := httptest.NewRequest(http.MethodPost, "/v1/archives", gzipPayload(t, payload))
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("X-Gfs-Filename", "run.tar.gz")
	post.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %d", rr.Code)
	}

	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.Host = "gfs.example.com"
	home.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, home)
	if rr.Code != http.StatusOK {
		t.Fatalf("home %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "copy-link") || !strings.Contains(rr.Body.String(), "http://gfs.example.com/v1/archives/") {
		t.Fatalf("missing copy link markup")
	}
}

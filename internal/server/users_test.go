package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestUsersCRUD(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	view := createUserViaAPI(t, s, admin, "view", "view-secret-1", "viewer")
	listUsersContains(t, s, admin, "view")
	getUserRole(t, s, admin, view.ID, "viewer")
	patchUserRole(t, s, st, admin, view.ID, auth.RoleUploader)
	deactivateUser(t, s, st, admin, view.ID)
}

func createUserViaAPI(t *testing.T, s *Server, admin *http.Cookie, username, password, role string) store.User {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `","role":"` + role + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created["role"] != role {
		t.Fatalf("created %v", created)
	}
	u, err := s.Store.UserByUsername(context.Background(), username)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func listUsersContains(t *testing.T, s *Server, admin *http.Cookie, username string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"`+username+`"`) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.String())
	}
}

func getUserRole(t *testing.T, s *Server, admin *http.Cookie, id int64, role string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+itoa(id), nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"`+role+`"`) {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
}

func patchUserRole(t *testing.T, s *Server, st *store.Store, admin *http.Cookie, id int64, want auth.Role) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/v1/users/"+itoa(id),
		strings.NewReader(`{"role":"`+string(want)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rr.Code, rr.Body.String())
	}
	got, err := st.UserByID(context.Background(), id)
	if err != nil || got.Role != want {
		t.Fatalf("patched %+v %v", got, err)
	}
}

func deactivateUser(t *testing.T, s *Server, st *store.Store, admin *http.Cookie, id int64) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/"+itoa(id), nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rr.Code)
	}
	got, err := st.UserByID(context.Background(), id)
	if err != nil || got.Active {
		t.Fatalf("inactive %+v", got)
	}
}

func TestPatchUserNotFound(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPatch, "/v1/users/99999", strings.NewReader(`{"role":"viewer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("patch missing %d %s", rr.Code, rr.Body.String())
	}
}

func TestPatchUserBadID(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	req := httptest.NewRequest(http.MethodPatch, "/v1/users/not-a-number", strings.NewReader(`{"role":"viewer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("patch bad id %d", rr.Code)
	}
}

func TestLastAdminCannotDeactivate(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	root, err := st.UserByUsername(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/users/"+itoa(root.ID),
		strings.NewReader(`{"active":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("last admin patch %d %s", rr.Code, rr.Body.String())
	}
}

func TestLastAdminCannotDemote(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	root, err := st.UserByUsername(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/v1/users/"+itoa(root.ID),
		strings.NewReader(`{"role":"uploader"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("demote %d %s", rr.Code, rr.Body.String())
	}
}

func TestInactiveUserCannotLogin(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	createBody := `{"username":"gone","password":"gone-secret-1","role":"viewer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d", rr.Code)
	}
	u, err := st.UserByUsername(context.Background(), "gone")
	if err != nil {
		t.Fatal(err)
	}
	del := httptest.NewRequest(http.MethodDelete, "/v1/users/"+itoa(u.ID), nil)
	del.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("deactivate %d", rr.Code)
	}
	login := httptest.NewRequest(http.MethodPost, "/login",
		bytes.NewReader([]byte(`{"username":"gone","password":"gone-secret-1"}`)))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Accept", "application/json")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("inactive login %d", rr.Code)
	}
}

func TestPatchMePassword(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	patch := httptest.NewRequest(http.MethodPatch, "/v1/me",
		strings.NewReader(`{"password":"new-secret-9"}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, patch)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch me %d %s", rr.Code, rr.Body.String())
	}
	login := httptest.NewRequest(http.MethodPost, "/login",
		bytes.NewReader([]byte(`{"username":"root","password":"new-secret-9"}`)))
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Accept", "application/json")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, login)
	if rr.Code != http.StatusOK {
		t.Fatalf("login new password %d", rr.Code)
	}
}

func TestPatchMeForbiddenForAPIKey(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	readKey := createAPIKey(t, s, admin, auth.KeyScopeRead)
	patch := httptest.NewRequest(http.MethodPatch, "/v1/me",
		strings.NewReader(`{"password":"nope-secret-9"}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.Header.Set("Authorization", "Bearer "+readKey)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, patch)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("api key patch me %d", rr.Code)
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

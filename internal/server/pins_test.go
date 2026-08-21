package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

func pinArchiveID(t *testing.T, s *Server, ck *http.Cookie, name string) string {
	t.Helper()
	post := httptest.NewRequest(http.MethodPost, "/v1/archives", gzipPayload(t, []byte("pin-test-"+name)))
	post.Header.Set("Content-Type", "application/gzip")
	post.Header.Set("X-Gfs-Filename", name+".tar.gz")
	post.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", name, rr.Code, rr.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode upload body %s: %v", rr.Body.String(), err)
	}
	if created.ID == "" {
		t.Fatalf("upload body has no id: %s", rr.Body.String())
	}
	return created.ID
}

func TestPinPOSTCreatesRow(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	id := pinArchiveID(t, s, admin, "pin-create")

	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
	req.AddCookie(admin)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("pin post: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"pinned":true`) {
		t.Fatalf("pin body: %s", rr.Body.String())
	}
	root, _ := st.UserByUsername(context.Background(), "root")
	pins, err := st.ListPins(context.Background(), root.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatalf("want 1 pin, got %d", len(pins))
	}
}

func TestPinPOSTIsIdempotent(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	id := pinArchiveID(t, s, admin, "pin-idem")

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
		req.AddCookie(admin)
		req.Header.Set("Accept", "application/json")
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("pin post #%d: %d %s", i, rr.Code, rr.Body.String())
		}
	}
	root, _ := st.UserByUsername(context.Background(), "root")
	pins, _ := st.ListPins(context.Background(), root.ID, 0)
	if len(pins) != 1 {
		t.Fatalf("idempotent pin must leave one row, got %d", len(pins))
	}
}

func TestPinDELETEClearsRow(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	id := pinArchiveID(t, s, admin, "pin-del")

	// Pin first
	post := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
	post.AddCookie(admin)
	post.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusCreated {
		t.Fatalf("pin: %d", rr.Code)
	}

	// Now delete
	del := httptest.NewRequest(http.MethodDelete, "/v1/pin/archives/"+id, nil)
	del.AddCookie(admin)
	del.Header.Set("Accept", "application/json")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("pin delete: %d %s", rr.Code, rr.Body.String())
	}

	// DELETE again is a no-op (idempotent, still 204)
	del = httptest.NewRequest(http.MethodDelete, "/v1/pin/archives/"+id, nil)
	del.AddCookie(admin)
	del.Header.Set("Accept", "application/json")
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("pin delete idempotent: %d", rr.Code)
	}

	root, _ := st.UserByUsername(context.Background(), "root")
	pins, _ := st.ListPins(context.Background(), root.ID, 0)
	if len(pins) != 0 {
		t.Fatalf("want 0 pins after delete, got %d", len(pins))
	}
}

func TestPinFormAliasRedirects(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	id := pinArchiveID(t, s, admin, "pin-form")

	// Pin
	post := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
	post.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, post)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("pin form post: %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("pin form redirect: %q", loc)
	}

	// Unpin via form alias
	del := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id+"/delete", nil)
	del.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, del)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("unpin form post: %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("unpin form redirect: %q", loc)
	}
}

func TestPinNotFound(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)

	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/does-not-exist", nil)
	req.AddCookie(admin)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing archive, got %d", rr.Code)
	}
}

func TestPinUnauthorized(t *testing.T) {
	s, _ := identServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/anything", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for anonymous, got %d", rr.Code)
	}
}

func TestPinViewerCanPin(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	id := pinArchiveID(t, s, admin, "pin-viewer")
	createUserWithRole(t, st, "view", "view-secret-1", auth.RoleViewer)
	ck := loginAs(t, s, "view", "view-secret-1")

	req := httptest.NewRequest(http.MethodPost, "/v1/pin/archives/"+id, nil)
	req.AddCookie(ck)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("viewer must be able to pin: %d %s", rr.Code, rr.Body.String())
	}
}

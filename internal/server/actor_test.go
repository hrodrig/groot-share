package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestActorCanNil(t *testing.T) {
	var ac *Actor
	if ac.Can(auth.PermArchivesRead) {
		t.Fatal("nil actor")
	}
	if shell := shellUserData(nil); len(shell) != 0 {
		t.Fatalf("shell nil %+v", shell)
	}
}

func TestShellUserDataRoles(t *testing.T) {
	admin := &Actor{User: store.User{Username: "root", Role: auth.RoleAdmin}, Method: auth.AuthSession}
	data := shellUserData(admin)
	if !data["CanDelete"].(bool) || !data["CanManageUsers"].(bool) || !data["CanManageKeys"].(bool) {
		t.Fatalf("admin shell %+v", data)
	}
	viewer := &Actor{User: store.User{Username: "v", Role: auth.RoleViewer}, Method: auth.AuthSession}
	data = shellUserData(viewer)
	if data["CanUpload"].(bool) || data["CanDelete"].(bool) {
		t.Fatalf("viewer shell %+v", data)
	}
}

func TestFirstFilePartSkipsNonFile(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("note", "skip-me"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "ok.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(fw, "payload"); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	mr := multipart.NewReader(&buf, mw.Boundary())
	rc, key, err := firstFilePart(mr)
	if err != nil || key != "ok.tar.gz" {
		t.Fatalf("key %q err %v", key, err)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != "payload" {
		t.Fatalf("body %q", body)
	}
}

func TestAuthFailRedirectsHTML(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("redirect %d loc=%s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestForbiddenJSONOnV1(t *testing.T) {
	s, _ := identServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/archives", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("json unauthorized %d", rr.Code)
	}
}

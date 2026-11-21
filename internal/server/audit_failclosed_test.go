package server

import (
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

func TestRequireUserAudit_failClosedOnCreateUser(t *testing.T) {
	s, st := identServer(t)
	s.auditInsert = func(ctx context.Context, ev store.Audit) error {
		return errors.New("forced audit failure")
	}

	cookie := loginCookie(t, s)
	body := strings.NewReader(`{"username":"bob","name":"Bob","password":"bob-secret-12","role":"uploader"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "audit unavailable" {
		t.Fatalf("error=%v", out["error"])
	}
	// Mutation may have applied; residual documented in SECURITY.md.
	if _, err := st.UserByUsername(context.Background(), "bob"); err != nil {
		t.Fatalf("expected user created before audit fail: %v", err)
	}
}

func TestRequireUserAudit_okOnCreateUser(t *testing.T) {
	s, st := identServer(t)
	cookie := loginCookie(t, s)
	body := strings.NewReader(`{"username":"carol","name":"Carol","password":"carol-secret1","role":"uploader"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("code %d body %s", rr.Code, rr.Body.String())
	}
	events, err := st.ListAuditFiltered(context.Background(), store.AuditFilter{Action: "user.create"}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected user.create audit row")
	}
}

func TestRequireUserAudit_failClosedOnAPIKeyRevoke(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	_ = createAPIKey(t, s, admin, auth.KeyScopeUpload)

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

	s.auditInsert = func(ctx context.Context, ev store.Audit) error {
		return errors.New("forced audit failure")
	}

	del := httptest.NewRequest(http.MethodDelete, "/v1/me/api-keys/"+strconv.FormatInt(keys.Items[0].ID, 10), nil)
	del.AddCookie(admin)
	drr := httptest.NewRecorder()
	s.Handler().ServeHTTP(drr, del)
	if drr.Code != http.StatusInternalServerError {
		t.Fatalf("revoke code %d body %s", drr.Code, drr.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(drr.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "audit unavailable" {
		t.Fatalf("error=%v", out["error"])
	}
}

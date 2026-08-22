package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
	"github.com/hrodrig/groot-share/internal/store"
)

func TestAuditFiltersByActionAndActor(t *testing.T) {
	_, st := identServer(t)
	ctx := context.Background()
	seed := []store.Audit{
		{Actor: "alice", Action: "upload", ObjectKey: "a.tar.gz"},
		{Actor: "bob", Action: "upload", ObjectKey: "b.tar.gz"},
		{Actor: "alice", Action: "download", ObjectKey: "a.tar.gz"},
		{Actor: "carol", Action: "delete", ObjectKey: "c.tar.gz"},
	}
	for _, ev := range seed {
		if err := st.InsertAudit(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}

	// action filter
	byAction, err := st.ListAuditFiltered(ctx, store.AuditFilter{Action: "upload"}, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(byAction) != 2 {
		t.Fatalf("action=upload: got %d rows, want 2", len(byAction))
	}

	// actor substring filter (case-insensitive)
	byActor, err := st.ListAuditFiltered(ctx, store.AuditFilter{Actor: "ALICE"}, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(byActor) != 2 {
		t.Fatalf("actor=ALICE: got %d rows, want 2", len(byActor))
	}

	// combined filter
	combo, err := st.ListAuditFiltered(ctx, store.AuditFilter{Actor: "alice", Action: "download"}, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(combo) != 1 || combo[0].ObjectKey != "a.tar.gz" {
		t.Fatalf("combo filter: got %+v", combo)
	}
}

func TestActivityExportAdminOnly(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	if err := st.InsertAudit(context.Background(), store.Audit{Actor: "root", Action: "upload", ObjectKey: "x.tar.gz"}); err != nil {
		t.Fatal(err)
	}

	// CSV export as admin
	req := httptest.NewRequest(http.MethodGet, "/v1/activity/export?format=csv", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("csv export: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.HasPrefix(rr.Body.String(), "actor,action,object_id,object_key,remote_ip,created_at\n") {
		t.Fatalf("csv header missing: %q", rr.Body.String()[:min(40, len(rr.Body.String()))])
	}
	if !strings.Contains(rr.Body.String(), ",upload,,x.tar.gz,") {
		t.Fatalf("csv row missing: %s", rr.Body.String())
	}

	// JSON export as admin
	req = httptest.NewRequest(http.MethodGet, "/v1/activity/export?format=json", nil)
	req.AddCookie(admin)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"items"`) {
		t.Fatalf("json export: %d %s", rr.Code, rr.Body.String())
	}
}

func TestActivityExportForbiddenForViewer(t *testing.T) {
	s, st := identServer(t)
	createUserWithRole(t, st, "viewer", "viewer-pass", auth.RoleViewer)
	ck := loginAs(t, s, "viewer", "viewer-pass")

	exp := httptest.NewRequest(http.MethodGet, "/v1/activity/export?format=csv", nil)
	exp.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, exp)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("viewer export got %d, want 403", rr.Code)
	}
}

func TestTypedConfirmMarkupOnDelete(t *testing.T) {
	s, _ := identServer(t)
	admin := loginCookie(t, s)
	dashboardArchive(t, s, admin, "typed-confirm-test.tar.gz")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(admin)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `data-confirm-require="typed-confirm-test.tar.gz"`) {
		t.Fatalf("typed confirm require missing on card/table delete: %s", body)
	}
	if !strings.Contains(body, `id="confirm-input"`) {
		t.Fatalf("typed confirm input missing in markup: %s", body)
	}
}

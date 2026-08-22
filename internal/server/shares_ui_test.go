package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hrodrig/groot-share/internal/auth"
)

// sharesPageGET is a tiny helper: admin GET of the shares page for an archive.
func sharesPageGET(t *testing.T, s *Server, ck *http.Cookie, archiveID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/archives/"+archiveID+"/shares", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestSharesPageRenders(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")

	rr := sharesPageGET(t, s, ck, created.ID)
	if rr.Code != http.StatusOK {
		t.Fatalf("shares page %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Create share link", "No share links", "Preset", "24 hours", "7 days"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
}

func TestSharesPageUnknownArchive404(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	rr := sharesPageGET(t, s, ck, "nope-not-real")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown archive %d", rr.Code)
	}
}

func TestSharesPageNonAdminForbidden(t *testing.T) {
	s, st := identServer(t)
	admin := loginCookie(t, s)
	created := postArchive(t, s, admin, "vendor.tar.gz", "vendor-bytes")

	createUserWithRole(t, st, "up", "up-secret-12", auth.RoleUploader)
	upler := loginAs(t, s, "up", "up-secret-12")

	req := httptest.NewRequest(http.MethodGet, "/archives/"+created.ID+"/shares", nil)
	req.AddCookie(upler)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("uploader shares page %d", rr.Code)
	}
}

func TestSharesCreateFormShowsURLOnce(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")

	form := url.Values{"expires_in": {"24h"}, "label": {"acme"}}
	req := httptest.NewRequest(http.MethodPost, "/archives/"+created.ID+"/shares", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create form %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "/s/") {
		t.Fatalf("create response missing share URL: %s", body)
	}
	// The URL must appear in the body (one-shot) and never as a redirect target.
	if loc := rr.Header().Get("Location"); loc != "" {
		t.Fatalf("create should not redirect (would leak token in history/log access): %q", loc)
	}
	// The raw token must not show up on a later list GET.
	list := sharesPageGET(t, s, ck, created.ID)
	if strings.Contains(list.Body.String(), "/s/") {
		t.Fatalf("list page leaked a share URL: %s", list.Body.String())
	}
}

func TestSharesCreateFormValidationFailsClosed(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")

	cases := []url.Values{
		{}, // no expiry at all
		{"expires_in": {"24h"}, "expires_at_local": {"2030-01-01T00:00"}}, // both
		{"expires_in": {"bogus"}},                   // bad duration
		{"expires_at_local": {"2020-01-01T00:00"}},  // past
		{"expires_in": {"24h"}, "max_uses": {"-1"}}, // negative max uses
	}
	for _, form := range cases {
		req := httptest.NewRequest(http.MethodPost, "/archives/"+created.ID+"/shares", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(ck)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("invalid form should re-render 200, got %d %s", rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if strings.Contains(body, "/s/") {
			t.Fatalf("invalid form leaked a share URL: %v -> %s", form, body)
		}
		if !strings.Contains(body, "notice-err") {
			t.Fatalf("invalid form missing error notice: %v -> %s", form, body)
		}
	}
}

func TestSharesCreateFormBadArchive404(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	form := url.Values{"expires_in": {"24h"}}
	req := httptest.NewRequest(http.MethodPost, "/archives/does-not-exist/shares", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("create for unknown archive should 404, got %d", rr.Code)
	}
}

func TestSharesRevokeForm(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")
	out := createShare(t, s, ck, created.ID, `{"expires_in":"24h"}`)
	shareID := formatID(out["id"].(float64))

	req := httptest.NewRequest(http.MethodPost, "/archives/"+created.ID+"/shares/"+shareID+"/revoke", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("revoke form %d %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "notice=revoked") {
		t.Fatalf("revoke redirect %q", loc)
	}

	// The link must now be revoked: the rendered status pill reads "revoked",
	// and no "active" pill is rendered. (CSS class names also appear in the
	// injected <style>, so assert on the rendered span content, not the class.)
	page := sharesPageGET(t, s, ck, created.ID)
	if !strings.Contains(page.Body.String(), ">revoked</span>") {
		t.Fatalf("revoked link missing revoked pill: %s", page.Body.String())
	}
	if strings.Contains(page.Body.String(), ">active</span>") {
		t.Fatalf("revoked link still active: %s", page.Body.String())
	}
}

func TestSharesRevokeUnknown404(t *testing.T) {
	s, _ := identServer(t)
	ck := loginCookie(t, s)
	created := postArchive(t, s, ck, "vendor.tar.gz", "vendor-bytes")

	req := httptest.NewRequest(http.MethodPost, "/archives/"+created.ID+"/shares/999999/revoke", nil)
	req.AddCookie(ck)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("unknown revoke %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "notice=missing") {
		t.Fatalf("unknown revoke redirect %q", loc)
	}
}

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct-horse") {
		t.Fatal("want match")
	}
	if CheckPassword(h, "wrong-password") {
		t.Fatal("wrong should fail")
	}
	if CheckPassword("", "anything1") {
		t.Fatal("empty hash")
	}
}

func TestHashPasswordTooShort(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAPIKey(t *testing.T) {
	raw, hash, prefix, err := NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "gfs_") || len(raw) < 20 {
		t.Fatalf("raw %q", raw)
	}
	if HashSecret(raw) != hash {
		t.Fatal("hash mismatch")
	}
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("prefix %q raw %q", prefix, raw)
	}
}

func TestExtractKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/me?api_key=nope", nil)
	r.Header.Set("X-API-Key", " from-header ")
	if got := ExtractKey(r); got != "from-header" {
		t.Fatalf("got %q", got)
	}
	r2 := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	r2.Header.Set("Authorization", "Bearer abc")
	if got := ExtractKey(r2); got != "abc" {
		t.Fatalf("bearer %q", got)
	}
	r3 := httptest.NewRequest(http.MethodGet, "/v1/me?api_key=secret", nil)
	if got := ExtractKey(r3); got != "" {
		t.Fatalf("query must be ignored, got %q", got)
	}
}

func TestEqualHash(t *testing.T) {
	h := HashSecret("tok")
	if !EqualHash(h, HashSecret("tok")) {
		t.Fatal("equal")
	}
	if EqualHash(h, HashSecret("other")) {
		t.Fatal("other")
	}
}

func TestNewSessionToken(t *testing.T) {
	raw, hash, err := NewSessionToken()
	if err != nil || raw == "" || hash != HashSecret(raw) {
		t.Fatalf("raw=%q hash=%q err=%v", raw, hash, err)
	}
}

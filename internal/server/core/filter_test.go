package core

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseFilterEmptyCore(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	f := ParseFilter(req)
	if !f.IsZero() {
		t.Fatalf("empty request should yield zero filter: %+v", f)
	}
}

func TestWindowSinceCore(t *testing.T) {
	now := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	got := WindowSince("24h", now)
	want := now.Add(-24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

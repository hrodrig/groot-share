package ratelimit

import (
	"testing"
	"time"
)

func TestNilAllows(t *testing.T) {
	var l *Limiter
	if !l.Allow("x") {
		t.Fatal("nil must allow")
	}
	if New(0, time.Minute) != nil || New(5, 0) != nil {
		t.Fatal("disabled New must return nil")
	}
}

func TestAllowWindow(t *testing.T) {
	l := New(2, time.Minute)
	if !l.Allow("a") {
		t.Fatal("first ok")
	}
	if !l.Allow("a") {
		t.Fatal("second ok")
	}
	if l.Allow("a") {
		t.Fatal("third must deny")
	}
	if !l.Allow("b") {
		t.Fatal("other key ok")
	}
}

// TestSweepEvictsExpiredKeys pins #43: keys whose events have all aged out of
// the window must be removed so the map does not grow unbounded with
// short-lived keys (one request per unique IP).
func TestSweepEvictsExpiredKeys(t *testing.T) {
	l := New(1, 10*time.Millisecond)

	// Populate several distinct keys.
	for i := 0; i < 10; i++ {
		l.Allow("key-" + string(rune('a'+i)))
	}
	if got := len(l.events); got != 10 {
		t.Fatalf("want 10 keys after populating, got %d", got)
	}

	// Wait past the window, then trigger a paced sweep via a new Allow.
	time.Sleep(20 * time.Millisecond)
	l.Allow("fresh")
	if got := len(l.events); got != 1 {
		t.Fatalf("want only the fresh key to remain after sweep, got %d", got)
	}
	if _, ok := l.events["fresh"]; !ok {
		t.Fatal("fresh key must survive the sweep")
	}
}

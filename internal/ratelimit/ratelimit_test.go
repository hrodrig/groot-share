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

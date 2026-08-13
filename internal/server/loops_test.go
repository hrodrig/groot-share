package server

import (
	"context"
	"testing"
	"time"
)

func TestRetryLoopStopsOnCancel(t *testing.T) {
	s, mem := vpsS3Server(t)
	mem.FailPuts = true
	ck := loginCookie(t, s)
	postArchive(t, s, ck, "loop.tar.gz", "retry-loop")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RetryLoop(ctx, 5*time.Millisecond)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RetryLoop did not stop")
	}
}

func TestSweepLoopStopsOnCancel(t *testing.T) {
	s, _ := identServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.SweepLoop(ctx, 5*time.Millisecond)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SweepLoop did not stop")
	}
}

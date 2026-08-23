// Package ratelimit provides in-process sliding-window limits.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter counts events per key within a fixed window.
// A nil Limiter allows all requests.
type Limiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
	max    int
	window time.Duration

	// lastSweep paces the inline janitor: the whole map is swept at most once
	// per window, on an Allow() call, so keys whose events have all expired
	// are evicted without a background goroutine.
	lastSweep time.Time
}

// New returns a limiter of limit events per window per key.
// limit <= 0 or window <= 0 disables limiting (returns nil).
func New(limit int, window time.Duration) *Limiter {
	if limit <= 0 || window <= 0 {
		return nil
	}
	return &Limiter{
		events: make(map[string][]time.Time),
		max:    limit,
		window: window,
	}
}

// Allow records one event for key and reports whether it is under the cap.
func (l *Limiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweepLocked(now)

	prev := l.events[key]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.events[key] = kept
		return false
	}
	l.events[key] = append(kept, now)
	return true
}

// sweepLocked evicts keys whose events have all fallen out of the window. It
// runs at most once per window (paced by lastSweep) so the map does not grow
// unbounded when callers rotate through many short-lived keys (e.g. one login
// attempt per unique IP). Caller must hold l.mu.
func (l *Limiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	cutoff := now.Add(-l.window)
	for key, times := range l.events {
		kept := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.events, key)
			continue
		}
		l.events[key] = kept
	}
}

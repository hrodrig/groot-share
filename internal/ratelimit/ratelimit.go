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

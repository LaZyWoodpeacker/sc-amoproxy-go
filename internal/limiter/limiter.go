package limiter

import (
	"context"
	"sync"
	"time"
)

type bucket struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func New() *Limiter { return &Limiter{buckets: make(map[string]*bucket)} }
func (l *Limiter) Allow(ctx context.Context, name string, rps float64) bool {
	l.mu.Lock()
	b := l.buckets[name]
	if b == nil {
		b = &bucket{interval: time.Duration(float64(time.Second) / rps)}
		l.buckets[name] = b
	}
	l.mu.Unlock()
	b.mu.Lock()
	now := time.Now()
	if b.next.Before(now) {
		b.next = now
	}
	wait := time.Until(b.next)
	if wait > 100*time.Millisecond {
		b.next = now
		b.mu.Unlock()
		return false
	}
	b.next = b.next.Add(b.interval)
	b.mu.Unlock()
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

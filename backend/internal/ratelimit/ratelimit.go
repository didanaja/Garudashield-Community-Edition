// Package ratelimit implements a bounded, single-process token-bucket limiter.
//
// IMPORTANT: this limiter is intentionally simple and local to one process. It
// is NOT a distributed rate-limiting solution and must not be presented as one:
// running multiple instances multiplies the effective limit. The limiter is
// bounded in both key count and idle memory so a public demo cannot be used to
// exhaust memory.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	last     time.Time
	lastSeen time.Time
}

// Limiter is a token-bucket rate limiter keyed by an arbitrary client string.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	burst    float64
	refill   float64 // tokens per second
	idleTTL  time.Duration
	maxKeys  int
	now      func() time.Time
}

// Option customises a Limiter.
type Option func(*Limiter)

// WithClock injects a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(l *Limiter) { l.now = now }
}

// New creates a limiter granting burst tokens per window and refilling at a
// steady rate of burst/window. maxKeys bounds memory; idle entries older than
// idleTTL are evicted lazily.
func New(burst int, window time.Duration, maxKeys int, opts ...Option) *Limiter {
	if burst <= 0 {
		burst = 1
	}
	if window <= 0 {
		window = time.Second
	}
	if maxKeys <= 0 {
		maxKeys = 10000
	}
	l := &Limiter{
		buckets: make(map[string]*bucket),
		burst:   float64(burst),
		refill:  float64(burst) / window.Seconds(),
		idleTTL: 2 * window,
		maxKeys: maxKeys,
		now:     time.Now,
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Result describes the outcome of an Allow check.
type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

// Allow consumes a token for key, returning whether the request may proceed.
func (l *Limiter) Allow(key string) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		l.evictIfNeeded()
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	// Refill based on elapsed time.
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.refill
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}
	b.lastSeen = now

	if b.tokens < 1 {
		missing := 1 - b.tokens
		retry := time.Duration(missing / l.refill * float64(time.Second))
		if retry < time.Second {
			retry = time.Second
		}
		return Result{Allowed: false, Remaining: 0, RetryAfter: retry}
	}

	b.tokens--
	return Result{Allowed: true, Remaining: int(b.tokens)}
}

func (l *Limiter) evictIfNeeded() {
	if len(l.buckets) < l.maxKeys {
		l.sweepIdle()
		if len(l.buckets) < l.maxKeys {
			return
		}
	}
	l.sweepIdle()
	if len(l.buckets) < l.maxKeys {
		return
	}
	// Still full: drop the least-recently-seen bucket.
	var oldestKey string
	var oldest time.Time
	for k, b := range l.buckets {
		if oldestKey == "" || b.lastSeen.Before(oldest) {
			oldestKey, oldest = k, b.lastSeen
		}
	}
	delete(l.buckets, oldestKey)
}

func (l *Limiter) sweepIdle() {
	now := l.now()
	// Full sweep on each call is O(n); acceptable given bounded maxKeys and the
	// low eviction frequency.
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.idleTTL {
			delete(l.buckets, k)
		}
	}
}

// Len reports the number of tracked buckets (used by tests and health output).
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// Package storage exposes a small Redis-compatible key/value interface plus an
// in-memory implementation used for local demonstrations. The interface mirrors
// the handful of Redis commands the demo needs (GET/SET/DEL/INCR/EXPIRE) so a
// real Redis client could be dropped in without touching call sites.
package storage

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a key does not exist or has expired.
var ErrNotFound = errors.New("storage: key not found")

// Store is the minimal storage contract used by the demo engine.
type Store interface {
	// Get returns the value for key or ErrNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value under key with an optional TTL (0 = no expiration).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete removes a key. It is not an error to delete a missing key.
	Delete(ctx context.Context, key string) error
	// Incr atomically increments an integer counter, applying the TTL on first
	// creation, and returns the new value.
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// Ping reports whether the store is reachable.
	Ping(ctx context.Context) error
	// Close releases resources.
	Close() error
}

type entry struct {
	value   []byte
	expires time.Time
}

// Memory is a bounded, concurrency-safe in-memory Store. It enforces a maximum
// number of keys and actively evicts expired entries so a public-facing demo
// cannot be used to grow memory without bound.
type Memory struct {
	mu       sync.Mutex
	items    map[string]entry
	maxKeys  int
	now      func() time.Time
}

// NewMemory creates an in-memory store holding at most maxKeys entries. When the
// limit is reached, newly inserted keys evict the oldest entry.
func NewMemory(maxKeys int) *Memory {
	if maxKeys <= 0 {
		maxKeys = 10000
	}
	return &Memory{
		items:   make(map[string]entry),
		maxKeys: maxKeys,
		now:     time.Now,
	}
}

func (m *Memory) get(key string) (entry, bool) {
	e, ok := m.items[key]
	if !ok {
		return entry{}, false
	}
	if !e.expires.IsZero() && m.now().After(e.expires) {
		delete(m.items, key)
		return entry{}, false
	}
	return e, true
}

// Get implements Store.
func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.get(key)
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), e.value...), nil
}

// Set implements Store.
func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evictIfNeeded(key)
	var expires time.Time
	if ttl > 0 {
		expires = m.now().Add(ttl)
	}
	m.items[key] = entry{value: append([]byte(nil), value...), expires: expires}
	return nil
}

// Delete implements Store.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

// Incr implements Store.
func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evictIfNeeded(key)
	var cur int64
	if e, ok := m.get(key); ok {
		cur = decodeInt(e.value)
	}
	cur++
	expires := m.now().Add(ttl)
	if e, ok := m.items[key]; ok && !e.expires.IsZero() {
		expires = e.expires // preserve the original window
	}
	m.items[key] = entry{value: encodeInt(cur), expires: expires}
	return cur, nil
}

// Ping implements Store.
func (m *Memory) Ping(_ context.Context) error { return nil }

// Close implements Store.
func (m *Memory) Close() error { return nil }

// Len returns the current number of live keys (used by health and tests).
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()
	return len(m.items)
}

func (m *Memory) evictIfNeeded(incoming string) {
	if _, exists := m.items[incoming]; exists {
		return
	}
	if len(m.items) < m.maxKeys {
		return
	}
	m.sweepLocked()
	if len(m.items) < m.maxKeys {
		return
	}
	var oldestKey string
	var oldest time.Time
	for k, e := range m.items {
		if oldestKey == "" || e.expires.Before(oldest) {
			oldestKey, oldest = k, e.expires
		}
	}
	delete(m.items, oldestKey)
}

func (m *Memory) sweepLocked() {
	now := m.now()
	for k, e := range m.items {
		if !e.expires.IsZero() && now.After(e.expires) {
			delete(m.items, k)
		}
	}
}

func encodeInt(v int64) []byte {
	return []byte(strconv64(v))
}

func decodeInt(b []byte) int64 {
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

func strconv64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

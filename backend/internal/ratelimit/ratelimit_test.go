package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucketAllowsBurst(t *testing.T) {
	l := New(3, time.Minute, 100)
	for i := 0; i < 3; i++ {
		if !l.Allow("client").Allowed {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}
	if l.Allow("client").Allowed {
		t.Fatal("burst exceeded: 4th request should be denied")
	}
}

func TestTokenBucketIsPerKey(t *testing.T) {
	l := New(1, time.Minute, 100)
	if !l.Allow("a").Allowed {
		t.Fatal("first request for a should be allowed")
	}
	if !l.Allow("b").Allowed {
		t.Fatal("first request for b should be allowed (independent buckets)")
	}
}

func TestTokenBucketRefills(t *testing.T) {
	now := time.Unix(1000, 0)
	l := New(1, time.Second, 100, WithClock(func() time.Time { return now }))
	if !l.Allow("c").Allowed {
		t.Fatal("initial request should be allowed")
	}
	if l.Allow("c").Allowed {
		t.Fatal("immediate second request should be denied")
	}
	now = now.Add(1100 * time.Millisecond)
	if !l.Allow("c").Allowed {
		t.Fatal("request after refill should be allowed")
	}
}

func TestRetryAfterReturned(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, time.Second, 100, WithClock(func() time.Time { return now }))
	_ = l.Allow("d")
	res := l.Allow("d")
	if res.Allowed {
		t.Fatal("should be denied")
	}
	if res.RetryAfter < time.Second {
		t.Fatalf("expected retry-after of at least 1s, got %v", res.RetryAfter)
	}
}

func TestMaxKeysBounded(t *testing.T) {
	l := New(5, time.Minute, 10)
	for i := 0; i < 50; i++ {
		l.Allow("key-" + string(rune('a'+i/26)) + string(rune('0'+i%26)) + "-" + itoa(i))
	}
	if l.Len() > 10 {
		t.Fatalf("expected bounded key count <= 10, got %d", l.Len())
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

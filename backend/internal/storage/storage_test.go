package storage

import (
	"context"
	"testing"
	"time"
)

func TestSetGetDelete(t *testing.T) {
	s := NewMemory(100)
	ctx := context.Background()
	if err := s.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatal(err)
	}
	v, err := s.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if string(v) != "v" {
		t.Fatalf("got %q", v)
	}
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "k"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTTLExpiry(t *testing.T) {
	s := NewMemory(100)
	ctx := context.Background()
	if err := s.Set(ctx, "k", []byte("v"), 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := s.Get(ctx, "k"); err != ErrNotFound {
		t.Fatalf("expected expiry, got %v", err)
	}
}

func TestIncrCreatesAndIncrements(t *testing.T) {
	s := NewMemory(100)
	ctx := context.Background()
	n, err := s.Incr(ctx, "counter", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("first incr = %d, %v", n, err)
	}
	n, err = s.Incr(ctx, "counter", time.Minute)
	if err != nil || n != 2 {
		t.Fatalf("second incr = %d, %v", n, err)
	}
	if _, err := s.Get(ctx, "missing"); err != ErrNotFound {
		t.Fatal("missing key should not exist")
	}
}

func TestMaxKeysEvicts(t *testing.T) {
	s := NewMemory(3)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if err := s.Set(ctx, "k"+itoa(i), []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
	}
	if s.Len() > 3 {
		t.Fatalf("expected bounded store, got %d", s.Len())
	}
}

func TestPing(t *testing.T) {
	s := NewMemory(1)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

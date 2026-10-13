package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"garudashield/internal/storage"
)

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "correct horse battery") {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("invalid password accepted")
	}
	if VerifyPassword("garbage", "x") {
		t.Fatal("malformed hash accepted")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := storage.NewMemory(100)
	m := NewManager(s, time.Minute, false)
	ctx := context.Background()

	sess, err := m.CreateSession(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := m.GetSession(ctx, sess.ID)
	if err != nil || loaded == nil || loaded.User != "admin" {
		t.Fatalf("expected loaded session, got %+v err=%v", loaded, err)
	}

	// Session via cookie round trip.
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(httpCookie(sess.ID))
	got, err := m.SessionFromRequest(req)
	if err != nil || got == nil {
		t.Fatalf("expected session from cookie, err=%v", err)
	}

	// Expired session is rejected.
	m2 := NewManager(s, 10*time.Millisecond, false)
	s2, _ := m2.CreateSession(ctx, "admin")
	time.Sleep(20 * time.Millisecond)
	if g, _ := m2.GetSession(ctx, s2.ID); g != nil {
		t.Fatal("expired session should be gone")
	}

	if err := m.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.GetSession(ctx, sess.ID); g != nil {
		t.Fatal("deleted session should be gone")
	}
}

func TestCSRFValidation(t *testing.T) {
	sess := &Session{ID: "x", CSRF: "secret-token", ExpiresAt: time.Now().Add(time.Minute)}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set(CSRFHeader, "secret-token")
	if !ValidCSRF(sess, r) {
		t.Fatal("valid CSRF rejected")
	}
	r2 := httptest.NewRequest("POST", "/", nil)
	if ValidCSRF(sess, r2) {
		t.Fatal("missing CSRF accepted")
	}
	if ValidCSRF(nil, r) {
		t.Fatal("nil session should fail CSRF")
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	s := storage.NewMemory(10)
	m := NewManager(s, time.Minute, true)
	ctx := context.Background()
	sess, _ := m.CreateSession(ctx, "admin")
	rec := httptest.NewRecorder()
	m.SetCookie(rec, sess)
	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.Name != CookieName {
		t.Fatalf("unexpected cookie: %+v", c)
	}
}

func httpCookie(id string) *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: id}
}

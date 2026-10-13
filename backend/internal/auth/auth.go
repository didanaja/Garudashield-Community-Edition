// Package auth provides password hashing (PBKDF2-HMAC-SHA256 via the standard
// library) and server-side session management for the admin control plane.
//
// Sessions are stored server-side in the configured Store and referenced by an
// opaque, high-entropy identifier in an HttpOnly cookie. CSRF protection uses a
// per-session token compared with a constant-time check.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"garudashield/internal/storage"
)

const (
	// CookieName is the session cookie name.
	CookieName = "garuda_session"
	// CSRFHeader is the header clients must echo on state-changing requests.
	CSRFHeader = "X-CSRF-Token"

	pbkdf2Iterations = 210_000
	keyLength        = 32
	saltLength       = 16
	sessionPrefix    = "session:"
)

// Session is a server-side authenticated session.
type Session struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	CSRF      string    `json:"csrf"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Manager creates and validates sessions.
type Manager struct {
	Store      storage.Store
	SessionTTL time.Duration
	Secure     bool // set Secure flag on cookies (true behind HTTPS)
}

// NewManager constructs a session manager.
func NewManager(store storage.Store, ttl time.Duration, secure bool) *Manager {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Manager{Store: store, SessionTTL: ttl, Secure: secure}
}

// HashPassword derives a PBKDF2-HMAC-SHA256 hash and returns a self-describing
// encoded string: pbkdf2_sha256$<iter>$<salt>$<key>.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters")
	}
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, keyLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s",
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword checks a password against an encoded hash in constant time.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// CreateSession stores a new session and returns it.
func (m *Manager) CreateSession(ctx context.Context, user string) (*Session, error) {
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	s := &Session{
		ID:        id,
		User:      user,
		CSRF:      csrf,
		CreatedAt: now,
		ExpiresAt: now.Add(m.SessionTTL),
	}
	blob, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	if err := m.Store.Set(ctx, sessionPrefix+id, blob, m.SessionTTL); err != nil {
		return nil, err
	}
	return s, nil
}

// GetSession loads a session by ID, returning nil when absent or expired.
func (m *Manager) GetSession(ctx context.Context, id string) (*Session, error) {
	if id == "" {
		return nil, nil
	}
	blob, err := m.Store.Get(ctx, sessionPrefix+id)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(blob, &s); err != nil {
		return nil, nil
	}
	if time.Now().After(s.ExpiresAt) {
		_ = m.Store.Delete(ctx, sessionPrefix+id)
		return nil, nil
	}
	return &s, nil
}

// DeleteSession removes a session (logout).
func (m *Manager) DeleteSession(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return m.Store.Delete(ctx, sessionPrefix+id)
}

// SetCookie writes the session cookie.
func (m *Manager) SetCookie(w http.ResponseWriter, s *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    s.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  s.ExpiresAt,
		MaxAge:   int(time.Until(s.ExpiresAt).Seconds()),
	})
}

// ClearCookie expires the session cookie.
func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// SessionFromRequest loads the session referenced by the request cookie.
func (m *Manager) SessionFromRequest(r *http.Request) (*Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return nil, nil
	}
	return m.GetSession(r.Context(), c.Value)
}

// ValidCSRF reports whether the request carries the session's CSRF token.
func ValidCSRF(s *Session, r *http.Request) bool {
	if s == nil {
		return false
	}
	got := r.Header.Get(CSRFHeader)
	if got == "" {
		// Fall back to a form field for progressive enhancement.
		got = r.FormValue("csrf_token")
	}
	if got == "" {
		return false
	}
	return hmac.Equal([]byte(got), []byte(s.CSRF))
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

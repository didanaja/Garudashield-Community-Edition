package api

import (
	"context"
	"net/http"

	"garudashield/internal/auth"
)

type sessionKey struct{}

func sessionFromRequest(r *http.Request) *auth.Session {
	if v := r.Context().Value(sessionKey{}); v != nil {
		if s, ok := v.(*auth.Session); ok {
			return s
		}
	}
	return nil
}

// requireAuth guards an admin-only handler. It also enforces CSRF on
// state-changing methods. Frontend route protection is never authoritative; this
// backend check is.
func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := a.auth.SessionFromRequest(r)
		if err != nil || session == nil {
			writeErr(w, r, http.StatusUnauthorized, "unauthorized", "A valid admin session is required.")
			return
		}
		if isMutating(r.Method) && !auth.ValidCSRF(session, r) {
			writeErr(w, r, http.StatusForbidden, "csrf_required", "A valid CSRF token is required.")
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		next(w, r.WithContext(ctx))
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// --- session -------------------------------------------------------------

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	session, err := a.auth.SessionFromRequest(r)
	if err != nil || session == nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          session.User,
		"csrf":          session.CSRF,
		"expiresAt":     session.ExpiresAt.UTC(),
	})
}

func (a *App) handleCSRF(w http.ResponseWriter, r *http.Request) {
	session, err := a.auth.SessionFromRequest(r)
	if err != nil || session == nil {
		writeErr(w, r, http.StatusUnauthorized, "unauthorized", "A valid admin session is required.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"csrf": session.CSRF})
}

// --- login ---------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !a.loginLimiter.Allow("login:" + ip).Allowed {
		w.Header().Set("Retry-After", "60")
		writeErr(w, r, http.StatusTooManyRequests, "rate_limited", "Too many login attempts. Please retry later.")
		return
	}

	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	a.mu.RLock()
	expectedUser := a.cfg.AdminUser
	expectedHash := a.cfg.AdminPasswordHash
	a.mu.RUnlock()

	if expectedUser == "" || expectedHash == "" {
		writeErr(w, r, http.StatusServiceUnavailable, "admin_not_configured", "The admin account is not configured on this demo instance.")
		return
	}

	if !auth.VerifyPassword(expectedHash, req.Password) || req.Username != expectedUser {
		writeErr(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password.")
		return
	}

	session, err := a.auth.CreateSession(r.Context(), expectedUser)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "session_failed", "Could not create a session.")
		return
	}
	a.auth.SetCookie(w, session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          session.User,
		"csrf":          session.CSRF,
		"expiresAt":     session.ExpiresAt.UTC(),
	})
}

// --- logout --------------------------------------------------------------

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if session, err := a.auth.SessionFromRequest(r); err == nil && session != nil {
		_ = a.auth.DeleteSession(r.Context(), session.ID)
	}
	a.auth.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

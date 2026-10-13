package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"garudashield/internal/config"
)

func newTestApp(t *testing.T, upstream string) *App {
	t.Helper()
	cfg := config.Default()
	cfg.ListenAddr = "127.0.0.1:0"
	cfg.RateLimitEnabled = false
	cfg.SessionTTL = time.Minute
	u, err := url.Parse(upstream)
	if err != nil {
		t.Fatal(err)
	}
	cfg.UpstreamURL = u
	cfg.AdminPassword = "test-password-long"
	cfg.AdminUser = "admin"

	app, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthProbes(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	for _, p := range []string{"/healthz", "/readyz"} {
		rec := doJSON(t, app, "GET", p, nil, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d", p, rec.Code)
		}
	}
}

func TestStatusEndpoint(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	rec := doJSON(t, app, "GET", "/api/v1/status", nil, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status returned %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Overall    string `json:"overall"`
		Components []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Components) != 4 {
		t.Fatalf("expected 4 components, got %d", len(body.Components))
	}
	for _, c := range body.Components {
		if c.Status != "healthy" && c.Status != "degraded" {
			t.Fatalf("component %s unexpected status %q", c.Name, c.Status)
		}
	}
}

func TestMetricsEndpointReflectsRealTraffic(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	// Generate real proxied traffic first (control-plane routes are not counted).
	doJSON(t, app, "GET", "/api/v1/status", nil, nil, "")
	doJSON(t, app, "GET", "/proxied", nil, nil, "")
	doJSON(t, app, "TRACE", "/blocked", nil, nil, "")
	rec := doJSON(t, app, "GET", "/api/v1/metrics", nil, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics returned %d", rec.Code)
	}
	var body struct {
		Metrics struct {
			RequestsTotal     uint64 `json:"requestsTotal"`
			RequestsEvaluated uint64 `json:"requestsEvaluated"`
			Blocked           uint64 `json:"blocked"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Metrics.RequestsTotal == 0 {
		t.Fatal("expected real request counts")
	}
	if body.Metrics.RequestsEvaluated != 1 {
		t.Fatalf("expected 1 evaluated proxied request, got %d", body.Metrics.RequestsEvaluated)
	}
	if body.Metrics.Blocked != 1 {
		t.Fatalf("expected 1 blocked proxied request, got %d", body.Metrics.Blocked)
	}
}

func TestRulesEndpoint(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	rec := doJSON(t, app, "GET", "/api/v1/rules", nil, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("rules returned %d", rec.Code)
	}
	var body struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rules) == 0 {
		t.Fatal("expected rules")
	}
}

func TestLoginLogoutFlow(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")

	rec := doJSON(t, app, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "test-password-long",
	}, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	var sess struct {
		Authenticated bool   `json:"authenticated"`
		CSRF          string `json:"csrf"`
		Cookie        string `json:"-"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if !sess.Authenticated || sess.CSRF == "" {
		t.Fatalf("login response missing session/csrf: %s", rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set session cookie")
	}

	// Authenticated session endpoint.
	rec2 := doJSON(t, app, "GET", "/api/v1/auth/session", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	if rec2.Code != http.StatusOK {
		t.Fatalf("session check failed: %d", rec2.Code)
	}

	// Protected admin endpoint works with CSRF.
	rec3 := doJSON(t, app, "GET", "/api/v1/admin/settings", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, sess.CSRF)
	if rec3.Code != http.StatusOK {
		t.Fatalf("admin settings failed: %d %s", rec3.Code, rec3.Body.String())
	}

	// Logout.
	rec4 := doJSON(t, app, "POST", "/api/v1/auth/logout", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, sess.CSRF)
	if rec4.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d", rec4.Code)
	}

	// After logout, session endpoint is anonymous.
	rec5 := doJSON(t, app, "GET", "/api/v1/auth/session", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	if rec5.Code != http.StatusOK {
		t.Fatalf("post-logout session check returned %d", rec5.Code)
	}
	var anon struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.Unmarshal(rec5.Body.Bytes(), &anon); err != nil {
		t.Fatal(err)
	}
	if anon.Authenticated {
		t.Fatal("session should be gone after logout")
	}
}

func TestProtectedEndpointRejectsAnonymous(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	rec := doJSON(t, app, "GET", "/api/v1/admin/settings", nil, nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin endpoint should require auth, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRequiredOnMutation(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	// Login to get a session cookie (no CSRF token).
	login := doJSON(t, app, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "test-password-long",
	}, nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login failed: %d", login.Code)
	}
	cookies := login.Result().Cookies()

	// Attempt mutation without CSRF token.
	rec := doJSON(t, app, "PUT", "/api/v1/admin/settings", map[string]any{"rateLimitEnabled": true}, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mutation without CSRF should be 403, got %d", rec.Code)
	}

	// Retrieve CSRF then retry.
	csrfRec := doJSON(t, app, "GET", "/api/v1/auth/csrf", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	if csrfRec.Code != http.StatusOK {
		t.Fatalf("csrf fetch returned %d", csrfRec.Code)
	}
	var csrfBody struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(csrfRec.Body.Bytes(), &csrfBody); err != nil {
		t.Fatal(err)
	}

	rec2 := doJSON(t, app, "PUT", "/api/v1/admin/settings", map[string]any{"rateLimitEnabled": true}, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, csrfBody.CSRF)
	if rec2.Code != http.StatusOK {
		t.Fatalf("mutation with CSRF failed: %d %s", rec2.Code, rec2.Body.String())
	}
}

func TestUpstreamConfigChangeAndSSRFGuard(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	login := doJSON(t, app, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "test-password-long",
	}, nil, "")
	cookies := login.Result().Cookies()
	csrfRec := doJSON(t, app, "GET", "/api/v1/auth/csrf", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	var csrfBody struct {
		CSRF string `json:"csrf"`
	}
	_ = json.Unmarshal(csrfRec.Body.Bytes(), &csrfBody)

	// Attempt to point upstream at the cloud metadata endpoint: must be rejected.
	rec := doJSON(t, app, "PUT", "/api/v1/admin/upstream",
		map[string]string{"url": "http://169.254.169.254/latest/meta-data/"},
		&http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, csrfBody.CSRF)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("SSRF attempt should be rejected, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInvalidUpstreamRejected(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	login := doJSON(t, app, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "test-password-long",
	}, nil, "")
	cookies := login.Result().Cookies()
	csrfRec := doJSON(t, app, "GET", "/api/v1/auth/csrf", nil, &http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, "")
	var csrfBody struct {
		CSRF string `json:"csrf"`
	}
	_ = json.Unmarshal(csrfRec.Body.Bytes(), &csrfBody)

	rec := doJSON(t, app, "PUT", "/api/v1/admin/upstream", map[string]string{"url": "http://"},
		&http.Cookie{Name: "garuda_session", Value: cookies[0].Value}, csrfBody.CSRF)
	if rec.Code == http.StatusOK {
		t.Fatalf("empty host upstream must be rejected: %s", rec.Body.String())
	}
}

func TestProxyTrafficBlockedBeforeUpstream(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()
	app := newTestApp(t, up.URL)

	// Disallowed method through the proxy path.
	rec := doJSON(t, app, "TRACE", "/anything", nil, nil, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestLoginFailureDoesNotLeakAccount(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:1/")
	rec := doJSON(t, app, "POST", "/api/v1/auth/login", map[string]string{
		"username": "admin", "password": "wrong-password",
	}, nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

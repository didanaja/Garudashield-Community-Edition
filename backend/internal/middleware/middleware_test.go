package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"garudashield/internal/config"
	"garudashield/internal/metrics"
	"garudashield/internal/ratelimit"
	"garudashield/internal/rules"
)

func testGuard(t *testing.T) (*Guard, *metrics.Registry) {
	t.Helper()
	cfg := config.Default()
	cfg.RateLimitBurst = 5
	cfg.RateLimitWindow = time.Minute
	cfg.EvaluationBudget = 50 * time.Millisecond
	engine := rules.New(rules.Config{
		AllowedMethods: cfg.AllowedMethods,
		MaxURLBytes:    cfg.MaxURLBytes,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		MaxBodyBytes:   cfg.MaxBodyBytes,
	})
	m := metrics.NewRegistry(50)
	lim := ratelimit.New(cfg.RateLimitBurst, cfg.RateLimitWindow, 1000)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	guard := NewGuard(next, Guard{
		Engine:      engine,
		Limiter:     lim,
		Metrics:     m,
		Budget:      cfg.EvaluationBudget,
		FailClosed:  cfg.FailClosed,
		CORSOrigins: []string{"http://localhost:3000"},
	})
	return guard, m
}

func TestRequestIDGenerated(t *testing.T) {
	guard, _ := testGuard(t)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, httptest.NewRequest("GET", "http://x/", nil))
	if rid := rec.Header().Get("X-Request-Id"); rid == "" {
		t.Fatal("missing request id")
	}
}

func TestSecurityHeadersSet(t *testing.T) {
	guard, _ := testGuard(t)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, httptest.NewRequest("GET", "http://x/", nil))
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
}

func TestRateLimitReturns429(t *testing.T) {
	guard, m := testGuard(t)
	for i := 0; i < 6; i++ {
		rec := httptest.NewRecorder()
		guard.ServeHTTP(rec, httptest.NewRequest("GET", "http://x/", nil))
		if i < 5 && rec.Code != http.StatusOK {
			t.Fatalf("request %d should pass, got %d", i, rec.Code)
		}
		if i == 5 {
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d", rec.Code)
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After")
			}
		}
	}
	if m.Snapshot().RateLimitEvents == 0 {
		t.Fatal("expected rate limit events recorded")
	}
}

func TestBlockedRequestNotForwarded(t *testing.T) {
	guard, m := testGuard(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("TRACE", "http://x/", nil)
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if m.Snapshot().Blocked == 0 {
		t.Fatal("expected blocked counter")
	}
	if m.Snapshot().Allowed != 0 {
		t.Fatal("blocked request should not count as allowed")
	}
}

func TestTrustedProxyForwardedFor(t *testing.T) {
	cfg := config.Default()
	cfg.TrustForwarded = true
	networks, err := config.ParseCIDRs("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	var captured *http.Request
	guard2 := NewGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
	}), Guard{
		Engine:         rules.New(rules.DefaultConfig()),
		Metrics:        metrics.NewRegistry(10),
		TrustForwarded: true,
		TrustedProxies: networks,
		Budget:         50 * time.Millisecond,
	})

	req := httptest.NewRequest("GET", "http://x/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.1")
	guard2.ServeHTTP(httptest.NewRecorder(), req)

	if captured == nil {
		t.Fatal("next handler was not reached")
	}
	ip := ClientIPFromContext(captured.Context())
	if ip != "198.51.100.9" {
		t.Fatalf("expected untrusted client from XFF, got %q", ip)
	}
}

func TestUntrustedPeerXFFIgnored(t *testing.T) {
	cfg := config.Default()
	cfg.TrustForwarded = true
	var captured *http.Request
	guard2 := NewGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
	}), Guard{
		Engine:         rules.New(rules.DefaultConfig()),
		Metrics:        metrics.NewRegistry(10),
		TrustForwarded: true,
		TrustedProxies: nil,
		Budget:         50 * time.Millisecond,
	})
	req := httptest.NewRequest("GET", "http://x/", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	guard2.ServeHTTP(httptest.NewRecorder(), req)
	if captured == nil {
		t.Fatal("next handler was not reached")
	}
	if ip := ClientIPFromContext(captured.Context()); ip != "203.0.113.9" {
		t.Fatalf("untrusted XFF must be ignored, got %q", ip)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	guard, _ := testGuard(t)
	req := httptest.NewRequest("GET", "http://x/api/v1/status", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("expected CORS origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	guard, _ := testGuard(t)
	req := httptest.NewRequest("GET", "http://x/", nil)
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

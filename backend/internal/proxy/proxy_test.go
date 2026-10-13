package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func mustUpstream(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestValidateUpstreamRejectsBadSchemes(t *testing.T) {
	cases := []string{"ftp://example.com", "file:///etc/passwd", "gopher://x"}
	for _, c := range cases {
		u := mustUpstream(t, c)
		if err := ValidateUpstream(u, true); err == nil {
			t.Fatalf("expected %s to be rejected", c)
		}
	}
}

func TestValidateUpstreamRejectsMetadata(t *testing.T) {
	u := mustUpstream(t, "http://169.254.169.254/latest/meta-data/")
	if err := ValidateUpstream(u, true); err == nil {
		t.Fatal("cloud metadata endpoint must be rejected")
	}
}

func TestValidateUpstreamRejectsCredentials(t *testing.T) {
	u := mustUpstream(t, "http://user:pass@example.com/")
	if err := ValidateUpstream(u, true); err == nil {
		t.Fatal("upstream with userinfo must be rejected")
	}
}

func TestValidateUpstreamPrivateGating(t *testing.T) {
	u := mustUpstream(t, "http://127.0.0.1:8099/")
	if err := ValidateUpstream(u, true); err != nil {
		t.Fatalf("loopback allowed when allowPrivate=true: %v", err)
	}
	if err := ValidateUpstream(u, false); err == nil {
		t.Fatal("loopback must be rejected when allowPrivate=false")
	}
}

func TestProxyForwardsToConfiguredUpstream(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Upstream", "hit")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("upstream-body"))
	}))
	defer up.Close()

	p, err := New(Options{
		UpstreamURL:        mustUpstream(t, up.URL),
		Timeout:            2 * time.Second,
		ResponseHeaderWait: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "http://frontend.local/api/things", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
	if rec.Body.String() != "upstream-body" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestProxyStripsClientForwardingHeaders(t *testing.T) {
	var seenXFF, seenXFH string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenXFF = r.Header.Get("X-Forwarded-For")
		seenXFH = r.Header.Get("X-Forwarded-Host")
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()

	p, err := New(Options{UpstreamURL: mustUpstream(t, up.URL), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "http://frontend.local/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if seenXFF != "" {
		t.Fatalf("client-supplied XFF must be replaced, got %q", seenXFF)
	}
	if seenXFH != "frontend.local" {
		t.Fatalf("expected X-Forwarded-Host to be frontend.local, got %q", seenXFH)
	}
}

func TestProxyErrorHandlerIsSanitized(t *testing.T) {
	u := mustUpstream(t, "http://127.0.0.1:1/")
	p, err := New(Options{UpstreamURL: u, Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://frontend.local/", nil)
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body == "" || contains(body, "127.0.0.1:1") {
		t.Fatalf("error response leaks internal address: %q", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

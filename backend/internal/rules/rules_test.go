package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newReq(method, target string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = "203.0.113.10:54321"
	return r
}

func TestMethodAllowlist(t *testing.T) {
	e := New(DefaultConfig())
	d := e.Evaluate(context.Background(), newReq("TRACE", "/"))
	if !d.Blocked || d.RuleID != "L7-METHOD" {
		t.Fatalf("expected TRACE blocked by L7-METHOD, got %+v", d)
	}
}

func TestURLLengthLimit(t *testing.T) {
	e := New(DefaultConfig())
	long := "/" + strings.Repeat("a", 3000)
	d := e.Evaluate(context.Background(), newReq("GET", long))
	if !d.Blocked || d.RuleID != "L7-URL-LEN" {
		t.Fatalf("expected URL length block, got %+v", d)
	}
}

func TestPathTraversal(t *testing.T) {
	e := New(DefaultConfig())
	d := e.Evaluate(context.Background(), newReq("GET", "/%2e%2e/etc/passwd"))
	if !d.Blocked || d.RuleID != "L7-PATH-TRAVERSAL" {
		t.Fatalf("expected traversal block, got %+v", d)
	}
}

func TestSQLInjectionSignature(t *testing.T) {
	e := New(DefaultConfig())
	d := e.Evaluate(context.Background(), newReq("GET", "/search?q=union+select+from+users"))
	if !d.Blocked || d.RuleID != "L7-SQLI" {
		t.Fatalf("expected SQLi block, got %+v", d)
	}
}

func TestXSSSignature(t *testing.T) {
	e := New(DefaultConfig())
	d := e.Evaluate(context.Background(), newReq("GET", "/q?x=%3Cscript%3Ealert(1)%3C/script%3E"))
	if !d.Blocked || d.RuleID != "L7-XSS" {
		t.Fatalf("expected XSS block, got %+v", d)
	}
}

func TestScannerUserAgent(t *testing.T) {
	e := New(DefaultConfig())
	r := newReq("GET", "/")
	r.Header.Set("User-Agent", "sqlmap/1.7")
	d := e.Evaluate(context.Background(), r)
	if !d.Blocked || d.RuleID != "L7-SCANNER-UA" {
		t.Fatalf("expected scanner UA block, got %+v", d)
	}
}

func TestDenyPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DenyPaths = []string{"/admin"}
	e := New(cfg)
	d := e.Evaluate(context.Background(), newReq("GET", "/admin/users"))
	if !d.Blocked || d.RuleID != "L7-DENY-PATH" {
		t.Fatalf("expected deny path block, got %+v", d)
	}
}

func TestAllowPathBypassesPatterns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AllowPaths = []string{"/internal"}
	e := New(cfg)
	d := e.Evaluate(context.Background(), newReq("GET", "/internal/%2e%2e"))
	if d.Blocked {
		t.Fatalf("expected allowlisted path to pass, got %+v", d)
	}
}

func TestCleanRequestPasses(t *testing.T) {
	e := New(DefaultConfig())
	d := e.Evaluate(context.Background(), newReq("GET", "/products?page=2"))
	if d.Blocked {
		t.Fatalf("expected clean request to pass, got %+v", d)
	}
}

func TestContextCancellationBlocks(t *testing.T) {
	e := New(DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := e.Evaluate(ctx, newReq("GET", "/"))
	if !d.Blocked || d.RuleID != "L7-BUDGET" {
		t.Fatalf("expected budget block on cancelled context, got %+v", d)
	}
}

func TestRulesMetadata(t *testing.T) {
	e := New(DefaultConfig())
	rules := e.Rules()
	if len(rules) == 0 {
		t.Fatal("expected rule metadata")
	}
	for _, r := range rules {
		if r.ID == "" || r.Name == "" {
			t.Fatalf("rule missing metadata: %+v", r)
		}
	}
}

func TestBodyLengthLimit(t *testing.T) {
	e := New(DefaultConfig())
	r := newReq("POST", "/upload")
	r.ContentLength = 10 << 20
	d := e.Evaluate(context.Background(), r)
	if !d.Blocked || d.RuleID != "L7-BODY-LEN" {
		t.Fatalf("expected body length block, got %+v", d)
	}
}

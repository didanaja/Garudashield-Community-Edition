// Package rules implements the educational Layer-7 request evaluation engine.
//
// The engine is deliberately small and explainable. It is a DEMONSTRATION
// pattern matcher, not a production WAF. It does not claim OWASP Top 10
// coverage and its detections are intentionally limited to a handful of
// documented, well-understood patterns so that observers can read and reason
// about every decision it makes.
package rules

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// Action is the verdict a rule assigns.
type Action string

const (
	ActionBlock Action = "block"
	ActionAllow Action = "allow"
)

// Category groups rules for documentation and the dashboard.
type Category string

const (
	CategoryMethod    Category = "method"
	CategorySize      Category = "size"
	CategoryPattern   Category = "pattern"
	CategoryAccessList Category = "access-list"
)

// RuleInfo is the static, self-describing metadata for a rule.
type RuleInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Action      Action   `json:"action"`
	Enabled     bool     `json:"enabled"`
}

// Decision is the outcome of evaluating a request.
type Decision struct {
	Blocked bool
	Status  int
	RuleID  string
	Reason  string
}

// Config parameterises the rule engine.
type Config struct {
	AllowedMethods     []string
	MaxURLBytes        int
	MaxHeaderBytes     int
	MaxBodyBytes       int64
	DenyPaths          []string
	AllowPaths         []string
	// SuspiciousUserAgents are matched case-insensitively as substrings.
	SuspiciousUserAgents []string
}

type compiledRule struct {
	info RuleInfo
	re   *regexp.Regexp
}

// Engine evaluates HTTP requests against the demo rule set.
type Engine struct {
	allowedMethods map[string]bool
	cfg            Config
	patterns       []compiledRule
	uaScanners     []string
	rules          []RuleInfo
}

// DefaultConfig returns the built-in demo rule configuration.
func DefaultConfig() Config {
	return Config{
		AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		MaxURLBytes:    2048,
		MaxHeaderBytes: 32 << 10,
		MaxBodyBytes:   1 << 20,
		SuspiciousUserAgents: []string{
			"sqlmap", "nikto", "nmap", "masscan", "acunetix", "nessus",
			"dirbuster", "gobuster", "wpscan", "zgrab", "nuclei",
		},
	}
}

// New builds a rule engine from cfg. Invalid regex patterns are skipped rather
// than panicking so a misconfiguration degrades safely.
func New(cfg Config) *Engine {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = DefaultConfig().AllowedMethods
	}
	methods := make(map[string]bool, len(cfg.AllowedMethods))
	for _, m := range cfg.AllowedMethods {
		methods[strings.ToUpper(strings.TrimSpace(m))] = true
	}

	type patternDef struct {
		id, name, desc string
		re             string
	}
	defs := []patternDef{
		{"L7-PATH-TRAVERSAL", "Path traversal", "Rejects directory traversal sequences in the decoded request path.", `(?:\.\./|\.\.\\|%2e%2e(?:%2f|/)|\.\.%5c)`},
		{"L7-NULL-BYTE", "Null byte injection", "Rejects null-byte injection attempts in the request target.", `%00|\x00`},
		{"L7-SQLI", "SQL injection signature", "Rejects a small set of classic SQL comment/union signatures (demonstrational only).", `(?i)(union[\s/*]+select|or\s+1\s*=\s*1|'\s*or\s*'1'\s*=\s*'1|sleep\s*\(\s*\d+\s*\)|information_schema)`},
		{"L7-XSS", "Reflected XSS signature", "Rejects obvious inline script or javascript: payloads in the request target (demonstrational only).", `(?i)(<script|javascript:|onerror\s*=|onload\s*=|%3cscript)`},
		{"L7-SENSITIVE-FILE", "Sensitive file probe", "Rejects probes for well-known sensitive files.", `(?i)(/\.git/|/\.env|/wp-config\.php|/\.svn/|/\.aws/credentials)`},
	}

	var patterns []compiledRule
	for _, d := range defs {
		re, err := regexp.Compile(d.re)
		if err != nil {
			continue
		}
		patterns = append(patterns, compiledRule{
			info: RuleInfo{ID: d.id, Name: d.name, Description: d.desc, Category: CategoryPattern, Action: ActionBlock, Enabled: true},
			re:   re,
		})
	}

	e := &Engine{
		allowedMethods: methods,
		cfg:            cfg,
		patterns:       patterns,
		uaScanners:     cfg.SuspiciousUserAgents,
	}

	e.rules = []RuleInfo{
		{ID: "L7-METHOD", Name: "HTTP method allowlist", Description: "Only explicitly allowed HTTP methods are forwarded.", Category: CategoryMethod, Action: ActionBlock, Enabled: true},
		{ID: "L7-URL-LEN", Name: "Maximum URL length", Description: "Requests whose request-target exceeds the configured limit are rejected.", Category: CategorySize, Action: ActionBlock, Enabled: true},
		{ID: "L7-HEADER-LEN", Name: "Maximum header size", Description: "Requests whose header block exceeds the configured limit are rejected.", Category: CategorySize, Action: ActionBlock, Enabled: true},
		{ID: "L7-BODY-LEN", Name: "Maximum body size", Description: "Requests declaring a Content-Length above the limit are rejected.", Category: CategorySize, Action: ActionBlock, Enabled: true},
		{ID: "L7-DENY-PATH", Name: "Path denylist", Description: "Configured path prefixes are always blocked.", Category: CategoryAccessList, Action: ActionBlock, Enabled: len(cfg.DenyPaths) > 0},
		{ID: "L7-ALLOW-PATH", Name: "Path allowlist", Description: "Configured path prefixes bypass pattern scanning for trusted internal routes (never forwarded beyond this process).", Category: CategoryAccessList, Action: ActionAllow, Enabled: len(cfg.AllowPaths) > 0},
		{ID: "L7-SCANNER-UA", Name: "Scanner user-agent", Description: "Rejects a small list of known scanner/automation user-agents.", Category: CategoryPattern, Action: ActionBlock, Enabled: len(cfg.SuspiciousUserAgents) > 0},
	}
	for _, p := range patterns {
		e.rules = append(e.rules, p.info)
	}

	return e
}

// Rules returns the active rule metadata for the API.
func (e *Engine) Rules() []RuleInfo {
	out := make([]RuleInfo, len(e.rules))
	copy(out, e.rules)
	return out
}

// Evaluate applies the rule set. It checks ctx for cancellation so callers can
// enforce an evaluation time budget; a cancelled context yields a block.
func (e *Engine) Evaluate(ctx context.Context, r *http.Request) Decision {
	if err := ctx.Err(); err != nil {
		return Decision{Blocked: true, Status: http.StatusServiceUnavailable, RuleID: "L7-BUDGET", Reason: "evaluation budget exceeded"}
	}

	// Avoid scanning expensive request bodies here; the body limit is enforced
	// by the middleware around the proxy.
	target := r.URL.EscapedPath()
	if raw := r.URL.RawQuery; raw != "" {
		target += "?" + raw
	}
	// Also account for the raw request URI to catch unparsed payloads.
	if r.RequestURI != "" {
		target = r.RequestURI
	}

	method := strings.ToUpper(r.Method)
	if !e.allowedMethods[method] {
		return Decision{Blocked: true, Status: http.StatusMethodNotAllowed, RuleID: "L7-METHOD", Reason: "method not allowed"}
	}

	if len(target) > e.cfg.MaxURLBytes {
		return Decision{Blocked: true, Status: http.StatusRequestURITooLong, RuleID: "L7-URL-LEN", Reason: "request target too long"}
	}

	headerBytes := 0
	for k, vs := range r.Header {
		headerBytes += len(k) + 2
		for _, v := range vs {
			headerBytes += len(v) + 2
		}
	}
	if headerBytes > e.cfg.MaxHeaderBytes {
		return Decision{Blocked: true, Status: http.StatusRequestHeaderFieldsTooLarge, RuleID: "L7-HEADER-LEN", Reason: "request headers too large"}
	}

	if r.ContentLength > e.cfg.MaxBodyBytes {
		return Decision{Blocked: true, Status: http.StatusRequestEntityTooLarge, RuleID: "L7-BODY-LEN", Reason: "request body too large"}
	}

	path := r.URL.Path

	for _, p := range e.cfg.DenyPaths {
		if strings.HasPrefix(path, p) {
			return Decision{Blocked: true, Status: http.StatusForbidden, RuleID: "L7-DENY-PATH", Reason: "path denied by policy"}
		}
	}

	// Allowlisted paths skip pattern scanning but are still subject to the size
	// and method checks above. This models trusted internal endpoints, not an
	// open-proxy bypass.
	for _, p := range e.cfg.AllowPaths {
		if strings.HasPrefix(path, p) {
			return Decision{Blocked: false, Status: http.StatusOK, RuleID: "L7-ALLOW-PATH", Reason: "path allowlisted"}
		}
	}

	lowerTarget := strings.ToLower(target)
	// Query strings encode spaces as '+'; normalise so signature matching sees
	// the same bytes a human would see in a browser.
	scanTarget := strings.ReplaceAll(lowerTarget, "+", " ")

	ua := strings.ToLower(r.UserAgent())
	if ua != "" {
		for _, scanner := range e.uaScanners {
			if strings.Contains(ua, strings.ToLower(scanner)) {
				return Decision{Blocked: true, Status: http.StatusForbidden, RuleID: "L7-SCANNER-UA", Reason: "known scanner user-agent"}
			}
		}
	}

	for _, p := range e.patterns {
		if p.re.MatchString(scanTarget) {
			return Decision{Blocked: true, Status: http.StatusForbidden, RuleID: p.info.ID, Reason: p.info.Name + " matched"}
		}
	}

	return Decision{Blocked: false, Status: http.StatusOK, RuleID: "", Reason: "passed demonstration ruleset"}
}

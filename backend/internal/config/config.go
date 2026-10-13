// Package config loads and validates the GarudaShield Demo Edition runtime
// configuration. All values come from environment variables so that secrets are
// never baked into the binary. Every default is deliberately conservative and
// suitable only for local demonstrations.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration for the demo engine.
type Config struct {
	// HTTP server
	ListenAddr      string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	// Reverse proxy
	UpstreamURL          *url.URL
	UpstreamTimeout      time.Duration
	ResponseHeaderWait   time.Duration
	MaxBodyBytes         int64
	MaxURLBytes          int
	MaxHeaderBytes       int
	AllowPrivateUpstream bool

	// Security evaluation
	EvaluationBudget time.Duration
	FailClosed       bool

	// Layer 7 rule engine
	AllowedMethods []string
	SuspiciousPatterns []string
	DenyPaths          []string
	AllowPaths         []string

	// Rate limiting
	RateLimitEnabled bool
	RateLimitBurst   int
	RateLimitWindow  time.Duration
	RateLimitMaxKeys int
	TrustedProxies   []*net.IPNet
	TrustForwarded   bool

	// Control-plane API
	CORSOrigins []string
	AppEnv      string

	// Authentication (admin control plane)
	AdminUser         string
	AdminPasswordHash string
	// AdminPassword is an optional development convenience. When set and no
	// hash is supplied, the server derives the hash in memory at startup.
	// Never use the plaintext variable in production.
	AdminPassword     string
	SessionSecret     []byte
	SessionTTL        time.Duration
	LoginMaxAttempts  int
	LoginWindow       time.Duration

	// Storage
	StoreKind string
}

// Default returns a configuration suitable for local development. The upstream
// defaults to a harmless loopback test server so the demo never forwards to an
// arbitrary third party out of the box.
func Default() *Config {
	u, _ := url.Parse("http://127.0.0.1:8099")
	return &Config{
		ListenAddr:      "127.0.0.1:8080",
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,

		UpstreamURL:        u,
		UpstreamTimeout:    10 * time.Second,
		ResponseHeaderWait: 10 * time.Second,
		MaxBodyBytes:       1 << 20, // 1 MiB
		MaxURLBytes:        2048,
		MaxHeaderBytes:     32 << 10, // 32 KiB
		AllowPrivateUpstream: true,   // demo default: community test upstreams run on loopback

		EvaluationBudget: 50 * time.Millisecond,
		FailClosed:       true,

		AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		DenyPaths:      []string{},
		AllowPaths:     []string{},

		RateLimitEnabled: true,
		RateLimitBurst:   60,
		RateLimitWindow:  10 * time.Second,
		RateLimitMaxKeys: 10000,
		TrustForwarded:   false,

		CORSOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AppEnv:      "development",

		AdminUser:        "admin",
		SessionTTL:       8 * time.Hour,
		LoginMaxAttempts: 5,
		LoginWindow:      5 * time.Minute,

		StoreKind: "memory",
	}
}

// Load builds a Config from the process environment, applying Default values for
// any unset variable and validating the result.
func Load() (*Config, error) {
	c := Default()

	c.ListenAddr = env("GARUDA_LISTEN_ADDR", c.ListenAddr)
	c.AppEnv = env("GARUDA_APP_ENV", c.AppEnv)
	c.StoreKind = env("GARUDA_STORE", c.StoreKind)

	if v := env("GARUDA_UPSTREAM_URL", ""); v != "" {
		u, err := url.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid GARUDA_UPSTREAM_URL: %w", err)
		}
		c.UpstreamURL = u
	}

	c.MaxBodyBytes = envInt64("GARUDA_MAX_BODY_BYTES", c.MaxBodyBytes)
	c.MaxURLBytes = envInt("GARUDA_MAX_URL_BYTES", c.MaxURLBytes)
	c.MaxHeaderBytes = envInt("GARUDA_MAX_HEADER_BYTES", c.MaxHeaderBytes)
	c.AllowPrivateUpstream = envBool("GARUDA_ALLOW_PRIVATE_UPSTREAM", c.AllowPrivateUpstream)
	c.EvaluationBudget = envDuration("GARUDA_EVALUATION_BUDGET", c.EvaluationBudget)
	c.FailClosed = envBool("GARUDA_FAIL_CLOSED", c.FailClosed)
	c.UpstreamTimeout = envDuration("GARUDA_UPSTREAM_TIMEOUT", c.UpstreamTimeout)
	c.ResponseHeaderWait = envDuration("GARUDA_RESPONSE_HEADER_TIMEOUT", c.ResponseHeaderWait)

	c.RateLimitEnabled = envBool("GARUDA_RATE_LIMIT_ENABLED", c.RateLimitEnabled)
	c.RateLimitBurst = envInt("GARUDA_RATE_LIMIT_BURST", c.RateLimitBurst)
	c.RateLimitWindow = envDuration("GARUDA_RATE_LIMIT_WINDOW", c.RateLimitWindow)
	c.RateLimitMaxKeys = envInt("GARUDA_RATE_LIMIT_MAX_KEYS", c.RateLimitMaxKeys)
	c.TrustForwarded = envBool("GARUDA_TRUST_FORWARDED", c.TrustForwarded)

	if v := env("GARUDA_TRUSTED_PROXIES", ""); v != "" {
		nets, err := ParseCIDRs(v)
		if err != nil {
			return nil, err
		}
		c.TrustedProxies = nets
	}

	if v := env("GARUDA_CORS_ORIGINS", ""); v != "" {
		c.CORSOrigins = splitCSV(v)
	}
	if v := env("GARUDA_ALLOWED_METHODS", ""); v != "" {
		c.AllowedMethods = splitCSV(v)
	}
	if v := env("GARUDA_DENY_PATHS", ""); v != "" {
		c.DenyPaths = splitCSV(v)
	}
	if v := env("GARUDA_ALLOW_PATHS", ""); v != "" {
		c.AllowPaths = splitCSV(v)
	}

	c.AdminUser = env("GARUDA_ADMIN_USER", c.AdminUser)
	c.AdminPasswordHash = env("GARUDA_ADMIN_PASSWORD_HASH", "")
	c.AdminPassword = env("GARUDA_ADMIN_PASSWORD", "")
	c.SessionTTL = envDuration("GARUDA_SESSION_TTL", c.SessionTTL)
	c.LoginMaxAttempts = envInt("GARUDA_LOGIN_MAX_ATTEMPTS", c.LoginMaxAttempts)
	c.LoginWindow = envDuration("GARUDA_LOGIN_WINDOW", c.LoginWindow)

	if v := env("GARUDA_SESSION_SECRET", ""); v != "" {
		c.SessionSecret = []byte(v)
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate enforces the invariants required for a safe demonstration run.
func (c *Config) Validate() error {
	if c.UpstreamURL == nil || c.UpstreamURL.Host == "" {
		return fmt.Errorf("upstream URL must include a host")
	}
	if c.UpstreamURL.Scheme != "http" && c.UpstreamURL.Scheme != "https" {
		return fmt.Errorf("upstream URL scheme must be http or https")
	}
	if c.MaxBodyBytes <= 0 {
		return fmt.Errorf("max body bytes must be positive")
	}
	if c.EvaluationBudget <= 0 {
		return fmt.Errorf("evaluation budget must be positive")
	}
	if c.RateLimitBurst <= 0 {
		return fmt.Errorf("rate limit burst must be positive")
	}
	if len(c.SessionSecret) > 0 && len(c.SessionSecret) < 16 {
		return fmt.Errorf("session secret must be at least 16 bytes")
	}
	return nil
}

// ParseCIDRs parses a comma or space separated list of CIDR blocks or bare IPs.
func ParseCIDRs(raw string) ([]*net.IPNet, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' })
	var out []*net.IPNet
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.Contains(p, "/") {
			ip := net.ParseIP(p)
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted proxy address %q", p)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", p, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// IsTrustedProxy reports whether the provided address belongs to a configured
// trusted proxy network. This is what allows the engine to honour
// X-Forwarded-For only from infrastructure we control.
func (c *Config) IsTrustedProxy(ip net.IP) bool {
	for _, n := range c.TrustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func splitCSV(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}

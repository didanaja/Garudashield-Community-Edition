// Package api assembles the demo engine into a single http.Handler. Control-plane
// routes live under /api/v1 and the health probes at /healthz and /readyz; every
// other path is handed to the guarded reverse proxy.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"garudashield/internal/auth"
	"garudashield/internal/config"
	"garudashield/internal/metrics"
	"garudashield/internal/middleware"
	"garudashield/internal/proxy"
	"garudashield/internal/ratelimit"
	"garudashield/internal/rules"
	"garudashield/internal/storage"
)

// Version is the demo engine version string.
const Version = "0.1.0-demo"

// App owns the mutable runtime state of the demo engine.
type App struct {
	mu     sync.RWMutex
	cfg    *config.Config
	store  storage.Store
	metrics *metrics.Registry
	auth   *auth.Manager
	logger *slog.Logger

	// Swappable, hot-reloadable components. Reads are lock-free.
	guard   atomic.Pointer[middleware.Guard]
	proxy   atomic.Pointer[proxy.Proxy]
	engine  atomic.Pointer[rules.Engine]
	limiter atomic.Pointer[ratelimit.Limiter]
	upstream atomic.Pointer[url.URL]

	loginLimiter *ratelimit.Limiter
	apiRouter    http.Handler
}

// New builds the application, applying any persisted runtime configuration.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	store := storage.NewMemory(cfg.RateLimitMaxKeys * 4)
	a := &App{
		cfg:     cfg,
		store:   store,
		metrics: metrics.NewRegistry(500),
		logger:  logger,
	}
	a.auth = auth.NewManager(store, cfg.SessionTTL, cfg.AppEnv == "production")
	a.loginLimiter = ratelimit.New(cfg.LoginMaxAttempts, cfg.LoginWindow, 50000)

	// Development convenience: accept a plaintext admin password from the
	// environment and hash it in memory. Prefer GARUDA_ADMIN_PASSWORD_HASH.
	if cfg.AdminPasswordHash == "" && cfg.AdminPassword != "" {
		h, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return nil, err
		}
		cfg.AdminPasswordHash = h
	}

	a.loadPersisted(context.Background())

	if err := a.Rebuild(); err != nil {
		return nil, err
	}
	a.apiRouter = a.routes()
	return a, nil
}

// Store exposes the backing store (used by main for shutdown).
func (a *App) Store() storage.Store { return a.store }

// Metrics exposes the metrics registry.
func (a *App) Metrics() *metrics.Registry { return a.metrics }

// Rebuild reconstructs the rules engine, limiter, proxy and guard from the
// current configuration. It is safe to call at runtime after a settings change.
func (a *App) Rebuild() error {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()

	engine := rules.New(rules.Config{
		AllowedMethods: cfg.AllowedMethods,
		MaxURLBytes:    cfg.MaxURLBytes,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		MaxBodyBytes:   cfg.MaxBodyBytes,
		DenyPaths:      cfg.DenyPaths,
		AllowPaths:     cfg.AllowPaths,
	})

	var limiter *ratelimit.Limiter
	if cfg.RateLimitEnabled {
		limiter = ratelimit.New(cfg.RateLimitBurst, cfg.RateLimitWindow, cfg.RateLimitMaxKeys)
	}

	userProxy, err := proxy.New(proxy.Options{
		UpstreamURL:        cfg.UpstreamURL,
		Timeout:            cfg.UpstreamTimeout,
		ResponseHeaderWait: cfg.ResponseHeaderWait,
	})
	if err != nil {
		return err
	}

	guard := middleware.NewGuard(userProxy, middleware.Guard{
		Engine:         engine,
		Limiter:        limiter,
		Metrics:        a.metrics,
		Logger:         a.logger,
		TrustForwarded: cfg.TrustForwarded,
		TrustedProxies: cfg.TrustedProxies,
		MaxBodyBytes:   cfg.MaxBodyBytes,
		Budget:         cfg.EvaluationBudget,
		FailClosed:     cfg.FailClosed,
		CORSOrigins:    cfg.CORSOrigins,
	})

	a.engine.Store(engine)
	a.limiter.Store(limiter)
	a.proxy.Store(userProxy)
	a.guard.Store(guard)
	a.upstream.Store(cfg.UpstreamURL)
	return nil
}

// Handler returns the fully-assembled application handler.
func (a *App) Handler() http.Handler { return a }

// ServeHTTP routes control-plane traffic and health probes, forwarding
// everything else through the guarded proxy.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/healthz" || p == "/healthz/":
		a.handleHealthz(w, r)
		return
	case p == "/readyz" || p == "/readyz/":
		a.handleReadyz(w, r)
		return
	case p == "/api/v1" || strings.HasPrefix(p, "/api/v1/"):
		a.serveAPI(w, r)
		return
	default:
		guard := a.guard.Load()
		if guard == nil {
			http.Error(w, "engine unavailable", http.StatusServiceUnavailable)
			return
		}
		guard.ServeHTTP(w, r)
	}
}

// --- JSON helpers --------------------------------------------------------

const maxControlBody = 64 << 10 // 64 KiB

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

type apiError struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

func writeErr(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	writeJSON(w, status, apiError{
		Error:     code,
		Message:   msg,
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxControlBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, r, http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	if ip := middleware.ClientIPFromContext(r.Context()); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

func nowUTC() time.Time { return time.Now().UTC() }

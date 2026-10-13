// Package middleware wires the demo engine's defensive layers around the
// reverse proxy: request IDs, structured logging, security headers, restrictive
// CORS, rate limiting, and the fail-closed Layer-7 evaluation step.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"garudashield/internal/metrics"
	"garudashield/internal/ratelimit"
	"garudashield/internal/rules"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxClientIP
)

// RequestIDFromContext returns the request ID assigned by the Guard.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// ClientIPFromContext returns the resolved client address stored by the Guard.
func ClientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxClientIP).(string); ok {
		return v
	}
	return ""
}

// Guard is the composed request-processing pipeline for proxied traffic.
type Guard struct {
	Engine         *rules.Engine
	Limiter        *ratelimit.Limiter
	Metrics        *metrics.Registry
	Logger         *slog.Logger
	TrustForwarded bool
	TrustedProxies []*net.IPNet
	MaxBodyBytes   int64
	Budget         time.Duration
	FailClosed     bool
	CORSOrigins    []string

	next http.Handler
}

// NewGuard constructs a Guard wrapping next.
func NewGuard(next http.Handler, opts Guard) *Guard {
	g := opts
	g.next = next
	if g.Logger == nil {
		g.Logger = slog.Default()
	}
	return &g
}

// ServeHTTP runs the full pipeline.
func (g *Guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	g.Metrics.IncRequestsTotal()

	requestID := newRequestID()
	r = r.WithContext(context.WithValue(r.Context(), ctxRequestID, requestID))
	w.Header().Set("X-Request-Id", requestID)

	clientIP := g.resolveClientIP(r)
	r = r.WithContext(context.WithValue(r.Context(), ctxClientIP, clientIP))

	g.setSecurityHeaders(w)
	if done := g.handleCORS(w, r); done {
		return
	}

	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

	// --- Rate limiting ---------------------------------------------------
	if g.Limiter != nil {
		res := g.Limiter.Allow(clientIP)
		if !res.Allowed {
			g.Metrics.IncRateLimit()
			rec.Header().Set("Retry-After", retryAfterSeconds(res.RetryAfter))
			g.writeError(rec, r, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please retry later.")
			g.finish(r, rec, start, clientIP, metrics.VerdictRateLimited, "RATE-LIMIT", "token bucket exhausted")
			return
		}
	}

	// --- Layer-7 evaluation with fail-closed budget ----------------------
	decision := g.evaluateWithBudget(r)

	switch {
	case decision.TimedOut:
		g.Metrics.IncEvaluationTimeout()
		g.Metrics.IncBlocked()
		g.writeBlocked(rec, r, http.StatusServiceUnavailable, "Security evaluation exceeded its time budget; the request was rejected.", "L7-BUDGET")
		g.finish(r, rec, start, clientIP, metrics.VerdictTimeout, "L7-BUDGET", decision.Reason)
		return
	case decision.Blocked:
		g.Metrics.IncBlocked()
		g.writeBlocked(rec, r, decision.Status, decision.Reason, decision.RuleID)
		g.finish(r, rec, start, clientIP, metrics.VerdictBlocked, decision.RuleID, decision.Reason)
		return
	}

	g.Metrics.IncEvaluated()

	// --- Forward ---------------------------------------------------------
	if g.MaxBodyBytes > 0 && r.Body != nil {
		r.Body = http.MaxBytesReader(rec, r.Body, g.MaxBodyBytes)
	}
	g.next.ServeHTTP(rec, r)

	verdict := metrics.VerdictAllowed
	reason := decision.Reason
	if rec.status >= http.StatusInternalServerError {
		verdict = metrics.VerdictError
		g.Metrics.IncUpstreamError()
	} else {
		g.Metrics.IncAllowed()
	}
	g.finish(r, rec, start, clientIP, verdict, decision.RuleID, reason)
}

type evalResult struct {
	Blocked  bool
	TimedOut bool
	Status   int
	RuleID   string
	Reason   string
}

func (g *Guard) evaluateWithBudget(r *http.Request) evalResult {
	ctx, cancel := context.WithTimeout(r.Context(), g.Budget)
	defer cancel()

	type outcome struct{ d rules.Decision }
	ch := make(chan outcome, 1)
	go func() {
		ch <- outcome{g.Engine.Evaluate(ctx, r)}
	}()

	select {
	case <-ctx.Done():
		if g.FailClosed {
			return evalResult{Blocked: true, TimedOut: true, Status: http.StatusServiceUnavailable, RuleID: "L7-BUDGET", Reason: "security evaluation budget exceeded"}
		}
		// Fail-open is available only for development and is loudly logged.
		g.Logger.Warn("evaluation budget exceeded; failing open (NOT safe for production)",
			"request_id", RequestIDFromContext(r.Context()), "path", r.URL.Path)
		return evalResult{}
	case o := <-ch:
		return evalResult{
			Blocked: o.d.Blocked,
			Status:  o.d.Status,
			RuleID:  o.d.RuleID,
			Reason:  o.d.Reason,
		}
	}
}

func (g *Guard) finish(r *http.Request, rec *responseRecorder, start time.Time, clientIP string, verdict metrics.Verdict, ruleID, reason string) {
	dur := time.Since(start)
	g.Metrics.Record(metrics.Event{
		Timestamp:  time.Now(),
		RequestID:  RequestIDFromContext(r.Context()),
		ClientIP:   clientIP,
		Method:     r.Method,
		Path:       sanitizePath(r.URL.Path),
		Verdict:    verdict,
		Status:     rec.status,
		RuleID:     ruleID,
		Reason:     reason,
		DurationMs: float64(dur.Microseconds()) / 1000.0,
	})
	g.Logger.Info("request",
		"request_id", RequestIDFromContext(r.Context()),
		"client_ip", clientIP,
		"method", r.Method,
		"path", sanitizePath(r.URL.Path),
		"status", rec.status,
		"verdict", verdict,
		"rule", ruleID,
		"duration_ms", float64(dur.Microseconds())/1000.0,
	)
}

func (g *Guard) setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("X-Garuda-Engine", "demo-edition")
}

func (g *Guard) handleCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || !g.originAllowed(origin) {
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// Preflight for a disallowed origin: reject without CORS headers.
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		return false
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-Id")
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Max-Age", "600")

	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func (g *Guard) originAllowed(origin string) bool {
	for _, o := range g.CORSOrigins {
		if strings.EqualFold(strings.TrimRight(o, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

func (g *Guard) resolveClientIP(r *http.Request) string {
	remoteIP := remoteIPOf(r.RemoteAddr)
	if !g.TrustForwarded || remoteIP == nil {
		return remoteIP.String()
	}
	if !g.isTrusted(remoteIP) {
		return remoteIP.String()
	}
	// The immediate peer is a trusted proxy: walk the XFF chain right-to-left
	// and return the first address that is not itself a trusted proxy.
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remoteIP.String()
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}
		if !g.isTrusted(ip) {
			return ip.String()
		}
	}
	return remoteIP.String()
}

func (g *Guard) isTrusted(ip net.IP) bool {
	for _, n := range g.TrustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (g *Guard) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	body := map[string]string{
		"error":     code,
		"message":   message,
		"requestId": RequestIDFromContext(r.Context()),
	}
	_ = json.NewEncoder(w).Encode(body)
}

// writeBlocked returns a sanitized block response that names the rule which
// rejected the request, so the dashboard can explain the decision.
func (g *Guard) writeBlocked(w http.ResponseWriter, r *http.Request, status int, message, rule string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	body := map[string]string{
		"error":     "blocked",
		"message":   message,
		"rule":      rule,
		"requestId": RequestIDFromContext(r.Context()),
	}
	_ = json.NewEncoder(w).Encode(body)
}

// --- helpers -------------------------------------------------------------

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	if rr.wroteHeader {
		return
	}
	rr.status = code
	rr.wroteHeader = true
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if !rr.wroteHeader {
		rr.WriteHeader(http.StatusOK)
	}
	return rr.ResponseWriter.Write(b)
}

// Flush forwards streaming flushes when the underlying writer supports it.
func (rr *responseRecorder) Flush() {
	if f, ok := rr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func remoteIPOf(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(host)
}

func retryAfterSeconds(d time.Duration) string {
	secs := int(d.Round(time.Second).Seconds())
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

func sanitizePath(p string) string {
	if p == "" {
		return "/"
	}
	if len(p) > 256 {
		return p[:256] + "…"
	}
	return p
}

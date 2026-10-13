// Package metrics maintains real, locally-observed demonstration counters and a
// bounded ring buffer of sanitized security events. Nothing in this package is
// fabricated: every number is incremented by genuine request handling.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Verdict describes the outcome the engine assigned to a request.
type Verdict string

const (
	VerdictAllowed     Verdict = "allowed"
	VerdictBlocked     Verdict = "blocked"
	VerdictRateLimited Verdict = "rate_limited"
	VerdictTimeout     Verdict = "evaluation_timeout"
	VerdictError       Verdict = "error"
)

// Event is a single sanitized security log entry. It contains no request bodies,
// credentials, cookies, or raw query strings.
type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"requestId"`
	ClientIP   string    `json:"clientIp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Verdict    Verdict   `json:"verdict"`
	Status     int       `json:"status"`
	RuleID     string    `json:"ruleId,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	DurationMs float64   `json:"durationMs"`
	Upstream   string    `json:"upstream,omitempty"`
}

// Snapshot is an immutable view of the counters at a point in time.
type Snapshot struct {
	RequestsTotal     uint64    `json:"requestsTotal"`
	RequestsEvaluated uint64    `json:"requestsEvaluated"`
	Allowed           uint64    `json:"allowed"`
	Blocked           uint64    `json:"blocked"`
	RateLimitEvents   uint64    `json:"rateLimitEvents"`
	EvaluationTimeouts uint64   `json:"evaluationTimeouts"`
	UpstreamErrors    uint64    `json:"upstreamErrors"`
	UptimeSeconds     int64     `json:"uptimeSeconds"`
	StartedAt         time.Time `json:"startedAt"`
}

// Registry stores counters and the recent event log.
type Registry struct {
	requestsTotal      atomic.Uint64
	requestsEvaluated  atomic.Uint64
	allowed            atomic.Uint64
	blocked            atomic.Uint64
	rateLimitEvents    atomic.Uint64
	evaluationTimeouts atomic.Uint64
	upstreamErrors     atomic.Uint64

	startedAt time.Time

	mu       sync.Mutex
	recent   []Event
	maxRecent int
}

// NewRegistry creates a registry retaining up to maxRecent events.
func NewRegistry(maxRecent int) *Registry {
	if maxRecent <= 0 {
		maxRecent = 200
	}
	return &Registry{
		startedAt: time.Now(),
		recent:    make([]Event, 0, maxRecent),
		maxRecent: maxRecent,
	}
}

// IncRequestsTotal counts every request that reaches the engine.
func (r *Registry) IncRequestsTotal() { r.requestsTotal.Add(1) }

// IncEvaluated counts requests that entered the layer-7 rule evaluation.
func (r *Registry) IncEvaluated() { r.requestsEvaluated.Add(1) }

// IncAllowed counts requests that passed evaluation and were forwarded.
func (r *Registry) IncAllowed() { r.allowed.Add(1) }

// IncBlocked counts requests rejected by a rule or by fail-closed behaviour.
func (r *Registry) IncBlocked() { r.blocked.Add(1) }

// IncRateLimit counts requests rejected by the rate limiter.
func (r *Registry) IncRateLimit() { r.rateLimitEvents.Add(1) }

// IncEvaluationTimeout counts evaluation-budget overruns.
func (r *Registry) IncEvaluationTimeout() { r.evaluationTimeouts.Add(1) }

// IncUpstreamError counts failed upstream connection or timeout events.
func (r *Registry) IncUpstreamError() { r.upstreamErrors.Add(1) }

// Record appends a sanitized event to the bounded recent log.
func (r *Registry) Record(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.recent) >= r.maxRecent {
		// Drop the oldest entry.
		copy(r.recent, r.recent[1:])
		r.recent[len(r.recent)-1] = e
		return
	}
	r.recent = append(r.recent, e)
}

// Recent returns up to limit most-recent events, newest first.
func (r *Registry) Recent(limit int) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > len(r.recent) {
		limit = len(r.recent)
	}
	out := make([]Event, 0, limit)
	for i := len(r.recent) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.recent[i])
	}
	return out
}

// Snapshot returns the current counter values.
func (r *Registry) Snapshot() Snapshot {
	return Snapshot{
		RequestsTotal:      r.requestsTotal.Load(),
		RequestsEvaluated:  r.requestsEvaluated.Load(),
		Allowed:            r.allowed.Load(),
		Blocked:            r.blocked.Load(),
		RateLimitEvents:    r.rateLimitEvents.Load(),
		EvaluationTimeouts: r.evaluationTimeouts.Load(),
		UpstreamErrors:     r.upstreamErrors.Load(),
		UptimeSeconds:      int64(time.Since(r.startedAt).Seconds()),
		StartedAt:          r.startedAt,
	}
}

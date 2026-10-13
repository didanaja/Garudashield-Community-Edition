package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"garudashield/internal/config"
	"garudashield/internal/proxy"
)

const persistedKey = "runtime:config"

// persistedConfig is the subset of runtime configuration that survives a restart
// when a persistent Store is configured. Secrets are never persisted.
type persistedConfig struct {
	UpstreamURL      string `json:"upstreamUrl,omitempty"`
	RateLimitEnabled bool   `json:"rateLimitEnabled"`
	RateLimitBurst   int    `json:"rateLimitBurst"`
	RateLimitWindowS int    `json:"rateLimitWindowSeconds"`
	MaxBodyBytes     int64  `json:"maxBodyBytes"`
	MaxURLBytes      int    `json:"maxUrlBytes"`
	MaxHeaderBytes   int    `json:"maxHeaderBytes"`
	EvaluationBudgetMs int  `json:"evaluationBudgetMs"`
	FailClosed       bool   `json:"failClosed"`
}

func (a *App) loadPersisted(ctx context.Context) {
	blob, err := a.store.Get(ctx, persistedKey)
	if err != nil {
		return
	}
	var p persistedConfig
	if err := json.Unmarshal(blob, &p); err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.UpstreamURL != "" {
		if u, err := url.Parse(p.UpstreamURL); err == nil {
			a.cfg.UpstreamURL = u
		}
	}
	a.cfg.RateLimitEnabled = p.RateLimitEnabled
	a.cfg.RateLimitBurst = p.RateLimitBurst
	a.cfg.RateLimitWindow = time.Duration(p.RateLimitWindowS) * time.Second
	a.cfg.MaxBodyBytes = p.MaxBodyBytes
	a.cfg.MaxURLBytes = p.MaxURLBytes
	a.cfg.MaxHeaderBytes = p.MaxHeaderBytes
	a.cfg.EvaluationBudget = time.Duration(p.EvaluationBudgetMs) * time.Millisecond
	a.cfg.FailClosed = p.FailClosed
}

func (a *App) persist(ctx context.Context) {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	p := persistedConfig{
		UpstreamURL:        cfg.UpstreamURL.String(),
		RateLimitEnabled:   cfg.RateLimitEnabled,
		RateLimitBurst:     cfg.RateLimitBurst,
		RateLimitWindowS:   int(cfg.RateLimitWindow.Seconds()),
		MaxBodyBytes:       cfg.MaxBodyBytes,
		MaxURLBytes:        cfg.MaxURLBytes,
		MaxHeaderBytes:     cfg.MaxHeaderBytes,
		EvaluationBudgetMs: int(cfg.EvaluationBudget.Milliseconds()),
		FailClosed:         cfg.FailClosed,
	}
	blob, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = a.store.Set(ctx, persistedKey, blob, 0)
}

// --- upstream configuration (admin) --------------------------------------

func (a *App) handleGetUpstream(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	u := a.cfg.UpstreamURL
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"upstreamUrl": u.String(),
		"upstreamHost": proxy.SanitizeHost(u),
		"note":        "Upstream is fixed by configuration. Clients cannot select a destination.",
	})
}

type upstreamRequest struct {
	URL string `json:"url"`
}

func (a *App) handlePutUpstream(w http.ResponseWriter, r *http.Request) {
	var req upstreamRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, "invalid_url", "The upstream URL is not valid.")
		return
	}
	a.mu.RLock()
	allowPrivate := a.cfg.AllowPrivateUpstream
	a.mu.RUnlock()
	if err := proxy.ValidateUpstream(u, allowPrivate); err != nil {
		writeErr(w, r, http.StatusUnprocessableEntity, "invalid_upstream", "The upstream destination is not permitted.")
		return
	}

	a.mu.Lock()
	a.cfg.UpstreamURL = u
	a.mu.Unlock()

	if err := a.Rebuild(); err != nil {
		writeErr(w, r, http.StatusInternalServerError, "rebuild_failed", "Could not apply the new upstream.")
		return
	}
	a.persist(r.Context())

	a.mu.RLock()
	updated := a.cfg.UpstreamURL.String()
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"upstreamUrl": updated, "upstreamHost": proxy.SanitizeHost(u)})
}

// --- settings (admin) ----------------------------------------------------

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, settingsView(cfg))
}

type settingsRequest struct {
	RateLimitEnabled   *bool  `json:"rateLimitEnabled,omitempty"`
	RateLimitBurst     *int   `json:"rateLimitBurst,omitempty"`
	RateLimitWindowS   *int   `json:"rateLimitWindowSeconds,omitempty"`
	MaxBodyBytes       *int64 `json:"maxBodyBytes,omitempty"`
	MaxURLBytes        *int   `json:"maxUrlBytes,omitempty"`
	MaxHeaderBytes     *int   `json:"maxHeaderBytes,omitempty"`
	EvaluationBudgetMs *int   `json:"evaluationBudgetMs,omitempty"`
	FailClosed         *bool  `json:"failClosed,omitempty"`
}

func (a *App) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	a.mu.Lock()
	cfg := a.cfg
	if req.RateLimitEnabled != nil {
		cfg.RateLimitEnabled = *req.RateLimitEnabled
	}
	if req.RateLimitBurst != nil && *req.RateLimitBurst > 0 && *req.RateLimitBurst <= 100000 {
		cfg.RateLimitBurst = *req.RateLimitBurst
	}
	if req.RateLimitWindowS != nil && *req.RateLimitWindowS > 0 && *req.RateLimitWindowS <= 3600 {
		cfg.RateLimitWindow = time.Duration(*req.RateLimitWindowS) * time.Second
	}
	if req.MaxBodyBytes != nil && *req.MaxBodyBytes > 0 && *req.MaxBodyBytes <= 64<<20 {
		cfg.MaxBodyBytes = *req.MaxBodyBytes
	}
	if req.MaxURLBytes != nil && *req.MaxURLBytes > 0 && *req.MaxURLBytes <= 16384 {
		cfg.MaxURLBytes = *req.MaxURLBytes
	}
	if req.MaxHeaderBytes != nil && *req.MaxHeaderBytes > 0 && *req.MaxHeaderBytes <= 1<<20 {
		cfg.MaxHeaderBytes = *req.MaxHeaderBytes
	}
	if req.EvaluationBudgetMs != nil && *req.EvaluationBudgetMs > 0 && *req.EvaluationBudgetMs <= 5000 {
		cfg.EvaluationBudget = time.Duration(*req.EvaluationBudgetMs) * time.Millisecond
	}
	if req.FailClosed != nil {
		cfg.FailClosed = *req.FailClosed
	}
	a.mu.Unlock()

	if err := a.Rebuild(); err != nil {
		writeErr(w, r, http.StatusInternalServerError, "rebuild_failed", "Could not apply the new settings.")
		return
	}
	a.persist(r.Context())

	a.mu.RLock()
	view := settingsView(a.cfg)
	a.mu.RUnlock()
	writeJSON(w, http.StatusOK, view)
}

func settingsView(cfg *config.Config) map[string]any {
	return map[string]any{
		"rateLimitEnabled":     cfg.RateLimitEnabled,
		"rateLimitBurst":       cfg.RateLimitBurst,
		"rateLimitWindowSeconds": int(cfg.RateLimitWindow.Seconds()),
		"maxBodyBytes":         cfg.MaxBodyBytes,
		"maxUrlBytes":          cfg.MaxURLBytes,
		"maxHeaderBytes":       cfg.MaxHeaderBytes,
		"evaluationBudgetMs":   int(cfg.EvaluationBudget.Milliseconds()),
		"failClosed":           cfg.FailClosed,
		"note":                 "Changes apply to this instance only and are not a distributed control.",
	}
}

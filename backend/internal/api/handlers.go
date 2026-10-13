package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"garudashield/internal/proxy"
)

// routes builds the versioned control-plane router once.
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/status", a.handleStatus)
	mux.HandleFunc("GET /api/v1/metrics", a.handleMetrics)
	mux.HandleFunc("GET /api/v1/rules", a.handleRules)

	mux.HandleFunc("GET /api/v1/logs", a.requireAuth(a.handleLogs))

	mux.HandleFunc("POST /api/v1/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/session", a.handleSession)
	mux.HandleFunc("GET /api/v1/auth/csrf", a.handleCSRF)

	mux.HandleFunc("GET /api/v1/admin/upstream", a.requireAuth(a.handleGetUpstream))
	mux.HandleFunc("PUT /api/v1/admin/upstream", a.requireAuth(a.handlePutUpstream))
	mux.HandleFunc("GET /api/v1/admin/settings", a.requireAuth(a.handleGetSettings))
	mux.HandleFunc("PUT /api/v1/admin/settings", a.requireAuth(a.handlePutSettings))

	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, r, http.StatusNotFound, "not_found", "No such endpoint.")
	})
	return mux
}

// serveAPI applies CORS then dispatches to the control-plane router.
func (a *App) serveAPI(w http.ResponseWriter, r *http.Request) {
	if a.applyAPICORS(w, r) {
		return
	}
	a.apiRouter.ServeHTTP(w, r)
}

func (a *App) applyAPICORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	allowed := false
	a.mu.RLock()
	for _, o := range a.cfg.CORSOrigins {
		if strings.EqualFold(strings.TrimRight(o, "/"), strings.TrimRight(origin, "/")) {
			allowed = true
			break
		}
	}
	a.mu.RUnlock()

	if !allowed {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		return false
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-Id")
	h.Set("Access-Control-Max-Age", "600")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// --- health probes -------------------------------------------------------

func (a *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"edition": "demo",
	})
}

func (a *App) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ready := true
	components := map[string]string{}
	if a.engine.Load() == nil || a.proxy.Load() == nil {
		ready = false
		components["engine"] = "not_ready"
	} else {
		components["engine"] = "ready"
	}
	if err := a.store.Ping(r.Context()); err != nil {
		ready = false
		components["store"] = "unreachable"
	} else {
		components["store"] = "ok"
	}
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ready": ready, "components": components})
}

// --- component status ----------------------------------------------------

type componentStatus struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	Detail    string  `json:"detail"`
	LatencyMs float64 `json:"latencyMs"`
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := a.metrics.Snapshot()
	components := make([]componentStatus, 0, 4)

	// Shield (Layer-7 evaluation engine)
	shieldStart := time.Now()
	engine := a.engine.Load()
	if engine != nil && len(engine.Rules()) > 0 {
		components = append(components, componentStatus{
			Name: "Shield", Status: "healthy",
			Detail:    "Layer-7 demonstration rule engine ready",
			LatencyMs: float64(time.Since(shieldStart).Microseconds()) / 1000.0,
		})
	} else {
		components = append(components, componentStatus{Name: "Shield", Status: "down", Detail: "rule engine unavailable"})
	}

	// Proxy
	if a.proxy.Load() != nil {
		components = append(components, componentStatus{Name: "Proxy", Status: "healthy", Detail: "reverse proxy configured for a single upstream"})
	} else {
		components = append(components, componentStatus{Name: "Proxy", Status: "down", Detail: "proxy not configured"})
	}

	// RedisStore (in-memory)
	storeStart := time.Now()
	if err := a.store.Ping(r.Context()); err == nil {
		components = append(components, componentStatus{
			Name: "RedisStore", Status: "healthy",
			Detail:    "in-memory Redis-compatible store reachable",
			LatencyMs: float64(time.Since(storeStart).Microseconds()) / 1000.0,
		})
	} else {
		components = append(components, componentStatus{Name: "RedisStore", Status: "down", Detail: "store unreachable"})
	}

	// Upstream (reachability probe, short timeout, result cached by client poll)
	upStart := time.Now()
	px := a.proxy.Load()
	upStatus, upDetail := "degraded", "upstream target not configured"
	if px != nil {
		if px.Healthy(1200 * time.Millisecond) {
			upStatus, upDetail = "healthy", "upstream target reachable"
		} else {
			upStatus, upDetail = "degraded", "upstream target not reachable"
		}
	}
	components = append(components, componentStatus{
		Name: "Upstream", Status: upStatus, Detail: upDetail,
		LatencyMs: float64(time.Since(upStart).Microseconds()) / 1000.0,
	})

	overall := "healthy"
	for _, c := range components {
		if c.Status == "down" {
			overall = "down"
			break
		}
		if c.Status == "degraded" {
			overall = "degraded"
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"edition": "Demo Edition",
		"overall": overall,
		"engine": map[string]any{
			"name":          "GarudaShield Demo Engine",
			"version":       Version,
			"startedAt":     snap.StartedAt.UTC(),
			"uptimeSeconds": snap.UptimeSeconds,
			"upstreamHost":  proxy.SanitizeHost(a.upstream.Load()),
		},
		"components": components,
		"checkedAt":  nowUTC(),
	})
}

// --- metrics -------------------------------------------------------------

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snap := a.metrics.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"edition":  "demo",
		"actual":   true,
		"note":     "All values are produced by real backend activity on this instance. Nothing here is simulated.",
		"metrics":  snap,
		"recordedAt": nowUTC(),
	})
}

// --- rules ---------------------------------------------------------------

func (a *App) handleRules(w http.ResponseWriter, r *http.Request) {
	engine := a.engine.Load()
	var list any = []any{}
	if engine != nil {
		list = engine.Rules()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"edition": "demo",
		"disclaimer": "Demonstration ruleset only. This is not a production WAF and does not claim OWASP Top 10 coverage.",
		"rules": list,
	})
}

// --- logs ----------------------------------------------------------------

func (a *App) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	events := a.metrics.Recent(limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"edition": "demo",
		"count":   len(events),
		"events":  events,
	})
}

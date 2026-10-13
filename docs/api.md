# API Documentation — GarudaShield Demo Edition

Base URL: `http://127.0.0.1:8080`

## Health Probes

### GET /healthz

Basic process health.

**Response 200:**
```json
{"status":"ok","edition":"demo"}
```

### GET /readyz

Readiness check (engine + store).

**Response 200:**
```json
{"ready":true,"components":{"engine":"ready","store":"ok"}}
```

## Control Plane (Public)

### GET /api/v1/status

Component status. Green badges only when health checks actually succeed.

**Response 200:**
```json
{
  "edition": "Demo Edition",
  "overall": "healthy",
  "engine": {
    "name": "GarudaShield Demo Engine",
    "version": "0.1.0-demo",
    "startedAt": "2026-10-13T12:00:00Z",
    "uptimeSeconds": 0,
    "upstreamHost": "127.***"
  },
  "components": [
    {"name":"Shield","status":"healthy","detail":"Layer-7 demonstration rule engine ready","latencyMs":0.1},
    {"name":"Proxy","status":"healthy","detail":"reverse proxy configured for a single upstream","latencyMs":0},
    {"name":"RedisStore","status":"healthy","detail":"in-memory Redis-compatible store reachable","latencyMs":0.02},
    {"name":"Upstream","status":"healthy","detail":"upstream target reachable","latencyMs":12.3}
  ],
  "checkedAt": "2026-10-13T12:00:05Z"
}
```

### GET /api/v1/metrics

Real backend metrics. Nothing is simulated.

**Response 200:**
```json
{
  "edition": "demo",
  "actual": true,
  "note": "All values are produced by real backend activity on this instance.",
  "metrics": {
    "requestsTotal": 42,
    "requestsEvaluated": 38,
    "allowed": 30,
    "blocked": 8,
    "rateLimitEvents": 2,
    "evaluationTimeouts": 0,
    "upstreamErrors": 1,
    "uptimeSeconds": 120,
    "startedAt": "2026-10-13T11:58:00Z"
  },
  "recordedAt": "2026-10-13T12:00:00Z"
}
```

### GET /api/v1/rules

Active demo rules with metadata.

**Response 200:**
```json
{
  "edition": "demo",
  "disclaimer": "Demonstration ruleset only. Not a production WAF.",
  "rules": [
    {"id":"L7-METHOD","name":"HTTP method allowlist","description":"...","category":"method","action":"block","enabled":true},
    {"id":"L7-PATH-TRAVERSAL","name":"Path traversal","description":"...","category":"pattern","action":"block","enabled":true}
  ]
}
```

## Authentication

### POST /api/v1/auth/login

**Request:**
```json
{"username":"admin","password":"..."}
```

**Response 200:**
```json
{"authenticated":true,"user":"admin","csrf":"<token>","expiresAt":"..."}
```

Sets HttpOnly session cookie.

### POST /api/v1/auth/logout

Clears session. **Response 204.**

### GET /api/v1/auth/session

**Response 200:**
```json
{"authenticated":true,"user":"admin","csrf":"<token>"}
```
or
```json
{"authenticated":false}
```

### GET /api/v1/auth/csrf

Returns CSRF token for current session. Required header: `X-CSRF-Token` on mutations.

## Admin Endpoints (Auth + CSRF Required)

### GET /api/v1/admin/upstream

**Response 200:**
```json
{"upstreamUrl":"http://127.0.0.1:8099","upstreamHost":"127.***","note":"..."}
```

### PUT /api/v1/admin/upstream

**Request:**
```json
{"url":"http://127.0.0.1:8099"}
```

SSRF validation applied. Rejects metadata endpoints, invalid schemes, credentials in URL.

### GET /api/v1/admin/settings

**Response 200:**
```json
{
  "rateLimitEnabled": true,
  "rateLimitBurst": 60,
  "rateLimitWindowSeconds": 10,
  "maxBodyBytes": 1048576,
  "maxUrlBytes": 2048,
  "maxHeaderBytes": 32768,
  "evaluationBudgetMs": 50,
  "failClosed": true,
  "note": "..."
}
```

### PUT /api/v1/admin/settings

Partial update. Only provided fields are changed.

**Request:**
```json
{"rateLimitBurst": 120, "evaluationBudgetMs": 100}
```

## Logs (Auth Required)

### GET /api/v1/logs?limit=50

Recent security events, newest first.

**Response 200:**
```json
{
  "edition": "demo",
  "count": 2,
  "events": [
    {
      "timestamp": "2026-10-13T12:00:00Z",
      "requestId": "abc123",
      "clientIp": "203.0.113.10",
      "method": "GET",
      "path": "/api/v1/status",
      "verdict": "allowed",
      "status": 200,
      "durationMs": 0.5
    }
  ]
}
```

## Error Responses

All errors return JSON:
```json
{"error":"blocked","message":"...","requestId":"..."}
```

Common status codes:
- `400` — invalid input
- `401` — authentication required
- `403` — CSRF required or path denied
- `404` — endpoint not found
- `405` — method not allowed
- `413` — request too large
- `422` — upstream validation failed
- `429` — rate limited (with `Retry-After` header)
- `502` — upstream unavailable
- `503` — evaluation timeout or service unavailable

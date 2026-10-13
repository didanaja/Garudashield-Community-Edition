# Arsitektur GarudaShield Demo Edition

## Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        Client                                │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                  Go Server (single process)                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Control Plane (/api/v1/*)                 │  │
│  │  /healthz  /readyz  /status  /metrics  /rules  /logs  │  │
│  │  /auth/login  /auth/logout  /auth/session              │  │
│  │  /admin/upstream  /admin/settings                     │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Guard Middleware Pipeline                 │  │
│  │  1. Request ID                                         │  │
│  │  2. Security Headers                                   │  │
│  │  3. CORS (restrictive)                                 │  │
│  │  4. Rate Limiting (token bucket)                       │  │
│  │  5. Layer-7 Evaluation (fail-closed budget)            │  │
│  │  6. Body Size Limit                                    │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │           Reverse Proxy (httputil)                     │  │
│  │  - Single configured upstream                          │  │
│  │  - Header sanitization                                  │  │
│  │  - Timeout enforcement                                  │  │
│  │  - SSRF validation                                     │  │
│  └───────────────────────┬───────────────────────────────┘  │
└──────────────────────────┼──────────────────────────────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │  Upstream Server │
                  │  (configured)    │
                  └─────────────────┘
```

## Komponen

### Backend (Go)

| Package | Tanggung Jawab |
|---------|---------------|
| `config` | Env loading, validation, defaults |
| `storage` | Redis-compatible interface + in-memory implementation |
| `metrics` | Atomic counters + bounded event log |
| `ratelimit` | Token bucket, bounded key count, idle eviction |
| `rules` | Layer-7 rule engine (method, size, pattern, access-list) |
| `proxy` | Hardened reverse proxy, SSRF validation, header sanitization |
| `middleware` | Guard pipeline (request ID, logging, CORS, rate limit, evaluation) |
| `auth` | PBKDF2 password hashing, server-side sessions, CSRF |
| `api` | Control plane REST API, routing, admin handlers |

### Frontend (Next.js)

| Path | Fungsi |
|------|--------|
| `/` | Landing page dengan hero, status card, feature cards |
| `/login` | Admin login form |
| `/dashboard` | Real-time metrics, security events, rules overview |
| `/docs` | Dokumentasi arsitektur dan batasan |

## Data Flow

1. Client mengirim request ke Go server
2. Guard middleware: request ID → security headers → CORS → rate limit → L7 evaluation
3. Jika lolos, request di-forward ke upstream yang dikonfigurasi
4. Response dikembalikan ke client dengan header sanitasi
5. Metrics dan security event dicatat secara real-time
6. Frontend polling `/api/v1/status` dan `/api/v1/metrics` untuk dashboard

## Storage

Redis-compatible interface (`storage.Store`) dengan operasi:
- `Get`, `Set`, `Delete`, `Incr`, `Ping`

In-memory implementation untuk demo lokal. Dapat diganti dengan Redis client nyata tanpa mengubah call sites.

## Security Boundaries

- **SSRF Prevention** — upstream divalidasi (scheme, host, IP ranges, metadata endpoint)
- **Open Proxy Prevention** — upstream fixed by configuration, klien tidak bisa memilih
- **Header Sanitization** — forwarding headers dihapus dan di-set ulang oleh proxy
- **Fail-Closed** — evaluasi yang melebihi budget ditolak, bukan diteruskan
- **Session Security** — HttpOnly, Secure, SameSite=Strict cookies; CSRF token per-session

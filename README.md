# GarudaShield Demo Edition

> **Experimental Protection Engine** — demonstrasi edukasi multi-layer reverse proxy dan web protection system.

GarudaShield Demo Edition adalah proyek full-stack untuk kompetisi keamanan siber. Backend Go menyediakan reverse proxy dengan security middleware (rate limiting, Layer-7 rule evaluation, fail-closed budget), dan frontend Next.js menampilkan dashboard dengan metrics nyata dari aktivitas backend.

## Fitur Utama

- **Reverse Proxy** — forward ke satu upstream yang dikonfigurasi eksplisit, dengan validasi SSRF
- **Layer-7 Evaluation** — method allowlist, size limits, pattern detection (path traversal, SQLi, XSS, scanner UA)
- **Rate Limiting** — token bucket per-client, bounded memory, HTTP 429 + Retry-After
- **Fail-Closed Budget** — evaluasi yang melebihi budget ditolak aman
- **Health & Metrics** — `/healthz`, `/readyz`, `/api/v1/status`, `/api/v1/metrics`, `/api/v1/rules`, `/api/v1/logs`
- **Admin Control Plane** — konfigurasi upstream dan settings via REST API dengan session + CSRF protection
- **Dashboard Next.js** — real-time polling, loading/empty/error/offline states

## Menjalankan Demo

### Prasyarat

- Go 1.24+
- Node.js 20+

### Langkah

```bash
# 1. Generate admin password hash
cd backend
go run ./cmd/server -hash-password "your-strong-password"
# Copy output ke GARUDA_ADMIN_PASSWORD_HASH di .env

# 2. Setup environment
cp .env.example .env
# Edit .env sesuai kebutuhan

# 3. Jalankan backend
go run ./cmd/server

# 4. Jalankan frontend (terminal lain)
cd ../frontend
cp .env.local.example .env.local
npm install
npm run dev

# 5. Buka http://localhost:3000
```

### Test Upstream

Untuk menguji proxy, jalankan server upstream sederhana:

```bash
# Terminal 3
cd backend
go run ./cmd/testupstream  # atau gunakan server HTTP apa pun di port 8099
```

## Struktur Proyek

```
backend/
  cmd/server/main.go
  internal/
    config/      — env loading, validation
    storage/     — Redis-compatible interface + in-memory impl
    metrics/     — real counters + sanitized event log
    ratelimit/   — token bucket, bounded
    rules/       — Layer-7 rule engine
    proxy/       — hardened reverse proxy
    middleware/  — guard pipeline
    auth/        — PBKDF2 + session + CSRF
    api/         — control plane REST API
frontend/
  app/           — Next.js App Router pages
  components/    — reusable React components
  lib/           — API client
  types/         — TypeScript types
docs/            — architecture, security model, limitations
```

## Testing

```bash
cd backend
go test ./...
go vet ./...
```

Lihat `docs/test-report.md` untuk hasil test yang benar-benar dieksekusi.

## Batasan Penting

- **Bukan WAF produksi** — pattern matcher demonstrasional, tidak mengklaim OWASP Top 10 coverage
- **Rate limiter single-process** — bukan distributed rate limiting
- **Tidak ada L4 scrubbing network** — modul L4 hanya edukasi + local metrics
- **Tidak ada fake statistics** — semua metrics dari aktivitas backend nyata
- **Upstream fixed** — klien tidak bisa memilih destination arbitrary

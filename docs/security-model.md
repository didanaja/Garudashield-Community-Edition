# Model Keamanan GarudaShield Demo Edition

## Threat Model

### Aset yang Dilindungi

1. **Upstream server** — server tujuan yang di-forward oleh proxy
2. **Client data** — request dan response yang melewati proxy
3. **Admin control plane** — konfigurasi upstream dan settings
4. **Process integrity** — availability dan correctness dari evaluation engine

### Ancaman yang Ditangani

| Ancaman | Mitigasi |
|---------|----------|
| SSRF | Validasi upstream (scheme http/https, host required, metadata endpoint diblokir, private IP gated) |
| Open Proxy | Upstream fixed by configuration; klien tidak bisa memilih destination |
| Header Spoofing | Forwarding headers dihapus dan di-set ulang oleh proxy |
| Rate Limiting Bypass | Client IP derived dari connection atau trusted proxy chain |
| Resource Exhaustion | Max body size, max URL/header size, bounded rate limiter storage |
| Session Hijacking | HttpOnly, Secure, SameSite=Strict cookies; server-side session storage |
| CSRF | Per-session token, constant-time comparison, required on mutations |
| Password Attack | PBKDF2-HMAC-SHA256 (210k iterations), login rate limiting |
| Information Disclosure | Sanitized error responses; no internal IPs, paths, or stack traces |

### Ancaman yang TIDAK Ditangani (di luar scope demo)

- Distributed DDoS / volumetric attacks
- TCP/UDP scrubbing network
- BGP hijacking / routing attacks
- Application logic vulnerabilities di upstream
- Zero-day exploits
- OWASP Top 10 comprehensive coverage

## Security Properties

### Fail-Closed

Jika evaluation tidak selesai dalam budget (default 50ms), request ditolak dengan HTTP 503. Ini berbeda dari upstream timeout (HTTP 502).

### Defense in Depth

1. Method allowlist (blokir method berbahaya)
2. Size limits (URL, header, body)
3. Rate limiting (token bucket per-client)
4. Pattern detection (path traversal, SQLi, XSS, scanner UA)
5. Fail-closed budget (evaluasi timeout → reject)

### Principle of Least Privilege

- Upstream hanya satu, dikonfigurasi eksplisit
- Admin endpoints memerlukan session + CSRF
- CORS restrictive (allowlist origins)
- Secrets di environment variables, bukan hardcoded

## Known Limitations

- Rate limiter single-process (bukan distributed)
- Pattern matcher sederhana (bukan WAF produksi)
- Tidak ada TLS termination (harus di-deploy di belakang reverse proxy dengan TLS)
- Session storage in-memory (atau Redis jika dikonfigurasi)
- Tidak ada audit log persistence (events di-memory only)

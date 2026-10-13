# Batasan GarudaShield Demo Edition

Dokumen ini secara eksplisit mendokumentasikan apa yang **tidak** dilindungi oleh demo ini. Ini penting untuk judge dan pengguna memahami scope implementasi.

## Yang TIDAK Dilindungi

### 1. Distributed DDoS / Volumetric Attacks

Demo ini **tidak** memiliki:
- TCP/UDP scrubbing network
- Packet-flood mitigation infrastructure
- Distributed filtering network
- High-capacity DDoS defense

Rate limiter token bucket single-process hanya melindungi dari request rate berlebihan di level aplikasi untuk satu instance. Ini bukan pengganti network-layer mitigation.

### 2. Comprehensive WAF / OWASP Top 10

Pattern matcher demo hanya mendeteksi:
- Path traversal (../, %2e%2e)
- Null byte injection
- SQL injection signatures (union select, OR 1=1, dll.)
- XSS signatures (<script>, javascript:, dll.)
- Sensitive file probes (.git, .env, wp-config.php)
- Scanner user-agents (sqlmap, nikto, nmap, dll.)

Ini **bukan** comprehensive WAF. Tidak mengklaim OWASP Top 10 coverage.

### 3. Distributed Rate Limiting

Rate limiter adalah single-process, in-memory. Jika di-deploy multiple instance, effective limit dikalikan dengan jumlah instance. Untuk distributed rate limiting, diperlukan shared storage (Redis) dan konsistensi across instances.

### 4. Production-Grade Security Engine

Demo ini adalah **educational demonstration**. Tidak boleh digunakan di production tanpa:
- Comprehensive security audit
- Penetration testing
- Production-grade WAF (ModSecurity, Coraza, dll.)
- TLS termination
- Proper logging dan monitoring
- Hardened OS dan network configuration

### 5. Session Persistence

Session storage default adalah in-memory. Jika server restart, semua session hilang. Untuk production, gunakan Redis atau database.

### 6. Audit Log Persistence

Security events disimpan di-memory ring buffer (max 500 events). Tidak ada persistence ke disk atau database. Untuk production, diperlukan centralized logging.

## Yang Dilindungi (Scope Demo)

- HTTP method allowlisting
- Request size limits (URL, header, body)
- Basic pattern detection (lihat di atas)
- Rate limiting (single-process)
- Fail-closed evaluation budget
- SSRF prevention (upstream validation)
- Open proxy prevention (fixed upstream)
- Header sanitization
- Session security (HttpOnly, Secure, SameSite)
- CSRF protection (per-session token)
- Login rate limiting

## Rekomendasi Production Hardening

1. Deploy di belakang reverse proxy dengan TLS (nginx, Caddy, atau cloud LB)
2. Gunakan production-grade WAF (ModSecurity, Coraza, atau cloud WAF)
3. Implementasikan distributed rate limiting dengan Redis
4. Tambahkan centralized logging (ELK, Loki, atau cloud logging)
5. Hardened OS (SELinux/AppArmor, minimal services)
6. Network segmentation (upstream di isolated network)
7. Regular security updates dan vulnerability scanning
8. Penetration testing berkala

# Demo Walkthrough untuk Judge

## Tujuan

Menunjukkan bahwa GarudaShield Demo Edition adalah implementasi nyata dengan prinsip defensive engineering, bukan mockup statis.

## Dashboard (login: `admin` / `garuda-demo-2026`)

Dashboard `http://localhost:3000/dashboard` memiliki tiga tab:

- **Ringkasan** — metric cards (requests, allowed, blocked, rate limited), recent security events, dan daftar rule aktif.
- **Traffic** — grafik traffic real-time (allowed/blocked/errors per interval) dan **Traffic Tester** untuk mengirim request demo langsung ke protection engine.
- **Konfigurasi** — ubah upstream dan settings runtime (rate limit, batas ukuran, evaluation budget, fail-closed) dengan proteksi CSRF.

Traffic Tester mengirim request langsung ke engine (`http://127.0.0.1:8080`) agar request target mentah dipertahankan, sehingga verdict rule sama seperti request klien nyata.

## Skenario 1: Normal Traffic

1. Jalankan backend dan test upstream
2. Buka `http://localhost:3000`
3. Status card menunjukkan semua komponen healthy (hanya jika health check benar-benar sukses)
4. Kirim request normal ke proxy: `curl http://127.0.0.1:8080/hello`
5. Dashboard menunjukkan metrics bertambah secara real-time

## Skenario 2: Blocked Request (Method)

1. Kirim request dengan method tidak diizinkan: `curl -X TRACE http://127.0.0.1:8080/`
2. Engine memblokir dengan HTTP 405 (L7-METHOD)
3. Dashboard menunjukkan blocked count bertambah

## Skenario 3: Pattern Detection

1. Path traversal: `curl "http://127.0.0.1:8080/%2e%2e/etc/passwd"` (atau preset Path traversal di Traffic Tester)
2. SQL injection: `curl "http://127.0.0.1:8080/search?q=union+select+from+users"` (preset SQL injection)
3. Reflected XSS: preset XSS di Traffic Tester
4. Sensitive file: preset Sensitive file di Traffic Tester (`/.env`)
5. Oversized URL: preset Oversized URL di Traffic Tester (HTTP 414)
6. Scanner UA (via `curl`): `curl -A "sqlmap/1.7" http://127.0.0.1:8080/`
7. Semua diblokir dengan rule id yang sesuai (L7-PATH-TRAVERSAL, L7-SQLI, L7-XSS, L7-SENSITIVE-FILE, L7-URL-LEN, L7-SCANNER-UA)

## Skenario 4: Rate Limiting

1. Kirim banyak request cepat: `for i in {1..70}; do curl -s http://127.0.0.1:8080/ & done`
2. Setelah burst habis, engine mengembalikan HTTP 429 dengan header Retry-After
3. Dashboard menunjukkan rate limit events

## Skenario 5: Upstream Failure

1. Hentikan test upstream
2. Kirim request ke proxy
3. Engine mengembalikan HTTP 502 dengan pesan sanitized (tidak leak internal address)
4. Status card menunjukkan Upstream degraded

## Skenario 6: SSRF Prevention

1. Login ke dashboard sebagai admin
2. Buka tab **Konfigurasi**, coba ubah upstream ke `http://169.254.169.254/latest/meta-data/`
3. Engine menolak dengan HTTP 422 (metadata endpoint prohibited)

## Skenario 7: Fail-Closed Behavior

1. Set evaluation budget sangat kecil (misal 1ms) via admin settings
2. Kirim request lambat ke upstream
3. Jika evaluation melebihi budget, request ditolak dengan HTTP 503

## Checklist Judge

- [ ] Backend berjalan dan merespons di `/healthz`
- [ ] Frontend terhubung dan menampilkan status nyata
- [ ] Dashboard memiliki tab Ringkasan, Traffic, dan Konfigurasi
- [ ] Traffic Tester mengirim request nyata dan menampilkan verdict + rule
- [ ] Metrics bertambah sesuai traffic aktual
- [ ] Request yang diblokir tercatat di security events
- [ ] Rate limiting berfungsi (429 + Retry-After)
- [ ] Upstream failure menghasilkan 502 sanitized
- [ ] SSRF attempt ditolak
- [ ] Tidak ada fake uptime atau statistik yang dikarang

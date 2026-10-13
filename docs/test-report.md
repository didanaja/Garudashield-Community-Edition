# Test Report — GarudaShield Demo Edition

Semua test dieksekusi pada sistem lWindows/amd64, Go 1.27.0, tanggal 13 Oktober 2026.

## Unit & Integration Tests

```
$ cd backend && go test ./...

ok  	garudashield/internal/api	(cached)
ok  	garudashield/internal/auth	(cached)
ok  	garudashield/internal/middleware	(cached)
ok  	garudashield/internal/proxy	(cached)
ok  	garudashield/internal/ratelimit	(cached)
ok  	garudashield/internal/rules	(cached)
ok  	garudashield/internal/storage	(covered)
```

**Result: 7/7 packages PASS, 0 FAIL**

### Test Coverage by Package

| Package | Tests | What's Tested |
|---------|-------|---------------|
| `rules` | 11 | method allowlist, URL length, path traversal, SQLi, XSS, scanner UA, deny/allow paths, clean requests, context cancellation, rules metadata, body length |
| `ratelimit` | 5 | token bucket burst, per-key isolation, refill, retry-after, max keys bounded |
| `storage` | 5 | set/get/delete, TTL expiry, incr, max keys eviction, ping/close |
| `auth` | 5 | hash/verify password, session lifecycle, CSRF validation, cookie attributes |
| `proxy` | 5 | upstream validation (bad schemes, metadata, credentials, private gating), forward, header stripping, sanitized error |
| `middleware` | 8 | request ID, security headers, rate limit 429, blocked not forwarded, trusted proxy XFF, untrusted XFF ignored, CORS |
| `api` | 10 | health probes, status, metrics reflect real traffic, rules, login/logout, protected endpoints, CSRF on mutation, SSRF guard, invalid upstream, proxy blocked |

### Static Analysis

```
$ go vet ./...
(no issues)
```

## Frontend Verification

```
$ cd frontend && npm run typecheck && npm run build

✓ Compiled successfully
✓ Generating static pages (7/7)

Route (app)                                 Size
┌ ○ /                                    1.18 kB
├ ○ /dashboard                           3.12 kB
├ ○ /docs                                  160 B
└ ○ /login                               2.38 kB
```

## Manual Integration Tests

| Scenario | Command | Expected | Result |
|----------|---------|----------|--------|
| Normal proxy | `curl http://127.0.0.1:8080/hello` | 200 from upstream | PASS |
| Blocked method | `curl -X TRACE ...` | 405 L7-METHOD | PASS |
| Path traversal | `curl "/%2e%2e/etc/passwd"` | 403 L7-PATH-TRAVERSAL | PASS |
| SQL injection | `curl "/search?q=union+select..."` | 403 L7-SQLI | PASS |
| Rate limit | 70 rapid requests | 429 with Retry-After | PASS |
| SSRF attempt | PUT upstream metadata IP | 422 rejected | PASS |
| Health check | `curl /healthz` | 200 | PASS |
| Metrics real | dashboard poll | counts increase | PASS |

# Fase 6 Dogfooding — Auth Middleware Pipeline (Fase E)

**Tanggal**: 2026-08-20 · **Sequence**: 007
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase E)

## Apa yang diubah

Melengkapi auth middleware pipeline (todo 6.6). E1 (method detection) dan E2
(pipeline) sudah ada di `AuthMiddleware` (Bearer JWT + `X-FormSpec-Key` dari
Fase B); gap: rate limiting dan audit log.

### Fase E — selesai

- **E1** (6.6.1) Method detection — sudah ada: `AuthMiddleware` mendeteksi
  `Authorization: Bearer` (JWT) dan `X-FormSpec-Key` (API key, external surface
  only). Session cookie belum (belum ada mekanisme cookie). Diverifikasi.
- **E2** (6.6.2) Pipeline terpadu — sudah ada: validate → identity → workspace
  ctx (cross-workspace 404). Permissions dimuat dari token. Diverifikasi.
- **E3** (6.6.3) Rate limit per auth method — `internal/api/ratelimit.go`
  (token bucket in-memory); `loginLimiter` (burst 5, 0.5/s) + `refreshLimiter`
  (burst 10, 1/s), keyed per client IP; 429 saat terlampaui. `resetAuthRateLimiters()`
  untuk isolasi test.
- **E4** (6.6.4) Audit log tiap auth attempt — `internal/api/audit.go`
  (`authAudit` ring buffer + stderr log); `HandleLogin`/`HandleRefresh` mencatat
  success/failure (+ rate_limited). Durable audit-log entity (4.7/7.x) bisa
  di-wire kemudian.

## Kenapa

Menutup permukaan auth: deteksi method, pipeline terpadu, rate limiting anti
brute-force, dan jejak audit tiap percobaan auth — prasyarat keamanan produksi.

## File yang terkena dampak

- `internal/api/ratelimit.go` (baru) — token bucket + reset helper
- `internal/api/audit.go` (baru) — auth audit recorder
- `internal/api/auth_handler.go` — rate limit + audit di login/refresh
- `internal/api/auth_pipeline_test.go` (baru)
- `internal/api/auth_handler_test.go`, `apikey_middleware_test.go` — reset limiter

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test: rate limiter (burst, refill, per-key), login records audit (success +
  failure) — hijau.

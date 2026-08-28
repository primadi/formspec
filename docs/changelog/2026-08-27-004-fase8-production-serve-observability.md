# 2026-08-27-004 — Fase 8: Production serve + Observability engine

## Apa yang diubah

Implementasi Fase 8 (Production Self-Hosting Single Server) — batch pertama
yang mencakup 8.1.1–8.1.5 dan 8.2.1–8.2.6:

- **`internal/observability`** (package baru): structured JSON-lines logger
  dengan 12 field wajib + PII discipline (8.2.1/8.2.2), Prometheus metric set
  12 metric dengan bounded labels (8.2.4/8.2.5), health registry dengan
  kosakata `healthy/degraded/unhealthy` + reason codes terkontrol (8.2.6),
  dan request-ID context helpers (8.2.3).
- **`internal/api`**: `RequestIDMiddleware` meneruskan upstream
  `X-Request-ID`; `NewCORSMiddleware(allowList)` menggantikan CORS `*`
  hardcoded (8.1.5); `LoggingMiddleware` JSON-lines; `MetricsMiddleware`;
  `/health` machine-readable saat health registry di-wire; `NewAdminMux`
  untuk listener administratif terpisah.
- **`internal/starlark`**: `ctx.request_id` — request ID dari HTTP boundary
  terbaca di script (8.2.3).
- **`resource`**: `Config.CORSOrigins/Logger/Metrics/Health` + probe
  datastore & db pool otomatis terdaftar saat Health di-wire.
- **`cmd/formspec/serve.go`** (baru): `formspec serve --mode=production`
  dengan gate production — Postgres wajib (8.1.4), JWT wajib HS256 secret
  atau RS256/ES256 public key (8.1.2), CORS allow-list wajib tanpa `*`
  (8.1.5), TLS via `--tls-cert/--tls-key` (8.1.3), admin listener
  `--metrics-addr` (default `:9102`).
- **`internal/auth/jwt_asym_test.go`** (baru): test RS256/ES256 end-to-end +
  penolakan algorithm confusion (8.1.2).

## Kenapa

`formspec dev` sudah stabil; Fase 8 membutuhkan jalur production yang
menonaktifkan semua dev shortcut dan memenuhi kontrak observability
(`docs/spec/platform/09-observability.md`). Request-ID context key dipindah
ke `internal/observability` agar Starlark bisa membacanya tanpa import cycle.

## File terdampak

- Baru: `internal/observability/*`, `cmd/formspec/serve.go`,
  `internal/auth/jwt_asym_test.go`, `internal/api/middleware_observability_test.go`,
  `internal/starlark/ctx_requestid_test.go`
- Ubah: `internal/api/middleware.go`, `internal/api/router.go`,
  `internal/api/handler.go`, `internal/starlark/context.go`,
  `internal/starlark/executor.go`, `resource/formspec.go`,
  `cmd/formspec/main.go`

## Referensi

- Plan: `docs/plan/fase8-production-serve.md`
- Todo: `docs/plan/todo.md` Fase 8
- Spec: `docs/spec/platform/09-observability.md`,
  `docs/runtimes/05-engine-api-layer.md` §2.2/§5

## Deferred

8.1.6 (DB least-privilege), 8.2.7 (OTel tracing — butuh wire contract di
semua SDK sidecar), 8.3.1 (scheduled backup) — lihat plan file.

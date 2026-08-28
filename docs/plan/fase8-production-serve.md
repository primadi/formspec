# Fase 8: Production Self-Hosting Single Server — Rencana Teknis

> Referensi spec: `docs/spec/platform/09-observability.md`,
> `docs/runtimes/05-engine-api-layer.md` §2.2/§5,
> `docs/spec/platform/06-datastore.md` §8, `docs/plan/todo.md` Fase 8.

## Status Implementasi

| Item  | Scope                                                                | File                                                                                                | LOE | Status                    |
| ----- | -------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | --- | ------------------------- |
| 8.2.1 | Structured JSON-lines logging (12 field wajib)                       | `internal/observability/logger.go`                                                                  | M   | ✅                        |
| 8.2.2 | PII discipline — debug gated, redact                                 | `internal/observability/logger.go`                                                                  | S   | ✅                        |
| 8.2.3 | Request ID — accept upstream, propagate ke Starlark `ctx.request_id` | `internal/observability/requestid.go`, `internal/api/middleware.go`, `internal/starlark/context.go` | M   | ✅                        |
| 8.2.4 | Prometheus `/metrics` (admin listener terpisah, 12 metric)           | `internal/observability/metrics.go`, `internal/api/metrics_middleware.go`                           | M   | ✅                        |
| 8.2.5 | Cardinality discipline — bounded labels                              | `internal/observability/metrics.go`                                                                 | S   | ✅                        |
| 8.2.6 | `/health` machine-readable `{status, reasons, checked_at}`           | `internal/observability/health.go`, `internal/api/router.go`                                        | M   | ✅                        |
| 8.1.1 | `formspec serve --mode=production`                                   | `cmd/formspec/serve.go`                                                                             | M   | ✅                        |
| 8.1.2 | JWT RS256/ES256 — test + wire ke config                              | `internal/auth/jwt_test.go`, `cmd/formspec/serve.go`                                                | S   | ✅                        |
| 8.1.3 | HTTPS — TLS config                                                   | `cmd/formspec/serve.go`                                                                             | S   | ✅                        |
| 8.1.4 | Production datastore — Postgres wajib di production                  | `cmd/formspec/serve.go`                                                                             | S   | ✅                        |
| 8.1.5 | CORS origin allow-list                                               | `internal/api/middleware.go`                                                                        | S   | ✅                        |
| 8.2.7 | OpenTelemetry tracing — W3C Trace Context ke sidecar                 | —                                                                                                   | L   | ⏸️ deferred               |
| 8.1.6 | Peran DB least-privilege (`formspec_ops_backup`, `formspec_ops_ddl`) | —                                                                                                   | M   | ⏸️ deferred               |
| 8.3.1 | Scheduled backup automation                                          | —                                                                                                   | M   | ⏸️ deferred               |
| 8.3.2 | Restore procedure documented + tested                                | `cmd/formspec/backup.go` (create/inspect/restore sudah ada)                                         | S   | ⏸️ deferred (dokumentasi) |

## Keputusan Teknis

1. **Package baru `internal/observability`** — satu tempat untuk logger, metrics,
   health, dan request-id context. `internal/api` mendelegasikan ke sini supaya
   `internal/starlark` bisa membaca request ID tanpa import cycle.
2. **Request ID context key dipindah** ke `internal/observability` (dulu private
   di `internal/api/handler.go`). `internal/api` tetap expose API lama sebagai
   wrapper agar tidak break caller.
3. **Metrics memakai `prometheus/client_golang`** (sudah ada di go.mod sebagai
   indirect — dipromosikan ke direct). Label dibatasi ke dimensi bounded:
   `route_class`, `method`, `status_class`, `module`, `action`, `error_code`.
   Dilarang: entity ID, request_id, actor, raw URL (spec §3.2).
4. **`formspec serve`** adalah command baru yang reuse pipeline server `dev.go`
   tetapi dengan constraint production: DSN Postgres wajib, JWT wajib
   (HS256 secret ATAU RS256/ES256 public key file), TLS opsional tapi
   direkomendasikan, CORS allow-list wajib eksplisit, dev auth dimatikan.
5. **Health checker** berbasis probe registry: datastore ping, outbox backlog,
   db pool. Kosakata status/reasons persis spec §5.

## Deferred (butuh keputusan/kerja besar)

- **8.2.7 OTel tracing**: butuh wire contract `traceparent` di SDK sidecar
  (semua bahasa: PHP/Python/Node/Java/Ruby/.NET/TS) — kerja lintas SDK, layak
  fase sendiri.
- **8.1.6 DB least-privilege**: butuh provisioning script + keputusan apakah
  engine membuat role sendiri atau operator yang membuat.
- **8.3.1 Scheduled backup**: butuh scheduler in-process (ctx.queue tick atau
  cron eksternal) — keputusan desain belum ada.

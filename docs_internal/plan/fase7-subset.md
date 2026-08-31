# Fase 7 — Subset Implementasi (rate limit, money, sandbox, hooks)

**Status:** ✅ Complete (2026-08-25) · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/` (01–06), `docs/plan/todo.md` §7
**Todo:** `docs/plan/todo.md` §7.5, §7.8, §7.12, §7.14, §7.16
**Changelog:** `docs/changelog/2026-08-25-001` s/d `2026-08-25-002`

## Konteks (state 2026-08-25)

Fase 7 (Engine Extended) adalah fase terbesar yang tersisa (~60 item). Banyak item **sudah
terimplementasi di code tapi belum ditandai** di todo — diverifikasi & di-sync. Item yang
genuinely besar (Service runtime, Subscription Tier 2, Workflow, Webhook, Integrator,
Sidecar multi-runtime, KindDefinition, Mockup) butuh design/scope tersendiri dan di-defer.

Subset ini fokus pada item yang **feasible, bernilai tinggi, dan tidak terblokir design**:

## Workstream

### WS-1 — Rate limiter per-resource (7.12.1–7.12.3) ✅

- `RateLimitSpec` sudah ada di `pkg/spec` (`EntitySpec.RateLimit` + `Action.RateLimit`).
- Token-bucket `rateLimiter` sudah ada (`internal/api/ratelimit.go`, dipakai auth 6.6.3).
- Implementasi: `ResourceRateLimiter` — baca `EntitySpec.RateLimit` (resource-level) +
  `Action.RateLimit` (per-action override); scope `tenant|user|ip|global`; strategy
  `token_bucket|sliding_window`; `429` sebelum handler jalan.
- Wire: middleware/handler lookup via `specLookup` (sudah ada di HandlerFactory).

### WS-2 — Money type first-class (7.16.1–7.16.4) ✅

- `FieldMoney` + `CurrencySettings` sudah ada di `pkg/spec`.
- Implementasi: `Money` value type (amount decimal + currency ISO-4217) + validasi:
  currency resolution order (explicit field → `settings.currency` → error, never guess);
  banker's rounding default; non-default currency wajib `decimal_places`.
- Frontend `lib/format.ts` sudah punya `roundTo` + `RoundingMode` — backend kini punya padanan.

### WS-3 — Starlark sandbox hard limits (7.14.1–7.14.3) ✅

- `internal/starlark/executor.go` sudah sandboxed (no imports, no I/O).
- Implementasi: hard limits — wall-clock 5000ms, memory 64MB, iterations 100K, max 50 DB
  queries, max 1000 records read; exceeding → abort dengan error, no partial results.

### WS-4 — Hook points before_deliver/after_deliver (7.8.5) ✅

- `internal/action/hooks.go` sudah punya before/after/on_error (7.8.1–7.8.4).
- Implementasi: `before_deliver` (may suppress delivery or enrich payload) +
  `after_deliver` (post-delivery side effects) di `DeliverEvents`.

### WS-5 — Verifikasi & tandai yang sudah ada ✅

- 7.5.1–7.5.3: state machine engine (transition validation, Starlark guards, `sum_line`
  builtin) — `internal/entity/state_machine.go` + test.
- 7.8.1–7.8.4, 7.8.6, 7.8.7: hook engine (before/after/on_error + priority ordering +
  cross-module uses inheritance) — `internal/action/hooks.go`.
- 7.3.1: Tier 1 outbox — `internal/action/deliver.go` + `OutboxStore` (outbox + delivery
  ada; Subscription matching ke handler masih belum — di-defer).

## Level of effort

| WS  | Effort |
| --- | ------ |
| 1   | medium |
| 2   | medium |
| 3   | medium |
| 4   | small  |
| 5   | small  |

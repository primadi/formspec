# 2026-08-25-001 — Fase 7 subset: rate limiter, money type, sandbox limits (7.12, 7.16, 7.14)

## Apa yang diubah

Subset pertama Fase 7 (Engine Extended) — item yang feasible, bernilai tinggi, dan tidak
terblokir design (plan: `docs/plan/fase7-subset.md`).

**7.12 Rate limiter per-resource:**

- `internal/api/resource_ratelimit.go` — `ResourceRateLimiter` (token_bucket + sliding_window,
  scope tenant|user|ip|global, key per-action).
- `EntitySpec.RateLimit` (resource-level) + `Action.RateLimit` (per-action override menang).
- `checkRateLimit` di-wire ke semua handler (List/Find/Create/Update/Delete/CustomAction) →
  `RATE_LIMITED` 429 sebelum handler jalan.
- Test: `resource_ratelimit_test.go`.

**7.16 Money type first-class:**

- `pkg/spec/money.go` — `Money` struct (amount decimal + currency ISO-4217), `ResolveMoneyCurrency`
  (explicit field → settings.currency → error, never guess), `RoundMoney` (banker's rounding
  default, override via settings.rounding), `ValidateMoneyField` (non-default currency wajib
  `decimal_places`), `ValidateMoneyValue`.
- `Field.Currency` + `Field.DecimalPlaces` ditambahkan; `ValidateEntitySpec` menolak money field
  override currency tanpa decimal_places.
- Test: `money_test.go`.

**7.14 Starlark sandbox hard limits:**

- `internal/starlark/limits.go` — `ScriptLimits` (max 50 DB queries, max 1000 records read).
- `ExecuteScript` — `SetMaxExecutionSteps(100K)` (iterations) + context timeout 5s (wall-clock).
- `builtinQuery` — `CheckQuery` + `AddRecordsRead` → abort dengan error, no partial results.
- Memory 64MB tidak terukur langsung di interpreter Starlark — step limit adalah bound praktis.
- Test: `limits_test.go`.

## File terdampak

- `internal/api/resource_ratelimit.go` (baru) + `resource_ratelimit_test.go` (baru)
- `internal/api/handler.go`, `internal/api/router.go`
- `pkg/spec/money.go` (baru) + `money_test.go` (baru), `pkg/spec/entity.go`
- `internal/starlark/limits.go` (baru) + `limits_test.go` (baru), `executor.go`, `primitive.go`

## Referensi

- Plan: `docs/plan/fase7-subset.md` (WS-1, WS-2, WS-3)
- Todo: `docs/plan/todo.md` §7.12, §7.14, §7.16

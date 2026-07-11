# 2026-07-09 — transaction_date Backdate/Forward-Date Policy

**File:** `pkg/spec/entity.go`, `internal/db/transaction_date.go` (NEW), `internal/db/crud.go`

## Ringkasan

Implementasi per-task terakhir dari `docs/plan/document-model-code-alignment.md`: **2.4.3 — Validate transaction_date backdate/forward-date policy**.

## Perubahan

### `pkg/spec/entity.go`
- `DocumentSpec`: tambah field `BackdatePolicy` dan `ForwardDatePolicy`
- Type `BackdatePolicy` — `max_days_back`, `override_permission`
- Type `ForwardDatePolicy` — `max_days_forward`, `override_permission`

### `internal/db/transaction_date.go` (NEW)
- `ValidateTransactionDate(date, maxDaysBack, maxDaysForward)` — core validation
- `DefaultBackdatePolicy()` / `DefaultForwardDatePolicy()` — defaults (3 days back, 0 forward)
- `TransactionDatePolicyError` — error type dengan `FORMA.TXN.BACKDATE_EXCEEDED` dan `FORMA.TXN.FORWARD_DATE_EXCEEDED`

### `internal/db/transaction_date_test.go` (NEW)
- 8 tests: within limit, exceeded, forward blocked, unlimited, empty, etc.

### `internal/db/crud.go`
- `EntityStore` struct: tambah `backdatePolicy` dan `forwardDatePolicy`
- `NewEntityStore()`: populate dari `entity.BackdatePolicy` / `entity.ForwardDatePolicy`
- `validateTransactionDatePolicy()` method — baca `transaction_date` dari data, apply policy
- Dipanggil di `Insert()` dan `Update()` (setelah referenceability guard)

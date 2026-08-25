# Fase 7 — Integrator Saga Compensate (7.7.4)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §5 (Integrator — Saga
compensate)
**Todo:** `docs/plan/todo.md` §7.7.4

## Konteks

Integrator engine (7.7.1–7.7.3) sudah selesai. 7.7.4 melengkapi **saga
compensate**: cross-boundary call mendaftarkan `compensate` ke Saga log;
`FORMSPEC.SAGA.*` errors. Same-transaction call tidak butuh `compensate` (ACID
rollback sudah cukup); cross-boundary call mendaftarkan `compensate` ke Saga
log.

## Scope

### SAGA-1 — Saga log store (7.7.4) ✅

- `renderers/jsonb-persist/saga.go` — `SagaStore` + tabel `formspec_saga_log`:
  mencatat cross-boundary call + compensate action-nya.
- Kolom: id, tenant_id, source (event), target (call.resource.action),
  compensate (action ref), status (pending | compensated | completed),
  error, created_at, updated_at.
- `Register`, `ListPending`, `MarkCompensated`, `MarkCompleted`, `GetByID`.

### SAGA-2 — Register compensate di integrator dispatch (7.7.4) ✅

- `internal/integrator/dispatch.go` — saat dispatch target action, jika
  integrator punya `compensate`, register ke Saga log (cross-boundary call).
- `Dispatcher` menerima `*db.SagaStore` (opsional, variadic).

### SAGA-3 — Compensate invocation (7.7.4) ✅

- Saat dispatch target action gagal → invoke compensate action (resolve dari
  `compensate` ref pada target resource) + tandai saga entry `compensated`.
- Error `FORMSPEC.SAGA.COMPENSATE_FAILED` jika compensate juga gagal.
- Dispatch sukses → saga entry `completed`.

## Level of effort

| SAGA | Effort |
| ---- | ------ |
| 1    | medium |
| 2    | small  |
| 3    | small  |

## Verifikasi

- Unit test `internal/integrator/dispatch_test.go`: dispatch sukses → saga
  `completed`; dispatch gagal → compensate di-invoke → saga `compensated`;
  tanpa compensate → tidak ada saga entry. ✅
- `go test ./...` hijau (828 pass).

## File terdampak

- `renderers/jsonb-persist/saga.go` (baru) — saga store
- `renderers/jsonb-persist/migrate.go` — tabel `formspec_saga_log`
- `internal/integrator/dispatch.go` — register + invoke compensate
- `internal/integrator/dispatch_test.go` (baru) — unit test
- `resource/formspec.go` — wire saga store ke integrator dispatcher
# Integrator Saga Compensate (7.7.4) — Cross-Boundary Compensation

**Tanggal:** 2026-08-25 · **Todo:** §7.7.4 · **Plan:** `docs/plan/fase7-integrator-saga.md`

## Apa yang ditambahkan

Integrator engine kini mendukung **saga compensate** — cross-boundary call
mendaftarkan `compensate` ke Saga log (02-core-extended.md §5):

- **Saga log store** — `renderers/jsonb-persist/saga.go` (`SagaStore` + tabel
  `formspec_saga_log`): mencatat cross-boundary call + compensate action-nya.
  `Register`/`ListPending`/`MarkCompleted`/`MarkCompensated`/`GetByID`.
- **Register compensate** — `internal/integrator/dispatch.go`: saat dispatch
  target action, jika integrator punya `compensate`, register ke Saga log.
- **Compensate invocation** — saat dispatch target action gagal → invoke
  compensate action (resolve dari `compensate` ref pada target resource) +
  tandai saga entry `compensated`. Error `FORMSPEC.SAGA.COMPENSATE_FAILED`
  jika compensate juga gagal. Dispatch sukses → saga entry `completed`.
- **Wire** — saga store di-wire ke integrator dispatcher di
  `resource/formspec.go` (boot + reload).

## Verifikasi

- Unit test `internal/integrator/dispatch_test.go`: dispatch sukses → saga
  `completed`; dispatch gagal → compensate di-invoke → saga `compensated`;
  tanpa compensate → tidak ada saga entry. ✅
- `go test ./...` hijau (828 pass).

## File terdampak

- `renderers/jsonb-persist/saga.go` (baru) — saga store
- `renderers/jsonb-persist/migrate.go` (tabel `formspec_saga_log`)
- `internal/integrator/dispatch.go` (register + invoke compensate)
- `internal/integrator/dispatch_test.go` (baru) — unit test
- `resource/formspec.go` (wire saga store boot + reload)
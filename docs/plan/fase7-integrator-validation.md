# Fase 7 — Integrator Validation (7.7.2, 7.7.3)

**Status:** ✅ Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §5 (Integrator)
**Todo:** `docs/plan/todo.md` §7.7.2, §7.7.3

## Konteks

Integrator engine (7.7.1) sudah selesai. 7.7.2 dan 7.7.3 adalah **aturan
validasi** yang di-enforce di `formspec validate` / `formspec apply`:

- **7.7.2** — setiap Integrator yang membuat efek samping dari satu event
  **wajib** juga menyediakan handler simetris untuk event pembatalannya —
  tanpa itu, cancel di sisi source akan terblokir permanen.
- **7.7.3** — target action **wajib** `idempotent: true` untuk pemanggilan
  cross-boundary.

## Scope

### INT-V1 — Symmetric cancel handler (7.7.2) ✅

- `cmd/formspec/validate.go` — `validateIntegrators` cross-manifest check:
  untuk setiap Integrator yang listen ke event non-cancel (mis. `on_submit`,
  `on_create`), harus ada Integrator lain yang listen ke event cancel
  (`on_cancel`/`before_cancel`) untuk resource yang sama.
- Event cancel yang dikenali: `on_cancel`, `before_cancel`.

### INT-V2 — Idempotent target (7.7.3) ✅

- `cmd/formspec/validate.go` — `validateIntegrators` juga cek target action
  (`call.resource` + `call.action`) harus `idempotent: true`.
- Resolve target action dari entity manifest set.

## Level of effort

| INT-V | Effort |
| ----- | ------ |
| 1     | small  |
| 2     | small  |

## Verifikasi

- `formspec validate` pada spec dengan Integrator tanpa cancel handler →
  error (terverifikasi di service-demo: `product-created-to-tax` di-flag
  sampai `product-cancelled-to-tax` ditambahkan). ✅
- `go test ./...` hijau (825 pass, termasuk unit test `validateIntegrators`).

## File terdampak

- `cmd/formspec/validate.go` — `validateIntegrators` + wiring
- `cmd/formspec/validate_test.go` — unit test
- `examples/service-demo/` — contoh integrator simetris (on_create + on_cancel)
# Integrator Validation (7.7.2, 7.7.3) — Symmetric Cancel + Idempotent Target

**Tanggal:** 2026-08-25 · **Todo:** §7.7.2, §7.7.3 · **Plan:** `docs/plan/fase7-integrator-validation.md`

## Apa yang ditambahkan

Integrator engine kini punya **aturan validasi** yang di-enforce di
`formspec validate` (02-core-extended.md §5):

- **7.7.2 — Symmetric cancel handler** — `validateIntegrators` di
  `cmd/formspec/validate.go`: setiap Integrator yang listen ke event
  non-cancel (mis. `on_submit`, `on_create`) wajib punya pasangan Integrator
  yang listen ke event cancel (`on_cancel`/`before_cancel`) untuk resource
  yang sama — tanpa itu, cancel di sisi source akan terblokir permanen.
- **7.7.3 — Idempotent target** — `validateIntegrators` juga cek target action
  (`call.resource` + `call.action`) harus `idempotent: true` untuk pemanggilan
  cross-boundary (resolve dari entity manifest set).

## Verifikasi

- `formspec validate` pada service-demo: `product-created-to-tax` di-flag
  (tanpa cancel handler) sampai `product-cancelled-to-tax` ditambahkan. ✅
- `go test ./...` hijau (825 pass, termasuk unit test `validateIntegrators`).

## File terdampak

- `cmd/formspec/validate.go` (`validateIntegrators` + wiring)
- `cmd/formspec/validate_test.go` (unit test)
- `examples/service-demo/` (contoh integrator simetris on_create + on_cancel)

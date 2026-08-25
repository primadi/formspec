# Fase 7 — Integrator Engine (7.7)

**Status:** ✅ 7.7.1 Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §5 (Integrator),
`pkg/spec/resources.go` (IntegratorSpec)
**Todo:** `docs/plan/todo.md` §7.7

## Konteks

`kind: Integrator` menjembatani dua Entity/Module yang **tidak saling kenal
langsung** — konsisten dengan prinsip "module tidak saling import definisi satu
sama lain". `listen.resource`+`event` memicu `call.resource`+`action`.

`IntegratorSpec` sudah ada di `pkg/spec` (Listen + Call + Compensate) tapi
tidak ada runtime — tidak ada registry, tidak ada bridge ke event dispatch.

## Scope

### INT-1 — Integrator registry (7.7.1) ✅

- `internal/integrator/registry.go` — `Registry` memetakan `{module}.{name}` →
  `IntegratorSpec`; `Add`/`Get`/`List`/`ForEvent`.
- `buildIntegratorRegistry` di `resource/formspec.go` (boot + reload).
- Index by `{listen.resource}.{listen.event}` → integrator yang bereaksi;
  re-registration (hot reload) menghapus index lama.

### INT-2 — Listen → call bridge (7.7.1) ✅

- `internal/integrator/dispatch.go` — `Dispatcher` memanggil target action
  (`call.resource` + `call.action`) saat event source ter-emit. Target bisa
  Entity action atau Service action (di-resolve via entity/service registry).
- Wire ke `DeliveryEventHandler.Subscriptions` (composed bersama subscription
  dispatch) — event → match integrator → dispatch target action.
- Payload event diteruskan sebagai params; metadata event (`name`, `resource`)
  di-merge ke key reserved `_event`.

### INT-3 — Symmetric cancel handler (7.7.2) ⏸️ deferred

- Setiap Integrator yang membuat efek samping dari satu event wajib menyediakan
  handler simetris untuk event pembatalannya. Validasi di `formspec apply` /
  `validate` belum diimplementasikan.

### INT-4 — Idempotent target (7.7.3) ⏸️ deferred

- Target action wajib `idempotent: true` untuk pemanggilan cross-boundary.
- Validasi di `formspec apply` / `validate` belum diimplementasikan.

## Level of effort

| INT | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | small  |
| 4   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` emits `on_create` (durable); Integrator
  `demo.product-created-to-tax` listen `demo.product.on_create` → call
  `demo.tax-calculator.calculate`.
- Create product → outbox worker dispatch integrator → target service action
  dipanggil → outbox status `completed`. ✅
- `go test ./...` hijau (816 pass, termasuk unit test `internal/integrator`).

## File terdampak

- `internal/integrator/registry.go` (baru) — registry
- `internal/integrator/dispatch.go` (baru) — bridge ke target action
- `internal/integrator/registry_test.go` (baru) — unit test
- `resource/formspec.go` — `buildIntegratorRegistry` + wiring (boot + reload)
- `internal/manifest/loader.go` — `RawSpecToIntegratorSpec`
- `examples/service-demo/` — contoh integrator (entity + integrator)
# Integrator Engine (7.7.1) — Listen → Call Bridge

**Tanggal:** 2026-08-25 · **Todo:** §7.7.1 · **Plan:** `docs/plan/fase7-integrator-engine.md`

## Apa yang ditambahkan

`kind: Integrator` kini punya runtime bridge — menjembatani dua Entity/Module
yang **tidak saling kenal langsung** (02-core-extended.md §5):

- **Registry (7.7.1)** — `internal/integrator/registry.go` memetakan
  `{module}.{name}` → `IntegratorSpec`; index by `{listen.resource}.{listen.event}`;
  `buildIntegratorRegistry` di `resource/formspec.go` (boot + reload).
- **Listen → call bridge (7.7.1)** — `internal/integrator/dispatch.go`
  memanggil target action (`call.resource` + `call.action`) saat event source
  ter-emit. Target bisa Entity action atau Service action (di-resolve via
  entity/service registry + action dispatcher). Payload event jadi params,
  metadata event (`name`, `resource`) di-merge ke key reserved `_event`.
- **Wire ke outbox worker** — dispatch integrator di-compose bersama
  subscription dispatch ke `DeliveryEventHandler.Subscriptions` (boot + reload).

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` emits `on_create` (durable); Integrator
  `demo.product-created-to-tax` listen `demo.product.on_create` → call
  `demo.tax-calculator.calculate`.
- Create product → outbox worker dispatch integrator → target service action
  dipanggil → outbox status `completed`. ✅
- `go test ./...` hijau (816 pass, termasuk unit test `internal/integrator`).

## File terdampak

- `internal/integrator/registry.go`, `dispatch.go`, `registry_test.go` (baru)
- `resource/formspec.go` (`buildIntegratorRegistry` + wiring boot/reload)
- `internal/manifest/loader.go` (`RawSpecToIntegratorSpec`)
- `examples/service-demo/` — contoh integrator (entity + integrator)

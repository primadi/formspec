# Fase 7 — Subscription Engine (7.3)

**Status:** ✅ Tier 1 Complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §3 (Subscription & Event
Delivery), `docs/spec/backend/01-core-basic.md` §7 (Event & Outbox),
`pkg/spec/resources.go` (SubscriptionSpec)
**Todo:** `docs/plan/todo.md` §7.3

## Konteks

`kind: Subscription` membuat module lain bereaksi terhadap event resource lain
tanpa mengubah publisher. Dua tier:

- **Tier 1 (Core, outbox)** — event → match Subscription → call handler;
  transaksional. Storage: outbox PersistBackend.
- **Tier 2 (Streaming)** — Redis/Kafka; at-least-once, positioned replay,
  filter/transform Starlark. **Di-defer** (butuh infrastruktur streaming).

`SubscriptionSpec` sudah ada di `pkg/spec` (Events + Handler + Tier 2 fields)
tapi tidak ada runtime — tidak ada registry, tidak ada dispatch ke handler.

## Scope (Tier 1 — yang dikerjakan sekarang)

### SUB-1 — Subscription registry (7.3.1) ✅

- `internal/subscription/registry.go` — `Registry` memetakan event name →
  daftar Subscription yang melanggan; `Add`/`Get`/`List`/`ForEvent`.
- `buildSubscriptionRegistry` di `resource/formspec.go` (boot + reload).
- `SubscriptionSpec.Events` berisi event name (mis. `billing.invoice.on_submit`)
  → registry index by event name; re-registration (hot reload) menghapus index
  lama.

### SUB-2 — Dispatch ke handler (7.3.1) ✅

- `internal/subscription/dispatch.go` — `Dispatcher.Dispatch` memanggil handler
  subscription untuk sebuah event.
- `handler` (`ImplDecl`) adalah implementasi handler itu sendiri (script_ref,
  native, compiled, sidecar) — di-dispatch via action dispatcher persis seperti
  action impl / hook impl. **Bukan** referensi Service action (desain awal
  salah — `handler.ref` untuk `script_ref` adalah `{module}/{script}`, bukan
  `{module}.{service}`).
- Payload event diteruskan sebagai params; metadata event (`name`, `resource`)
  di-merge ke key reserved `_event`.
- Idempotent: subscription handler dipanggil sekali per event (outbox worker
  sudah menjamin at-least-once; handler harus idempotent).

### SUB-3 — Wire ke outbox worker (7.3.1) ✅

- `DeliveryEventHandler` (renderers/jsonb-persist/event_handler.go) ditambah
  `Subscriptions` field → setelah channel fan-out, dispatch ke subscription
  handler yang match event (fully-qualified event name `{module}.{entity}.{event}`).
- `resource/formspec.go` — wire subscription registry + dispatcher ke
  `DeliveryEventHandler` (boot + reload).
- **Fix gap**: outbox worker tidak pernah di-start di dev mode (dev command
  pakai `http.Server` sendiri, bukan `app.ListenAndServe()`). Ditambah
  `App.StartBackgroundWorkers()` + dipanggil di `cmd/formspec/dev.go` sebelum
  serve — durable events (dan subscription) kini ter-deliver di dev.

### SUB-4 — `emits:` custom event (7.3.3) ✅ (sudah ada di code)

- `ResolveEmission` + `ValidateActionEmits` + custom action emission sudah
  diimplementasikan (handler.go). Ditandai ✅ di todo.

## Level of effort

| SUB | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | small  |
| 4   | done   |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` emits `on_create` (durable); Subscription `demo.product-created-audit`
  melanggan → create product → outbox worker dispatch handler script
  `handle_product_created.star` → outbox status `completed`. ✅
- `go test ./...` hijau (798 pass, termasuk unit test `internal/subscription`).

## File terdampak

- `internal/subscription/registry.go` (baru) — registry
- `internal/subscription/dispatch.go` (baru) — dispatch ke handler
- `internal/subscription/registry_test.go` (baru) — unit test
- `renderers/jsonb-persist/event_handler.go` — `Subscriptions` field + dispatch
- `resource/formspec.go` — `buildSubscriptionRegistry` + wiring (boot + reload)
- `cmd/formspec/dev.go` — `app.StartBackgroundWorkers()` sebelum serve
- `examples/service-demo/` — contoh subscription (entity event + subscription + script)

# Subscription Engine Tier 1 (7.3.1) + outbox worker di dev mode

**Tanggal:** 2026-08-25 · **Todo:** §7.3.1, §7.3.3 · **Plan:** `docs/plan/fase7-subscription-engine.md`

## Apa yang ditambahkan

`kind: Subscription` kini punya runtime Tier 1 (outbox) — module lain bisa
bereaksi terhadap event resource lain tanpa mengubah publisher
(02-core-extended.md §3):

- **Registry (7.3.1)** — `internal/subscription/registry.go` memetakan event
  name → daftar Subscription (`Add`/`Get`/`List`/`ForEvent`); index by event
  name, re-registration (hot reload) menghapus index lama.
- **Dispatch (7.3.1)** — `internal/subscription/dispatch.go` memanggil handler
  subscription (ImplDecl: script_ref/native/compiled/sidecar) via action
  dispatcher; payload event jadi params, metadata event (`name`, `resource`)
  di-merge ke key reserved `_event`. Desain awal (handler = referensi Service
  action) salah — `handler.ref` untuk `script_ref` adalah `{module}/{script}`,
  bukan `{module}.{service}`; dikoreksi ke dispatch langsung.
- **Wire ke outbox worker (7.3.1)** — `DeliveryEventHandler.Subscriptions`
  (renderers/jsonb-persist/event_handler.go) dispatch setelah channel fan-out
  dengan fully-qualified event name `{module}.{entity}.{event}`;
  `buildSubscriptionRegistry` di `resource/formspec.go` (boot + reload).
- **Fix gap: outbox worker di dev mode** — dev command pakai `http.Server`
  sendiri (bukan `app.ListenAndServe()`), jadi outbox worker tidak pernah
  di-start → durable events tidak pernah ter-deliver di dev. Ditambah
  `App.StartBackgroundWorkers()` + dipanggil di `cmd/formspec/dev.go` sebelum
  serve.
- **7.3.3 `emits:`** — sudah ada di code (`ResolveEmission` +
  `ValidateActionEmits` + custom action emission); ditandai ✅ di todo.

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` emits `on_create` (durable); Subscription
  `demo.product-created-audit` melanggan → create product → outbox worker
  dispatch handler script `handle_product_created.star` → outbox status
  `completed`. ✅
- `go test ./...` hijau (798 pass, termasuk unit test `internal/subscription`).

## File terdampak

- `internal/subscription/registry.go`, `dispatch.go`, `registry_test.go` (baru)
- `renderers/jsonb-persist/event_handler.go` (`Subscriptions` field + dispatch)
- `resource/formspec.go` (`buildSubscriptionRegistry` + wiring boot/reload)
- `cmd/formspec/dev.go` (`app.StartBackgroundWorkers()` sebelum serve)
- `internal/manifest/loader.go` (`RawSpecToSubscriptionSpec`)
- `examples/service-demo/` — contoh subscription (entity event + subscription + script)
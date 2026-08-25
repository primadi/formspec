# Delivery Channels — Pubsub (7.3.5)

**Tanggal:** 2026-08-25 · **Todo:** §7.3.5 (pubsub) · **Plan:** `docs/plan/fase7-delivery-channels.md`

## Apa yang ditambahkan

Event delivery kini mendukung **`pubsub` channel** — non-durable, at-most-once
(02-core-extended.md §3):

- **Pubsub channel** — `DeliveryEventHandler.PubSub` di
  `renderers/jsonb-persist/event_handler.go`: channel `pubsub` → publish event
  payload ke channel (dari `DeliveryTarget.Scope` atau default
  `{resource}.{event}`).
- **Shared pubsub instance** — `resource/formspec.go` membuat satu
  `memory.PubSub` yang dipakai BOTH `ctx.pubsub()` (via resolver) dan delivery
  channel, jadi subscriber via script menerima event yang di-publish. Disimpan
  di `App.pubsub` untuk reuse saat reload. `NewCtxPrimitiveResolver` menerima
  shared pubsub param.
- **Contoh** — service-demo `on_create` deliver pubsub `demo.products`.

`webhook` (outbound) dan `notification` (bridge ke `formspec/notify`) di-defer.

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` dengan event `on_create` deliver `pubsub` (channel
  `demo.products`); create product → outbox worker publish ke channel → outbox
  status `completed`. ✅
- Unit test `TestDeliveryEventHandler_PubsubChannel*` (channel dari
  `target.scope` + default `{resource}.{event}`). ✅
- `go test ./...` hijau (830 pass).

## File terdampak

- `renderers/jsonb-persist/event_handler.go` (`PubSub` field + channel handling)
- `renderers/jsonb-persist/event_handler_test.go` (unit test)
- `resource/formspec.go` (shared pubsub + wire boot + reload)
- `resource/ctxresolver.go` (`NewCtxPrimitiveResolver` shared pubsub param)
- `examples/service-demo/` (contoh pubsub delivery channel)
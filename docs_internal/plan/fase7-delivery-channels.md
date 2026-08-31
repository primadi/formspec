# Fase 7 — Delivery Channels (7.3.5)

**Status:** ✅ Pubsub channel complete · **Tanggal:** 2026-08-25
**Referensi:** `docs/spec/backend/02-core-extended.md` §3 (Subscription & Event
Delivery — Delivery channel)
**Todo:** `docs/plan/todo.md` §7.3.5

## Konteks

Event delivery saat ini mendukung `websocket` dan `audit_log` channels.
7.3.5 menambah delivery channel lain:

- `webhook` (outbound, HMAC signed, retry)
- `notification` (bridge ke `formspec/notify`)
- `pubsub` (non-durable, at-most-once)

## Scope

### DC-1 — Pubsub channel (7.3.5) ✅

- `renderers/jsonb-persist/event_handler.go` — `DeliveryEventHandler` ditambah
  `PubSub` field (interface `Publish(ctx, channel, payload)`).
- Channel `pubsub` → publish event payload ke channel (dari `DeliveryTarget.Scope`
  atau nama channel default `{resource}.{event}`). Non-durable, at-most-once.
- **Shared pubsub instance** — `resource/formspec.go` membuat satu
  `memory.PubSub` yang dipakai BOTH `ctx.pubsub()` (via resolver) dan delivery
  channel, jadi subscriber via script menerima event yang di-publish. Disimpan
  di `App.pubsub` untuk reuse saat reload.
- Contoh: service-demo `on_create` deliver pubsub `demo.products`.

### DC-2 — Webhook channel (7.3.5) ⏸️ deferred

- Outbound webhook delivery: HMAC signed, retry. Butuh HTTP client + URL config.
- Di-defer (butuh design decision untuk URL config + signing key).

### DC-3 — Notification channel (7.3.5) ⏸️ deferred

- Bridge ke `formspec/notify` module. Di-defer (module resmi belum ada).

## Level of effort

| DC  | Effort |
| --- | ------ |
| 1   | small  |
| 2   | medium |
| 3   | small  |

## Verifikasi end-to-end (via `formspec dev` + curl)

- Entity `product` dengan event `on_create` deliver `pubsub` (channel
  `demo.products`); create product → outbox worker publish ke channel → outbox
  status `completed`. ✅
- Unit test `TestDeliveryEventHandler_PubsubChannel*` (channel dari
  `target.scope` + default `{resource}.{event}`). ✅
- `go test ./...` hijau (830 pass).

## File terdampak

- `renderers/jsonb-persist/event_handler.go` — `PubSub` field + channel handling
- `renderers/jsonb-persist/event_handler_test.go` — unit test
- `resource/formspec.go` — shared pubsub + wire ke delivery handler (boot + reload)
- `resource/ctxresolver.go` — `NewCtxPrimitiveResolver` shared pubsub param
- `examples/service-demo/` — contoh pubsub delivery channel

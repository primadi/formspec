# Realtime: broadcast semua mutasi + listener-gated publish

## Perubahan

Realtime sekarang mengikuti kontrak Spec Resolution API §5 — channel
`entity:{module}.{name}` dengan event `created | updated | deleted` — untuk
**semua** mutasi, bukan hanya declared `events:` ber-`deliver: websocket`.

### Backend (listener-gated broadcast)

- **`internal/events/hub.go`**: interface `events.Hub` + method baru
  `HasListeners(workspaceID string) bool`. `NoopHub` → `false`.
- **`internal/api/wshub.go`**: `WSHub.HasListeners` (cheap RLock check).
- **`internal/action/deliver.go`**: helper baru `NotifyMutation(deps,
  workspaceID, resource, event)` — broadcast event generic ke hub **hanya
  jika ada ≥1 listener** (no-op kalau tidak ada). `DeliverEvents` websocket
  channel kini juga listener-gated: tanpa listener → skip push **dan** skip
  outbox insurance (realtime non-durable, tidak ada replay).
- **`internal/api/handler.go`**: `NotifyMutation` dipanggil setelah setiap
  mutasi sukses: `create`→`created`, `update`→`updated`,
  `delete`→`deleted`, `submit/cancel/amend`→`updated`, custom action →
  nama action-nya.
- **`renderers/jsonbpersist/event_handler.go`**: outbox worker websocket
  broadcast juga listener-gated (audit_log tetap jalan — itu governance,
  bukan realtime).

### Fix transport realtime di mode `--dev-ui` (akar masalah "realtime tidak jalan")

- **`renderers/web/vite.config.ts`**: proxy `/_ui/` dan `/api/v1` kini
  `ws: true` — tanpa ini WebSocket dari browser ke `/:ws/_ui/_ws` hang
  (proxy Vite tidak meneruskan upgrade), jadi realtime tidak pernah
  menerima event saat SPA diakses lewat Vite (`--dev-ui`).
- **`internal/api/wshub.go`**: `websocket.Accept` kini
  `InsecureSkipVerify: true` — coder/websocket default menolak handshake
  saat `Origin` ≠ `Host` (lewat proxy Vite: Origin browser `:5173` vs Host
  backend). Auth tetap dijaga: handshake via `?token=` (AuthMiddleware),
  filter permission per-pesan di `Broadcast` (2.6.6), channel push-only.

## Verifikasi (end-to-end, `--dev-ui`, backend :8080 + Vite :5173)

- Kanban: plain `POST /_ui/entity/clinic/visit` → kartu baru muncul live di
  kolom "Menunggu" (2→3) **tanpa reload** lewat event `created`.
- Custom action `cancel` → board live-update (Menunggu 3→0) lewat event
  action name.
- Direct WS `:8080` dan proxied WS `:5173` keduanya handshake OK.
- Unit test baru: `TestWSHub_HasListeners`, `TestNotifyMutation_*`,
  `TestDeliverEvents_NoListeners_SkipsWebsocketPublish`.

## Files affected

- `internal/events/hub.go`, `internal/api/wshub.go`, `internal/action/deliver.go`,
  `internal/api/handler.go`, `renderers/jsonbpersist/event_handler.go`
- `renderers/web/vite.config.ts`
- Test: `internal/api/wshub_test.go`, `internal/action/deliver_test.go`,
  `renderers/jsonbpersist/event_handler_test.go` (+ fakeHub `HasListeners`)

## Referensi

- Plan: `docs/plan/use-realtime-hook.md` §2 (kontrak §5 Realtime)
- Spek: `docs/spec/frontend/04-spec-resolution-api.md` §5

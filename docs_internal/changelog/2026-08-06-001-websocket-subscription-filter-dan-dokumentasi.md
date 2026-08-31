# Feat: Server-side subscription filter untuk realtime + dokumentasi WebSocket

## Perubahan

Realtime kini **subscription-based di sisi server** (Spec Resolution API §5):
server hanya mengirim event yang sesuai subscription client, menutup gap §7
(broadcast ke seluruh koneksi workspace tanpa filter resource).

### Server (`internal/api/wshub.go`)
- `wsConn` menyimpan state subscription (`subs map[resource]set[event]` + flag
  `all` untuk resource `"*"`), dilindungi mutex.
- Protokol frame dari client: `{op: subscribe|unsubscribe, resource, event?}`
  — `readPump` mem-parse frame teks dan menerapkannya; frame malformed/op
  tak dikenal diabaikan.
- `Broadcast` memfilter per subscription (`wants(resource, event)`) di atas
  filter workspace + permission (2.6.6). Semantik: tanpa subscription → tidak
  menerima apa pun; `"*"` → semua; resource → semua event-nya; resource+event
  → event itu.
- Test: `TestWSHub_*` baru (no-subscription, filter resource, star, event
  filter, unsubscribe, unsubscribe-event) + e2e frame di wire
  (`TestHandleWS_SubscribeFrameFiltersEndToEnd`,
  `TestHandleWS_UnsubscribeFrameEndToEnd`). Test lama di-update untuk
  subscribe dulu (semantik baru).

### Client (`renderers/web/src/hooks/useRealtime.ts`)
- `RealtimeClient` meng-agregasi union semua subscriber → mengirim **delta**
  subscribe/unsubscribe ke server saat set subscriber berubah (mis. pindah
  halaman), dan **resubscribe penuh** di `onopen` (setelah reconnect).
- Filter lokal resource/event tetap dipertahankan sebagai safety net.
- Header & komentar diperbarui; tsc bersih.

### Dokumentasi
- Baru: `docs/renderers/realtime.md` — transport realtime end-to-end (server +
  client): cara handle, optimasi, kind yang mendukung realtime, lifecycle
  navigasi/refresh/reconnect, wire protocol, gap.
- `docs/renderers/README.md` — tautan topik lintas realtime.
- `docs/spec/frontend/04-spec-resolution-api.md` §7 — catatan gap diperbarui
  (subscription filter & permission sudah ditutup implementasi resmi; sisa
  gap: `scope: user`, heartbeat).

## Verifikasi (end-to-end, `--dev-ui`)
- WS node: subscribe hanya `clinic/visit` → create visit (event diterima),
  create polyclinic (event **tidak** diterima — ter-filter).
- Browser: kanban live-update (Menunggu 2→3) via protokol subscription baru,
  tanpa reload.
- `go test ./internal/api/...` & paket terkait hijau; `tsc -b --noEmit` bersih.

## Files affected
- `internal/api/wshub.go`, `internal/api/wshub_test.go`,
  `internal/api/wshub_permission_test.go`
- `renderers/web/src/hooks/useRealtime.ts`
- Docs: `docs/renderers/realtime.md` (baru), `docs/renderers/README.md`,
  `docs/spec/frontend/04-spec-resolution-api.md`

## Referensi
- Plan: `docs/plan/use-realtime-hook.md`
- Spek: `docs/spec/frontend/04-spec-resolution-api.md` §5
- Implementasi: `docs/renderers/realtime.md`

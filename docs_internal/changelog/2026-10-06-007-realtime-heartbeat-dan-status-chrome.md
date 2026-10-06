# Feat: Heartbeat realtime (satu per sesi) + indikator status non-intrusif di chrome

## Perubahan

**Keputusan pemilik (2026-10-06):** heartbeat **hanya satu dan berlaku global per
sesi** — dimiliki transport, bukan per subscription/komponen. Sesi = 1 koneksi
WebSocket = 1 tab. Saat heartbeat mati, client menampilkan teks notifikasi yang
**tidak mengganggu** di **chrome app**; halaman yang tidak memakai realtime harus
**mengabaikan** koneksi yang putus.

### Server (`internal/api/wshub.go`)

- `heartbeatLoop` per `wsConn` (satu per koneksi = per sesi): tiap `hbInterval`
  (25 s) mengirim frame aplikasi `{"op":"hb","ts":…}` lewat `writePump`, dan
  menutup socket kalau tidak ada frame masuk dalam `hbInterval + hbTimeout`
  (35 s) → `readPump` unwinding → `defer unregister` (tidak ada koneksi stale).
- `wsConn.lastSeen` (atomic) diperbarui `readPump` pada **setiap** frame masuk;
  `hb_ack` ditangani eksplisit (liveness saja).
- Pulse-nya **frame teks, bukan ping protokol**: ping/pong level-protokol tidak
  terlihat JavaScript, jadi tanpa pulse aplikasi koneksi "sehat tapi sunyi" tak
  bisa dibedakan dari koneksi mati.
- Interval/timeout jadi field `WSHub` (bukan konstanta) supaya test bisa
  memperpendek tanpa pakai global.

### Client (`renderers/react-shadcn/src/hooks/useRealtime.ts`)

- `onmessage` memisahkan **frame kontrol lebih dulu**: `op` dikenal (saat ini
  `hb`) dibalas `hb_ack` dan **tidak pernah difan-out**. Tanpa ini subscriber
  `"*"` mencocoki frame tanpa `resource` dan memicu refetch palsu tiap 25 s.
- Satu watchdog liveness per koneksi (budget 45 s): di-reset setiap frame masuk,
  dimatikan saat tab `hidden`, dan saat habis → `stalled` + socket ditutup →
  jalur reconnect yang sudah ada mengambil alih. Watchdog sendiri yang menutup
  socket, jadi label `stalled` dipertahankan sampai frame berikutnya tiba.
- **Demand gate:** watchdog dan status hanya hidup kalau ada subscriber. Tanpa
  subscriber → `active: false`, status `idle`, dan `onclose` **tidak** dilaporkan
  sebagai masalah.

### Chrome (`shell/RealtimeStatus.tsx`, `stores/realtime.ts`, `shell/RegionShell.tsx`)

- Store baru `stores/realtime.ts` (`status`, `active`, `lastMessageAt`, `attempt`)
  sebagai satu-satunya sumber liveness.
- Pill kecil (`role="status"`, `aria-live="polite"`, dot amber) — **bukan**
  modal/toast — ditampilkan hanya saat `active` **dan** status `stalled`/
  `reconnecting`; `idle`/`connecting`/`live` render `null`. Ditempel di topbar
  auto-fill ketiga archetype (sidebar-nav, topnav, no-nav) di samping
  `ThemeSwitcher`.

## Verifikasi

- `go test ./internal/api/...` hijau; 3 test baru (`wshub_heartbeat_test.go`):
  frame `hb` tiba di wire, client yang menjawab `hb_ack` **tidak** ditutup,
  client yang diam ditutup **dan** ter-unregister (`HasListeners` kembali false).
- `vitest` **649/649** (13 test baru): `hb` dibalas tanpa fan-out, event asli
  tetap lewat, `live → stalled → live` saat heartbeat berhenti/lanjut, koneksi
  mati setelah `unsubscribe` tetap `idle`, dan indikator `null` saat `!active`.
- `tsc -b` exit 0; `gofmt -l internal/api/` bersih.

## Files affected

- `internal/api/wshub.go`, `internal/api/wshub_heartbeat_test.go`
- `renderers/react-shadcn/src/hooks/useRealtime.ts`, `.../useRealtime.test.ts`
- `renderers/react-shadcn/src/stores/realtime.ts`
- `renderers/react-shadcn/src/shell/RealtimeStatus.tsx`, `.../RealtimeStatus.test.tsx`
- `renderers/react-shadcn/src/shell/RegionShell.tsx`
- Docs: `docs/renderers/realtime.md`, `docs/spec/frontend/04-spec-resolution-api.md`,
  `docs_internal/plan/realtime-heartbeat.md`, `docs_internal/plan/todo.md`

Sisa (item bernomor): knob interval → **5.8.7 ⏸️**, API status untuk asset →
**5.8.8 ⏸️**.

# Plan & Implementasi: Widget Query Server-Side Filter + useRealtime

**Status**: Implemented & verified 2026-08-04 (browser `localhost:18080`).
**Referensi**: `docs/spec/frontend/04-spec-resolution-api.md` §5 (Realtime),
`docs/spec/frontend/06-page-kinds.md` §7 (Dashboard/Widget).

---

## 1. Widget `query` — server-side filter (bukan regex manual)

**Keputusan**: widget `query` diterjemahkan ke **list filter server-side**
(`field[op]=value`) — DB pre-filter sebelum data lewat wire; client-side
`applySimpleQuery` jadi **fallback/safety-net** saja.

### Kenapa
- `EntityStore.List` sudah dukung `eq/neq/in/nin/gt/gte/lt/lte/...`
  (`renderers/jsonbpersist/crud.go`), diterima lewat `parseListQuery`
  (`internal/api/handler.go`) dan `buildListParams` (`lib/api/client.ts`).
- Regex manual (approach lama) cuma paham 1 klausa dan tidak scalable.
- Starlark ditolak: evaluasi server-side = overhead VM per request + risiko
  sandbox/DoS; interpreter Starlark di browser = dependency berat. FormaExpr
  (`lib/formaexpr/`) adalah pilihan fallback client-side yang benar bila
  query tidak bisa di-push.

### Implementasi (`renderers/web/src/kinds/dashboard/DashboardRenderer.tsx`)
- `translateWidgetQuery(query)` → `Record<field, FilterOpValue>`:
  `field = today()` → `eq` (tanggal server), `field in [...]` → `in`,
  `field != 'v'` → `neq`, `field = 'v'`/`== 'v'` → `eq`; compound `and`
  → multi-filter (AND default). Klausa tak dikenal → `undefined` (fallback).
- `MetricWidget`/`ChartWidget` fetch pakai `buildListParams({ filters })`,
  lalu tetap jalankan `applySimpleQuery` sebagai final filter (idempotent).
- `applySimpleQuery` diperluas: compound `and`, `!=`, `==` (alias `=`).

### Bug timezone yang ditemukan saat verifikasi
`today()` awalnya pakai wall-clock browser (WIB, UTC+7). Tepat setelah tengah
malam WIB browser membaca tanggal baru (2026-08-05) sementara server masih
2026-08-04 → "Kunjungan Hari Ini" jadi 0. **Fix**: `today()` = **tanggal
server/business** (`new Date().toISOString().slice(0,10)`, UTC — semua
timestamp server RFC3339 UTC). `serverToday()` di kedua jalur (translate +
applySimpleQuery).

### Widget showcase (exclude `cancelled`)
- `visits-today`: `query: "transaction_date = today() and status != 'cancelled'"`
- `revenue-today`: `query: "transaction_date = today() and status != 'cancelled'"`
- `visits-by-polyclinic`: `query: "status != 'cancelled'"`

---

## 2. `useRealtime` hook (event-driven refetch)

### Kontrak backend (diverifikasi)
- Endpoint `/{workspace}/_ui/_ws` (push-only; `internal/api/wshub.go`,
  `router.go`). Pesan `EventMessage { event, resource: "module/entity",
  payload, emitted_at }` (`internal/events/hub.go`).
- Hub workspace-scoped + filter permission per pesan (`{module}.{plural}.view`,
  2.6.6). Dev (identity nil) → unfiltered → **client wajib filter by resource**.
- Hanya **declared entity events** ber-`deliver: websocket` yang di-push
  (mis. `visit.completed`) — bukan generic created/updated/deleted.
- **Non-durable** (spec §5): reconnect → refetch, tanpa replay.

### Implementasi frontend
- `renderers/web/src/types/events.ts` — `RealtimeMessage`.
- `renderers/web/src/hooks/useRealtime.ts` — koneksi WebSocket **singleton**
  dibagi antar konsumen; reconnect + exponential backoff (1s→15s);
  `useRealtime(resource, { event? })` mengembalikan `tick` yang naik tiap
  event cocok (filter `resource`/`event` lokal) DAN tiap reconnect (non-durable).
- `DashboardRenderer`: `realtime` (dari `DashboardSpec.realtime`) diturunkan
  ke `MetricWidget`/`ChartWidget`; masing-masing subscribe `module/name`
  (`resolveEntityRef(...).join("/")`) dan `tick` masuk dependency useEffect →
  refetch. Polling `refresh_secs` tetap sebagai backstop.
- `clinic-dashboard.yaml`: `realtime: true` (refresh: 60 tetap).

### Backend auth untuk WS (prod)
- Browser tidak bisa set header pada WS handshake → `AuthMiddleware`
  (`internal/api/middleware.go`) kini juga baca token dari `?token=` query
  param (fallback header). Catatan: token query bisa bocor ke log → prefer
  header untuk REST. Test: `TestAuthMiddleware_TokenQueryParam`.

### Verifikasi (end-to-end)
- Node WS client menerima `{"event":"completed","resource":"clinic/visit",...}`
  setelah `POST /_ui/entity/clinic/visit/{id}/complete`.
- Browser (tanpa reload): create visit #2 + complete → dashboard berubah
  **Kunjungan 1→2**, **Pendapatan Rp 0→Rp 25.000** lewat event push.

---

## 3. Bug pre-existing yang ditemukan & diperbaiki (blocking realtime demo)

| Bug | Fix |
|---|---|
| Guard `complete`: `!empty(diagnosis)` — `!` bukan operator Starlark (`not`) → konsultasi tidak pernah bisa selesai | `visit/entity.yaml` → `not empty(diagnosis)` |
| `complete.star` crash `NoneType value is not iterable` saat `treatments` kosong | `(resource.field.treatments or [])` |
| `visit.total` rule `positive` menolak 0 → visit tanpa treatment tidak bisa selesai | rule `[min: 0]` |

---

## Deferred
- Push `today()` literal ke server (server-resolve) alih-alih resolve client —
  lebih robust lintas zona waktu bisnis (opsional, backend change).
- Per-message server filter by resource sesuai kontrak penuh (sudah 2.6.6;
  §7 gap note soal broadcast — sebagian sudah ditutup).
- Timeline renderer (`TimelineRenderer`) realtime refetch — belum; pola sama
  dengan Kanban/Table, tinggal wiring.

## Done (2026-08-05)
- KanbanRenderer & TableRenderer row-level realtime refetch — `useRealtime`
  subscribe entity → silent refetch pada event/reconnect. Verified Kanban
  board: `visit.completed` → kolom "Selesai" card count 2→3 tanpa reload.
  Changelog: `2026-08-05-001-...`.

## Done (2026-08-05) — broadcast semua mutasi + listener-gated publish
Kontrak §5 diterapkan penuh: **semua** mutasi (create/update/delete/
submit/cancel/amend/custom action) kini di-broadcast ke listener WS lewat
channel `entity:{module}.{name}` (`created | updated | deleted` / nama
action), bukan hanya declared events. Broadcast **listener-gated**:
`HasListeners(workspaceID)` — tanpa listener, publish tidak dijalankan
(bahkan jika spec mendeklarasikan `deliver: websocket`); durable outbox
untuk channel non-websocket (audit_log/queue/reliable_event) tetap utuh.

Dua bug transport yang membuat realtime "tidak jalan" di `--dev-ui` ikut
ditemukan & diperbaiki: Vite proxy tidak meneruskan WS (kurang `ws: true`)
dan `websocket.Accept` menolak handshake saat Origin ≠ Host (proxy) →
`InsecureSkipVerify: true` (auth tetap via `?token=` + filter permission
per-pesan 2.6.6).

- Done (2026-08-06) — **server-side subscription filter** + dokumentasi:
  - Server (`internal/api/wshub.go`): `wsConn` simpan subscription
    (`subs map[resource]set[event]` + `all`); protokol frame
    `{op: subscribe|unsubscribe, resource, event?}` diparse `readPump`;
    `Broadcast` filter per subscription (`wants`) di atas workspace +
    permission (2.6.6). Tanpa subscription → tidak menerima apa pun.
  - Client (`useRealtime.ts`): `RealtimeClient` agregasi union → delta
    subscribe/unsubscribe saat set berubah (pindah halaman), resubscribe
    penuh di `onopen`; filter lokal tetap sebagai safety net.
  - Docs: `docs/renderers/realtime.md` (baru), README renderers, spec §7
    gap diperbarui.
  - Verified: WS node (subscribe clinic/visit → visit diterima, polyclinic
    ter-filter); kanban live-update via protokol baru. Changelog:
    `2026-08-06-001-...`.

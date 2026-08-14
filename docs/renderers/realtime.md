# Realtime WebSocket — Implementasi Transport

**Updated:** 2026-08-06

Dokumen implementasi transport realtime FormSpec: bagaimana WebSocket di-handle di
sisi server (`internal/api/wshub.go`) dan di sisi client (renderer shadcn-shell,
`renderers/react-shadcn/src/hooks/useRealtime.ts`), optimasi yang sudah dilakukan, kind
yang mendukung realtime, dan bagaimana lifecycle perpindahan halaman / refresh /
putus koneksi ditangani. Kontrak normatifnya ada di
[`../spec/frontend/04-spec-resolution-api.md`](../spec/frontend/04-spec-resolution-api.md) §5.

---

## 1. Gambaran

Realtime adalah **kapabilitas inti** Spec Resolution API §5: shell browser
subscribe secara deklaratif terhadap perubahan entity, event dikirim lewat
WebSocket ke client yang sedang terhubung. Prinsip kuncinya:

- **Non-durable by definition** — tidak ada replay. Client yang reconnect
  (atau baru membuka halaman) refetch data segar; event yang terlewat selama
  offline tidak diputar ulang.
- **Subscription-based** — client memberi tahu server resource (dan event)
  apa yang ia minati; server hanya mengirim event yang cocok dengan
  subscription tersebut (ditambah filter permission).
- **Satu koneksi per tab** — semua komponen dalam satu tab berbagi satu
  WebSocket; fan-out terjadi di sisi client.

---

## 2. Sisi Server

### 2.1 Endpoint, auth, dan koneksi

| Aspek        | Detail                                                                                                                                                                                                                             |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Endpoint     | `/{workspace}/_ui/_ws` (`internal/api/router.go` → `HandleWS` di `internal/api/wshub.go`)                                                                                                                                          |
| Auth         | `AuthMiddleware` — identity dari header `Authorization` atau query `?token=` (browser tidak bisa set header saat WS handshake). Dev fallback → identity `nil`                                                                      |
| Origin check | `websocket.Accept(..., &AcceptOptions{InsecureSkipVerify: true})` — diperlukan saat SPA diakses lewat reverse proxy dev (Vite) yang membuat `Origin` ≠ `Host`; auth tetap dijaga oleh AuthMiddleware + filter permission per-pesan |
| Protocol     | Push-only untuk data; frame masuk dari client hanya subscription-control                                                                                                                                                           |

Koneksi (`wsConn`) punya channel `send` buffered (kapasitas 32) dan satu
writer goroutine sendiri (`writePump`) — socket yang lambat/macet di-drop
pesannya (`select … default:`), tidak pernah memblokir hub atau koneksi lain.

Lifecycle: `HandleWS` meng-upgrade → `hub.register(conn)` → `readPump` berjalan
(menyajikan control frame ping/pong/close dan menerapkan subscription frame) →
saat client putus, `readPump` return → `defer hub.unregister(conn)` membersihkan
koneksi. Tidak ada kebocoran koneksi stale (termasuk saat browser di-refresh:
koneksi lama terdeteksi putus dan di-unregister).

### 2.2 Protokol subscription (subscribe / unsubscribe)

Setelah handshake, client mengirim frame JSON:

```jsonc
{ "op": "subscribe",   "resource": "clinic/visit" }               // semua event resource
{ "op": "subscribe",   "resource": "clinic/visit", "event": "created" } // satu event
{ "op": "unsubscribe", "resource": "clinic/visit" }               // lepas resource
{ "op": "unsubscribe", "resource": "clinic/visit", "event": "created" }
{ "op": "subscribe",   "resource": "*" }                          // semua resource di workspace
```

- `resource` `"*"` = subscribe ke seluruh workspace.
- `event` kosong = semua event pada resource; terisi = hanya event tersebut.
- Frame malformed / `op` tak dikenal diabaikan (koneksi tetap hidup).

`readPump` mem-parse frame teks ini dan memperbarui state subscription
per-koneksi (`wsConn.subs map[resource]set[event]` + flag `all` untuk `"*"`),
dilindungi mutex.

### 2.3 Filtering saat broadcast

`WSHub.Broadcast(workspaceID, msg)` mengirim `msg` ke koneksi dalam workspace
dengan **tiga lapis filter**:

1. **Workspace** — hanya koneksi di workspace yang sama.
2. **Permission per-pesan (2.6.6)** — hanya koneksi yang identity-nya punya
   `{module}.{plural}.view` untuk `msg.Resource` yang menerima; identity `nil`
   (dev fallback) atau resource yang tidak bisa di-resolve → tidak difilter
   (fail-open, bukan deny).
3. **Subscription** — `wsConn.wants(resource, event)`:
   - tanpa subscription → **tidak menerima apa pun**;
   - subscribe `"*"` → menerima semua;
   - subscribe resource → menerima event resource itu;
   - subscribe resource+event → hanya event yang cocok.

Artinya: server **hanya mengirim event yang sesuai subscription client** —
tidak lagi mengirim semua event workspace lalu menyuruh client membuang yang
tidak relevan (menutup gap yang dicatat di spec §7).

### 2.4 Delivery event dari mutasi

Dua jalur yang mengisi hub:

- **`action.NotifyMutation`** (`internal/action/deliver.go`) — dipanggil tiap
  handler mutasi sukses (create/update/delete/submit/cancel/amend/custom
  action) → broadcast event generic `created | updated | deleted` (atau nama
  action) pada `module/entity`. **Listener-gated**: kalau `HasListeners`
  false, tidak ada pekerjaan sama sekali.
- **`action.DeliverEvents`** — mengirim declared events (`events:` dengan
  `deliver: [websocket]`) pada resource. Untuk channel websocket juga
  listener-gated; event durable di-enqueue ke outbox sebagai insurance, dan
  outbox worker (`renderers/jsonb-persist/event_handler.go`) ikut
  listener-gated saat me-broadcast ke websocket. Channel non-websocket
  (audit_log, queue, reliable_event) tidak terpengaruh oleh gating ini.

`events.Hub` (`internal/events/hub.go`) adalah kontrak minimal yang dipakai
delivery code: `Broadcast(workspaceID, msg)` + `HasListeners(workspaceID) bool`.

---

## 3. Sisi Client

### 3.1 Hook `useRealtime`

`renderers/react-shadcn/src/hooks/useRealtime.ts` — hook inti:

```ts
const tick = useRealtime("clinic/visit") // semua event entity
const tick = useRealtime("clinic/visit", { event: "completed" }) // satu event
```

- Mengembalikan `tick` (angka) yang naik setiap ada event cocok **dan** setiap
  reconnect. Consumer memperlakukan `tick` sebagai pemicu refetch: mengubah
  dependency `useEffect`-nya dan me-refetch data.
- Refetch hasil realtime bersifat **silent** (tidak menampilkan spinner
  loading) di renderer Kanban/Table/Dashboard.

### 3.2 `RealtimeClient` — singleton, satu koneksi per tab

Semua pemanggil `useRealtime` dalam satu tab berbagi satu instance
`RealtimeClient` (modul-level singleton). Tiap komponen hanya mendaftarkan
objek subscription ringan `{ resource, event?, onEvent, onReconnect }` ke
sebuah `Set`; **tidak membuka koneksi WebSocket sendiri**.

`RealtimeClient` melakukan:

1. **Agregasi union** — menghitung gabungan interest semua subscriber
   (resource → set event; `""` = semua event).
2. **Delta subscribe/unsubscribe** — saat set subscriber berubah, mengirim
   hanya perbedaan ke server (bukan seluruh state):
   - resource yang tidak lagi diinginkan siapa pun → `unsubscribe`;
   - resource baru / event baru → `subscribe`;
   - perpindahan mode "semua event" ↔ "event spesifik" → disinkronkan ulang.
3. **Resubscribe penuh saat (re)connect** — di `onopen`, karena realtime
   non-durable, seluruh union dikirim ulang ke server.
4. **Filter lokal (safety net)** — `onmessage` tetap memfilter `resource`/
   `event` sebelum fan-out ke subscriber; idempotent terhadap filter server.
5. **Reconnect otomatis** — `onclose` → retry dengan exponential backoff
   (1s → 2s → … → cap 15s), reset ke 1s saat `onopen`. Setiap close memanggil
   `onReconnect` semua subscriber → `tick` naik → refetch.
6. **`configure(url)`** — mengganti URL (ganti workspace/token) menutup &
   membuka koneksi baru; no-op kalau URL sama.

### 3.3 Perpindahan antar halaman (page navigation)

- Setiap `useRealtime` adalah `useEffect`; saat komponen **unmount** (pindah
  halaman), React menjalankan cleanup → subscription dihapus dari `Set` →
  `RealtimeClient` menghitung ulang union → mengirim delta `unsubscribe`
  untuk resource yang tidak lagi di-subscribe siapa pun.
- Komponen page baru mount → `useRealtime(resourceBaru)` → subscribe baru.
- **Subscriber level global app** (komponen di shell/layout yang tidak pernah
  unmount, mis. badge notifikasi) tetap ter-subscribe lintas halaman — resource
  yang masih dipegang subscriber global tidak di-unsubscribe.
- **Koneksi WebSocket tidak ikut mati** saat pindah halaman — singleton
  bertahan; hanya set subscription yang berubah.

### 3.4 Refresh browser & putus koneksi

- **Refresh (F5):** JS context lama hancur (WS lama ditutup browser; server
  mendeteksi via `readPump` → `unregister`), halaman baru membuat koneksi
  segar, subscribe ulang, dan refetch data awal. Aman — tidak ada replay yang
  hilang (non-durable).
- **Internet putus:** browser men-detect → `onclose` → reconnect otomatis
  (backoff) → begitu `onopen`, resubscribe + semua subscriber `onReconnect` →
  refetch. Belum ada heartbeat ping/pong eksplisit; deteksi putus bergantung
  browser/OS (umumnya cukup cepat).

---

## 4. Optimasi yang Sudah Dilakukan

| #   | Optimasi                                    | Keterangan                                                                                                                                           |
| --- | ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Satu WebSocket per tab**                  | Singleton `RealtimeClient` dibagi semua konsumen; bukan satu koneksi per listener                                                                    |
| 2   | **Server-side subscription filter**         | Server hanya mengirim event sesuai subscription; hemat bandwidth & privasi (tidak bocorkan event entity lain)                                        |
| 3   | **Filter permission per-pesan** (2.6.6)     | Event tidak diterima koneksi tanpa permission `view` atas resource-nya                                                                               |
| 4   | **Listener-gated publish** (`HasListeners`) | Tanpa listener → `NotifyMutation`/`DeliverEvents`/outbox-worker websocket tidak menjalankan apa pun, meski spec mendeklarasikan `deliver: websocket` |
| 5   | **Slow-consumer drop**                      | Channel send buffered (32) + `select default` — socket lambat tidak pernah memblokir hub                                                             |
| 6   | **Satu writer goroutine per koneksi**       | `writePump` mengisolasi penulisan per socket                                                                                                         |
| 7   | **Delta subscription sync**                 | Client hanya mengirim perubahan subscription, bukan seluruh state, tiap kali ada perubahan                                                           |
| 8   | **Exponential backoff reconnect**           | 1s → 15s (cap), reset saat sukses; + refetch on reconnect (non-durable)                                                                              |
| 9   | **Auto unsubscribe / global persist**       | Lifecycle React: unsubscribe otomatis saat pindah halaman; subscriber global di shell bertahan                                                       |

---

## 5. Kind / Widget yang Mendukung Realtime

### Sudah berjalan (renderer memakai `useRealtime`)

| Kind          | Flag                       | Perilaku                                                                |
| ------------- | -------------------------- | ----------------------------------------------------------------------- |
| **Table**     | `realtime: true`           | Silent refetch baris pada event entity (created/updated/deleted/action) |
| **Kanban**    | `realtime: true` (default) | Silent refetch — kartu muncul/pindah/berubah status dari client lain    |
| **Dashboard** | `realtime: true`           | Widget metric & chart refetch pada event entity sumbernya               |

Widget (`kind: Widget`) **tidak punya flag realtime sendiri** — mewarisi flag
`realtime` dashboard tempat ia ditempel; `refresh` (polling) tetap berfungsi
sebagai backstop.

### Terdefinisi di spec tapi belum diimplementasikan

| Kind                   | Status renderer                                                                                           |
| ---------------------- | --------------------------------------------------------------------------------------------------------- |
| **Calendar**           | Field `realtime` ada di spec; renderer `calendar/` belum ada                                              |
| **ApprovalInbox**      | Field `realtime` ada; renderer `approval-inbox/` belum ada                                                |
| **NotificationCenter** | Field `realtime` ada; renderer `notification-center/` belum ada                                           |
| **Timeline**           | Renderer `timeline/` ada, tapi belum memakai `useRealtime` — tinggal wiring pola sama dengan Kanban/Table |

---

## 6. Wire Protocol (Referensi)

```
Client ──► Server (text frame)
  { "op": "subscribe",   "resource": "clinic/visit" }
  { "op": "subscribe",   "resource": "clinic/visit", "event": "created" }
  { "op": "unsubscribe", "resource": "clinic/visit" }
  { "op": "unsubscribe", "resource": "clinic/visit", "event": "created" }
  { "op": "subscribe",   "resource": "*" }

Server ──► Client (text frame, push-only)
  { "event": "created", "resource": "clinic/visit", "payload": {...}, "emitted_at": "..." }
```

Semantik subscription server (`wsConn.wants`):

- Tidak pernah subscribe → tidak menerima apa pun.
- subscribe `"*"` → semua resource di workspace (tetap tunduk filter permission).
- subscribe `resource` (tanpa event) → semua event resource itu.
- subscribe `resource` + `event` → hanya event itu; unsubscribe event spesifik
  tidak menghapus resource selama masih ada event/subscription lain.

---

## 7. Gap & Pekerjaan ke Depan

- **Timeline realtime** belum di-wire (renderer sudah ada).
- **Heartbeat ping/pong** belum ada — deteksi putus bergantung browser/OS;
  untuk deteksi lebih agresif bisa ditambah ping interval.
- **`scope: user`** belum didukung — hanya `{scope: workspace}` (satu-satunya
  target yang dipakai; target `user` adalah penambahan index kedua, bukan
  redesign).
- Renderer **Calendar / ApprovalInbox / NotificationCenter** belum dibuat.

---

## 8. File Kunci

| Path                                                         | Peran                                                                 |
| ------------------------------------------------------------ | --------------------------------------------------------------------- |
| `internal/api/wshub.go`                                      | WSHub, connection manager, subscription filter, read/write pump       |
| `internal/api/router.go`                                     | Route `/{workspace}/_ui/_ws`                                          |
| `internal/events/hub.go`                                     | Kontrak `events.Hub` (`Broadcast` + `HasListeners`)                   |
| `internal/action/deliver.go`                                 | `NotifyMutation` (generic events) + `DeliverEvents` (declared events) |
| `renderers/jsonb-persist/event_handler.go`                    | Outbox worker → websocket (listener-gated)                            |
| `renderers/react-shadcn/src/hooks/useRealtime.ts`            | Hook + singleton `RealtimeClient`                                     |
| `renderers/react-shadcn/src/kinds/{table,kanban,dashboard}/` | Renderer yang memakai realtime                                        |

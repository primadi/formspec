# Plan: Realtime Heartbeat — Satu Heartbeat, Global per Sesi

**Status**: Planned → implementasi setelah plan ini disetujui
**Prioritas**: Medium — deteksi putus yang agresif + visibilitas status ke user
**LoE**: Medium (server + client + chrome, ±1–2 hari)
**Todo**: **5.8.5**
**Referensi**:

- `internal/api/wshub.go` — `WSHub`, `wsConn`, `HandleWS`, `readPump`, `writePump`
- `renderers/react-shadcn/src/hooks/useRealtime.ts` — singleton `RealtimeClient`
- `renderers/react-shadcn/src/shell/RegionShell.tsx` — chrome regions
- `docs/renderers/realtime.md` §3.4/§7 (gap yang ditutup)
- `docs/spec/frontend/04-spec-resolution-api.md` §5/§7

---

## 1. Keputusan yang mengikat (dari pemilik proyek, 2026-10-06)

1. **Heartbeat hanya satu dan berlaku global per sesi.** Dimiliki transport —
   satu `wsConn` di server, satu `RealtimeClient` (singleton) per tab di client —
   **bukan** per `useRealtime`/subscription. N komponen dalam satu sesi tetap
   menghasilkan **satu** heartbeat.
2. **Sesi = 1 koneksi WebSocket = 1 tab.** Tidak ada koordinasi antar-tab
   (`SharedWorker`/`BroadcastChannel`) — di luar cakupan.
3. **Saat heartbeat mati, client menampilkan teks notifikasi yang tidak
   mengganggu**, ditempatkan di **chrome app** (bukan modal/dialog/alert).
4. Plan dulu, implementasi kemudian.

## 2. Masalah

- Transport realtime hari ini **tidak punya heartbeat**. Deteksi putus
  bergantung browser/OS: koneksi _half-open_ (kabel dicabut, NAT/proxy
  membuang state, laptop sleep) bisa tetap terlihat `OPEN` di sisi JS selama
  menit-menit, sementara tidak ada event yang masuk. Karena realtime
  **non-durable**, diamnya koneksi = data basi tanpa satu pun sinyal ke user.
- Protokol ping/pong bawaan WebSocket (yang ditangani library `coder/websocket`)
  **tidak terlihat oleh JavaScript** — jadi ping level-protokol saja tidak bisa
  dipakai client untuk menyimpulkan apa pun. Kalau workspace kebetulan sepi
  event, client tidak bisa membedakan "sehat tapi sunyi" dari "mati".

Konsekuensi: sinyal liveness harus **terlihat di level aplikasi** (frame teks),
dan statusnya harus bisa ditampilkan di chrome.

## 3. Desain

### 3.1 Prinsip

- **Satu mekanisme**, bukan dua: heartbeat level-aplikasi (frame teks) di kanal
  yang sudah ada. Protokol ping/pong bawaan tetap dibiarkan apa adanya (library),
  tidak dijadikan sumber kebenaran.
- **Kepemilikan transport.** Timer/ticker hidup di `wsConn` (server) dan
  `RealtimeClient` (client). Tidak ada API `useRealtime` baru untuk heartbeat.
- **Non-intrusif.** Indikator hanya muncul saat bermasalah; saat `live` chrome
  tidak berubah sama sekali (tidak ada dot permanen, tidak ada toast default).

### 3.2 Wire protocol (tambahan)

Di atas protokol subscription yang sudah ada (§2.2 `realtime.md`):

```
Server ──► Client   { "op": "hb", "ts": 1759718400123 }   // tiap `hbInterval`
Client ──► Server   { "op": "hb_ack" }                    // jawaban; juga liveness
```

- Frame `op`-nya **bukan event** — tidak punya `resource`/`event`, jadi tidak
  boleh difan-out ke subscriber. (Ini sekaligus menutup lubang: subscriber
  `resource: "*"` saat ini akan **cocok** dengan frame apa pun yang tidak punya
  `resource`, sehingga `onEvent` ikut terpanggil — harus difilter sebelum
  fan-out.)
- `hb_ack` juga berfungsi sebagai liveness server-side: setiap frame masuk
  (subscribe/unsubscribe/hb_ack) adalah bukti peer hidup.

Default: `hbInterval = 25s`, toleransi server `hbTimeout = 10s`, toleransi
client `HB_STALL_MS = 45s` (satu heartbeat boleh hilang sebelum dinyatakan
macet).

### 3.3 Server (`internal/api/wshub.go`)

| Perubahan   | Detail                                                                                                                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `wsConn`    | tambah `hb chan struct{}` (pulse ke `writePump`) + `lastSeen atomic.Int64` + `touch()` / `lastSeenAt()`                         |
| `WSHub`     | tambah field `hbInterval`, `hbTimeout` (default dari `NewWSHub`; test boleh memperpendek)                                       |
| `HandleWS`  | buat channel `hb`; jalankan **satu** `heartbeatLoop` per koneksi dalam `WaitGroup` yang sudah ada; `close(stop)` tetap di akhir |
| `writePump` | `select` tambah `case <-c.hb:` → tulis `{"op":"hb","ts":…}` (satu-satunya penulis socket, jadi tidak ada write bersamaan)       |
| `readPump`  | `c.touch()` pada **setiap** frame masuk; `case "hb_ack":` eksplisit (no-op, liveness sudah dicatat)                             |

`heartbeatLoop` (satu goroutine per koneksi):

```text
setiap tick (hbInterval):
    jika now - lastSeen > hbInterval + hbTimeout  → peer macet
        → conn.Close(StatusPolicyViolation, "heartbeat timeout") → return
    kalau tidak → kirim pulse ke c.hb (non-blocking; pulse yang belum sempat
                 ditulis digantikan tick berikutnya)
```

Batas deteksi yang dinyatakan (jujur, bukan klaim lebih): dengan pemeriksaan
yang menempel ke ticker, koneksi macet ditutup dalam **≈2 × hbInterval**
(±50 s pada default). Ini tetap jauh lebih cepat daripada menunggu TCP/browser.

Penutupan oleh `heartbeatLoop` membuat `readPump` unwinding → `defer
unregister` membersihkan koneksi (tidak ada perubahan lifecycle lain).

### 3.4 Client (`useRealtime.ts`)

| Perubahan      | Detail                                                                                                                                                                                               |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `onmessage`    | (a) `markAlive()`, (b) **filter frame kontrol sebelum fan-out** (`typeof frame.op === "string"` → jika `"hb"` balas `{"op":"hb_ack"}`; jangan teruskan ke subscriber)                                |
| liveness timer | satu `setTimeout` per koneksi (di singleton, bukan per hook). Di-reset setiap frame masuk dan saat `onopen`; saat habis → status `stalled` lalu `ws.close()` (memicu jalur reconnect yang sudah ada) |
| visibility     | saat tab `hidden`, timer stall **dimatikan** (browser men-throttle timer → jangan memvonis macet palsu); saat `visible` lagi, timer dipasang ulang dan stale-check dijalankan sekali                 |
| status         | dipublikasikan ke store baru `stores/realtime.ts` (`idle`/`connecting`/`live`/`stalled`/`reconnecting` + `lastMessageAt` + `attempt`)                                                                |
| `configure()`  | reset status ke `connecting` dan `attempt` ke 0 saat URL/token berubah                                                                                                                               |

`useRealtime` **tidak berubah** dari sisi konsumen: tetap mengembalikan `tick`
(naik saat event cocok dan saat reconnect). Heartbeat hanya menambah kecepatan
deteksi reconnect.

### 3.5 Chrome: `RealtimeStatus`

Komponen baru `renderers/react-shadcn/src/shell/RealtimeStatus.tsx`:

- `live`/`connecting`/`idle` → **render `null`** (chrome bersih).
- `stalled` → pill kecil: dot amber + **"Realtime tidak merespons"**,
  `title="Koneksi realtime tidak merespons — data mungkin tidak terbaru"`.
- `reconnecting` → pill kecil: dot amber berdenyut +
  **"Menyambung ulang…"** (`(n)` bila ada percobaan).
- `role="status"` + `aria-live="polite"`; `text-xs`, border muted — bukan
  modal, bukan toast default (agar tidak mengganggu).

Penempatan: **topbar auto-fill** `RegionSlot` (`RegionShell.tsx`), di samping
`ThemeSwitcher` — jadi terlihat di ketiga archetype (`sidebar-nav`, `topnav`,
`no-nav`). Komponen membaca store, jadi **tidak** menambah koneksi/timer.

**Batas yang dinyatakan:** kalau App mengganti topbar dengan component custom
(`regions.topbar: <component>`), indikator bawaan tidak ikut tampil. Store-nya
bisa dibaca component itu; menyediakan API `formspec.realtimeStatus()` untuk
asset dicatat sebagai pekerjaan lanjutan, bukan bagian plan ini.

## 4. Alternatif yang ditolak

| Alternatif                                         | Alasan ditolak                                                                                       |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Heartbeat per `useRealtime`                        | Melanggar keputusan #1 — N komponen = N timer; dan komponen unmount akan mematikan sinyal milik sesi |
| Hanya ping/pong protokol (`conn.Ping`)             | Tidak terlihat JS; client tetap tidak bisa memutuskan status                                         |
| Ping protokol + timer client "ada pesan?"          | Salah klasifikasi: workspace yang sehat tapi sepi event akan dianggap mati                           |
| Toast `sonner` saat putus                          | Mengganggu (keputusan #3: teks di chrome, non-intrusif)                                              |
| Koordinasi antar-tab (1 heartbeat untuk semua tab) | Di luar cakupan keputusan #2 (sesi = 1 tab)                                                          |

## 5. File yang tersentuh

**Backend**

- `internal/api/wshub.go` — heartbeat loop, `touch`, frame `hb`, `writePump`/`readPump`
- `internal/api/wshub_heartbeat_test.go` (baru) — frame `hb` tiba; koneksi tanpa ack ditutup; ack menjaga koneksi hidup

**Frontend**

- `renderers/react-shadcn/src/stores/realtime.ts` (baru) — status store
- `renderers/react-shadcn/src/hooks/useRealtime.ts` — filter frame kontrol + ack + liveness timer + status publish
- `renderers/react-shadcn/src/shell/RealtimeStatus.tsx` (baru) — pill
- `renderers/react-shadcn/src/shell/RegionShell.tsx` — mount di topbar auto-fill
- `renderers/react-shadcn/src/shell/RealtimeStatus.test.tsx` (baru)
- `renderers/react-shadcn/src/hooks/useRealtime.test.ts` (baru) — status `live → stalled → reconnecting`, frame `hb` tidak difan-out

**Docs**

- `docs/renderers/realtime.md` — §2.2 (wire), §2.3/§3 (heartbeat), §4 (optimasi), §7 (gap ditutup)
- `docs/spec/frontend/04-spec-resolution-api.md` §5 + §7
- `docs_internal/plan/todo.md` (5.8.5 ✅ + 5.8.6 tetap ⏸️)
- `docs_internal/changelog/2026-10-06-007-*.md`

## 6. Verifikasi

- **Go**: `go test ./internal/api/...` — 3 test baru hijau; test e2e lama tetap
  hijau (heartbeat default 25 s tidak menyentuh jendela baca 2 s mereka).
- **Client**: `npx vitest run src/hooks/useRealtime.test.ts src/shell/RealtimeStatus.test.tsx`
  - `npx tsc -b` bersih.
- **Manual (dev-ui)**: buka App dengan `realtime: true`, putuskan jaringan
  (DevTools → Offline) → pill "Realtime tidak merespons"/"Menyambung ulang…"
  muncul di topbar tanpa mengganggu; sambungkan kembali → pill hilang dan
  `tick` naik (refetch).

## 7. Risiko

| Risiko                                                             | Mitigasi                                                                                                                              |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| Timer stall kena throttle di tab background → notif palsu          | Timer dimatikan saat `hidden`, dipasang ulang + stale-check saat `visible`                                                            |
| Client lama (SPA ter-cache) menganggap `{"op":"hb"}` sebagai event | Hanya subscriber `"*"` yang terpengaruh, dan efeknya refetch ekstra tiap 25 s — dicatat sebagai catatan kompatibilitas, bukan blocker |
| Heartbeat menambah trafik                                          | Satu frame kecil per koneksi per 25 s; listener-gated seperti publish lain                                                            |
| Test lama membaca frame `hb` alih-alih broadcast                   | Interval default 25 s > semua jendela baca test                                                                                       |

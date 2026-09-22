# Plan: WS Handshake Auth via Single-Use Ticket (`?ticket=`)

**Status**: Planned (belum diimplementasi)
**Prioritas**: Medium — security hardening, bukan blocker
**LoE**: Small (backend + client, ±1 hari)
**Referensi**:
- `docs_internal/plan/use-realtime-hook.md` §"Backend auth untuk WS" (asal-usul `?token=`)
- `docs/renderers/realtime.md` (kontrak auth realtime yang akan diupdate)
- `docs/spec/frontend/04-spec-resolution-api.md` §5 (Realtime)
- Pola existing yang konsisten: OAuth `state` single-use (`internal/api/oauth_handler.go`),
  storage `?link_token=` (`internal/api/file.go`), reset/verify token single-use (`internal/auth/service.go`)

---

## 1. Masalah

Saat ini WS handshake ke `/{ws}/_ui/_ws` mengandalkan `?token=<JWT>` (query param)
karena browser tidak bisa set header pada WS handshake. Token di query string
berisiko bocor ke:

- **Server/proxy access log** (nginx, Caddy, request logging) — risiko utama;
  JWT full-lifetime ikut terekam.
- Referrer / browser history — risiko rendah untuk SPA, tapi tetap ada.

Risiko saat ini *mitigated* karena yang dikirim hanya access token ber-TTL
pendek, koneksi push-only, dan permission difilter per-message di `Broadcast`
(2.6.6). Tapi pola yang lebih benar adalah **one-time ticket**:

```
1. Client →  POST /{ws}/_ui/_ws/ticket        (Auth: Bearer header — aman)
2. Server →  { ticket: "rnd-…", ttl: 30s }    (opaque, single-use, bound ke identity+workspace)
3. Client →  WS   /{ws}/_ui/_ws?ticket=...    (handshake)
4. Server →  konsumsi ticket sekali → resolve identity → hapus dari store
```

Log hanya melihat ticket opaque yang mati dalam ±30 detik; replay kedua kali
ditolak; JWT tidak pernah muncul di URL.

## 2. Desain

### 2.1 Ticket store (backend)

- Di `internal/api` — in-memory `map[ticket]wsTicket` + mutex, persis pola
  `oauthStates` di `oauth_handler.go`:
  ```go
  type wsTicket struct {
      workspaceID string
      identity    *Identity // resolved saat issuance
      expiresAt   time.Time // TTL 30s
  }
  ```
- Issuance menyalin identity hasil validasi Bearer — ticket **terikat** ke
  identity + workspace penerbit, tidak bisa dipakai user lain.
- Konsumsi (di `HandleWS`): `GetAndDelete` — single-use; expired/unknown →
  close sebelum upgrade (`401`, bukan upgrade lalu gagal).

### 2.2 Endpoint issuance

`POST /{ws}/_ui/_ws/ticket` (UI surface, butuh auth standar via header Bearer
atau session cookie):

- Request: kosong (identity dari middleware).
- Response: `{"ticket": "…", "expires_in": 30}`.
- Rate-limit ringan (mis. maks N ticket/identity/menit) untuk cegah abuse.

### 2.3 Handshake

- `HandleWS` (`internal/api/wshub.go`) membaca `?ticket=` **sebelum**
  `websocket.Accept`; ticket valid → inject identity ke context lalu lanjut
  jalur lama. Tanpa `?ticket=` → fallback `?token=` **tetap dipertahankan**
  (deprekasi bertahap; bantu migrasi bertahap + kompatibilitas client lama).
- Setelah semua konsumen pindah ke ticket, `?token=` untuk path `_ws` bisa
  dicabut (follow-up).

### 2.4 Client (`renderers/react-shadcn/src/hooks/useRealtime.ts`)

- Sebelum `new WebSocket(...)`: `POST /_ui/_ws/ticket` (dengan header Bearer
  standar dari `lib/api/client.ts`) → ambil ticket → connect dengan
  `?ticket=`.
- Reconnect (exponential backoff) selalu **minta ticket baru** — ticket
  single-use tidak bisa dipakai ulang.
- Refresh token/expiry: ticket di-request fresh tiap koneksi, jadi tidak ada
  masalah token expired saat long-lived connection.

## 3. File yang dibuat/diubah

| File | Perubahan |
|---|---|
| `internal/api/wsticket.go` (baru) | Ticket store + issuance handler + konsumsi |
| `internal/api/router.go` | Route `POST /{ws}/_ui/_ws/ticket` |
| `internal/api/wshub.go` | `HandleWS`: konsumsi `?ticket=` sebelum upgrade |
| `internal/api/middleware.go` | (opsional) strip `token=` dari logging WS; tetap dukung `?token=` fallback |
| `renderers/react-shadcn/src/hooks/useRealtime.ts` | Fetch ticket sebelum connect + di reconnect |
| `docs/renderers/realtime.md` | Update tabel Auth: ticket = jalur utama, `?token=` = fallback deprecated |
| `docs/spec/frontend/04-spec-resolution-api.md` | §5: sebutkan ticket flow |

## 4. Test

- `TestWSTicketIssueAndConnect` — issue dengan Bearer → connect `?ticket=` →
  identity ter-resolve → event diterima.
- `TestWSTicketSingleUse` — ticket kedua kali → ditolak (401 / no upgrade).
- `TestWSTicketExpired` — tiket > TTL → ditolak.
- `TestWSTicketCrossWorkspace` — ticket workspace A dipakai di path
  workspace B → ditolak (terikat penerbit).
- Test existing `api_test.go:944` (`?token=ws-secret`) tetap hijau (fallback).

## 5. Dependensi

- Tidak ada dependensi kind/spec baru — murni layer `internal/api` + hook.
- Kompatibel dengan `--dev-ui` reverse proxy (Origin ≠ Host sudah ditangani
  `InsecureSkipVerify`; ticket tidak mengubah itu).

## 6. Deferred / follow-up

- Cabut `?token=` fallback untuk path `_ws` setelah semua konsumen migrasi.
- Pertimbangkan rotasi/perpanjangan ticket untuk long-lived connection —
  tidak perlu: koneksi setelah upgrade tidak butuh auth ulang; reconnect
  minta ticket baru.
- WSS/TLS termination logging: dokumentasikan rekomendasi strip query string
  di access log proxy untuk path `_ws` (mitigasi sementara sebelum ticket).

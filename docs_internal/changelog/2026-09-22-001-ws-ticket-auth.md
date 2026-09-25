# 2026-09-22-001 — WS handshake auth via single-use ticket (todo 5.8.4)

**Plan**: `docs_internal/plan/ws-ticket-auth-plan.md` (status: Planned → implemented).

Handshake WebSocket di `/{ws}/_ui/_ws` sebelumnya mengandalkan `?token=<JWT>`
karena browser tidak bisa men-set header `Authorization` saat handshake. Token
di query string terekam ke access log proxy/server, riwayat browser, dan header
`Referer` — dan JWT ber-TTL panjang jauh lebih lama daripada koneksi yang
dibukanya. Ditutup dengan alur ticket: client `POST /{ws}/_ui/_ws/ticket`
(header Bearer, tidak pernah di URL) → `{ticket, expires_in: 30}` → connect
`?ticket=<opaque>`. Ticket opaque (32 byte acak), single-use, terikat ke
identity + workspace penerbit; `HandleWS` mengonsumsinya **sebelum** upgrade
sehingga ticket buruk = `401` bersih (bukan socket di-upgrade lalu ditutup), dan
ticket workspace A dipakai di path workspace B ditolak. Issuance dibatasi
60 ticket/(workspace,user)/menit. Jalur `?token=` dipertahankan sebagai fallback
deprekasi agar client lama tidak putus.

**File terkena dampak**:

- `internal/api/wsticket.go` (baru) — `wsTicketStore` (map + sliding-window
  limiter), `HandleWSTicket`.
- `internal/api/router.go` — field `wsTickets`, route `POST /_ws/ticket`.
- `internal/api/wshub.go` — `HandleWS` konsumsi `?ticket=` sebelum upgrade.
- `internal/api/wsticket_test.go` (baru) — 7 test.
- `renderers/react-shadcn/src/hooks/useRealtime.ts` — `configure(url, {ticketUrl,
  token})`, `fetchTicket()` sebelum `new WebSocket`, generasi guard agar
  `configure()` baru membatalkan attempt in-flight; fallback `?token=` bila
  issuance gagal; ticket baru di setiap reconnect.
- `docs/renderers/realtime.md` §2.1, `docs/spec/frontend/04-spec-resolution-api.md` §5.

**Verifikasi**: `go test ./internal/api/` → ok (7 test baru hijau, termasuk
regresi `TestAuthMiddleware_TokenQueryParam` yang membuktikan fallback jalan);
`go test ./...` hijau; `npx tsc --noEmit` bersih; `vitest run` **288 lulus**.

**Sisa** → **5.8.4 ⏸️** (pencabutan `?token=` setelah migrasi konsumen) — bukan
prosa saja, item todo menunjuknya.

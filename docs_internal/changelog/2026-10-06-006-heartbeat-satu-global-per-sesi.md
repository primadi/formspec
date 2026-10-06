# Docs: gap heartbeat realtime jadi item bernomor — satu heartbeat, global per sesi

## Perubahan

Gap heartbeat pada realtime sebelumnya hanya hidup sebagai **prosa** di
`docs/renderers/realtime.md` §7 dan `docs/spec/frontend/04-spec-resolution-api.md`
§7, tanpa item bernomor di todo — jadi tidak bisa di-grep sebagai pekerjaan
terbuka (melanggar aturan "pekerjaan terbuka WAJIB jadi item `[⏸️]` bernomor").
Kini dicatat sebagai **5.8.5 ⏸️** dengan keputusan desain dari pemilik proyek:
heartbeat **hanya satu dan berlaku global per sesi**.

Konsekuensinya heartbeat dimiliki **transport/koneksi** — satu `wsConn` di server,
satu `RealtimeClient` singleton per tab di client — **bukan** per `useRealtime`/
subscription, supaya N komponen dalam satu sesi tidak menghasilkan N heartbeat.
Implementasi tetap terbuka; deteksi putus saat ini bergantung browser/OS.

Sekalian dicatat **5.8.6 ⏸️** untuk target `{scope: user}` (hub hanya meng-index
`{scope: workspace}`), yang berada di kalimat gap yang sama.

## Files affected

- `docs_internal/plan/todo.md` (item 5.8.5, 5.8.6 + `Last Updated`)
- `docs/renderers/realtime.md` §7
- `docs/spec/frontend/04-spec-resolution-api.md` §7

# 2026-09-14-001 — Fase 0 kafe: fix guard script Starlark + `ctx.db().query` bind args

Verifikasi ulang gap aplikasi kafe (`examples/kafe/gaps_found/TODO.md` Fase 0)
menemukan tiga cacat yang **belum tercatat** di ledger, dan semuanya memblokir
penulisan data di app kafe:

1. **#49** — tiga guard script (`cafe-master/guard_menu_item_price_unique.star`,
   `cafe-order/guard_shift_open_unique.star`, `cafe-stock/guard_stock_level_unique.star`)
   memakai _implicit adjacent string-literal concatenation_ (kebiasaan Python)
   yang tidak didukung dialek Starlark → `script compile error: got string
literal, want ','`. Akibatnya `create`/`update` pada `menu-item-price`, `shift`,
   dan `stock-level` **selalu** gagal `HOOK_ABORTED`.
2. **#51** — setelah #49 diperbaiki, muncul `query: got 2 arguments, want at
most 1`. Kontrak `Querier` mendokumentasikan `ctx.db().query(sql, args...)`
   (`internal/starlark/primitive.go:21`, `context.go:11`) tetapi `builtinQuery`
   hanya meng-`UnpackArgs` `sql` dan memanggil `q.Query(ctx, sql)` **tanpa**
   argumen. Jadi bind parameter tidak pernah didukung → penulis spec terpaksa
   menginterpolasi nilai ke dalam teks SQL.
3. **#50** — `formspec validate` melaporkan **0 problem** (hijau) padahal ketiga
   script di atas tidak bisa dikompilasi; tidak ada pemeriksaan kompilasi Starlark.

Yang diubah:

- `internal/starlark/primitive.go` — `builtinQuery` menerima argumen opsional
  kedua (`args?`) sebagai list/tuple, mengonversinya lewat `fromStarlark`, dan
  meneruskannya ke `Querier.Query(ctx, sql, params...)`. Nilai non-iterable
  ditolak dengan pesan jelas.
- `internal/starlark/primitive_test.go` — test regresi `TestCtxDBQuery_BindArgs`
  (`captureQuerier`) membuktikan bind args diteruskan.
- `examples/kafe/spec/modules/*/scripts/*.star` — konkatenasi implisit → `+`.

Diverifikasi: `go test ./internal/starlark/ -run TestCtxDBQuery` PASS; guard
`menu-item-price` kini **berjalan** (sebelumnya gagal kompilasi) dan mengembalikan
`fail(...)` sesuai logika bisnisnya. Sisa kegagalan pada jalur itu adalah **#44**
(target relasi `draft`) — gap terpisah yang sudah terverifikasi.

Ledger baru: `examples/kafe/gaps_found/14-temuan-fase-0.md` (hasil verifikasi +
#49/#50/#51), `examples/kafe/gaps_found/validate-baseline.md` (baseline = 0
problem, dijanjikan `docs/architecture.md` §0 tapi belum ada), dan
`examples/kafe/gaps_found/TODO.md` (checklist penutupan 10 fase).

Referensi: `examples/kafe/gaps_found/TODO.md` (Fase 0, 2.9, 8.7),
`examples/kafe/gaps_found/14-temuan-fase-0.md`.
Sisa: #50 (validator compile-check) belum dikerjakan.

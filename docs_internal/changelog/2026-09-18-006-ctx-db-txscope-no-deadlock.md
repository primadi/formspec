# 4.5 — `ctx.db()` di dalam transaksi aksi tidak deadlock di SQLite (#30)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.5) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.5

## Apa yang diubah

`datastore.DBQuerier.Query` (`renderers/jsonb-persist/datastore/querier.go`)
kini memakai `db.TxReadDB(ctx, q.DB)` alih-alih `q.DB` langsung. `TxReadDB`
diekstrak (ekspor) dari `txReadDB` yang sudah dipakai jalur baca `EntityStore`.
`TxScope.Join` juga diekspor untuk pemanggil luar paket.

## Kenapa

Aksi kustom membuka satu transaksi request-scoped (`TxScope`) dan SQLite dibuka
dengan `SetMaxOpenConns(1)`. `ctx.db().query()` memakai `q.DB` (pool) → query
kedua menunggu koneksi yang tidak akan pernah bebas → **hang selamanya**, bukan
error. Akibatnya guard yang memanggil `ctx.db()` di dalam transaksi aksi tidak
bisa diuji di dev (GAP-30).

Dengan `TxReadDB`, query berjalan di koneksi transaksi aksi — tidak ada koneksi
kedua, dan sekaligus memberi read-your-own-writes (guard melihat baris yang baru
ditulis di transaksi yang sama).

## File yang terkena dampak

- `renderers/jsonb-persist/datastore/querier.go` — `Query` memakai `TxReadDB`.
- `renderers/jsonb-persist/tx.go` — `TxReadDB` diekspor.
- `renderers/jsonb-persist/txscope.go` — `TxScope.Join` diekspor.
- `renderers/jsonb-persist/datastore/querier_test.go` —
  `TestDBQuerier_QueryInsideTxScopeNoDeadlock`.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/datastore/ -run TestDBQuerier` | 3 PASS |
| test dengan fix **dikembalikan** (`target := q.DB`) | **FAIL** `context deadline exceeded` (5s) — membuktikan test menangkap deadlock |
| `go test ./...` | hijau |

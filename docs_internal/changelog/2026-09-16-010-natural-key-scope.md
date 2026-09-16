# 2026-09-16-010 — Natural key ber-scope: scope menembus counter, keunikan, dan DDL (#9)

Item `examples/kafe/gaps_found/TODO.md` **3.6** (gap #9). Plan:
`docs_internal/plan/natural-key-scope.md`.

**Temuan: bukan sekadar "teruskan nilai scope".** Saat diuji, cabang kedua gagal
`500 UNIQUE constraint failed: cafe_order_orders.tenant_id,
cafe_order_orders._number` — deret per cabang sudah benar, tetapi **jaminan
keunikannya tidak ikut ber-scope**. Nilai scope harus menembus tiga lapis:

1. **Counter.** Jalur script (`ctx.next_key`) mengirim scope kosong (hardcoded
   `""` di `internal/entity/registry.go`), sedangkan jalur otomatis membacanya
   dari
   record. Kini `ctx.next_key(field, scope=<nilai>)` meneruskan scope, dan
   registry **menolak** mencetak nomor saat rule ber-`scope_field` tetapi scope
   kosong — dengan pesan yang menyebut field yang harus diisi.
2. **Keunikan.** Index unik natural key adalah `(tenant_id, _number)`, **tanpa
   cabang**, sehingga `ORD-…-00001` milik B2 menabrak milik B1. Kini
   `(tenant_id, _branch_id, _number)` bila rule-nya ber-scope.
3. **DDL.** Scope field belum tentu punya kolom turunan (`order.branch_id` tidak
   `index: true`), sehingga index yang menyebut `_branch_id` gagal dibuat
   (`no such column`). Generator kini membuat kolom turunan untuk scope field
   natural key — sama seperti yang sudah dilakukan untuk field yang dirujuk
   `indexes:`.

**Bukti runtime** (dev server, DB segar, empat create anonim): B1 →
`ORD-2026-00001`, B2 → `ORD-2026-00001`, B1 → `…00002`, B2 → `…00002`, semuanya
`201`; **nomor yang sama hidup di dua cabang**. Sebelum perbaikan: create B2 →
`500`.

**Bukti unit:** `TestGenerateNaturalKey_ScopedPerBranch` (fixture baru
`registry_fixtures/scoped-counter/spec`),
`TestGenerateNaturalKey_ScopedCounterRefusesEmptyScope`,
`TestGenerateNaturalKey_UnscopedUnaffected`, dan `TestCtxNextKey_*`
(`internal/starlark`) yang membuktikan argumen `scope=` sampai ke handler.

Normatif: `docs/spec/backend/04-persist-backend.md` §2. Komentar GAP-09 di
`order/entity.yaml` kini sudah tidak berlaku (nomor pesanan per cabang boleh
lewat script).

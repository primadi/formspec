# 4.4 — Keunikan atomik: `indexes:` adalah penegak, guard hanya lapis kedua (#32)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.4) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.4

## Apa yang diubah

Tidak ada konstruk baru — jawaban kanoniknya sudah ada (`indexes:` dengan
`where:` untuk keunikan bersyarat, item 1.6). Yang dikerjakan adalah
**membuktikan** dan **mendokumentasikan** bahwa database adalah penegaknya:

- `renderers/jsonb-persist/migrate_test.go` —
  `TestMigrationRunner_UniqueIndexRejectsDuplicates`.
- `docs/spec/backend/01-core-basic.md` §3 — paragraf "Keunikan adalah urusan
  database, bukan script".
- `examples/kafe/spec/modules/cafe-master/master/menu-item-price/entity.yaml` —
  komentar guard diperbarui (GAP-22/GAP-30 sudah tertutup; guard = lapis kedua).

## Kenapa

GAP-32 mengeluhkan guard keunikan di script tidak atomik (SELECT-lalu-INSERT),
sehingga butuh `ctx.lock` — reimplementasi UNIQUE yang lebih rapuh daripada
constraint-nya. Jawabannya bukan menambah `ctx.lock` ke setiap guard, melainkan
menyatakan aturannya sebagai **unique index** (dengan `where:` bila bersyarat):
index berlaku untuk semua jalur tulis dan atomik di level database, sedangkan
guard hanya berjalan pada jalur yang melewatinya.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./renderers/jsonb-persist/ -run TestMigrationRunner_UniqueIndexRejectsDuplicates` | PASS |
| `migrate apply` pada DB segar → `sqlite_master` | `CREATE UNIQUE INDEX … (_branch_id, _menu_item_id)` **dan** `… (_branch_id, _cashier_id) WHERE _status = 'open'` |
| INSERT duplikat `(B1,C1)` open | **REJECTED** `UNIQUE constraint failed` |
| INSERT shift `closed` ganda | **OK** — membuktikan index parsial |
| INSERT open di cabang lain | **OK** — kunci berbeda |
| `formspec validate` kafe | **0 problem** |
| `go test ./...` | hijau |

## Sisa

Guard script tetap ada sebagai lapis kedua (pesan ramah + menutup GAP-36:
constraint tidak bisa merapikan duplikat yang sudah ada). Ia tidak lagi memakai
raw SQL (item 4.3) dan tidak deadlock (item 4.5).

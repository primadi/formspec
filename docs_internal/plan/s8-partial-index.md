# Plan — 1.6 / S8: unique parsial + index atas relasi

**Kafe ledger:** `examples/kafe/gaps_found/TODO.md` 1.6 (prioritas §F #6) ·
kelengkapan spec S8 (`13-kelengkapan-spec-untuk-kafe.md:377`)
**Menutup akar:** #22 (sebagian sudah lewat 3.1), #23, GAP-35 (untuk kasus ini)
**Effort:** medium (spec + 2 jalur persist + adopsi contoh + docs)

---

## Masalah

Tiga aturan keunikan kafe hari ini dinyatakan sebagai **DDL mentah** lewat
`kind: Migration` — bukti bahwa konsepnya tidak ada di bahasa spec:

| Aturan                                 | Bentuk                                               | Berkas penutup sekarang                              |
| -------------------------------------- | ---------------------------------------------------- | ---------------------------------------------------- |
| Satu harga per menu per cabang         | `unique (branch_id, menu_item_id)`                   | `cafe-master/migrations/menu-item-price-unique.yaml` |
| Satu baris stok per (cabang, bahan)    | `unique (branch_id, ingredient_id)`                  | `cafe-stock/migrations/stock-level-unique.yaml`      |
| Satu shift terbuka per (cabang, kasir) | `unique (branch_id, cashier_id) WHERE status='open'` | `cafe-order/migrations/shift-open-unique.yaml`       |

Dua kekurangan di bahasa spec:

1. **Tidak ada predikat parsial** — `IndexDecl` hanya `{fields, unique}`.
2. ~~Relasi tidak bisa diindeks~~ — **sudah tertutup** oleh 3.1/#22
   (`migrate.go:639` menambah kolom turunan untuk field relasi yang disebut
   di index). Yang benar-benar sisa adalah predikat parsial (ditambah satu bug
   jalur alter, lihat di bawah).

DDL mentah itu tidak portabel (GAP-35: `json_extract` SQLite vs `->>` Postgres),
jadi penutup resmi hari ini **salah di produksi**.

## Yang akan dikerjakan

### 1. `pkg/spec` — predikat sebagai konstruk bahasa

- `IndexDecl.Where string` (`where:`), hanya bermakna sebagai predikat index.
- Parser + validator di `pkg/spec/indexwhere.go` dengan **grammar tertutup**
  (declarative, bukan SQL bebas):
  ```
  predicate := term (AND term)*
  term      := <field> <op> <literal> | <field> IS NULL | <field> IS NOT NULL
  op        := = | != | <> | > | >= | < | <=
  literal   := 'teks' | angka | true | false
  ```
  Field wajib ada di entity. Di luar grammar → error yang menyebut bentuk yang
  didukung dan mengarahkan ke `kind: Migration` untuk DDL bebas.
- Validation dipanggil dari `ValidateEntitySpec`; hasil parse boleh dipakai ulang
  renderer (satu sumber kebenaran).

**Alasan grammar tertutup, bukan string bebas:** predikat ini masuk ke DDL.
String bebas = SQL injection dari manifest + tidak bisa divalidasi. Grammar
tertutup juga memberi pesan error yang menyebut field yang salah.

### 2. `renderers/jsonb-persist/ddl.go` — terjemahan field → kolom

- Predikat ditulis dalam **nama field** (`status = 'open'`), dirender ke
  **kolom turunan** (`_status = 'open'`) — konsisten dengan `fields:` yang juga
  memetakan ke `_field`.
- Emisi: `CREATE [UNIQUE] INDEX ... ON <table> (<cols>) WHERE <predicate>;`
- SQLite & PostgreSQL sama-sama mendukung partial index → portabel, GAP-35
  tertutup untuk kasus ini.

### 3. `renderers/jsonb-persist/migrate.go` — index baru ikut terpasang

Bug nyata yang ikut ketemu: pada jalur _tabel sudah ada_ / _checksum berubah_,
`diffExistingTable` hanya menambah **kolom**, tidak pernah membuat **index**.
`if added > 0` — kalau tidak ada kolom baru, tidak ada DDL sama sekali. Jadi
index yang baru dideklarasikan **tidak pernah** dibuat di DB yang sudah jalan.

Perbaikan: jalur alter juga mengemisikan `CREATE INDEX IF NOT EXISTS` /
`CREATE UNIQUE INDEX IF NOT EXISTS` (didukung SQLite ≥3.8 dan PostgreSQL ≥9.5),
sehingga idempoten tanpa introspeksi katalog.

### 4. Adopsi `examples/kafe` (bukti konstruknya cukup)

- `shift`: `indexes: [{fields: [branch_id, cashier_id], unique: true, where: "status = 'open'"}]`
- `menu-item-price` & `stock-level`: cukup `{fields, unique}` (sudah ada;
  komentar GAP-22 yang menyatakan "tidak dihormati" dihapus karena sudah basi).
- **Hapus** ketiga `kind: Migration` mentah itu — kalau konstruk bahasa sudah
  ada, workaround-nya yang harus pergi. (Kalau dibiarkan, tidak ada yang
  membuktikan S8 benar-benar tertutup.)

### 5. Verifikasi (harus bisa gagal)

- `formspec validate` → 0 problem (dengan `--schema schemas`; registry masih
  stale — tiket 3.6.7).
- `formspec migrate plan` → memuat `CREATE UNIQUE INDEX ... (branch_id,
menu_item_id)` **dan** `... WHERE _status = 'open'`.
- `formspec check` → 0 error.
- Test baru: parser (grammar valid/ditolak), DDL (partial + rewrite kolom),
  migrate (index muncul di jalur alter).

## Dependensi

- Tidak ada. Berdiri sendiri; tidak menunggu 1.7/1.8.
- Bertalian: GAP-35 (DDL multi-dialek, kafe 3.4) — setelah ini, satu lagi
  alasan memakai DDL mentah hilang.

## Catatan risiko

- Predikat yang ditulis terhadap kolom non-derived (mis. `deleted_at`) harus
  tetap boleh — kolom itu nyata di tabel, bukan bagian JSONB. Parser hanya
  memetakan nama yang **cocok dengan nama field entity**.
- `where` pada index non-unique sah secara SQL; dibiarkan, tapi dokumentasi
  menyatakan kebutuhan utama kafe adalah partial **unique**.

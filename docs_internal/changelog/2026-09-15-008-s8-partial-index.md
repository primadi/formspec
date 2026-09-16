# 2026-09-15-008 — S8: index parsial dinyatakan di manifest (kafe 1.6)

Kafe ledger **1.6** (`examples/kafe/gaps_found/TODO.md`), menutup akar #22
sisa. Tiga aturan keunikan kafe sebelumnya dinyatakan sebagai **DDL mentah**
lewat `kind: Migration`; itu bukan pilihan desain melainkan bukti bahwa
konsepnya tidak ada di bahasa spec. Sekarang ketiganya jadi konstruk bahasa —
dan ketiga berkas DDL mentahnya **dihapus**.

**Konstruk baru.** `IndexDecl.Where` (S8) + parser grammar **tertutup** di
`pkg/spec/indexwhere.go`: `<field> <op> <literal>` dan `<field> IS [NOT] NULL`,
digabung `AND`. Predikat ditulis dengan nama field, divalidasi
`ValidateEntitySpec` di kedua situs deklarasi (`indexes` dan `persist.indexes`),
lalu diterjemahkan ke kolom turunan saat render (`status = 'open'` →
`_status = 'open'`), konsisten dengan cara `fields:` sudah dipetakan. Partial
index didukung SQLite maupun PostgreSQL, jadi aturannya **portabel** — GAP-35
(ddl mentah tidak portabel) ikut tertutup untuk kasus ini; DDL yang benar-benar
di luar bahasa tetap tempatnya di `kind: Migration`.

Grammar dipilih tertutup, bukan string bebas SQL: predikat ini masuk ke DDL, jadi
string bebas berarti injeksi lewat manifest **dan** tidak bisa divalidasi. Grammar
tertutup juga bisa menyebut nama field yang salah ketik, yang tidak bisa dilakukan
passthrough SQL.

**Dua bug nyata ikut ketemu** — keduanya kelas "gagal senyap" yang sedang dikejar
di seluruh ledger kafe:

1. `diffExistingTable` hanya merekonsiliasi **kolom**, tidak pernah membuat
   **index**. Jadi `indexes:` yang ditambahkan setelah tabelnya ada **tidak
   pernah** terpasang di database yang sudah berjalan — manifest terlihat benar,
   `formspec validate` hijau, dan aturan bisnisnya tidak ada. Kini jalur alter
   juga membuat index yang benar-benar belum ada (intropeksi nama index, emisi
   `IF NOT EXISTS`), dan plan tetap **0 saat sudah konvergen** — properti yang
   dijaga test, karena tanpa itu setiap `migrate plan` akan mengemisikan index
   yang sama selamanya.
2. Field yang hanya disebut di **predikat** (bukan di `fields:`) tidak pernah
   mendapat kolom turunan → `CREATE INDEX … WHERE _status = …` gagal
   `no such column: _status`. `indexDeclFields` kini mengumpulkan field dari
   index **dan** predikatnya.

**Adopsi kafe.** `shift` menyatakan `{fields: [branch_id, cashier_id],
unique: true, where: "status = 'open'"}`; `menu-item-price` dan `stock-level`
cukup `{fields, unique}` (sudah ada, kini benar-benar ditegakkan). Tiga
`kind: Migration` DDL mentah dihapus, dan komentar GAP-22/GAP-35 yang menyatakan
"tidak bisa dinyatakan" diperbarui — komentar yang membual seperti itu justru
yang membuat gap tampak abadi.

**Bug dokumen ikut ketemu.** `docs/spec/backend/01-core-basic.md` §3
mendokumentasikan `indexes: [{field: status, type: btree}]` — bentuk yang
**tidak pernah ada** di `IndexDecl` (yang nyata: `fields` + `unique`). Contohnya
dikoreksi, dan dokumentasi index parsial ditambahkan.

**Verifikasi.**

| Perintah                                                       | Hasil                                                                                                                  |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `formspec migrate plan`                                        | `CREATE UNIQUE INDEX …menu_item_prices (…, _menu_item_id);` **dan** `…shifts (…, _cashier_id) WHERE _status = 'open';` |
| `formspec migrate apply` (DB segar)                            | 24 structural + 0 custom; ketiga index ada di `sqlite_master`                                                          |
| INSERT 2 shift `open` (cabang B1, kasir C1)                    | **REJECTED** — `UNIQUE constraint failed: …_branch_id, …_cashier_id`                                                   |
| INSERT shift `closed` (B1, C1) dua kali                        | **OK** — parsial: hanya baris `open` yang dibatasi                                                                     |
| INSERT shift `open` (B2, C1)                                   | **OK** — aturan berlaku per cabang                                                                                     |
| `go test ./...`                                                | hijau                                                                                                                  |
| `formspec validate --schema schemas --spec examples/kafe/spec` | 69 manifest, 0 problem                                                                                                 |
| `formspec check -f examples/kafe/spec`                         | 0 error, 0 warning                                                                                                     |

Test baru: `pkg/spec/indexwhere_test.go` (grammar diterima/ditolak — termasuk
`OR` yang dulu diam-diam tertelan ke dalam literal, dan field salah ketik),
`ddl_test.go` (`TestGenerateEntityDDL_PartialIndex*`, SQLite + PostgreSQL),
`migrate_test.go` (`TestMigrationRunner_NewDeclaredIndexReachesExistingTable`,
termasuk konvergensi). Guard `TestGeneratedSchemas_MatchOnDisk` (dibuat di
changelog sebelumnya) **benar-benar menangkap** schema yang stale saat field baru
ditambahkan — tepat kelas kegagalan yang dimaksudkan.

**Catatan.** Jumlah manifest kafe turun 72 → 69 karena tiga `kind: Migration`
mentah dihapus; `formspec validate` dan `check` tetap hijau. GAP-36 tetap
terbuka dan bukan bagian 1.6: constraint menolak baris **baru**, ia tidak bisa
merapikan duplikat yang sudah ada (DML ditolak di `kind: Migration`), jadi guard
script tetap dipertahankan sebagai lapis kedua yang memberi pesan ramah.

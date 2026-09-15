# 2026-09-14-003 — DDL: `indexes:` dihormati (root + persist) & tipe SQL driver-aware

Dua gap DDL dari ledger kafe ditutup. Keduanya diverifikasi lewat
`formspec migrate plan` pada `examples/kafe/spec`.

**#22 — `indexes:` tidak menghasilkan index apa pun.** `GenerateEntityDDL`
(`renderers/jsonb-persist/ddl.go` §4) hanya membaca `entity.Persist.Indexes`,
sedangkan spec menaruh `indexes:` di root (`EntitySpec.Indexes`,
`pkg/spec/entity.go:87`) — jadi setiap index yang dideklarasikan hilang tanpa
peringatan. Lebih buruk: field `relation` juga tidak pernah mendapat kolom
turunan, sehingga index atas relasi mustahil. Akibatnya aturan bisnis
"harga per (cabang, menu)" dan "satu saldo per (cabang, bahan)" tidak dijaga DB,
dan penulis spec harus menulis guard script sebagai pengganti constraint.

Perbaikan: `GenerateEntityDDL` membaca **kedua** lokasi dan **membuat kolom
turunan** untuk setiap field yang disebut index (termasuk `relation`, bertipe
`text`), dengan pelacakan kolom agar tidak ada duplikat;
`MigrationRunner.diffExistingTable` (`migrate.go`) juga memakai himpunan field
yang sama sehingga tabel yang sudah ada mendapat kolom turunan baru.
Bukti:

```
CREATE UNIQUE INDEX idx_cafe_master_menu_item_prices_branch_id_menu_item_id
  ON cafe_master_menu_item_prices (_branch_id, _menu_item_id);
CREATE INDEX idx_cafe_order_shifts_branch_id_transaction_date
  ON cafe_order_shifts (_branch_id, _transaction_date);
```

**#27 — tipe PostgreSQL bocor ke DDL SQLite.** `fieldTypeToSQL` tidak menerima
driver, sehingga `timestamptz`/`jsonb`/`uuid`/`bigint` muncul apa adanya di DDL
SQLite. Ditambahkan `fieldTypeToSQLFor(ft, enum, driver)` yang memetakan
`timestamptz`/`jsonb`/`uuid` → `text` dan `bigint` → `integer` pada SQLite
(PostgreSQL tetap native), dipakai di kolom turunan, index deklaratif, extension
(`ddl.go`) dan `diffExistingTable` (`migrate.go`). Bukti: `grep -c timestamptz`
pada `migrate plan` SQLite = **0**.

Test baru: `TestGenerateEntityDDL_DeclaredIndexes`,
`TestGenerateEntityDDL_PersistIndexesStillWorked`,
`TestFieldTypeToSQLFor_NoPostgresTypesOnSQLite`. `go test ./...` → 35 paket `ok`.

Catatan batasan: index deklaratif baru terpasang pada **database baru**;
database lama mendapat kolom turunan lewat diff, tetapi index-nya perlu recreate
atau `kind: Migration` (masih bagian dari TODO 3.4).

Referensi: `examples/kafe/gaps_found/TODO.md` (3.1, 3.3),
`examples/kafe/gaps_found/README.md` (#22, #27).

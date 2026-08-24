# Fix: Migration "duplicate column name" pada generated column

## Masalah

Menambah nilai enum (mis. `not_available` pada `status` di `cafe-master/table`)
mengubah checksum DDL (CHECK constraint berubah), sehingga `diffExistingTable`
dijalankan. Diff menganggap kolom generated `_code`/`_status` hilang dan
mencoba `ALTER TABLE ADD COLUMN` → error SQLite `duplicate column name: _code`.

## Akar masalah

`existingColumns` (renderers/jsonb-persist/migrate.go) memakai
`PRAGMA table_info(?)` untuk SQLite. `table_info` **menyembunyikan kolom
generated** (`GENERATED ALWAYS AS ... STORED`). Kolom `_code`/`_status` adalah
generated column, jadi selalu dianggap "tidak ada" oleh diff → ADD COLUMN
duplikat.

## Perbaikan

Ganti `PRAGMA table_info(?)` → `PRAGMA table_xinfo(?)` di `existingColumns`.
`table_xinfo` menyertakan kolom generated, sehingga diff mendeteksi kolom
tersebut sudah ada dan tidak membuat ADD COLUMN.

## File terdampak

- `renderers/jsonb-persist/migrate.go` — query `existingColumns` (SQLite)
- `renderers/jsonb-persist/migrate_test.go` — test regresi
  `TestMigrationRunner_EnumChangeNoDuplicateColumn`

## Referensi

- `docs/plan/` — structural diff field add (2026-08-17-027)

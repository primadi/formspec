# Structural diff — field add (todo 4.2.1)

**Date**: 2026-08-17

Mengimplementasikan structural diff field-add di migration engine.

- `renderers/jsonb-persist/migrate.go`:
  - `diffExistingTable` — bandingkan kolom existing vs spec; field
    indexed/unique/natural-key baru → `ALTER TABLE ADD COLUMN`.
  - `existingColumns` — list kolom (SQLite `pragma_table_info`, PG
    `information_schema.columns`).
  - `PlanMigrations` kini memanggil diff pada checksum mismatch DAN tabel
    existing (sebelumnya add-only skip).
- Test: `renderers/jsonb-persist/migrate_test.go`
  `TestMigrationRunner_FieldAddDiff`.

Catatan: modernc SQLite driver tidak bisa `ALTER TABLE ADD COLUMN` dengan
`GENERATED ALWAYS AS` (silent no-op) — diff SQLite memakai plain column; PG
memakai generated column. Field removal/type-change tetap dua-fase (4.2.2).

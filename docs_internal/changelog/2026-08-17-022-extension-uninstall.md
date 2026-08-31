# Extension uninstall (todo 4.3.3)

**Date**: 2026-08-17

Mengimplementasikan uninstall entity extension.

- `renderers/jsonb-persist/migrate.go`: `MigrationRunner.UninstallExtension(ctx,
tableName, namespace)` — `DROP COLUMN ext_{namespace}` + set status
  `formspec_extensions.status = 'locked'` (namespace never reused) dalam satu
  transaksi.
- Test: `renderers/jsonb-persist/migrate_test.go`
  `TestMigrationRunner_UninstallExtension` (drop column + lock namespace).

Catatan: SQLite tidak bisa `DROP COLUMN` bila ada generated column yang
mereferensikan kolom itu — uninstall extension dengan field unique/indexed
butuh table recreation (belum ditangani; test memakai field non-unique).
4.3.1/4.3.2 (extension read/write) dan 4.3.4 (namespace collision) sudah
terimplementasi sebelumnya.

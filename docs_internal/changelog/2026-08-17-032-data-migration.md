# Data migration ber-versi (todo 4.2.5)

**Date**: 2026-08-17

Mengimplementasikan tipe migrasi ketiga: data migration (backfill) ber-versi.

- `pkg/spec/resources.go`: `DataMigrationSpec` (`version`/`run`/`rollback`/
  `module`).
- `cmd/formspec/migrate.go`: `formspec migrate data <name> run|rollback` —
  load `kind: DataMigration`, resolve script (`migrations/` atau
  `modules/{module}/migrations|scripts/`), eksekusi Starlark via engine
  (ctx.\* + resource predeclared).
- Test: `cmd/formspec/migrate_test.go` (resolve script, load manifest).

Catatan: belum ada tracking versi yang sudah di-run (idempotency per data
migration) — enhancement berikutnya.

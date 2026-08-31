# `kind: Migration` custom DDL (todo 4.2.4)

**Date**: 2026-08-17

Mengimplementasikan eksekusi `kind: Migration` (custom DDL) di `formspec
migrate plan|apply`.

- `cmd/formspec/migrate.go`: `loadCustomMigrations` (load `kind: Migration`
  manifests), `applyCustomMigrations` (validasi DDL-only + eksekusi),
  `validateDDLOnly` (tolak INSERT/UPDATE/DELETE/SELECT/DROP DATABASE/DROP
  TABLE), `reparseAny` (generic YAML re-parse).
- Test: `cmd/formspec/migrate_test.go` (validateDDLOnly, load+apply custom
  DDL, reject DML).

Catatan: `kind: Migration` DDL dijalankan langsung via `ExecContext`; belum
ada tracking idempotency per custom migration (structural runner punya
`formspec_schema_migrations`). Data migration ber-versi (4.2.5) tetap item
terpisah.

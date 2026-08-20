# PersistBackend interface (todo 4.1.1)

**Date**: 2026-08-17

Mendefinisikan seam penyimpanan PersistBackend.

- `renderers/jsonb-persist/persist_backend.go`: interface `PersistBackend`
  technology-agnostic (tanpa `*sql.DB`/`ExecContext`/`QueryContext`/`Driver()`)
  — `SyncSchema`, `PlanSchema`, `NextKey`, `UninstallExtension`,
  `EntityStore`, `DriverName` (docs/spec/backend/04-persist-backend.md §2).

Catatan: 4.1.2 (required capabilities) dan 4.1.3 (refactor jsonb-persist
mengimplementasi interface) tetap item terpisah — MigrationRunner belum
sepenuhnya memenuhi interface (belum ada `EntityStore`/`NextKey` method
publik di level itu).

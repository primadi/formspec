# jsonb-persist implements PersistBackend (todo 4.1.3)

**Date**: 2026-08-17

Membuat jsonb-persist memenuhi kontrak `PersistBackend`.

- `renderers/jsonb-persist/migrate.go`: `MigrationRunner` kini memenuhi
  `PersistBackend` — `SyncSchema` (=ApplyMigrations), `PlanSchema`
  (=PlanMigrations), `NextKey` (delegasi ke natural-key counter via
  `SetRegistry`), `EntityStore` (via `SetRegistry`), `DriverName`,
  `UninstallExtension`. Compile-time check `var _ PersistBackend`.
- `renderers/jsonb-persist/persist_backend.go`: interface (4.1.1).

Catatan: `SetRegistry` opsional — `NextKey`/`EntityStore` error jelas bila
registry belum di-wire. Framework inti belum sepenuhnya bicara lewat interface
(EntityStore langsung dipakai di banyak tempat) — itu refactor lebih luas.

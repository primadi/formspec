# Per-entity migration transaction (todo 4.2.3)

**Date**: 2026-08-17

Mengimplementasikan atomicity per-entity migration di jsonb-persist.

- `renderers/jsonb-persist/migrate.go`: `ApplyMigrations` kini membungkus DDL +
  record migration + namespace reservation per entity dalam satu
  `BeginTx`/`Commit` — kegagalan → `Rollback` (fail = full rollback, 4.2.3).
  Data di kolom `data` JSONB tidak pernah ditulis ulang oleh migrasi
  structural (tetap add-only untuk field baru).

Catatan: 4.2.1 (structural diff field add/remove/type-change) masih add-only
untuk tabel baru — field-level diff belum; 4.2.2 (`renamed_from`) dan 4.2.5
(data migration ber-versi) tetap item terpisah.

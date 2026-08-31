# Filterable backup (todo 4.8.2)

**Date**: 2026-08-18

Menambahkan filter backup.

- `cmd/formspec/backup.go`: `formspec backup create --filter <module|module/entity>`
  — `matchesFilter` helper; workspace = "demo" saat ini (single-server).
- Test: `cmd/formspec/backup_test.go` `TestMatchesFilter`.

Catatan: 4.8.1 (`--incremental` + file storage ctx.storage ikut backup) dan
4.8.3 (`--map-resource`/`remap`) tetap gap.

# Backup file storage (todo 4.8.1)

**Date**: 2026-08-18

Menambahkan file storage (ctx.storage) ke backup.

- `cmd/formspec/backup.go`: `writeDirToTar` — file di `{state}/storage`
  ditambahkan ke arsip di bawah `storage/` (4.8.1). `--full` + storage
  implemented; `--incremental` belum (gap).

Catatan: 4.8.3 (`--map-resource`/`remap`) tetap gap.

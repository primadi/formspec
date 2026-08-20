# Outbox reconciliation after restore (todo 4.8.5)

**Date**: 2026-08-17

Mengimplementasikan outbox reconciliation pass setelah `formspec restore`
(4.8.5, MUST).

- `cmd/formspec/backup.go`: `reconcileOutbox(ctx, database)` — hitung entri
  outbox pending via `OutboxStore.CountByStatus` dan lapor; replay penuh
  adalah tugas outbox worker (yang meng-drain pending saat start).
  Dipanggil setelah restore non-dry-run.

Catatan: 4.8.1 (`--incremental`/`--filter` + file storage ikut backup) dan
4.8.3 (`--map-resource`/`remap`) tetap gap (dicatat di 3.7.1/3.7.3).

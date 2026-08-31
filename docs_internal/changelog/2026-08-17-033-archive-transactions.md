# Archive transactions (todo 4.9.1, 4.9.2, 4.9.5)

**Date**: 2026-08-17

Mengimplementasikan arsip transaksi.

- `cmd/formspec/archive.go`:
  - `formspec archive run --max-age <dur> [--dry-run]` — cari transaksi
    (`characteristic: transaction`) dengan `transaction_date` sebelum cutoff,
    tulis ke arsip JSONL open (`{state}/archive/{batch-id}/{module}_{entity}.jsonl`),
    hapus baris transaksi (soft delete). `parseDuration` (y/m/d).
  - `snapshotMasters` (4.9.2) — snapshot master yang direferensikan
    belongs*to ke `masters*{module}\_{entity}.jsonl`+ set`locked_for_deletion=true` (4.9.3).
  - `formspec archive view --batch-id <id>` — baca batch, lapor record count
    per file.
- `cmd/formspec/main.go`: dispatch `archive`.
- Test: `cmd/formspec/archive_test.go` (parseDuration, dry-run, run, arsip
  file).

Catatan: format Parquet = enhancement (JSONL open dulu). `restore-batch`
tetap item terpisah.

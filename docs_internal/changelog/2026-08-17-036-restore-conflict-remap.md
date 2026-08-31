# 2026-08-17-036-restore-conflict-remap.md

## Restore conflict resolution `remap` + dry-run compatibility report (4.8.3)

Melengkapi `formspec restore` dengan mode konflik ketiga (`remap`) dan
laporan kompatibilitas `--dry-run` per-entity.

**Apa yang diubah:**

- `cmd/formspec/backup.go`:
  - `runRestore` menerima `--conflict skip|overwrite|remap` (sebelumnya
    `remap` ditolak sebagai "not yet implemented").
  - `restoreFrom` kini mengembalikan `RestoreReport` (total + per-entity:
    restored/skipped/remapped/failed) alih-alih tiga int.
  - Mode `remap`: saat natural key sudah ada, `remapNaturalKey` menetapkan
    key baru (`<base>-r1`, `-r2`, …) dan insert sebagai record baru —
    record lama dipertahankan.
  - `--dry-run` mencetak compatibility report per-entity sehingga operator
    bisa memilih mode konflik sebelum commit.
- `cmd/formspec/backup_test.go`: update call site `restoreFrom` ke
  `RestoreReport`; tambah `TestBackupRestoreRemap` (2 record → remap kedua
  konflik → 4 record total, key asli dipertahankan, key remap unik).

**Kenapa:** menutup gap bahwa `remap` dideklarasikan di spec tapi belum
diimplementasikan, dan `--dry-run` hanya melaporkan hitungan total tanpa
rincian per-entity.

**Referensi:** `docs/plan/todo.md` 4.8.3, `docs/spec/backend/04-persist-backend.md`.

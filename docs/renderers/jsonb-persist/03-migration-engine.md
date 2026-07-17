# Migration Engine

**Updated:** 2026-07-16 · Status: Outline

> Outline: heading menetapkan cakupan; isi ditulis bertahap.

## 1. Structural Diff → SQL
Kontrak structural diff ([`../../spec/backend/01-core-basic.md`](../../spec/backend/01-core-basic.md)
§4) dijawab backend ini lewat `internal/db/migrate.go`: `PlanMigrations`
membandingkan Document versi lama vs baru per `EntityMigration`, menghasilkan
`DDLResult` (statement SQL) yang lantas dieksekusi `ApplyMigrations` dalam
satu transaksi per Document. Field rename **wajib** dideklarasikan lewat
`renamed_from` pada field — tanpa itu diff membacanya sebagai drop lama +
add baru (kehilangan data pada backend manapun, bukan cuma jsonb-persist).

## 2. Operasi yang Didukung
- **Field ditambah** — pada backend ini umumnya no-op DDL (field baru cuma
  masuk `data`/kolom extension JSONB), kecuali `index: true` (§ Index
  Generation, [`02-schema-strategies.md`](02-schema-strategies.md) §3) yang
  menambah generated column + index.
- **Field dihapus** — dua tahap lintas dua versi ter-apply (deprecate lalu
  remove), sesuai kontrak §4; pada JSONB ini berarti key tetap ada di `data`
  sampai tahap kedua, generated column (kalau ada) di-drop di tahap itu.
- **Field berubah tipe** — generated column di-drop dan dibuat ulang dengan
  cast baru; data mentah di `data` tidak diubah (tetap JSON asli), cuma
  proyeksinya yang direvisi.
- **Relasi** — foreign key logis diverifikasi di level aplikasi (bukan FK
  constraint SQL literal, karena target relation kadang lintas kategori/
  schema yang sengaja tidak boleh di-join langsung, lihat
  [`01-architecture.md`](01-architecture.md) §1 soal isolasi kategori).
  **Gap implementasi:** resolusi nama tabel target (`ValidateRelationTargets`)
  memakai module milik entity itu sendiri + pluralisasi naif (tambah `s`) —
  belum benar-benar resolve module/plural asli entity target. Relasi lintas
  module atau entity ber-plural tidak beraturan bisa diam-diam lolos dari
  guard referenceability (§1.2 core-basic) alih-alih ditolak.
- **Index** — lihat [`02-schema-strategies.md`](02-schema-strategies.md) §3.

## 3. Keamanan Migrasi
Migrasi structural per Document dieksekusi dalam satu transaksi — gagal di
tengah berarti rollback penuh, tidak ada DDL setengah-jalan yang ter-commit.
Data existing di `data`/kolom extension JSONB tidak pernah ditulis ulang oleh
migrasi structural (hanya kolom struktural dan generated column yang
berubah) — backfill nilai (mis. mengisi default untuk field baru pada baris
lama) adalah data migration terpisah, ber-versi, dijalankan/rollback manual,
bukan bagian structural diff (kontrak §4 core-basic).

## 4. Status Implementasi Hari Ini
Lihat [`01-architecture.md`](01-architecture.md) §4 — `DDLResult` masih
representasi SQL langsung, belum diformulasikan ulang sebagai diff
storage-agnostic yang baru diterjemahkan ke SQL di lapisan backend ini.

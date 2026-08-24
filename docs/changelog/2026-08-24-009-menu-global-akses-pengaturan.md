# Menu Global — Akses User dan Peran + Pengaturan

**Tanggal**: 2026-08-24

## Apa yang diubah

Sidebar menu cafe dirombak: item "Access Management" (top-level) dipindah ke
kategori baru **"Global"** di bagian akhir sidebar, di-rename jadi **"Akses
User dan Peran"**, dan ditambah item **"Pengaturan"**.

**File:**

- `examples/cafe/spec/apps/cafe.yaml` — menu: hapus item top-level "Access
  Management", tambah kategori `Global` (icon `globe`) dengan 2 children:
  - `Akses User dan Peran` (icon `shield`, route `/access-management`)
  - `Pengaturan` (icon `settings`, route `/settings`)
- `examples/cafe/spec/modules/formspec.core/pages/settings.yaml` (baru) —
  `kind: Page` route `/settings`, menampilkan global settings (spec §10)
  dalam 4 kartu: Mata Uang, Lokale & Zona Waktu, Tanggal & Angka, dan Contoh
  Tampilan (nilai statis sesuai `config.yaml`).

## Kenapa

Memberi akses UI ke global settings yang baru diimplementasikan
(`2026-08-24-008`), dan mengelompokkan item admin/global di satu kategori
sidebar yang jelas.

## Catatan

Halaman Pengaturan saat ini menampilkan nilai statis (HTML block) yang
mencerminkan `config.yaml`. Jika settings berubah, halaman perlu di-update
manual — belum ada binding dinamis ke `bundle.settings` (bisa jadi follow-up
dengan component/asset block).

## Test

- `formspec validate --schema schemas` — 20 manifest cafe pass.
- Verified via browser: sidebar menampilkan kategori "Global" dengan 2 item;
  kedua route (`/access-management`, `/settings`) merender dengan benar.

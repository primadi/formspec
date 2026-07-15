# Katalog Kind — Tier Page

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap. Setiap kind di sini adalah instance VisualSpecKind
> `tier: page` — skema shell-agnostic, satu definisi untuk semua shell.

## 1. `data-entry` (Form)
Form entry berbasis entity: field, section, validasi, mode create/edit/view.

## 2. `table-list` (Table)
Daftar ber-filter/sort/paginasi; kolom terderivasi dari entity.

## 3. `wizard`
Multi-step flow; state antar step; submit transaksional.

## 4. `dashboard`
Grid slot `widget` (lihat slot system): posisi, ukuran, data binding.

## 5. `kanban`
Kolom berbasis status/field; kartu; drag transition.

## 6. `report` dan `print`
Output baca/berkas; parameterisasi.

## 7. `timeline` / `timeseries`
Tampilan kronologis/deret waktu.

## 8. `listing`
Katalog publik (e-commerce, movie search) — pasangan alami App kind
`landing-page`.

## 9. Derivasi Otomatis (Layer 0)
Page yang digenerate otomatis dari Entity tanpa spec eksplisit; aturan
override-nya.

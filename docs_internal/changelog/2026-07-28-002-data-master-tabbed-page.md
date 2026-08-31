# Data Master — Tabbed Page untuk Dokter, Pasien, Poliklinik

## Perubahan

Menambahkan **Data Master** sebagai menu grup di sidebar Klinik dengan tabbed page untuk mengelola tiga entity master: Dokter, Pasien, dan Poliklinik.

### File baru

- `spec/modules/clinic/master/doctor/tables/list.yaml` — authored Table minimal untuk entity `doctor` (derived columns)
- `spec/modules/clinic/master/patient/tables/list.yaml` — authored Table minimal untuk entity `patient`
- `spec/modules/clinic/master/polyclinic/tables/list.yaml` — authored Table minimal untuk entity `polyclinic`
- `spec/modules/clinic/pages/data-master.yaml` — kind: Page varian tabs, route `/app/master`, 3 tab (Dokter/Pasien/Poliklinik)

### File diubah

- `spec/modules/clinic/module.yaml` — tambah menu "Data Master" dengan children (Master Dokter, Master Pasien, Master Poliklinik) yang masing-masing me-link ke tab berbeda via `?tab=` query param
- `renderers/web/src/kinds/table/TableRenderer.tsx` — fix: authored Table tanpa columns eksplisit fallback ke derived columns, sehingga Table minimal cukup deklarasi `entity:` tanpa perlu daftar kolom

### Cara kerja

- Menu "Data Master" > "Master Dokter" → `/app/master?tab=Dokter`
- Menu "Data Master" > "Master Pasien" → `/app/master?tab=Pasien`
- Menu "Data Master" > "Master Poliklinik" → `/app/master?tab=Poliklinik`
- Tiap tab menampilkan Table dengan columns derived dari entity spec
- Tabel pendukung (View, Edit, Delete, New, Search) berfungsi penuh

### Referensi

- Plan: todo.md §5.2.2 Tabs variant
- Spec: frontend/06-page-kinds.md §1 — Tabbed Resources pattern

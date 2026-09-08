# Dokumentasi Proyek — Clinic UI Showcase (`klinik-sehat`)

Dokumentasi lengkap untuk aplikasi **Clinic UI Showcase** — contoh FormSpec
yang meng-exercise **semua fitur frontend renderer** (UI kinds, pola
lifecycle, derived-by-default). Domain klinik dipilih karena contoh-contoh
kanonik di spec frontend (wizard pendaftaran pasien, kanban antrian farmasi,
timeline rekam medis, struk thermal) memetakan 1:1 ke sini.

## Daftar Isi

| Dokumen                                | Isi                                                                           |
| -------------------------------------- | ----------------------------------------------------------------------------- |
| [`overview.md`](./overview.md)         | Pengenalan proyek, tujuan showcase, tech stack, prinsip kunci                 |
| [`architecture.md`](./architecture.md) | Arsitektur: 2 App / 2 module, karakteristik entity, pola lifecycle, sidecar   |
| [`domain-model.md`](./domain-model.md) | Model data: 11 entity, relasi, state machine, natural key (dengan diagram ER) |
| [`development.md`](./development.md)   | Panduan pengembangan: menjalankan, validasi, testing e2e, sidecar Node        |

## Referensi Cepat

```bash
# Validasi semua manifest (dari root repo)
go run ./cmd/formspec/ validate --spec examples/Clinic-UI-Showcase/spec

# Jalankan dev server (SQLite, SPA built-in)
go run ./cmd/formspec/ dev --spec examples/Clinic-UI-Showcase/spec \
  --dsn "sqlite:.formspec/clinic.db"

# UI aplikasi
# http://localhost:8080/default/_admin
```

- **Struktur spek**: `spec/apps/` (2 App), `spec/modules/clinic/`,
  `spec/modules/pharmacy/`
- **2 App**: `klinik-internal` (staf, menu penuh) + `klinik-portal`
  (publik, hanya pendaftaran pasien) — bukti 1 module bisa di-mount
  banyak App
- **2 module**: `clinic` (pasien, dokter, kunjungan, kasir, pengaturan) dan
  `pharmacy` (obat, resep, OTC) — sengaja dua supaya sidebar
  module-grouping, cross-module relation, dan cross-module widget ikut
  teruji
- **11 entity**, **12 script Starlark**, matriks lengkap fitur renderer →
  file lihat [`README.md`](../README.md) di root example

## Dokumen Terkait

| Dokumen                                                                                         | Isi                                                            |
| ----------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| [`../README.md`](../README.md)                                                                  | Matriks coverage fitur renderer → file (sumber utama showcase) |
| [`../how-to-run.md`](../how-to-run.md)                                                          | Panduan menjalankan (Persona A/B, sidecar, akses)              |
| [`docs/spec/05-frontend.md`](../../../docs/spec/05-frontend.md)                                 | Spec frontend — kontrak normatif UI kinds                      |
| [`docs/implementation/frontend-renderer.md`](../../../docs/implementation/frontend-renderer.md) | Implementasi renderer shadcn-shell                             |

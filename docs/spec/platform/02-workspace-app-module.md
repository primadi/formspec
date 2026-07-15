# Workspace, App, Module

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap.

## 1. Model Kepemilikan
Satu workspace berisi banyak App dan banyak Module. Module memiliki objek
(Document/Entity, Action, view kinds); **App adalah kurasi** — keranjang objek
dari module-module yang dideklarasikan lewat `depends_on`, bukan pemilik objek.

## 2. Module
Struktur module, objek yang bisa dimiliki, penamaan, versi.

## 3. App
Spec App: `depends_on`, pemilihan app renderer (lihat spec frontend), theme
binding, auth.

## 4. Menu
Dua mode: `module` (otomatis semua entity dari module yang di-depends_on) dan
`custom` (kurasi eksplisit; urutan list = urutan tampil; entity yang tidak
disebut tidak muncul). Skema menu di Module dan App identik bentuknya sehingga
bisa disalin langsung. Trade-off mode `custom`: lepas dari auto-sync saat module
menambah entity baru.

## 5. Qualifier Referensi Antar Module
Notasi `module/resource` untuk referensi lintas module (konsisten dengan
`sources.resource` dan penamaan named script `{module}/{script-name}`).
Referensi di dalam module sendiri tanpa qualifier.

## 6. Validasi `forma apply`
`menu.items[].entity` wajib tercakup dalam `depends_on` App; aturan validasi
kepemilikan dan referensi lainnya.

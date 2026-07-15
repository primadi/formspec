# Core Basic

**Version:** 0.1.0 · **Status:** Outline

> Dokumen berstatus Outline: heading di bawah menetapkan cakupan final; isi
> ditulis bertahap. Seluruh kontrak di dokumen ini storage-agnostic; contoh SQL
> konkret hidup di dokumentasi renderer persist-postgres.

## 1. Document (Entity)
Skema field, tipe, validasi, title, expose (default unexposed), relasi dan
target relasi.

## 2. Primary Key & Natural Key
Strategi PK (UUID v7 / integer / natural key), `natural_key_rule`, jaminan
gap-free sequence sebagai kontrak (bukan mekanisme spesifik backend).

## 3. Persistence Sebagai Kontrak
`persist.indexes` dan deklarasi persist lain: apa yang dijanjikan framework,
apa yang jadi urusan PersistBackend.

## 4. Migration = Structural Diff
Framework menghasilkan structural diff dari perubahan spec; PersistBackend
menerima diff dan menerjemahkannya ke storage-nya sendiri. Tidak ada asumsi
"framework generate SQL".

## 5. Action
`impl.native` / `impl.script` / `impl.script_ref` / `impl.sidecar`; context yang
tersedia; hooks (before/after/on_error).

## 6. Query & Filter Operator
Konvensi HTTP query (`eq`, `gt`, `between`, …) sebagai kontrak yang wajib
diimplementasikan setiap PersistBackend secara identik.

## 7. Event & Outbox
Event model, delivery guarantee.

## 8. API Runtime yang Digenerate
Bentuk REST API per entity yang dijanjikan ke konsumen (workspace-scoped).

## 9. Error Model
Kode `FORMA.*` (lihat error-glossary.yaml); kontrak error yang bisa
di-branch programatik.

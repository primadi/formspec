# Extension validate (todo 4.3.5)

**Date**: 2026-08-17

Menambahkan deklarasi additive business rule pada entity extension.

- `pkg/spec/entity.go`: `ExtendStorage.Validate` — script ref Starlark yang
  berjalan setelah validasi base L1–L6, read-only terhadap base fields, hanya
  boleh me-require field namespaced sendiri (docs/spec/backend/03-entity-extension.md §5).

Catatan: eksekusi runtime script validate (menjalankan script setelah
validasi base saat create/update) = enhancement — deklarasi + kontrak sudah
ada.

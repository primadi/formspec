# Cross-category JOIN block (todo 4.4.2)

**Date**: 2026-08-17

Mengimplementasikan blokir JOIN lintas kategori.

- `renderers/jsonb-persist/crud.go`: `EntityStore.category` +
  `SetTargetCategoryResolver`; `resolveRelations` memblokir resolusi relasi
  bila kategori target ≠ kategori sumber (log warning
  `FORMSPEC.PERSIST.CROSS_CATEGORY`).
- `internal/entity/registry.go`: `GetEntityStore` me-wire
  `SetTargetCategoryResolver` dari `specs` (persist.category).
- Test: `renderers/jsonb-persist/cross_category_test.go`.

Catatan: blokir saat ini log warning + skip resolusi (bukan hard error) agar
tidak memutus list yang sudah jalan; error code `FORMSPEC.PERSIST.CROSS_CATEGORY`
sudah ada di `pkg/spec/errors.go`.

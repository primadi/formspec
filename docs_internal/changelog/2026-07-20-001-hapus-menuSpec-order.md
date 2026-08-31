# Hapus `MenuSpec` dan `MenuItem.Order`

**Date:** 2026-07-20
**Plan:** `docs/plan/todo.md` — Fase 1.1 (Menu refactor)

## Changes

Revert `MenuSpec` wrapper dan `MenuItem.Order` — kembali ke `[]MenuItem` langsung untuk `App.spec.menu` dan `Module.spec.menu`.

### Rationale
- `MenuSpec.Mode` tidak pernah diimplementasi/dicek di kode (dead field)
- YAML examples tidak ada yang pakai format `{mode, items}` — semuanya flat array
- Array index sudah cukup sebagai urutan menu; `Order` redundant
- Lihat `docs/spec/platform/02-workspace-app-module.md` §4 untuk spesifikasi baru

### Files affected

| File | Perubahan |
|---|---|
| `pkg/spec/resources.go` | `ModuleSpec.Menu`: `*MenuSpec` → `[]MenuItem`; `AppSpec.Menu`: `*MenuSpec` → `[]MenuItem`; hapus struct `MenuSpec`; hapus `MenuItem.Order` |
| `pkg/spec/spec.go` | Update comment (tidak lagi menyebut `MenuSpec`) |
| `internal/app/resolve.go` | Hapus `menuItemsOrEmpty()`; callers langsung pakai field; hapus `Order` propagation di adopt node |
| `renderers/web/src/types/manifest.ts` | Hapus `order?: number` dari `MenuItem` |
| `examples/**/module.yaml` (3 files) | Hapus semua `order:` dari menu items |
| `examples/**/apps/*.yaml` (1 file) | Hapus `order:` dari menu items |
| `internal/entity/testdata/**/*.yaml` (4 files) | Hapus semua `order:` dari menu items |
| `docs/spec/platform/02-workspace-app-module.md` | §4: hapus mode `module/custom`, `MenuSpec`, `Order`; update validasi §6 |

### Reversi dari 2026-07-19
- `AppSpec.Menu` kembali ke `[]MenuItem` (tidak lagi `*MenuSpec`)
- `ModuleSpec.Menu` kembali ke `[]MenuItem` (tidak lagi `*MenuSpec`)
- `MenuSpec` struct dihapus

### References
- docs/spec/platform/02-workspace-app-module.md (§4 Menu, §6 Validasi)

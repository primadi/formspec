# Plan 2.5 — API Infrastructure

**Date**: 2026-07-27  
**Referensi**: `docs/spec/backend/01-core-basic.md` §8, `docs/plan/todo.md` §2.5

## Perubahan

### 2.5.1 Two API Surfaces
- **`internal/api/generator.go`**: 
  - `GenerateUIRoutes()` baru — generates routes for ALL entities on `/_ui/entity/` surface (bypasses spec.expose)
  - `GenerateUICustomActionRoutes()` baru — custom action routes untuk UI surface
  - `mergeRoutes()` jadi variadic untuk multiple slices
- **`internal/api/router.go`**:
  - `BuildHTTP()` sekarang punya dua route group: `/{ws}/api/v1/` (external) + `/{ws}/_ui/entity` (UI)
  - `registerRouteWithPattern()` baru untuk UI routes dengan path prefix berbeda
  - `BuildRoutes()` generate UI + external + custom action routes

### 2.5.2–2.5.9
- Semua sudah terimplementasi sebelumnya (chi radix-tree, single logic path, links, details, clamping, envelope, meta API, workspace middleware)

## File yang terkena
- `internal/api/generator.go`
- `internal/api/router.go`

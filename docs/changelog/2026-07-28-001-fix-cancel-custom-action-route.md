# Fix: Cancel Custom Action Route & Script Resolution

## What Changed

### 1. Custom action route generation colliding with standard lifecycle actions

**Problem:** The `cancel` action on Visit entity was returning 404. Two issues:
- `generateRESTRoutes()` skipped lifecycle actions (`cancel`/`submit`/`amend`) by default when no explicit `expose.actions` filter is set
- `GenerateCustomActionRoutes()` and `GenerateUICustomActionRoutes()` skipped `cancel` because `isStandardAction("cancel")` returned `true`

**Fix:** Replaced `isStandardAction()` with `isStandardCrudAction()` in both custom action generators. Lifecycle action names (`submit`/`cancel`/`amend`) are no longer filtered out by the custom action generators — entities that define these with a custom `impl` (Starlark script) get proper custom routes.

**File:** `internal/api/generator.go`
- `isStandardAction()` → `isStandardCrudAction()` (only checks list/find/create/update/delete)
- Both `GenerateCustomActionRoutes()` and `GenerateUICustomActionRoutes()` now use `isStandardCrudAction()`

### 2. Script resolver asumsi folder structure `modules/{module}/scripts/`

**Problem:** `resolveScriptPath()` punya hardcoded asumsi folder `modules/` prefix. Developer tidak bisa pakai struktur folder lain.

**Fix:** Pendekatan baru — script ref diresolve **relatif ke direktori entity YAML**, bukan ke spec root. Ditambahkan field `SpecDir` di `action.ExecuteParams`, diisi oleh handler dari `SpecInfo.Source`.

`resolveScript(basePath, specDir, ref)` — urutan resolusi:
1. `{specDir}/scripts/{ref}.star` — colocated dengan entity
2. `{specDir}/{ref}.star` — direct file
3. `{basePath}/modules/{module}/scripts/{name}.star` — fallback module-scoped
4. `{basePath}/{ref}.star` — direct from spec root
5. `{basePath}/scripts/{name}.star` — flat scripts/ dir

**Zero asumsi folder structure** — ref cukup nama file (`cancel`), diresolve relatif ke entity YAML.

**Files:**
- `internal/action/dispatcher.go` — tambah `SpecDir string` di `ExecuteParams`
- `internal/action/script.go` — `resolveScript()` baru, `resolveScriptPath()` tetap untuk backward compat
- `internal/api/handler.go` — `HandleCustomAction()` terima `specDir` dan set ke `execParams`
- `internal/api/router.go` — ekstrak `specDir` dari `specInfo.Source` dan kirim ke handler

### 3. Entity YAML refs disederhanakan

Ref diubah dari full path (`clinic/transaction/visit/cancel`) jadi **nama file saja** (`cancel`) karena resolver kini resolve relatif ke direktori entity.

**Affected files:**
- `spec/modules/clinic/transaction/visit/entity.yaml` — 3 refs
- `spec/modules/pharmacy/transaction/prescription/entity.yaml` — 5 refs
- `spec/modules/pharmacy/transaction/otc-sale/entity.yaml` — 3 refs

## Testing

```
POST /default/_ui/entity/clinic/visit/{id}/cancel → 200 OK
POST .../start-consultation → script found ✅
POST .../complete → script found ✅
All tests: go test ./internal/... → PASS
```

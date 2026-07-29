# Relation Resolution for Dot-Path Fields in Table Columns

## Changes

**Problem**: Table columns with dot-path notation (`patient.name`, `polyclinic.name`, `doctor.name`) showed raw UUIDs (Patient-ID, Polyclinic-ID) instead of display names because:

1. The backend API returned flat data with foreign key IDs (`patient_id: "uuid"`) instead of nested relation objects (`patient: {name: "..."}`).
2. The frontend `TableRenderer` always derived table specs from entity fields — it never used authored manifests like `visit-table.yaml`.

**Fix**:

### Backend (`renderers/jsonbpersist/crud.go`)

- Added `resolveRelations()` method to `EntityStore` that batch-fetches related records for all `belongs_to` relation fields and nests them in the response data. For example, `patient_id` → `Data["patient"] = {"id": "uuid", "name": "John", ...}`.
- Called `resolveRelations` in `List()` after scanning records.
- Modified `hydrateAndCompute()` to also accept `workspaceID` and call `resolveRelations` for single-record fetches (`GetByID`), ensuring detail views also get nested relation data.
- Updated all callers of `hydrateAndCompute` to pass `workspaceID`.

### Frontend (`renderers/web/src/kinds/table/TableRenderer.tsx`)

- Changed `tableSpec` resolution to check `metaBundle.tables` for an authored Table manifest whose `spec.entity` matches the current entity (by qualified `module.name` or short name).
- Falls back to `deriveTable(entity)` when no authored table is found.

### Vite Proxy (`cmd/forma/dev.go`)

- **Bug**: `viteSPAProxy` only passed through `/{ws}/api/*` routes to the backend. The UI surface routes (`/{ws}/_ui/_meta/*`, `/{ws}/_ui/entity/*`) were proxied to Vite and returned HTML instead of JSON, causing the SPA to fail loading data.
- **Fix**: Added `/{ws}/_ui/*` to the passthrough list so meta API and entity CRUD requests reach the backend.

### Files Affected

| File | Change |
|---|---|
| `renderers/jsonbpersist/crud.go` | Added `resolveRelations()` method; called from `List()` and `hydrateAndCompute()` |
| `renderers/web/src/kinds/table/TableRenderer.tsx` | Look up authored table manifest before deriving |
| `cmd/forma/dev.go` | `viteSPAProxy` now passes through `/_ui/*` routes |

## Notes

- Cross-module relations are supported (`other_module.entity` syntax).
- Only `belongs_to` relations are resolved (the common case for dot-path columns). `has_many`/`has_one` can be added later if needed.
- The resolution is a single batch query per relation type (N+1 → 1 per relation), not per record.
- The relation alias is derived by stripping `_id` suffix from the field name (e.g., `patient_id` → `patient`).
- The Vite proxy fix also enables WebSocket (`/_ui/_ws`) passthrough for realtime features.

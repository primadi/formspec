# Plan 2.3 — Lifecycle Engine (Phase 2: Error Codes + Summary + Reference Guard)

**Date**: 2026-07-27  
**Referensi**: `docs/plan/todo.md` §2.3, `docs/spec/backend/error-glossary.yaml`

## Perubahan

### 2.3.7 Error Codes
- **`renderers/jsonbpersist/lifecycle.go`**: 
  - `LifecycleGuard` now returns specific error codes per error-glossary.yaml:
    - `submit` from `submitted` → `FORMSPEC.DOC.ALREADY_SUBMITTED`
    - `submit` from `cancelled` → `FORMSPEC.DOC.SUBMIT_NOT_DRAFT`
    - `cancel` from `cancelled` → `FORMSPEC.DOC.ALREADY_CANCELLED`
    - `cancel` from `draft`/`null` → `FORMSPEC.DOC.CANCEL_NOT_SUBMITTED`
    - `update`/`delete` from any non-draft → appropriate code
  - Removed `Required` field from `LifecycleError` (simplified)
- **`renderers/jsonbpersist/lifecycle_test.go`**: Updated error message assertions

### 2.3.10 Relation on_delete Framework
- **`renderers/jsonbpersist/reference.go`** (new): 
  - `CheckReferencingDocuments()` — stub (full implementation needs reference tracking system)
  - `EnforceReferenceGuard()` — returns `FORMSPEC.REF.DELETE_BLOCKED` / `FORMSPEC.REF.CANCEL_BLOCKED`
- **`renderers/jsonbpersist/crud.go`**: `SoftDelete()` now calls `EnforceReferenceGuard()`

### 2.3.12 Summary Immutability
- **`renderers/jsonbpersist/crud.go`**: 
  - `EntityStore.characteristic` field baru
  - `Insert()`, `Update()`, `SoftDelete()` block summary entities with `ErrValidationRule`
  - Summary entities permanently read-only at store level (not just API routes)

## File yang terkena
- `renderers/jsonbpersist/lifecycle.go`
- `renderers/jsonbpersist/lifecycle_test.go`
- `renderers/jsonbpersist/reference.go` (new)
- `renderers/jsonbpersist/crud.go`

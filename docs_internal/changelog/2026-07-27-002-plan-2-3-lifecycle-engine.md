# Plan 2.3 — Lifecycle Engine (Phase 1: Core Guard Wiring)

**Date**: 2026-07-27  
**Referensi**: `docs/plan/todo.md` §2.3, `docs/plan/2-3-lifecycle-engine-plan.md`

## Perubahan

### Lifecycle Guard Wiring
- **`renderers/jsonbpersist/crud.go`**: 
  - `EntityRecord.DocStatus` field baru + `EffectiveDocStatus()` method
  - `scanEntityRecord`: scan `doc_status` sebagai `sql.NullString` (handle NULL)
  - Semua SELECT queries include `doc_status` column
  - `Update()`: panggil `LifecycleGuard("update")` sebelum eksekusi
  - `SoftDelete()`: panggil `LifecycleGuard("delete")` sebelum eksekusi
  - `Submit()`: panggil `LifecycleGuard("submit")` sebelum SQL guard
  - `Cancel()`: panggil `LifecycleGuard("cancel")` sebelum SQL guard
  - `MarshalJSON`: include `doc_status` di response JSON

### REST Routes untuk Lifecycle Actions
- **`internal/api/descriptor.go`**: Tambah `submit`, `cancel`, `amend` ke `StandardRESTActions` (POST /{id}/submit, /{id}/cancel, /{id}/amend)
- **`internal/api/generator.go`**: Wire `TransitiveDisabled()` untuk transitive gating; submit/cancel/amend tidak muncul di default (harus explicit di spec.expose.actions)
- **`internal/api/handler.go`**: Tambah `HandleSubmit()`, `HandleCancel()`, `HandleAmend()` dengan hook execution + event emission
- **`internal/api/handler.go`**: `writeStoreError` handle `LifecycleError` → 422 LIFECYCLE_ERROR
- **`internal/api/router.go`**: Tambah case submit/cancel/amend di route dispatcher

## File yang terkena
- `renderers/jsonbpersist/crud.go`
- `internal/api/descriptor.go`
- `internal/api/generator.go`
- `internal/api/handler.go`
- `internal/api/router.go`

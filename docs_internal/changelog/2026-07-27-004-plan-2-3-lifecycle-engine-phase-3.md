# Plan 2.3 — Lifecycle Engine (Phase 3: Child sequence_field + Child Lifecycle)

**Date**: 2026-07-27  
**Referensi**: `docs/plan/todo.md` §2.3, `docs/spec/backend/01-core-basic.md` §1.3

## Perubahan

### 2.3.8 child.sequence_field enforcement
- **`renderers/jsonbpersist/child.go`**:
  - `InsertChildren`: Deteksi apakah client menyediakan sequence values atau tidak
    - Jika client menyediakan → validasi monotonik via `validateSequenceField()`
    - Jika client tidak menyediakan → auto-assign `1, 2, 3, ...`
  - `validateSequenceField()` baru — validasi strict monotonically increasing; duplikat atau nilai tidak naik → `VALIDATION_ERROR` (422)
  - `extractSeqValue()` baru — helper untuk fallback ke index-based default
  - `validationErr` type baru — wraps `ErrValidationRule`

### 2.3.9 Child lifecycle propagation
- **`renderers/jsonbpersist/ddl.go`**: Child table DDL sekarang include `doc_status  VARCHAR(20) DEFAULT NULL` column
- **`renderers/jsonbpersist/child.go`**: 
  - `SubmitChildren()` — update doc_status='submitted' untuk semua child row
  - `CancelChildren()` — update doc_status='cancelled' untuk semua child row
- **`renderers/jsonbpersist/crud.go`**: 
  - `Submit()` — propagate ke children setelah parent update
  - `Cancel()` — propagate ke children setelah parent update

## File yang terkena
- `renderers/jsonbpersist/child.go`
- `renderers/jsonbpersist/crud.go`
- `renderers/jsonbpersist/ddl.go`

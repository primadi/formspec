# Plan 2.3 — Lifecycle Engine

**Status**: Draft  
**Referensi**: `docs/spec/backend/01-core-basic.md` §1–§2, `docs/plan/todo.md` §2.3  
**Estimasi**: Large

## Ringkasan Gap

`LifecycleGuard()` sudah ada di `lifecycle.go` tapi **tidak di-wire** ke jalur mutasi:
- `Update()` dan `SoftDelete()` tidak mengecek lifecycle guard
- `submit`/`cancel`/`amend` tidak punya REST route (tidak ada di `StandardRESTActions`)
- `create-submit`/`amend-submit` auto-derive tidak di-wire
- `TransitiveDisabled()` tidak dipakai
- `child.sequence_field` tidak divalidasi
- `relation.on_delete` 3 mode belum ada
- Child lifecycle propagation belum ada

## Perubahan per File

### Phase 1: Core Guard Wiring (2.3.1, 2.3.2, 2.3.3, 2.3.5, 2.3.6)

**`renderers/jsonbpersist/lifecycle.go`**
- Tambah error codes: `FORMSPEC.DOC.ALREADY_SUBMITTED`, `FORMSPEC.DOC.ALREADY_CANCELLED`, `FORMSPEC.DOC.CREATE_SUBMIT_NOT_AVAILABLE`, `FORMSPEC.REF.DELETE_BLOCKED`, `FORMSPEC.REF.CANCEL_BLOCKED`
- Tambah `LifecycleGuard` untuk `update` dengan pengecekan `submitEnabled` (jika submit nonaktif → lifecycle-free → update selalu allowed)

**`renderers/jsonbpersist/crud.go`**
- `Update()`: panggil `LifecycleGuard("update", docStatus)` sebelum eksekusi
- `SoftDelete()`: panggil `LifecycleGuard("delete", docStatus)` sebelum eksekusi
- `Submit()`, `Cancel()`, `Amend()`: sudah ada SQL-level guard (WHERE doc_status = 'draft'/'submitted'), tapi perlu tambah `LifecycleGuard` untuk konsistensi + error code
- Export `DocStatus()` method pada EntityRecord

**`internal/api/descriptor.go`**
- Tambah `submit, cancel, amend` ke `StandardRESTActions` (POST /{id}/submit, dll)

**`internal/api/generator.go`**
- Wire `TransitiveDisabled()` untuk filter action
- Wire `DeriveReservedActions()` untuk auto-derive create-submit/amend-submit
- Summary entity: sudah skip create/update/delete ✅

**`internal/api/handler.go`**
- Tambah `HandleSubmit()`, `HandleCancel()`, `HandleAmend()` method
- Gunakan `store.Submit()`/`store.Cancel()`/`store.Amend()` + lifecycle guard + event dispatch

**`internal/api/router.go`**
- Di `registerRoute`, tambah case untuk submit/cancel/amend yang panggil handler baru

### Phase 2: Relation on_delete (2.3.10)

**`renderers/jsonbpersist/crud.go`**
- `ValidateRelationTargets`: tambah dukungan `on_delete` mode:
  - `restrict` (default): sama seperti sekarang — cek doc_status
  - `cascade`: hapus referencing documents yang masih draft/lifecycle-free
  - `set_null`: set field ke null jika field tidak required
- `SoftDelete`/`Cancel`: sebelum execute, cek apakah ada doc lain yang mereferensi entity ini → blokir dengan `FORMSPEC.REF.DELETE_BLOCKED` / `FORMSPEC.REF.CANCEL_BLOCKED`

### Phase 3: Child Lifecycle & Sequence (2.3.8, 2.3.9)

**`renderers/jsonbpersist/child.go`**
- `ValidateSequenceField()`: validasi monotonik ordering pada insert
- `SubmitChildren()`/`CancelChildren()`: propagate lifecycle ke child table

**`renderers/jsonbpersist/lifecycle.go`**
- `PropagateChildLifecycle()`: update doc_status untuk semua child record

### Phase 4: Characteristic Enforcement (2.3.11, 2.3.12)

**`renderers/jsonbpersist/crud.go`**
- `Insert()`, `Update()`, `SoftDelete()`: block untuk `characteristic: summary` → `FORMSPEC.DOC.SUMMARY_IMMUTABLE`
- `ValidateDocumentSpec`: sudah validasi transaction_date + characteristic count ✅

### Phase 5: Error Codes (2.3.7)

**`renderers/jsonbpersist/lifecycle.go`**
- Lengkapi semua error code dari error-glossary.yaml

# 2026-07-08 — Document Model Code Alignment (Tahap 2)

**Plan:** `docs/plan/document-model-code-alignment.md`
**Referensi:** `docs/spec/02-core-basic.md` v0.3.0

## Ringkasan

Menyelaraskan Go code dengan Document Model spec v0.3.0. Semua perubahan backward-compatible — `kind: Entity` tetap diterima sebagai deprecated alias.

## Perubahan

### 1. `pkg/spec/spec.go` — Kind + Characteristic Constants

- Tambah `KindDocument = "Document"`, biarkan `KindEntity = "Entity"` sebagai deprecated alias
- Tambah `KindIntegrator = "Integrator"` untuk Core Extended
- Tambah `IsDocumentKind()` helper
- Update `IsValidKind()` untuk menyertakan KindDocument + KindIntegrator

### 2. `pkg/spec/entity.go` — Document Model Types

- `EntitySpec` → type alias `EntitySpec = DocumentSpec`
- `Characteristics []Characteristic` → `Characteristic Characteristic` (single value)
- Tambah `DocStatus` type + constants: `DocStatusDraft`, `DocStatusSubmitted`, `DocStatusCancelled`
- Tambah `ReservedFieldNames`: `owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version`
- Tambah `ReservedActionNames`: `create`, `update`, `submit`, `cancel`, `delete`, `amend`, `create-submit`, `amend-submit`
- Tambah `OnDelete` type: `OnDeleteRestrict`, `OnDeleteCascade`, `OnDeleteSetNull`
- `RelationDecl.OnDelete` field baru
- `ValidateDocumentSpec()` — validasi reserved fields + transaction_date wajib untuk characteristic:transaction
- `IsReservedField()`, `IsReservedAction()`, `IsDerivedReservedAction()`, `IsValidDocStatus()`

### 3. `internal/manifest/loader.go` — Backward Compat + Validasi

- `KnownKinds` tambah `"Document"` + `"Integrator"`
- `Validate()` terima `kind: Document` juga (bukan hanya `Entity`)
- `RawSpecToEntitySpec()` backward compat: `characteristics: [X]` → `characteristic: X`
- Panggil `ValidateDocumentSpec()` (bukan `ValidateEntitySpec` lama)

### 4. `internal/entity/registry.go` — Document-aware

- `LoadEntities()` terima `kind: Document` via `IsDocumentKind()`
- `ListEntities()` → Kind = `"Document"`, `Characteristic` dari single value
- Gunakan `ValidateDocumentSpec()`

### 5. `internal/db/ddl.go` — Reserved Columns

Auto-inject ke setiap Document table:
- `doc_status VARCHAR(20) DEFAULT NULL` (NULL = lifecycle-free)
- `amends UUID REFERENCES self(id)`
- `amended_by UUID REFERENCES self(id)`

### 6. `internal/db/crud.go` — Lifecycle Init

- `EntityStore.submitEnabled` field baru — ditentukan dari `spec.actions`
- `Insert()`: auto-set `doc_status = 'draft'` jika submit enabled, `NULL` jika lifecycle-free
- SQL insert sekarang 5 params (tenant_id, created_by, updated_by, doc_status, data)

### 7. `internal/db/lifecycle.go` (NEW) — Lifecycle Guards

- `LifecycleGuard(actionName, docStatus)` — framework-enforced lifecycle rules untuk 8 reserved actions
- `DeriveReservedActions()` — auto-derive `create-submit` + `amend-submit`
- `TransitiveDisabled()` — submit disabled → cancel + amend implicitly disabled
- `LifecycleError` type dengan error codes (FORMA.DOC.*)

### 8. `internal/db/lifecycle_test.go` (NEW) — 9 Tests

Mencakup semua 8 reserved actions + transitive gating + derive logic.

### 9. Test Data Migration

- 28 file YAML di `internal/entity/testdata/` + `examples/`: `characteristics: [X]` → `characteristic: X`
- 8 file YAML transaction entities: tambah field `transaction_date`
- `api_test.go`, `ddl_test.go`, `registry_test.go`: `Characteristics` → `Characteristic`

## Files Changed

| File | Change |
|---|---|
| `pkg/spec/spec.go` | +KindDocument, +KindIntegrator, +IsDocumentKind |
| `pkg/spec/entity.go` | DocStatus, reserved fields/actions, OnDelete, single Characteristic, ValidateDocumentSpec |
| `internal/manifest/loader.go` | +Document kind, backward compat characteristics, ValidateDocumentSpec |
| `internal/entity/registry.go` | IsDocumentKind, ValidateDocumentSpec, Kind="Document" |
| `internal/db/ddl.go` | +doc_status, amends, amended_by columns |
| `internal/db/crud.go` | submitEnabled, auto-set doc_status on insert |
| `internal/db/lifecycle.go` | **NEW** — LifecycleGuard, DeriveReservedActions, TransitiveDisabled |
| `internal/db/lifecycle_test.go` | **NEW** — 9 lifecycle tests |
| `internal/api/generator.go` | Characteristics → Characteristic |
| `internal/api/api_test.go` | Characteristics → Characteristic |
| `internal/db/ddl_test.go` | Characteristics → Characteristic |
| `internal/entity/registry_test.go` | Kind="Document", Characteristic fix |
| 28 YAML files (testdata + examples) | characteristics: [X] → characteristic: X |
| 8 YAML transaction files | +transaction_date field |

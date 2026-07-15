# Tahap 2 — Document Model Code Alignment

**Status:** In Progress  
**Referensi:** `docs/spec/02-core-basic.md` v0.3.0, `reff_docs/Forma-Technical-Note-Document-Model.md`  
**Dependensi:** Tahap 1 (Spec Updates) ✅  
**Estimasi:** Large (8-10 jam)

---

## Tujuan

Menyelaraskan Go code di `pkg/spec/`, `internal/manifest/`, `internal/action/`, `internal/db/`, `internal/entity/` dengan spec Document Model v0.3.0. Semua perubahan backward-compatible — `kind: Entity` tetap diterima sebagai deprecated alias.

---

## Task Breakdown

### 2.1 Spec Types — `pkg/spec/spec.go` & `pkg/spec/entity.go`

| # | Task | File | Effort |
|---|---|---|---|
| 2.1.1 | Tambah `KindDocument` = "Document", biarkan `KindEntity` sebagai deprecated alias | `spec.go` | Small |
| 2.1.2 | `Characteristics []Characteristic` → `Characteristic Characteristic` (single, bukan array) | `entity.go` | Small |
| 2.1.3 | Tambah reserved field names constant: `name`, `owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version` | `entity.go` | Small |
| 2.1.4 | Tambah `DocStatus` type & constants: `DocStatusDraft`, `DocStatusSubmitted`, `DocStatusCancelled` | `entity.go` | Small |
| 2.1.5 | Tambah `OnDelete` type: `OnDeleteRestrict`, `OnDeleteCascade`, `OnDeleteSetNull` ke `RelationDecl` | `entity.go` | Small |
| 2.1.6 | Tambah reserved action names constant: `create`, `update`, `submit`, `cancel`, `delete`, `amend`, `create-submit`, `amend-submit` | `entity.go` | Small |
| 2.1.7 | `ValidateEntitySpec` → `ValidateDocumentSpec`, tambah validasi characteristic (REJECT >1) | `entity.go` | Medium |

### 2.2 Manifest Loader — `internal/manifest/loader.go`

| # | Task | File | Effort |
|---|---|---|---|
| 2.2.1 | `kind: Entity` → dipetakan ke `KindDocument` dengan deprecation warning | `loader.go` | Small |
| 2.2.2 | Validasi reserved field names — REJECT jika field custom pakai nama reserved | `loader.go` | Medium |
| 2.2.3 | Validasi `characteristic: transaction` MUST punya `transaction_date` field | `loader.go` | Small |
| 2.2.4 | Validasi characteristic single value | `loader.go` | Small |

### 2.3 DDL Generator — `internal/db/ddl.go`

| # | Task | File | Effort |
|---|---|---|---|
| 2.3.1 | Auto-inject reserved columns: `id` (UUID PK), `name`, `owner`, `created_at`, `modified`, `doc_status` (nullable), `amends` (nullable UUID), `amended_by` (nullable UUID), `version` (int) | `ddl.go` | Medium |
| 2.3.2 | `doc_status` column: type VARCHAR(20) NULL, default NULL | `ddl.go` | Small |

### 2.4 CRUD Layer — `internal/db/crud.go`

| # | Task | File | Effort |
|---|---|---|---|
| 2.4.1 | Auto-set reserved fields on Create: `id` (UUID), `name`, `owner`, `created_at`, `modified`, `doc_status` (null unless lifecycle active), `version` (1) | `crud.go` | Medium |
| 2.4.2 | On Update: auto-increment `version`, update `modified` | `crud.go` | Small |
| 2.4.3 | Validate `transaction_date` backdate/forward-date policy on create/update | `crud.go` | Medium |
| 2.4.4 | Referenceability guard: hanya `submitted` atau `null` doc_status yang bisa jadi target relation | `crud.go` | Medium |

### 2.5 Action Dispatcher — `internal/action/dispatcher.go`

| # | Task | File | Effort |
|---|---|---|---|
| 2.5.1 | Lifecycle guards untuk 8 reserved actions sebelum dispatch ke executor | `dispatcher.go` | Large |
| 2.5.2 | Auto-derive `create-submit` jika `create` + `submit` enabled | `dispatcher.go` | Medium |
| 2.5.3 | Auto-derive `amend-submit` jika `amend` + `submit` enabled | `dispatcher.go` | Medium |
| 2.5.4 | Transitive gating: submit disabled → cancel & amend implicitly disabled | `dispatcher.go` | Medium |

### 2.6 Entity Registry — `internal/entity/`

| # | Task | File | Effort |
|---|---|---|---|
| 2.6.1 | `GetEntitiesByCharacteristic` → update untuk single value | `registry.go` | Small |
| 2.6.2 | Rename internal references Entity→Document (non-breaking, backward-compat) | `registry.go` | Small |

### 2.7 Event Naming Validation

| # | Task | File | Effort |
|---|---|---|---|
| 2.7.1 | Validate event prefix: `before_*` = sync, `on_*` = async | `loader.go` or validator | Small |

---

## Lifecycle Guard Rules (2.5.1 Detail)

| Action | Guard | Error |
|---|---|---|
| `create` | Always allowed (creates new doc) | — |
| `update` | `doc_status == draft OR doc_status IS NULL` | `FORMA.DOC.UPDATE_NOT_DRAFT` |
| `submit` | `doc_status == draft` | `FORMA.DOC.SUBMIT_NOT_DRAFT` |
| `cancel` | `doc_status == submitted` | `FORMA.DOC.CANCEL_NOT_SUBMITTED` |
| `delete` | `doc_status == draft OR doc_status IS NULL` (no reference) | `FORMA.DOC.DELETE_REFERENCED` |
| `amend` | `doc_status == submitted OR doc_status == cancelled` | `FORMA.DOC.AMEND_NOT_SUBMITTED_OR_CANCELLED` |
| `create-submit` | Auto-derived, combines create + submit logic | — |
| `amend-submit` | Auto-derived, combines amend + submit logic | — |

---

## Files Impacted

| File | Change Level | Backward Compat? |
|---|---|---|
| `pkg/spec/spec.go` | Minor (add KindDocument) | ✅ KindEntity tetap valid |
| `pkg/spec/entity.go` | Major (single characteristic, reserved names, doc_status) | ✅ backward-compat via loader |
| `internal/manifest/loader.go` | Medium (validation rules) | ✅ |
| `internal/db/ddl.go` | Medium (reserved columns) | ✅ old tables tetap work |
| `internal/db/crud.go` | Large (auto-set reserved, guards) | ✅ |
| `internal/action/dispatcher.go` | Large (lifecycle guards) | ✅ |
| `internal/entity/registry.go` | Small (single characteristic) | ✅ |
| `internal/entity/state_machine.go` | Small (event naming) | ✅ |

---

## Test Impact

Semua existing tests (~214) harus tetap passing. Test baru perlu ditambah untuk:
- Lifecycle guard behavior (5-7 tests)
- Reserved field validation rejection (3-4 tests)
- Characteristic single-value validation (2 tests)
- Referenceability guard (3 tests)
- Create-submit/amend-submit auto-derivation (2-3 tests)

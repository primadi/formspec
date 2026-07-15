# 2026-07-08 — Event Naming Validation (Core §12)

**File:** `pkg/spec/entity.go`, 8 YAML event declarations, `docs/spec/02-core-basic.md`

## Ringkasan

Implementasi validasi event naming convention per §12:
- `before_*` prefix → wajib sync (gate)
- `on_*` prefix → wajib async (notification)
- Custom events tanpa prefix → `type:` MUST ditulis eksplisit

## Perubahan

### Go Code

- `EventDecl.Type` field baru (`sync` | `async`)
- `ValidateEventNaming()` — validasi per-event
- `ValidateEvents()` — iterates events, calls ValidateEventNaming
- `ValidateDocumentSpec()` — calls ValidateEvents

### YAML Fixes (8 files)

6 events + `type: async`:
- `journal-posted` (2 files: examples + testdata)
- `journal-reversed` (2 files: examples + testdata)
- `stock-applied` (2 files: examples + testdata)
- `paid` (2 files: examples + testdata)

### Yang Sudah Ada Sebelumnya (dari round sebelumnya)

- `Submit()`, `Cancel()`, `Amend()` methods di `EntityStore` 
- `ValidateRelationTargets()` — referenceability guard wired ke Insert/Update
- `Amend()` — atomic cancel original + create new + set `amends`/`amended_by`
- 8 file YAML + `transaction_date` untuk characteristic: transaction

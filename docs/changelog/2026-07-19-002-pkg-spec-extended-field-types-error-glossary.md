# Pkg/Spec: Extended Field Types, Meta-Kinds, Frontend Kinds, Error Glossary

**Date:** 2026-07-19
**Plan:** `docs/plan/todo.md` — Fase 1.2, 1.3, 1.4, 1.5

## Changes

### 1.2 Meta-kind structs (`pkg/spec/resources.go`)
- `VisualSpecKindSpec` — declare new view types (`tier`, `schema`, `renderer_contract`, `accepts_slots`, `implements_slot`) + `SlotDecl`, `SlotContract`
- `RendererSpec` — concrete VisualSpecKind implementation (`implements`, `stack_family`, `trust_tier`)
- `PersistBackendSpec` — storage seam declaration (`implements`, `trust_tier`)

### 1.3 Frontend kind structs (`pkg/spec/frontend.go`)
- `CalendarSpec` — calendar view (`entity`, `date_field`, `end_field`, `title_field`, `resource_field`, `color_field`, `views[]`, `realtime`)
- `ApprovalInboxSpec` — pending approvals (`realtime`, `filters`, `search`)
- `NotificationCenterSpec` — in-app notifications (`realtime`)
- `ListingSpec` — public catalog (`entity`, `columns`, `filters`, `search`)

### 1.4 Extended field type structs (`pkg/spec/entity.go`)
- Added missing `FieldType` constants: `FieldText`, `FieldRichText`, `FieldMoney`, `FieldTime`, `FieldFile`
- Extended `Field` struct with: `Tree`, `Classification`, `RequiredPermission`, `Exclude`, `Encrypted`, `Masked`, `Storage`
- Added `RateLimit` to `Action` struct and `DocumentSpec` (resource-level)
- Added `Secrets` to `UsesDecl` (`uses.secrets` for ctx.secrets access)
- Added `SoftDeactivate` to `DocumentSpec`
- New types: `RateLimitSpec`, `FieldClassification` (with constants), `TreeDecl`, `SoftDeactivateDecl`, `StorageSpec`, `StorageTransform`

### 1.5 Error glossary (`pkg/spec/errors.go` — new file)
- `ErrorCode` type, 22 `FORMSPEC.*` error constants from `error-glossary.yaml`
- 3 observability error codes (`OBSERVABILITY_METRICS_DISABLED`, `OBSERVABILITY_DEBUG_FORBIDDEN`, `LOGS_FILTER_INVALID`)
- `FormSpecError` structured error type with `ErrorDetail`
- `ErrorCodeSet()` enumeration function

## Files affected
- `pkg/spec/entity.go` — extended field types + FieldType additions
- `pkg/spec/frontend.go` — CalendarSpec, ApprovalInboxSpec, NotificationCenterSpec, ListingSpec
- `pkg/spec/resources.go` — VisualSpecKindSpec, RendererSpec, PersistBackendSpec, SlotDecl, SlotContract
- `pkg/spec/errors.go` — new file: error code constants + FormSpecError type

## References
- docs/spec/frontend/02-visual-spec-kind.md (§1–§6)
- docs/spec/frontend/03-renderer-kind.md (§1–§5)
- docs/spec/backend/04-persist-backend.md (§1–§7)
- docs/spec/backend/05-field-types.md (§1–§5)
- docs/spec/backend/02-core-extended.md (§17–§19)
- docs/spec/frontend/06-page-kinds.md (§5 Calendar, §10 Listing, §11 ApprovalInbox, §12 NotificationCenter)
- docs/spec/backend/error-glossary.yaml

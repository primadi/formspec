# Pkg/Spec: Complete All Backend Kind Contract Types

**Date:** 2026-07-19
**Plan:** `docs/plan/todo.md` — Fase 1.1 (items 1.1.1 through 1.1.10)

## Changes

Added all missing backend kind structs to `pkg/spec/resources.go` and updated
dependent code in `internal/app/resolve.go`:

### New structs
- **`MigrationSpec`** — DDL-only migration with `ddl` and `module` (01-core-basic.md §4)
- **`WorkflowSpec`** — approval workflow with `entity`, `on.transition`, `steps[]`, `on_reject`, `escalation` (02-core-extended.md §2)
- **`ApiSpec`** — external surface override with `rest.base_path/version/disable` and `grpc.enabled/package` (02-core-extended.md §12)
- **`WebhookSpec`** — verified inbound endpoint with `for`, `method`, `path`, `auth.strategy/signature`, `idempotent` (02-core-extended.md §4)
- **`IntegratorSpec`** — cross-module bridge with `listen.resource/event`, `call.resource/action`, `compensate` (02-core-extended.md §5)
- **`MockupSpec`** — simulated connector with `for`, `config_ref` (02-core-extended.md §8)
- **`KindDefinitionSpec`** — CRD-like kind extension with `group`, `version`, `schema`, `handler`, `scope` (platform/03-kind-system.md §2)

### Updated structs
- **`SubscriptionSpec`** — added Tier 2 fields: `store`, `retention`, `position`, `max_retry`, `dead_letter`, `filter`, `transform`, `delivery` channel (02-core-extended.md §3)
- **`ConfigSpec`** — replaced `map[string]any` with structured `ConfigKey` type (`type`, `default`, `secret` per 01-core-basic.md §10)
- **`MenuSpec`/`MenuItem`** — added `MenuSpec` wrapper with `mode` (module/custom) and `items`; `MenuItem` updated with `When` (FormSpecExpr) (platform/02-workspace-app-module.md §4)

### Type changes
- `AppSpec.Menu` changed from `[]MenuItem` to `*MenuSpec`
- `ModuleSpec.Menu` changed from `[]MenuItem` to `*MenuSpec`
- Added `menuItemsOrEmpty()` helper in `internal/app/resolve.go`

### Supporting types
- `WorkflowTrigger`, `WorkflowTransitionRef`, `WorkflowStep`, `StepEscalation`, `WorkflowReject`, `WorkflowEscalation`
- `ApiRESTConfig`, `ApiGRPCConfig`
- `WebhookAuth`, `WebhookSigConfig`, `WebhookKeyRef`
- `IntegratorListen`, `IntegratorCall`
- `ConfigKey`
- `SubDeliveryDecl`
- `MenuSpec`

## Files affected
- `pkg/spec/resources.go` — all new/updated structs
- `internal/app/resolve.go` — updated Menu field access (as.Menu → menuItemsOrEmpty)

## References
- docs/spec/backend/01-core-basic.md (§4 Migration, §10 Config)
- docs/spec/backend/02-core-extended.md (§2 Workflow, §3 Subscription, §4 Webhook, §5 Integrator, §8 Mockup, §12 Api)
- docs/spec/platform/02-workspace-app-module.md (§4 Menu, §6 Validation)
- docs/spec/platform/03-kind-system.md (§2 KindDefinition)

## ⚠️ Reversed 2026-07-20
`MenuSpec` wrapper dan `MenuItem.Order` dihapus — kembali ke `[]MenuItem`
langsung untuk `App.spec.menu` dan `Module.spec.menu`. Lihat
`docs/changelog/2026-07-20-001-hapus-menuSpec-order.md`.

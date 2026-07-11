# Forma Implementation Progress

**Last Updated:** 2026-07-10  
**Total Tests:** ~210 (all passing)

> Checklist implementasi Forma framework. Setiap task ditandai saat selesai.

---

## Fase 0 — Plane Protocol & YAML Pipeline

Foundation phase: two-stage YAML pipeline (register → deploy) replacing direct filesystem loading.

### 0.1 Spec Updates

| # | Task | Status |
|---|---|---|
| 0.1.1 | Update `06-plane-protocol.md`: YAML Registration Pipeline §0, ETag conditional pull, hash optimization, dev mode, evidence types | ✅ |
| 0.1.2 | Update `02-core-basic.md` §6: two-stage pipeline, dev workflow, architectural rule | ✅ |
| 0.1.3 | Create changelog entry for v0.2.0 plane protocol | ✅ |

### 0.2 Control Plane — Artifact & Deployment API (`internal/artifact/`, `internal/control/`)

| # | Task | Status |
|---|---|---|
| 0.2.1 | `internal/artifact/artifact.go` — `Artifact`, `ArtifactEnvelope`, `FileManifest` types, sha256 computation | ⏳ |
| 0.2.2 | `internal/artifact/signing.go` — Ed25519 signing, envelope creation, verification | ⏳ |
| 0.2.3 | `internal/artifact/store.go` — DB-backed artifact CRUD with hash indexing (`forma_control.artifacts`, `deployments`, `evidence` tables) | ⏳ |
| 0.2.4 | `internal/control/register.go` — `POST /v1/artifacts`: receive YAML, validate, hash, sign, store | ⏳ |
| 0.2.5 | `internal/control/snapshot.go` — `GET /v1/snapshot`: build signed snapshot, ETag support, 304 handling | ⏳ |
| 0.2.6 | `internal/control/evidence.go` — `POST /v1/evidence`: receive deploy_status, health; append-only store | ⏳ |
| 0.2.7 | `internal/control/poll.go` — `POST /v1/poll` (dev-only): trigger immediate pull | ⏳ |
| 0.2.8 | `internal/control/server.go` — HTTP/gRPC server, middleware, routing | ⏳ |
| 0.2.9 | `cmd/forma-control/main.go` — Wire all components, `--dev` flag, SQLite control DB | ⏳ |

### 0.3 Resource Plane — Pull-Based Deployment (`internal/resource/`)

| # | Task | Status |
|---|---|---|
| 0.3.1 | `internal/resource/snapshot.go` — Snapshot fetcher, ETag tracking, version management | ⏳ |
| 0.3.2 | `internal/resource/artifact.go` — Artifact fetcher, verifier (signature chain), loader | ⏳ |
| 0.3.3 | `internal/resource/deployer.go` — Convergence engine: diff → hash compare → fetch → load → emit evidence | ⏳ |
| 0.3.4 | `internal/resource/evidence.go` — Evidence buffer (disk-backed), sender, retry | ⏳ |
| 0.3.5 | `internal/resource/local.go` — Local deployment manifest read/write (`deployment_manifest.json`) | ⏳ |
| 0.3.6 | `internal/resource/dev.go` — Dev mode: `POST /v1/poll` handler, 10s poll interval | ⏳ |

### 0.4 Modify Existing Code

| # | Task | Status |
|---|---|---|
| 0.4.1 | `internal/manifest/loader.go` — Add `LoadFromBytes()` for loading from artifact envelope | ⏳ |
| 0.4.2 | `internal/entity/registry.go` — Add `LoadFromArtifact()`, track deployment state (artifact_id, sha256, version) | ⏳ |
| 0.4.3 | `internal/entity/registry.go` — Replace `LoadEntities()` direct filesystem loading with artifact-based loading | ⏳ |
| 0.4.4 | `cmd/forma-resource/main.go` — Replace `--spec` with `--control-url`, implement deploy loop | ⏳ |
| 0.4.5 | `cmd/forma-dev-init/main.go` — Route through Control Plane registration API | ⏳ |
| 0.4.6 | `cmd/forma-entity-sync/main.go` — Mark obsolete; rewrite as `forma apply` or remove | ⏳ |

### 0.5 Dev Workflow

| # | Task | Status |
|---|---|---|
| 0.5.1 | `Makefile` — `make dev` starts both planes + watcher | ⏳ |
| 0.5.2 | `forma apply` CLI (in `cmd/forma-apply/`) — upload YAML to Control, `--watch` for hot-reload | ⏳ |
| 0.5.3 | Integration test: `forma apply` → Control receives → Resource pulls → deploys → API works | ⏳ |

### 0.6 Testing & Verification

| # | Task | Status |
|---|---|---|
| 0.6.1 | Unit tests: artifact signing/verification, snapshot generation | ⏳ |
| 0.6.2 | Unit tests: evidence buffering + flush | ⏳ |
| 0.6.3 | Integration test: hash optimization (deploy same artifact twice = no-op) | ⏳ |
| 0.6.4 | Integration test: dev mode hot-reload (file change → auto-register → auto-deploy) | ⏳ |

---

## Fase 1 — Core Runtime (Entity Engine & API)

### 1.1 Database Layer (`internal/db/`)

| # | Task | Status | Tests |
|---|---|---|---|
| 1.1.1 | DB Interface + Factory (`interface.go`, `db.go`) | ✅ | 5 |
| 1.1.2 | DSN Config Parser (`config.go`) | ✅ | 9 |
| 1.1.3 | SQLite Driver (`sqlite_db.go`) — WAL, FK, busy_timeout | ✅ | 7 |
| 1.1.4 | PostgreSQL Driver (`postgres_db.go`) — pgx/stdlib, pool | ✅ | — |
| 1.1.5 | Entity → DDL Generator (`ddl.go`) — dialect-aware, child tables | ✅ | 7 |
| 1.1.6 | Schema Migration Runner (`migrate.go`) — idempotent, checksum | ✅ | 7 |
| 1.1.7 | CRUD Query Builder (`crud.go`) — tenant isolation, CAS, pagination | ✅ | 8 |
| 1.1.8 | Child Storage (`child.go`) — JSONB inline + table mode | ✅ | 6 |
| 1.1.9 | Natural Key Counter (`counter.go`) — yearly/monthly/daily/never | ✅ | 8 |
| 1.1.10 | Idempotency Store (`idempotency.go`) — TryClaim/Complete/Fail | ✅ | 8 |
| 1.1.11 | Outbox Table (`outbox.go`) — at-least-once, exponential backoff | ✅ | 10 |
| — | Audit Logger (`audit.go`) — write-once audit trail | ✅ | — |
| — | Outbox Worker (`outbox_worker.go`) — background poll + deliver | ✅ | — |
| — | DB Dev Init (`cmd/forma-dev-init/`) — bootstrap SQLite sample | ✅ | — |

### 1.2 Entity Registry (`internal/entity/`)

| # | Task | Status | Tests |
|---|---|---|---|
| 1.2.1 | Registry core (`registry.go`) — LoadEntities, SyncSchema, GetEntityStore | ✅ | — |
| 1.2.2 | Info & Query Helpers — ListEntities, GetEntity, GetEntitiesByCharacteristic | ✅ | — |
| 1.2.3 | CLI Entity Sync (`cmd/forma-entity-sync/`) — load → register → sync | ✅ | — |
| 1.2.4 | Unit tests — load, filter, sync, CRUD, General-Ledger example | ✅ | 9 |
| 1.2.5 | Spec update: `ValidationRule.UnmarshalYAML` shorthand | ✅ | — |

### 1.3 REST API (`internal/api/`)

| # | Task | Status | Tests |
|---|---|---|---|
| 1.3.1 | Spec: deny-by-default exposure (D49), multi-protocol router (D50) | ✅ | — |
| 1.3.2 | Route Descriptor + Generator (`descriptor.go`, `generator.go`) | ✅ | 5 |
| 1.3.3 | Handler Factory (`handler.go`) — 5 auto-handlers | ✅ | — |
| 1.3.4 | Middleware Stack (`middleware.go`) — Tenant, Auth, RequestID, CORS, Recovery, Log | ✅ | — |
| 1.3.5 | Router Builder (`router.go`) — chi, `/{workspace}/api/v1/...` | ✅ | 2 |
| 1.3.6 | CLI Serve (`cmd/forma-serve/`) — load → sync → routes → serve HTTP | ✅ | — |
| 1.3.7 | Response Envelopes — SingleResponse, ListResponse, ErrorResponse | ✅ | 2 |

### 1.4 Auth & Tenant Middleware

| # | Task | Status | Tests |
|---|---|---|---|
| 1.4.1 | JWT/Session auth (prod mode) | ✅ | 8 |
| 1.4.2 | Token validation + identity resolution | ✅ | 8 |
| 1.4.3 | Permission enforcement middleware per `required_permission` | ✅ | 7 |
| 1.4.4 | Cross-tenant isolation at API level | ✅ | 3 |

**Files created:**
- `internal/auth/auth.go` — `Identity` struct, `TokenValidator` interface, `HasPermission()` (wildcard + exact)
- `internal/auth/jwt.go` — `JWTValidator` (HS256/RS256/ES256)
- `internal/auth/dev.go` — `DevValidator` (synthetic identity with `*` perm)
- `internal/auth/auth_test.go` — 19 test cases (permission matching, authentication)
- `internal/auth/jwt_test.go` — 8 test cases (valid/invalid/expired/wrong issuer/wrong key)

**Files modified:**
- `internal/api/descriptor.go` — `RouteDescriptor.RequiredPermission` field
- `internal/api/handler.go` — `IdentityFromContext`, `WithIdentity`, updated `tenantFromContext`/`userFromContext` to prefer identity
- `internal/api/middleware.go` — `SetAuthValidator`, refactored `AuthMiddleware` (dev/prod), `RequirePermission` factory
- `internal/api/generator.go` — populate `RequiredPermission` as `{module}.{plural}.{action}` per route
- `internal/api/router.go` — wire `RequirePermission` via chi `r.With()` per-route
- `internal/api/api_test.go` — 14 new test cases (permission middleware, identity helpers, auth middleware)
- `cmd/forma-serve/main.go` — `--prod`/`--jwt-secret`/`--jwt-issuer` flags, `SetAuthValidator` init
- `go.mod` — added `github.com/golang-jwt/jwt/v5`

**Design decisions for 1.4:**
- **Spec = data, not code** — permission strings and auth config live in YAML manifests, loaded at runtime. No recompile needed for spec changes.
- **Atomic-swap ready** — `RouterBuilder.BuildHTTP()` can be called again and swapped via `sync.RWMutex` for hot-reload. Middleware is stateless.
- **Dev mode default** — `NewDevValidator()` returns `Identity{UserID:"developer", Permissions:["*"]}` — all requests pass through.
- **Prod mode** — `--prod --jwt-secret <key>` enables JWT validation. Claims: `sub`, `ws`, `perms`, `roles`, `iss`, `exp`.
- **Permission model** — `{module}.{plural}.{action}` for standard CRUD. Wildcard: `*` (everything), `module.entity.*` (all actions on entity). `public` = anonymous.
- **Cross-tenant isolation** — workspace slug extracted from URL path → tenant ID in context → all DB queries scoped. Identity's workspace overrides URL tenant.

### 1.5 Permission Enforcement

| # | Task | Status | Tests |
|---|---|---|---|
| 1.5.1 | Permission data model — `PermissionEntry`, `UsesEntry`, `ModuleFootprint` (`internal/permission/`) | ✅ | 9 |
| 1.5.2 | PermissionRegistry — register, aggregate, query per module | ✅ | 4 |
| 1.5.3 | Permission string + uses validator — format check, auto-prefix, cross-module detection | ✅ | 7 |
| 1.5.4 | `ctx.auth.has()` — integrate `Identity.HasPermission()` into auth package | ✅ | 2 |
| 1.5.5 | Hook into entity registry — auto-register on `LoadEntities()` | ✅ | — |
| 1.5.6 | Route descriptor enrichment — custom action `RequiredPermission` from YAML | ✅ | — |
| 1.5.7 | `UsesEnforcement` middleware stub — error codes (`USES_VIOLATION`, `CONFIG_ACCESS_DENIED`) | ✅ | — |
| 1.5.8 | `forma-serve` — print module footprint, `--strict` flag | ✅ | — |
| 1.5.9 | Tests — validation, registry, footprint, middleware | ✅ | 22 |

**Files created:**
- `internal/permission/permission.go` — `PermissionEntry`, `UsesEntry`, `ModuleFootprint`, `AuthChecker`, `ValidatePermissionFormat`, `AutoPrefixPermission`, `ParseResourceTarget`
- `internal/permission/registry.go` — `Registry` (thread-safe, module→footprint), `RegisterAction`, `GetModuleFootprint`, `FindPermission`
- `internal/permission/validator.go` — `ValidateUses`, `ValidateAction`, `BuildUsesEntry`, `ValidateEntitySpec`
- `internal/permission/permission_test.go` — 22 test cases

**Files modified:**
- `internal/auth/auth.go` — `PermissionChecker` interface, `SetPermissionChecker`, `CtxAuthHas`, `defaultPermissionChecker`
- `internal/entity/registry.go` — `permRegistry` field, auto-register permissions in `LoadEntities()`, `GetPermissionRegistry()`, `GetModuleFootprint()`
- `internal/api/generator.go` — `GenerateCustomActionRoutes()`, `mergeRoutes()`, `isStandardAction()`
- `internal/api/router.go` — `BuildRoutes()` includes custom actions, `registerRoute()` handles `"custom"` handler type
- `internal/api/middleware.go` — `SetStrictMode()`, `UsesEnforcement()` stub, `writeUsesViolation()`, `writeConfigAccessDenied()`, `writeKvstoreAccessDenied()`
- `cmd/forma-serve/main.go` — `--strict` flag, permission registry wiring, footprint display

**Deferred to Fase 2:**
- Starlark runtime intercept `ctx.*` calls for `uses` enforcement
- `impl/**/*.go` honesty scan for undeclared `uses`
- Auto-suspend on `USES_VIOLATION` (needs outbox + incident audit from 3.x)

**Deferred to Fase 6:**
- `forma module install` consent UI — display `ModuleFootprint` at install time
- `forma validate` — scan scripts for undeclared uses (requires Starlark)

### 1.6 Validation Engine

| # | Task | Status | Tests |
|---|---|---|---|
| 1.6.1 | Field validation from YAML rules | ✅ | 5 |
| 1.6.2 | Custom action validation | ✅ | 5 |
| 1.6.3 | Cross-field validation | ✅ | 4 |

**Existing validators (in `internal/db/crud.go`):** `email`, `pattern`, `min_length`, `max_length`, `min`, `max`, `required`, `default`

**New validators added (1.6):**
- `positive` — numeric value must be > 0
- `url` — URL format validation (regex)
- `precision` — maximum decimal places (accounting-grade)
- `future` — datetime must be in the future
- `past` — datetime must be in the past
- `min_items` / `max_items` — array length validation (for JSON fields)
- `after:<field>` / `before:<field>` — cross-field date comparison

**Files created:**
- `internal/validation/validator.go` — `ValidateCrossField`, `ValidateActionParams`, cross-field date comparison, action param rule engine
- `internal/validation/validation_test.go` — 9 test cases

**Files modified:**
- `internal/db/crud.go` — `positive`, `url`, `precision`, `future`, `past`, `min_items`, `max_items` validators; cross-field wiring in `validateFieldRules`; `urlRegex`, `timeNow()`, `parseDateTime()` helpers
- `internal/db/crud_test.go` — 5 new test cases (positive, url, precision, future, min_items/max_items)
- `internal/api/handler.go` — `HandleCustomAction()` with param validation, `writeValidationErrors()`
- `internal/api/router.go` — custom action routes resolve entity spec from registry, wire `HandleCustomAction`

**Deferred:**
- `exists:<resource>` full DB query → Fase 2 (needs EntityStore reference in handler)
- Starlark inline validator escape hatch (`script` rule) → Fase 2

### 1.7 Core Basic Conformance Gaps

> Identified in `docs/audit/todo-spec-*.md` (plan-vs-spec audit): Core Basic (`02-core-basic.md` Part VII Conformance) declares these MUST, but no task anywhere in this file covers them. Tracked here so they aren't dropped.

| # | Task | Status | Spec ref |
|---|---|---|---|
| 1.7.1 | `forma.core` resource set — entities `workspace`, `user`, `app-membership`, `role`, `role-assignment`, `api-key`, `session`, `job`, `setting` (`audit-log` and `failed-event` partially covered by 1.1's Audit Logger / Outbox, but not exposed as `forma.core` resources); services `health`, `metrics` | ⏳ | §22, Conformance §10 |
| 1.7.2 | Cross-app grant verification — runtime MUST verify a signed grant before routing cross-app calls; ungranted → 404 | ⏳ | §15.3, Conformance §7 |
| 1.7.3 | Category → PostgreSQL schema mapping + cross-category join guard (`CROSS_CATEGORY_ACCESS_DENIED`) | ⏳ | §19, §16, Conformance §8 |
| 1.7.4 | `kind: Migration` — custom DDL kind with runtime DML rejection (distinct from the 1.1.6 structural migration runner) | ⏳ | §20, Conformance §9 |
| 1.7.5 | Loading/serving `kind: Service` (stateless compute), `kind: Config` (`ctx.config.get`), `kind: App` root manifest | ⏳ | §4.2, §4.4, §7, Conformance §2 |
| 1.7.6 | Workspace provisioning lifecycle — `create → provisioning → seed default roles + reference seeds → active` (emits `workspace.activated`), `suspend ⇄ reactivate`, `terminate`, `ctx.tenant.config()` | ⏳ | §21 |

---

## Fase 2 — Business Logic Engine (✅ Core Complete)

| # | Task | Status |
|---|---|---|
| 2.1 | Starlark runtime integration (`internal/starlark/` — executor, resource API, ctx) | ✅ |
| 2.2 | ScriptRef execution from manifest action | ✅ |
| 2.3 | Native (Go) handler registration | ✅ |
| 2.4 | Sidecar handler protocol | ✅ (stub) |
| 2.5 | State Machine engine — transitions + guards | ✅ |
| 2.6 | `ctx.*` primitives (db, cache, lock, queue, pubsub, storage) — stubs for queue/pubsub/storage | ✅ |

### 2.7 Fase-1 Deferred Fixes (Completed in Fase 2)

| # | Task | Status |
|---|---|---|
| 2.7.1 | `exists:<resource>` real DB query (was stub) | ✅ |
| 2.7.2 | after:/before: colon shorthand parsing | ✅ |
| 2.7.3 | Action condition evaluation (Starlark-based, spec §13) | ✅ |
| 2.7.4 | Standard CRUD permission auto-registration | ✅ |

### 2.8 New Files Created

| File | Purpose |
|---|---|
| `internal/action/dispatcher.go` | Central action dispatcher — routes `impl` type → executor |
| `internal/action/dispatcher_test.go` | Dispatcher unit tests (9 tests) |
| `internal/action/native.go` | Native Go handler registry + executor |
| `internal/action/native_test.go` | Native executor tests (4 tests) |
| `internal/action/sidecar.go` | Sidecar stub executor |
| `internal/action/script.go` | Script/ScriptRef executor (Starlark integration) |
| `internal/action/conditions.go` | Condition evaluator (spec §13 gates) |
| `internal/action/e2e_test.go` | E2E integration tests (8 test cases) |
| `internal/starlark/resource.go` | Starlark `resource` object API (set/save/call/load/field) |
| `internal/starlark/context.go` | Starlark `ctx` object API (tenant/user/auth/log/config/next_key) |
| `internal/starlark/executor.go` | Script execution engine (load .star, call execute()) |
| `internal/entity/state_machine.go` | State machine engine — transitions, guards, wildcard |
| `internal/entity/state_machine_test.go` | State machine tests (13 tests) |

### 2.9 Files Modified

| File | Change |
|---|---|
| `internal/api/handler.go` | `HandleCustomAction` → dispatcher (was 501), + conditions + dispatch |
| `internal/api/router.go` | Added `SetDispatcher()` to RouterBuilder |
| `internal/manifest/loader.go` | Added `RawSpecToServiceSpec()`, `RawSpecToConfigSpec()` |
| `internal/validation/validator.go` | `exists:<resource>` stub → real DB lookup via `EntityLookup` |
| `cmd/forma-serve/main.go` | Wire dispatcher, script/native/sidecar executors, exists lookup, script handlers |

---

## Fase 3 — Events & Async

| # | Task | Status |
|---|---|---|
| 3.1 | Event dispatch from EntityStore (Insert/Update/Delete hooks) | ⏳ |
| 3.2 | `deliver` declarative consumers | ⏳ |
| 3.3 | `kind: Subscription` runtime (D35) | ⏳ |
| 3.4 | Outbox → Event Bus bridge (outbox table already exists from 1.1.11) | ⏳ |
| 3.5 | Realtime WebSocket push | ⏳ |

---

## Fase 4 — Frontend Renderer

> **Design doc:** `docs/implementation/frontend-renderer.md` (komprehensif — arsitektur, Meta API, derivation engine, kind renderers, plan). Checklist di bawah mengikuti fase B/F1–F6 dokumen tersebut.

### 4.B Backend Prasyarat (Go) — ✅ Selesai 2026-07-11

| # | Task | Status |
|---|---|---|
| 4.B1 | `internal/manifest` — generic `RawSpecTo[T]()`; struct 12 kinds dilengkapi sesuai spec 05 (tabs, widget/compute, search/realtime, wizard search_select, kanban card penuh, timeline bind/display, print output/body, `MenuSpec.when`, `Action.ui`) + `ThemeSpec` baru | ✅ |
| 4.B2 | `internal/ui` — UIRegistry (parse + index 12 kinds by name) + cross-validation: entity/field refs (dot-path lewat relation & child), action refs (builtin view/edit/delete/export/print), route unik, blocks⊕tabs, kanban columns vs enum, dashboard/wizard refs. 10 test | ✅ |
| 4.B3 | Meta API — `GET /_meta/ui` (bundle permission-filtered per caller, ETag/304), `GET /_meta/me`, `GET /_meta/entities/{module}/{name}`; entity schema: label_field heuristik, lifecycle §1.7, permission per action | ✅ |
| 4.B4 | `sort` + `filters` query params ke `HandleList` — `?sort=-f&f=v&f[op]=v`, op eq/neq/gt/gte/lt/lte/like/in/nin; hanya field ber-index/unique/natural_key + kolom normatif; unknown → 422. Fix `in`/`nin` multi-placeholder di crud | ✅ |
| 4.B5 | Static SPA serving — `Config.WebDir`/`--web-dir`, mount di `/{ws}/_admin` + `/{ws}/app` dengan index.html fallback | ✅ |

> Bug diperbaiki dalam fase ini: (1) DDL nama tabel ber-hyphen → `sanitizeIdent` (`internal/db/ddl.go`); (2) error validasi Insert/Update dipetakan 500 → kini 422 `VALIDATION_ERROR` / 409 `CONFLICT` (`writeStoreError`).

### 4.F Frontend Renderer (`web/`)

| # | Task | Status |
|---|---|---|
| 4.F1.1 | API client (ky) — envelope unwrap, typed errors, CAS version, list params | ⏳ |
| 4.F1.2 | Meta client + zustand stores (session, meta, prefs) + TS types mirror `pkg/spec/frontend.go` | ⏳ |
| 4.F1.3 | FormaExpr interpreter (lexer + Pratt parser + evaluator, TS murni) + table-driven tests | ⏳ |
| 4.F1.4 | Permission gate — port `Identity.HasPermission` (wildcard) + test paritas dengan Go | ⏳ |
| 4.F2.1 | App shell — layout, topbar, breadcrumb, 403/404, login token screen | ⏳ |
| 4.F2.2 | Sidebar menu — merge derived module menus + `kind: Menu`, permission-filtered, `when:` expr | ⏳ |
| 4.F2.3 | Router dinamis dari meta bundle (Page routes + derived CRUD routes + wizard/kanban/timeline) | ⏳ |
| 4.F2.4 | OverlayHost — modal/drawer via query string (`?action=edit&id=`), design-time locking §1.6 | ⏳ |
| 4.F3.1 | Derivation engine — entity schema → default Table/Form/Detail/Menu + override resolution | ⏳ |
| 4.F3.2 | Table renderer — TanStack + server-side pagination/sort/filter/search, row actions + confirm | ⏳ |
| 4.F3.3 | Form renderer — react-hook-form + zod dari rules, 3 mode render, auto-save, CAS 409 handling | ⏳ |
| 4.F3.4 | Detail page + state machine transition buttons + lifecycle patterns §1.7 | ⏳ |
| 4.F3.5 | Field widget library — text/number/date/enum/relation-picker/child-grid/computed/badge | ⏳ |
| 4.F3.6 | ⭐ **Milestone D10**: app tanpa manifest frontend → `/_admin` CRUD lengkap (PocketBase benchmark) | ⏳ |
| 4.F4.1 | Page renderer — blocks + tabs variant + layout columns | ⏳ |
| 4.F4.2 | Dashboard + Widget (stat, chart line/bar dari summary entities) | ⏳ |
| 4.F4.3 | Wizard — stepper, `?step=N`, `depends_on` filter chain, final action submit | ⏳ |
| 4.F4.4 | Kanban — dnd-kit, optimistic status PATCH, 409 snap-back, state machine guard | ⏳ |
| 4.F4.5 | Timeline — infinite scroll, date grouping, read-only enforcement | ⏳ |
| 4.F4.6 | ⭐ **Milestone**: seluruh YAML UI `examples/Order-to-Cash` ter-render sesuai manifest | ⏳ |
| 4.F5.1 | Report — param form, group/totals, CSV export client-side | ⏳ |
| 4.F5.2 | Print `format: html` (window.print + `@page` CSS) | ⏳ |
| 4.F5.3 | Theme tokens → CSS custom properties | ⏳ |
| 4.F6 | Escape hatch — component contract (`mount/unmount`, `forma` client, `needs:`), headless form, `<FormaPage/>` embed | ⏳ |
| **4.7** | **Page-Scoped Routing (BFF)** — `/{ws}/{app}/{page}/api/v1/{module}/{plural}` — ⚠️ belum direkonsiliasi dgn §16 (IMP-3) | ⏳ |

**Blocked oleh fase lain:** realtime transport (Fase 3.5 — subscription manager didesain swap-able, v1 = polling/refetch), export async + download tray (job runtime Core §17), Print pdf/thermal/dotmatrix (server pipeline), codegen TS resmi (Fase 6.4).

### 4.7 Page-Scoped Routing Design

**Concept (Backend-for-Frontend at page level):**

```
Browser → SPA shell → page-gated API proxy → forma-serve handler
         ↑                                     ↑
    React Router                        EntityStore action
    PageGate check                      Resource permission (verification)
    Render page / 403                   Audit log granular
```

**URL structure:**

| Surface | URL Pattern | Auth |
|---|---|---|
| **Admin panel** (internal) | `/{ws}/{app}/{page}/api/v1/{module}/{plural}` | Page gate + resource permission |
| **Public API** (external) | `/{ws}/api/v1/{module}/{plural}` | Resource permission only |
| **Meta API** | `/_meta/ui` | Read-only, same-origin |

**Page footprint materialization (D38):**

```
Page "Order Management" composed from:
  └─ Table: orders        → orders.list
  └─ Form: order-edit     → orders.find, orders.update
  └─ Action: checkout     → orders.checkout
  └─ Widget: revenue      → gl-balances.view

Admin grant: "role kasir → Page Order Management"
  → auto-materialize 5 resource permissions
  → admin UX: 1 checkbox, backend: 5 permissions
```

**Why this is good:**

| Benefit | Mechanism |
|---|---|
| Consistent latency | Browser never hits backend directly; all through SPA → backend proxy |
| Single auth boundary | Page gate = one permission check per page entry |
| No backend leak | Backend endpoints hidden behind page namespace |
| Clean separation | Public API (`expose`) deliberate; admin panel auto-generated |
| Audit granularity | Resource-level enforcement preserves per-action audit trail |

---

## Fase 5 — Control Plane Governance

> **Note:** Plane Protocol implementation (artifact API, snapshot, evidence, deploy) moved to **Fase 0** (foundation). This fase covers pure governance features only.

| # | Task | Status |
|---|---|---|
| 5.1 | `kind: Environment` — validation, lifecycle | ⏳ |
| 5.2 | `kind: Policy` — OPA/Rego integration, `forma-ctl policy test` | ⏳ |
| 5.3 | Transparency log — Merkle tree, inclusion proofs (D30) | ⏳ |
| 5.4 | Two key classes: owner keys (public-only storage) + platform keys (HSM/KMS) | ⏳ |
| 5.5 | Contract model — grants, consents, licenses (D25, D27, D30, D31) | ⏳ |
| 5.6 | Deployment approval chains — multi-signer, consent delta | ⏳ |
| 5.7 | `forma-ctl` emergency CLI — freeze, revoke sessions, key rotate | ⏳ |

---

## Fase 6 — CLI & DX

| # | Task | Status |
|---|---|---|
| 6.1 | `forma validate` | ⏳ |
| 6.2 | `forma new <kind>` scaffold | ⏳ |
| 6.3 | `forma dev` (hot-reload) | ⏳ |
| 6.4 | `forma generate` (TypeScript types) | ⏳ |
| 6.5 | `forma migrate` | ⏳ |
| 6.6 | `forma backup create\|inspect\|restore` (D41) | ⏳ |
| 6.7 | LSP / JSON Schema per kind (D34) | ⏳ |

---

## Infrastructure

| # | Task | Status | Notes |
|---|---|---|---|
| INF-1 | Devcontainer: Go, MinIO, Valkey, Mailpit | ✅ | `compose.yaml` |
| INF-2 | PostgreSQL in devcontainer | ❌ Removed | SQLite for dev; PG for integration tests only |
| INF-3 | DBCode SQLite viewer | ✅ | Via `.forma/data.db` |
| INF-4 | Docs: specs (01–11) | ✅ | `docs/spec/` |
| INF-5 | Docs: implementation (db-layer, api-layer) | ✅ | `docs/implementation/` |
| INF-6 | Docs: plan tracking | ✅ | This file |

---

## Skipped / Deferred

| Item | Reason | Revisit |
|---|---|---|
| PostgreSQL in devcontainer | SQLite sufficient for dev; PG for integration tests | Fase 5 |
| Smart Internal Dispatch (1.3.8) | IMP-5: same-process = direct call; cross-process = configured adapter. Registry.IsLocal() needed first | Fase 2 |
| Page-Scoped Routing (4.7) | BFF pattern: `/{ws}/{app}/{page}/api/v1/...`. Browser never hits backend directly | Fase 4 |
| gRPC / WebSocket adapters | REST is priority; descriptors already designed | Fase 3 |

---

## Key Implementation Notes (from 1.4 implementation discussions)

> Implementation notes (IMP-1..IMP-8) are distinct from canonical spec decisions (D1–D50 in `docs/spec/11-reference.md`).
> They record design rationale from implementation discussions but are not ratified spec decisions.

| ID | Note | Impact |
|---|---|---|
| IMP-1 | **Spec = data, not code.** Permission + auth config in YAML, loaded at runtime. Hot-reload via atomic `http.Handler` swap. | No recompile; ~0ms downtime on spec change |
| IMP-2 | **Dual namespace routing.** Internal: `/_/api/v1/...` (auto, all entities). External: `/{ws}/api/v1/...` (only if `expose`). ⚠️ NOT IMPLEMENTED — contradicts D49 deny-by-default; proposed extension only. | Admin panel works without `expose`; public API is deliberate |
| IMP-3 | **Page-scoped routing (BFF).** Browser → SPA → `/{ws}/{app}/{page}/api/v1/...`. ⚠️ NOT IMPLEMENTED — Fase 4 proposal; conflicts with normative REST path §16. | Consistent latency; single auth boundary; no endpoint leak |
| IMP-4 | **Page → permission materialization (D38).** Admin grants page access; framework derives resource permissions from page composition. | Admin UX: 1 checkbox; backend: N permissions auto-granted |
| IMP-5 | **Smart internal dispatch (D50).** Same-process = direct Go call; cross-process = configured protocol adapter (REST required, gRPC recommended). | Zero serialization overhead for local calls |
| IMP-6 | **URL transparency.** No obfuscation. Readable URLs = debuggable, AI-friendly, diff-able. Security at auth + permission layer. | Consistent with D24 (manifests never encrypted) |
| IMP-7 | **Starlark not encrypted.** IP protection via binary handlers (`compiled`/`native`/WASM), not via script encryption. | Consent + audit + AI remain functional |
| IMP-8 | **Three impl tiers.** Local script (Tier 1) → Local binary/WASM (Tier 2) → Cloud-hosted handler (Tier 3). ⚠️ NOT RECONCILED with Five Implementation Types (§10) + D46 trust tiers; needs re-grounding. | Progressive complexity; vendor chooses trade-off |

---

## Test Summary

| Package | Tests | Status |
|---|---|---|
| `internal/db` | 103 | ✅ |
| `internal/entity` | 22 | ✅ |
| `internal/api` | 21 | ✅ |
| `internal/action` | 17 | ✅ |
| `internal/permission` | 14 | ✅ |
| `internal/auth` | 5 | ✅ |
| `internal/validation` | 9 | ✅ |
| `internal/starlark` | 9 | ✅ |
| `internal/manifest` | 6 | ✅ |
| `pkg/spec` | 8 | ✅ |
| **Total** | **~214** | **✅ All passing** |

---

## Legend

| Symbol | Meaning |
|---|---|
| ✅ Done | Completed + tests passing |
| ⏳ Pending | Planned, not started |
| ❌ Removed | Deliberately removed from scope |

---

## Key Design Decisions (from Reference)

See `docs/spec/11-reference.md` for full list. Highlights relevant to implementation:

| ID | Decision | Impact |
|---|---|---|
| D3 | Two processes even in dev (`forma-control` + `forma-resource`) | `forma serve` must run both |
| D17 | Derived by default — API/admin/docs born from Entity manifest | Registry → auto-generate routes |
| D20 | Explicit permission model — every action declares `required_permission` + `uses` | Fase 1.5 |
| D29 | Workspace = the one and only multi-tenancy model | `/{workspace}/api/...` prefix |
| D32 | Idempotency + optimistic concurrency enforced by framework | 1.1.10 + CAS in 1.1.7 |
| D49 | API exposure deny-by-default via `spec.expose` | Generator skips unexposed entities |
| D50 | Multi-protocol router, workspace slug, internal dispatch | chi router in 1.3.5 |

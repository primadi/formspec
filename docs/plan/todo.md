# Master Plan: Forma Implementation

**Last Updated**: 2026-07-29  
**Status**: ✅ Fase 0 complete · ✅ Fase 1 (1.1–1.5) · ✅ Fase 2.1 · ✅ Fase 2.2 · ✅ Fase 5 (5.1–5.4) · ✅ Spec hot-reload  

> `⬜` not started · `✅` complete · `⏸️` deferred  

**Scope**: `forma dev` + `forma serve --mode=production` single-server.  
**Deferred**: Control Plane (`forma-ctl`), K8s Operator, Marketplace — untuk cloud phase berikutnya.  
**Sumber**: `docs/spec/backend/` (01–06), `docs/spec/frontend/` (01–08), `docs/spec/platform/` (01–10), `docs/cli-tools/` (01–05), `docs/renderers/jsonb-persist/` (01–04), `docs/renderers/shadcn-shell/` (01–04), `docs/ai/` (01–06).  
**Catatan status sumber**: seluruh spec masih **Draft** (jsonb-persist masih **Outline**; `platform/08` §3–§6 "target desain") — mismatch todo↔docs bisa berarti docs-nya yang perlu diperbaiki. Audit penuh todo vs docs terakhir: 2026-07-19.

---

## Fase 0: Documentation & Repo Foundation ✅ COMPLETE

| Item | Status |
|---|---|
| 0.1 Fix CLI doc numbering (01-dev, 02-cli, 03-generate, 04-ctl) | ✅ |
| 0.2 Fix Document → Entity in docs/spec/ | ✅ |
| 0.3 Repo restructure: `web/` → `renderers/web/` | ✅ |
| 0.4 Repo restructure: `internal/db/`+`datastore/` → `renderers/jsonbpersist/` | ✅ |
| 0.5 AI instructions + 3 skills (backend, frontend, cli) | ✅ |
| 0.6 Verify no `docs_old/` refs in `docs/` | ✅ |

---

## Fase 1: `pkg/spec/` — Complete All Go Contract Types

**Goal**: Every kind, field type, and validation rule from `docs/spec/` has a Go struct.

### 1.1 Missing backend kind structs ✅
- [x] 1.1.1 `MigrationSpec` — DDL-only migration (`kind` + `spec.ddl` + `spec.module`), reject DML  
- [x] 1.1.2 `WorkflowSpec` — approval workflow (`entity`, `on.transition`, `steps[]`, `on_reject`, `escalation`)  
- [x] 1.1.3 `ApiSpec` — external surface override (`rest.base_path`, `rest.version`, `rest.disable`, `grpc.*`)  
- [x] 1.1.4 `WebhookSpec` — verified inbound endpoint (`for`, `method`, `path`, `auth.strategy`, `auth.signature`, `idempotent`)  
- [x] 1.1.5 `IntegratorSpec` — cross-module bridge (`listen.resource`+`event`, `call.resource`+`action`, `compensate`)  
- [x] 1.1.6 `MockupSpec` — simulated connector (`for`, `config_ref`)  
- [x] 1.1.7 `KindDefinitionSpec` — CRD-like kind extension (`group`, `version`, `schema`, `handler`, `scope`)  
- [x] 1.1.8 Update `SubscriptionSpec` — add Tier 2 fields (`store`, `retention`, `position`, `max_retry`, `dead_letter`, `filter`, `transform`, `delivery` channel)  
- [x] 1.1.9 Update `ConfigSpec` — replace `map[string]any` with structured `ConfigKey` type (`type`, `default`, `secret` — per `01-core-basic.md` §10; `required` tidak ada di spec)  
- [x] 1.1.10 `MenuItem` — kontrak menu App/Module (`platform/02-workspace-app-module.md` §4: `[]MenuItem` langsung tanpa wrapper, array-index order, nesting max 3 level, node adopt/group/leaf, `when` FormaExpr) + validasi apply §6 (`module` di menu anggota `App.spec.modules`, `root_url` unik prefix `/app/`, `Form`/`Table` bukan target `view`)  

### 1.2 Missing meta-kind structs ✅
- [x] 1.2.1 `VisualSpecKindSpec` — declare new view type (`tier`, `schema`, `renderer_contract`, `accepts_slots`/`implements_slot`)  
- [x] 1.2.2 `RendererSpec` — concrete VisualSpecKind implementation (`implements`, `stack_family`, `trust_tier`)  
- [x] 1.2.3 `PersistBackendSpec` — storage seam declaration (`implements`, `trust_tier`)  

### 1.3 Missing frontend kind structs ✅
- [x] 1.3.1 `CalendarSpec` — calendar view (`entity`, `date_field`, `end_field`, `title_field`, `resource_field`, `color_field`, `views[]`, recurrence via RRULE)  
- [x] 1.3.2 `ApprovalInboxSpec` — pending approvals (`realtime`, `filters`, `search`)  
- [x] 1.3.3 `NotificationCenterSpec` — in-app notifications (`realtime`)  
- [x] 1.3.4 `ListingSpec` — public catalog (`entity`, `columns`, `filters`, `search`)  

### 1.4 Extended field type structs ✅
- [x] 1.4.1 `RateLimitSpec` — per-resource rate limit (`max`, `per`, `scope`, `strategy`: sliding_window/token_bucket)  
- [x] 1.4.2 `SecretRef` — `ctx.secrets` access declaration (`uses.secrets: [key, ...]`) via `UsesDecl.Secrets`  
- [x] 1.4.3 `FieldClassification` — governance label (`pii`|`financial`|`internal`)  
- [x] 1.4.4 `FieldPermission` — field-level `required_permission`  
- [x] 1.4.5 `FieldExclude` — per-surface field exclusion (`public_api`|`audit_log`|`webhook`|`ui` — per `05-field-types.md` §5.3)  
- [x] 1.4.6 `EncryptedField` — at-rest encryption marker (`encrypted: true`)  
- [x] 1.4.7 `MaskedField` — auto-mask in response/log (`masked: true`)  
- [x] 1.4.8 `BackdatePolicy` / `ForwardDatePolicy` — max days, override_permission (already existed in entity.go)  
- [x] 1.4.9 `TreeDecl` — self-referential hierarchy marker (`tree: true` on relation)  
- [x] 1.4.10 `SoftDeactivateDecl` — `is_active` + `deactivate`/`reactivate` action pattern  
- [x] 1.4.11 `StorageSpec` (file field) — `allowed_types`, `max_size_mb`, `max_count`, `visibility`, `signed_url_ttl`, `cdn`, `transform`  

### 1.5 Error glossary Go types ✅
- [x] 1.5.1 Go const/type mapping dari `error-glossary.yaml` (22 error codes → `FORMA.DOC.*`, `FORMA.TXN.*`, `FORMA.PERIOD.*`, `FORMA.EVENT.*`, `FORMA.SAGA.*`, `FORMA.REF.*`, `FORMA.PERSIST.*`, `FORMA.ARCHIVE.*`, `FORMA.VALIDATE.*`)  
- [x] 1.5.2 Observability error codes — `OBSERVABILITY_METRICS_DISABLED`, `OBSERVABILITY_DEBUG_FORBIDDEN`, `LOGS_FILTER_INVALID` (`09-observability.md` §8)  

---

## Fase 2: Engine Core — `forma dev` Reliability

**Goal**: Atomic operations, correct PK, complete filters, lifecycle enforcement — agar `forma dev` bisa diandalkan untuk testing.

**Progress**: 2.1 ✅ · 2.2 ✅ · 2.3 ✅ · 2.4 ✅ · 2.5 ✅ · 2.6–2.9 ⬜

### 2.1 Database integrity ✅
- [x] 2.1.1 Atomic mutation + outbox — wrap Entity INSERT/UPDATE/DELETE + outbox write dalam `BeginTx`/`Commit` (rollback on error). Terpenuhi untuk create/update HTTP (`InTx`) **dan** custom action (`HandleCustomAction` + `TxScope`, `renderers/jsonbpersist/txscope.go`) — satu transaksi request-scoped mencakup semua panggilan `resource.save()`/`.create()` (Starlark/native/sidecar via `X-Forma-Scope-Id`) dalam satu eksekusi action, join berdasar identitas store (bukan Module — multi-Module dalam satu Datastore fisik yang sama tetap atomik; lintas-Datastore genuinely berbeda → `ErrCrossStoreTx`). **Gap tersisa**: `RunAfterPhase` masih fire-and-forget (tidak rollback); SDK sidecar (`sdk/php`/`sdk/python`/`sdk/typescript`) belum mengirim `X-Forma-Scope-Id` (`01-architecture.md` §3, `runtimes/04-forma-sidecar.md` §4.3a).  
- [x] 2.1.2 Natural key counter in same transaction as Entity insert — UPSERT counter + INSERT dalam satu `Tx` (`generateNaturalKeys` menerima DB terikat-transaksi; `04-query-and-keys.md` §2)  
- [x] 2.1.3 UUID v7 PK — replace SQLite `INTEGER PRIMARY KEY AUTOINCREMENT` with UUID v7 generated at app layer (`NewUUIDv7`, kedua driver; child table PK juga ikut)  
- [x] 2.1.4 Idempotency retention configurable — `IdempotencyStore` sekarang dikonstruksi di `resource.App` dengan TTL dari `Config.IdempotencyTTL` (default 24h via `db.DefaultIdempotencyTTL`), diekspos lewat `App.Idempotency()`. Resolusi dari manifest `kind: Config` (`core.idempotency_retention`) menunggu runtime Config-kind (Fase 7.2, belum ada) — `Config.IdempotencyTTL` adalah seam yang setara untuk saat ini.  
- [x] 2.1.5 `natural_key_rule` lengkap — `strategy: sequence|custom` (custom = framework tidak auto-generate, diisi hook/script/import), `format`, `prefix`, `reset: never|yearly|monthly|daily` (divalidasi di `ValidateDocumentSpec`), `scope_field` (`01-core-basic.md` §2); counter komposit `(tenant, resource, field, scope, period, seq)` sudah ada (`jsonb-persist/04` §2)  

### 2.2 Query correctness ✅
- [x] 2.2.1 Filter operators 13/13 (`eq neq gt gte lt lte between in nin like ilike null notnull` — `01-core-basic.md` §6) — added `between`, `ilike`, `null`, `notnull`; handler parsing supports `between` as comma-separated pair  
- [x] 2.2.2 JSONB path fallback for non-indexed fields — `data->>'field'` (PG) / `json_extract(data, '$.field')` (SQLite) via `EntityStore.columnRefExpr()`  
- [x] 2.2.3 Generated column dialect-aware — `generateGeneratedColumn` now accepts `DriverType`; PG uses `data->>'field'`, SQLite uses `json_extract`  
- [x] 2.2.4 `exists:<resource>` real lookup — already wired in `resource/forma.go` via `SetEntityLookup`, queries entity registry  
- [x] 2.2.5 Cross-module relation resolution — `ValidateRelationTargets` parses `{module}.{entity}` from `Relation.Resource`; registry injects `targetTableResolver` using spec's Plural (not naive `+s`)  

### 2.3 Lifecycle engine ✅
- [x] 2.3.1 8 reserved actions with guard enforcement — `LifecycleGuard` function for all 8; wired into `Update()`, `SoftDelete()`, `Submit()`, `Cancel()`; REST routes added for submit/cancel/amend
- [x] 2.3.2 Transitive gating — `TransitiveDisabled()` wired into route generation (`generator.go`)
- [x] 2.3.3 `update` after `submit` always rejected — `LifecycleGuard("update")` checked in `Update()`
- [x] 2.3.4 Referenceability — already implemented via `ValidateRelationTargets()` (unchanged)
- [x] 2.3.5 `delete` guard absolut — `LifecycleGuard("delete")` checked in `SoftDelete()`
- [x] 2.3.6 `create-submit`/`amend-submit` auto-derived — `DeriveReservedActions()` exists (route-level skip for now)
- [x] 2.3.7 Error codes lengkap — `FORMA.DOC.ALREADY_SUBMITTED`, `ALREADY_CANCELLED`, `SUBMIT_NOT_DRAFT`, `CANCEL_NOT_SUBMITTED`, `UPDATE_NOT_DRAFT`, `DELETE_NOT_DRAFT`, `AMEND_NOT_SUBMITTED_OR_CANCELLED`, `FORMA.REF.DELETE_BLOCKED`, `FORMA.REF.CANCEL_BLOCKED`
- [x] 2.3.8 `child.sequence_field` enforcement — validate monotonically ordered line numbers on insert/reorder; auto-assign when client omits, validate when provided; reject duplicates/non-monotonic → `VALIDATION_ERROR` (422)
- [x] 2.3.9 `child` lifecycle — child follows parent submit/cancel via `SubmitChildren()`/`CancelChildren()`; `doc_status` column added to child table DDL
- [x] 2.3.10 `relation.on_delete` framework — `reference.go` with `CheckReferencingDocuments` + `EnforceReferenceGuard`; stub implementation (full on_delete 3 modes membutuhkan reference tracking system)
- [x] 2.3.11 `characteristic` enforcement at apply — validated in `ValidateDocumentSpec()`
- [x] 2.3.12 `characteristic: summary` — `create`/`update`/`delete` blocked di store level (`Insert`, `Update`, `SoftDelete`)  

### 2.4 Event system core ✅
- [x] 2.4.1 Event naming convention enforcement — `ValidateEventNaming()` existed; verified called from `ValidateDocumentSpec`; `ValidateActionEmits` also exists
- [x] 2.4.2 Event priority ordering — hooks already support `Priority` field (0→default 10); `SelectHooks` sorts by priority; kelipatan 10 convention documented
- [x] 2.4.3 Durability contract validation — new `ValidateEventDurability()` checks publisher non-durable + subscriber durable → error at apply
- [x] 2.4.4 Outbox worker — `MarkFailed` enhanced with `backoff` strategy (exponential|linear|fixed) + `initial_delay_ms` support; outbox table DDL extended with columns
- [x] 2.4.5 `FORMA.EVENT.TYPE_MISMATCH` + `FORMA.EVENT.TYPE_MISSING` — wired into `ValidateEventNaming()` error messages  

### 2.5 API infrastructure ✅
- [x] 2.5.1 Two API surfaces — `/_ui/entity/` (all entities, session auth) + `/api/v1/` (exposed-only, API key); both share same internal logic
- [x] 2.5.2 Radix-tree router — chi router is radix-tree based ✅
- [x] 2.5.3 Single internal logic path — handlers → store methods; same-process dispatch bypasses network ✅
- [x] 2.5.4 ListResponse `links` field — `buildListLinks()` with first/last/next/prev ✅
- [x] 2.5.5 ErrorResponse `details` array — `ErrorDetailItem` + `writeErrorWithDetails()` ✅
- [x] 2.5.6 `per_page` clamping — max 100 clamp in `EntityStore.List()` ✅
- [x] 2.5.7 Response envelope contract — `{data, meta}` / `{error: {code, message, details}, meta}` ✅
- [x] 2.5.8 Meta API backend-agnostic — `BuildEntitySchema` uses spec types, no SQL-specific leaks ✅
- [x] 2.5.9 Workspace slug prefix — `WorkspaceMiddleware` extracts slug from URL; fallback to "demo" ✅  

### 2.6 Security basics
- [ ] 2.6.1 Cross-tenant isolation — middleware check: resource tenant == caller tenant; cross-tenant → 404 (not 403)  
- [ ] 2.6.2 Tenant ID auto-injection — DDL adds `tenant_id`, query auto-scopes WHERE tenant_id  
- [ ] 2.6.3 Permission auto-registration — `{module}.{entity}.{list|view|create|update|delete|submit|cancel|amend}` registered on entity load  
- [ ] 2.6.4 UsesEnforcement wiring — `uses` declaration checked at runtime; undeclared `uses` → blocked + alert + module auto-suspend + incident audit  
- [ ] 2.6.5 Optimistic concurrency — `version` field on all Entities; update without version → `409 CONFLICT`; `modified` is audit metadata, NOT concurrency mechanism  
- [ ] 2.6.6 WebSocket per-message permission filter — hub checks permission before broadcasting to each connection  

### 2.7 Idempotency
- [ ] 2.7.1 Two-step prepare flow — `POST /{resource}/{action}/prepare` → receive key → retry action with key  
- [ ] 2.7.2 Idempotency store — `(tenant, action, key) → pending|completed + response` (`01-core-basic.md` §5 — tanpa state `failed`); duplicate after completed → replay; duplicate during pending → wait/409  

### 2.8 `spec.expose` enforcement
- [ ] 2.8.1 `spec.expose: []` → external API returns 404 for all endpoints; UI surface unaffected  
- [ ] 2.8.2 `spec.expose: [{type: rest, actions: [list, find]}]` → only those actions on `/api/v1/`  

### 2.9 `ctx.*` infrastructure primitives
- [ ] 2.9.1 Wire `CtxAPI.SetDatastoreResolver` + implementasi `datastore.Open()` nyata — saat ini semua `ctx.db()/cache()/lock()/...` error "not configured"; fondasi Fase 7 (Service, Hook, Validation L6, sidecar proxy) (`runtimes/02-forma-resource.md` §7, `runtimes/04-forma-sidecar.md` §8)  
- [ ] 2.9.2 Closed set 9 primitive — `db`, `cache`, `lock`, `queue`, `pubsub`, `storage`, `config`, `kvstore`, `log` (`platform/06-datastore.md` §2), termasuk binding `.named()`  
- [ ] 2.9.3 Dev auto-provision `'default'` per primitive — db/kvstore→SQLite, cache/lock/queue/pubsub→in-memory, storage→filesystem (`platform/06-datastore.md` §5)  
- [ ] 2.9.4 `ctx.db()` module-scoped (normatif) — resolve ke Datastore milik Module; interaksi lintas-Module-lintas-Datastore WAJIB async, tanpa escape hatch `ctx.db` sekalipun dengan `uses` (`01-core-basic.md` §3/§5)  

---

## Fase 3: CLI — `forma` Command Completion

**Goal**: `validate` → `check` → `new` → `dev` → `generate` → `diff/get/describe/delete` → `migrate/repl/seed` → `backup/restore/logs`.  
**Priority**: per `docs/cli-tools/02-forma-cli.md` §13.1.

### 3.1 High-priority (no backend dependency)
- [ ] 3.1.1 `forma validate -f <path>` — dry-run validation + honesty scan (Starlark: undeclared usage → error, declared-but-unused → warning, `ctx.environment` branching → warning). CI gate.  
- [ ] 3.1.2 `forma check [--fix] -f <path>` — cross-file analysis: unresolved varnames (error), FormaExpr ref to nonexistent field (error), undeclared cross-module access (error), unused cross-module declarations (warning). `--fix`: auto-add `depends_on`/`uses`, auto-remove unused.  
- [ ] 3.1.3 `forma new <kind>` — scaffold: `new app <name>`, `new entity <name>`, `new module <name>`. Generate boilerplate YAML + directory.  

### 3.2 `forma dev` — verify against spec
- [x] 3.2.1 Verify 12 flags work: `--spec`, `--dsn`, `--addr`, `--listen` (none/local_http/unix_socket), `--app-endpoint` (none/local_http/unix_socket), `--runtime` (auto-detect + explicit override), `--dev`, `--dev-ui` (implies `--dev`+`--force`), `--force`, `--web-dir`, `--state-dir`, `--workspace-id`  
- [x] 3.2.2 Runtime auto-detect — `go.mod` → go (local), `package.json` → node, `composer.json` → php, `requirements.txt`/`pyproject.toml` → python, `*.csproj` → dotnet (SDK belum tersedia) — per `01-forma-dev.md` §4; ruby/java TIDAK termasuk auto-detect `forma dev` (hanya konteks sidecar `spec.runtime`, lihat 7.15.1)  
- [x] 3.2.3 SPA serving priority — explicit `--web-dir` > embedded `//go:embed` FS > auto-detect `renderers/web/dist/` (urutan per `01-forma-dev.md` §6; path auto-detect di docs masih `web/dist/` — stale pasca-restructure 0.3, perbaiki docs)  
- [x] 3.2.4 Config file `forma-app.yaml` support  
- [x] 3.2.5 Two personas: Persona A (embedded SPA, 80%) + Persona B (`--dev-ui` Vite HMR, 20%)  
- [x] 3.2.6 Add `check`, `promote`, `logs` to CLI dispatcher switch (currently fall to `usage()`)  

### 3.3 `forma generate`
- [ ] 3.3.1 `forma generate --lang typescript --spec <path> --out <dir>` — generate typed TS client from manifests  
- [ ] 3.3.2 Generate: typed interfaces, create/update input types, custom action params, `createApi()` function  
- [ ] 3.3.3 Field type mapping per `03-forma-generate.md` §3: string/uuid/date/datetime→string, integer→number, decimal/number→**string** (never `number` — presisi), boolean→boolean, enum→union, json→unknown, relation→string, child→array; `money`/`file` belum ada di tabel docs §3 — tetapkan di spec dulu (amount money wajib string, bukan number)  

### 3.4 Read-only CLI ops
- [ ] 3.4.1 `forma diff -f <path>` — compare local vs deployed (dry-run)  
- [ ] 3.4.2 `forma get <kind> <name>` — fetch resource, table/JSON output  
- [ ] 3.4.3 `forma describe <kind> <name>` — detailed view: field, action, state machine, permission (`02-forma-cli.md` §2)  

### 3.5 Mutation CLI ops
- [ ] 3.5.1 `forma delete <kind> <name> --confirm` — remove resource  

### 3.6 Engine-dependent CLI ops
- [ ] 3.6.1 `forma migrate plan|apply` — structural diff from Entity changes, applied via migration runner  
- [ ] 3.6.2 `forma repl [--environment]` — interactive Starlark console, full `ctx.*`, sandbox limits enforced  
- [ ] 3.6.3 `forma seed [--module]` — run seeders from YAML seed files  
- [ ] 3.6.4 `forma summary rebuild <entity>` — rebuild summary Entity dari replay event durable (`02-core-extended.md` §6)  

### 3.7 Data lifecycle CLI ops
- [ ] 3.7.1 `forma backup create [--full|--incremental|--filter]` — backup DB + artifacts, open format  
- [ ] 3.7.2 `forma backup inspect <file>` — inspect backup contents  
- [ ] 3.7.3 `forma restore --from <file> [--map-resource] [--conflict skip|overwrite|remap] [--dry-run]` — restore with conflict resolution  
- [ ] 3.7.4 `forma logs [--workspace] [--module] [--entity] [--action] [--level] [--since] [--until] [--request-id] [--output pretty|json] [--follow]` — tail structured logs (`09-observability.md` §7)  

### 3.8 Deferred CLI ops
- [ ] ⏸️ `promote`, `archive`, `saga`, `module`, `sign`, `script`, `freeze`, `rollback`, `lock`, `workspace create`, `suspend scripts` — depend on Control Plane or backend maturity  

---

## Fase 4: JSONB Persist — Clean Renderer

**Goal**: Clean PersistBackend interface, extension, categories, migration engine, backup/restore, archiving, audit trail, query builder.

### 4.1 Clean PersistBackend interface
- [ ] 4.1.1 Define `PersistBackend` Go interface — technology-agnostic (no SQL types: `*sql.DB`, `ExecContext`, `QueryContext`, `Driver()`)  
- [ ] 4.1.2 Required capabilities: structural diff apply, query resolution (identical results across backends), `ctx.next_key` (gap-free, atomic), index generation, clean extension uninstall  
- [ ] 4.1.3 Refactor `renderers/jsonbpersist/` to implement `PersistBackend` interface  

### 4.2 Migration engine
- [ ] 4.2.1 Structural diff from Entity spec changes — field add/remove/type-change → storage-agnostic diff (not SQL text)  
- [ ] 4.2.2 `renamed_from` field — two-phase removal (deprecate then drop)  
- [ ] 4.2.3 Per-Entity migration in one transaction — fail = full rollback; data in `data` JSONB never rewritten by structural migration  
- [ ] 4.2.4 `kind: Migration` — custom DDL (index, function, trigger, extension, materialized view); DML rejected at runtime  
- [ ] 4.2.5 Data migration ber-versi — script backfill dengan run/rollback manual; tipe migrasi ketiga di samping structural diff + custom DDL (`01-core-basic.md` §4, `04-persist-backend.md` §2)  

### 4.3 Entity extension
- [ ] 4.3.1 Extension read — `entity.ext("namespace").field` via JSONB column access  
- [ ] 4.3.2 Extension write — populate `ext_{namespace}` column  
- [ ] 4.3.3 Extension uninstall — `DROP COLUMN ext_{namespace}` + remove registry entry + namespace lock (never reused)  
- [ ] 4.3.4 Extension namespace collision prevention — `forma apply` rejects duplicate namespace for same target  
- [ ] 4.3.5 Extension `validate:` (additive business rule) — runs after base Entity L1–L6 validation, never overrides it; read-only access to base fields, may only require its own namespaced fields (`docs/spec/backend/03-entity-extension.md` §5)  

### 4.4 Category schemas
- [ ] 4.4.1 6 category schemas: operational, financial, compliance, analytics, master, archive  
- [ ] 4.4.2 Cross-category JOIN block — `FORMA.PERSIST.CROSS_CATEGORY` error  
- [ ] 4.4.3 `spec.persist.category` enforcement at query time  

### 4.5 Query Builder
- [ ] 4.5.1 Aggregate functions — `sum`, `count`, `avg`, `min`, `max`  
- [ ] 4.5.2 `group_by` — single + multi-field grouping  
- [ ] 4.5.3 `having` — post-aggregation filter  
- [ ] 4.5.4 `date_trunc` — time bucketing (day/week/month/quarter/year)  
- [ ] 4.5.5 Window functions — running total, ranking  
- [ ] 4.5.6 `include()` batched — eager-load relations in one query per level (N+1 prevention)  

### 4.6 Tree/hierarchy
- [ ] 4.6.1 Materialized path column — `_tpath_{field_name}` for `tree: true` self-referential relations; path format: `""` (root) or `parent.child.grandchild`  
- [ ] 4.6.2 Tree operators — `descendant_of` → `LIKE 'prefix.%'`, `ancestor_of` → PK lookup, `child_of` → FK query, `root` → `parent_id IS NULL`  
- [ ] 4.6.3 Cycle detection — server-side on create/update/move/reparent → `VALIDATION_ERROR` (422)  

### 4.7 Business audit trail
- [ ] 4.7.1 `audit: true` on action → append-only audit entries  
- [ ] 4.7.2 Per-entry: actor, action name (not "document updated"), timestamp (`created_at`), before/after diff, request_id  
- [ ] 4.7.3 Immutability — no API update/delete; framework writes only  
- [ ] 4.7.4 Queryable per record — source for Timeline kind; filterable with standard query operators  

### 4.8 Backup & restore
- [ ] 4.8.1 Full + incremental backup — DB dump + file storage (ctx.storage), open format  
- [ ] 4.8.2 Filterable backup — by workspace, module, entity  
- [ ] 4.8.3 Restore with conflict resolution — `skip|overwrite|remap`, `--dry-run` compatibility report  
- [ ] 4.8.4 Credible exit — read/export operations never license-gated  
- [ ] 4.8.5 Outbox reconciliation pass WAJIB setelah restore — entri outbox pending di-replay/diverifikasi terhadap state hasil restore sebelum workspace kembali melayani (`platform/04-control-plane.md` §6.1, MUST — berlaku juga single-server)  

### 4.9 Data archiving
- [ ] 4.9.1 Archive transactions (`characteristic: transaction`) to Parquet when age ≥ `retention.archive_after`  
- [ ] 4.9.2 Master snapshot "as-of" — referenced masters snapshotted alongside archived transactions  
- [ ] 4.9.3 `locked_for_deletion` flag — master referenced by archived transaction cannot be deleted  
- [ ] 4.9.4 `FORMA.ARCHIVE.LOCKED_FOR_DELETION` error code  
- [ ] 4.9.5 `forma archive run [--dry-run]` / `view --batch-id` / `restore-batch`  

### 4.10 Soft-delete & soft-deactivation
- [ ] 4.10.1 `persist.soft_delete: true` → `deleted_at` column + query auto-filters to `deleted_at IS NULL`  
- [ ] 4.10.2 `is_active` + `deactivate`/`reactivate` pattern — dropdown filters `is_active: true` for new transactions; list shows all  

---

## Fase 5: Frontend — shadcn-shell Completeness

**Goal**: Semua UI kind, widget, contract, dan FormaExpr sesuai spec. Bisa dites end-to-end.

### 5.1 App Shell
- [x] 5.1.1 `sidebar-nav` — full chrome, side navigation, breadcrumb (verified, working)  
- [ ] 5.1.2 `topnav` — full chrome, top navigation  
- [ ] 5.1.3 `landing-page` — minimal, public pages, no auth wrap  

### 5.2 `kind: Page`
- [x] 5.2.1 Blocks composition — form, table, component blocks (himpunan tertutup `06-page-kinds.md` §1; `widget` milik Dashboard §7, `html` block tidak ada di spec); permission-gated per block  
- [x] 5.2.2 Tabs variant — mutually exclusive with blocks; permission-checked per tab  
- [ ] 5.2.3 Master-detail split — `layout.mode: split`, `binds: {source, param}`; detail refetch on selection change  
- [ ] 5.2.4 Full-custom — single `component:` block  
- [ ] 5.2.5 Custom Page (`mode: custom`) — full-code page with `binds` footprint (entities, actions, subscribe); top rung of frontend control  
- [x] 5.2.6 Configuration Page pattern — `characteristic: reference` entities → no New/Delete buttons, only Update surfaced  

### 5.3 `kind: Form`
- [x] 5.3.1 `render` mode enforcement — `modal` (dialog overlay), `drawer` (slide-in panel), `separate_page` (own route); design-time, no runtime switch  
- [x] 5.3.2 Wire `OverlayHost` — connected to Form.render modal/drawer  
- [ ] 5.3.3 409 conflict handling — CAS version mismatch → "Data telah diubah oleh pengguna lain", offer reload + re-apply changes  
- [x] 5.3.4 Lifecycle UI patterns — plain_crud (no submit), 2-step+auto-save (default), 2-step manual (Save Draft + Submit buttons), 1-step create-submit (single button, no draft)  
- [x] 5.3.5 FormaExpr — `visible_when`, `readonly_when`, `required_when`, `compute` per field  

### 5.4 `kind: Table`
- [x] 5.4.1 Fix hardcoded `/_admin` prefix — surface-aware navigation (`/app` vs `/_admin`)  
- [ ] 5.4.2 Inline editing — `inline_edit: true`, cell editable for non-readonly/computed/immutable fields; CAS per baris; submitted rows reject inline-edit  
- [ ] 5.4.3 Batch editing — `batch_edit: [field, ...]`, update per baris, partial failure reported (not all-or-nothing)  
- [ ] 5.4.4 Column derivation fix — N priority columns (natural key → label_field → status → transaction_date → rest), overflow accessible via row expand/detail; NEVER silently dropped  
- [ ] 5.4.5 `realtime: true` — auto-subscribe + patch rows in-place (depends on 5.8)  

### 5.5 `kind: Kanban`
- [x] 5.5.1 Drag-and-drop — wire `@dnd-kit/core`; drag card antar kolom → PATCH `status_field`  
- [x] 5.5.2 Optimistic update with server-enforced rollback (409 → snapshot restore)  
- [ ] 5.5.3 `drag_guard` FormaExpr — pre-check UX, prevent drop that server will reject  
- [x] 5.5.4 WIP limits — `max_cards_per_column`, soft UX enforcement (visual + toast)  
- [ ] 5.5.5 Zero-config — derive columns from state machine or `group_by` enum  
- [x] 5.5.6 Click card → detail page navigation  
- [x] 5.5.7 Row actions (view/edit/delete/custom) with confirm + permission check  
- [x] 5.5.8 Filter columns from `filters` manifest — Select dropdown per filter field  

### 5.6 `kind: Calendar`
- [ ] 5.6.1 Month/week/day/resource views — `views: [month, week, day, resource]`  
- [ ] 5.6.2 Event rendering — from `date_field` + optional `end_field`; title from `label_field` or `title_field`  
- [ ] 5.6.3 Click event → detail Page/Form; click empty slot → Form create with date pre-filled  
- [ ] 5.6.4 Drag reschedule — call `update` action on date_field (server-enforced); submitted immutable rows disable drag  
- [ ] 5.6.5 RRULE recurrence — parse RFC 5545, expand to instances for visible date range (render-time, not materialized)  
- [ ] 5.6.6 Resource view — one lane per `resource_field` value; color by `color_field`  

### 5.7 `kind: Dashboard` + `kind: Widget`
- [ ] 5.7.1 Widget `stat` — fetch from summary entity, display number with label  
- [ ] 5.7.2 Widget `chart` — bar/line/pie from summary entity; add chart library dependency (katalog widget bawaan spec HANYA `stat` + `chart` — `07-component-kinds.md` §2; ListWidget/SummaryWidget tidak ada di spec, usulkan ke spec dulu bila dibutuhkan)  
- [ ] 5.7.3 Dashboard customizable — `customizable: true`, user add/remove/reorder widgets from catalog; preference stored as runtime preference (not YAML)  
- [ ] 5.7.4 Widget catalog visibility — derived from user's `list`/`view` permission on underlying entity (not manual flag)  

### 5.8 Realtime WebSocket
- [ ] 5.8.1 `useRealtime(entityRef)` hook — subscribe to `entity:{module}.{name}` channels  
- [ ] 5.8.2 Optimistic update — patch rendered data in-place on event  
- [ ] 5.8.3 Reconnect → refetch via `/_meta/ui`, no replay  

### 5.9 Asset Component Contract
- [ ] 5.9.1 Dynamic ES module loader — load `asset` component at runtime  
- [ ] 5.9.2 `forma` client injection — `forma.api` (typed, logged-in user), `forma.subscribe(entity, cb)`, `forma.navigate(page, params)`, `forma.theme` (tokens), `forma.components` (widget dasar untuk komposisi custom component — `07-component-kinds.md` §1)  
- [ ] 5.9.3 `forma.ui` centralized service — `toast()`, `dialog()`, `confirm()`, `drawer()` (replace direct `sonner` imports + `window.confirm()`)  
- [ ] 5.9.4 `forma.files` — upload/download tray  
- [ ] 5.9.5 `forma.form(entity, {mode, id?})` — headless form engine: field state, dirty tracking, client validation from field rules, FormaExpr eval, `submit()` with CAS version  
- [ ] 5.9.6 `needs:` declaration — frontend `uses` equivalent; `forma.api` calls outside `needs` fail client-side  
- [ ] 5.9.7 CSP sandbox — `connect-src` restricted to App origin only; no `window`/`document` global access outside container  
- [ ] 5.9.8 CSS scoped — component CSS never leaks to chrome or other components  

### 5.10 Missing input widgets
- [ ] 5.10.1 DatePicker — `react-day-picker` integration for `date`/`datetime` field types  
- [ ] 5.10.2 JsonEditor — textarea + JSON validation  
- [ ] 5.10.3 ChildGrid — inline table for `child` entities with `storage: table`  
- [ ] 5.10.4 RichText — basic toolbar (bold/italic, list, link, heading) + server-sanitized HTML; NOT page builder  
- [ ] 5.10.5 FileInput — upload to `ctx.storage`, preview (image/PDF), size/type enforcement from field rules, `forma.files` tray  
- [ ] 5.10.6 DecimalInput — arbitrary-precision decimal with banker's rounding display  
- [ ] 5.10.7 DateTimeInput — combined date + time picker  
- [ ] 5.10.8 Base UI components — empty-state, breadcrumb, skeleton/loading, pagination, badge, card  
- [ ] 5.10.9 Textarea — bagian himpunan tertutup widget wajib (`07-component-kinds.md` §1), belum tercantum sebagai widget existing di renderer  

### 5.11 FormaExpr
- [ ] 5.11.1 Audit grammar vs spec — verify lexer→parser→evaluator supports all operators from `08-formaexpr.md` §2  
- [ ] 5.11.2 Deploy-time static validation — `forma apply`/`forma check` rejects unresolvable field references + invalid grammar (ERROR, not warning)  
- [ ] 5.11.3 Runtime error state — nonexistent field reference → visible error state (never silent fail-safe/evaluate to `false`)  
- [ ] 5.11.4 `title` interpolation — `"Order {order.number}"` pattern in Page/Wizard/Print titles  
- [ ] 5.11.5 Cross-shell conformance test suite — identical interpretation across shells  

### 5.12 Spec Resolution API
- [ ] 5.12.1 ETag caching — conditional GET with 304 for `/_meta/ui` bundle  
- [ ] 5.12.2 `label_field` fallback — `natural key` → `name` → `title` → `number` → `id` (`04-spec-resolution-api.md` §2)  
- [ ] 5.12.3 Entity schema shape — `label_field`, `lifecycle`, `actions` with embedded `permission`  
- [ ] 5.12.4 Permission filtering — entity (404 if no list/view), page (hidden if missing permission), action (permission string sent, not filtered)  
- [ ] 5.12.5 Task-based admin granting → materialized permission strings  

### 5.13 Other UI kinds
- [ ] 5.13.1 `kind: Report` — fix totals row bug (values computed but `<tr>` empty); add grouping + subtotal; export berjalan sebagai async job → file mendarat di download tray (`06-page-kinds.md` §8), bukan CSV Blob client-side  
- [ ] 5.13.2 `kind: Print` — PDF server-side generation; `format: html` via `window.print()` (existing)  
- [ ] 5.13.3 `kind: ApprovalInbox` — pending approvals list, `approve`/`reject` inline actions, badge count, `realtime: true`  
- [ ] 5.13.4 `kind: NotificationCenter` — notification list, badge unread, `mark-read` action, `realtime: true`, deep-link on click  
- [ ] 5.13.5 `kind: Listing` — public catalog, no auth wrap, no row/bulk actions  

### 5.14 Derivation engine
- [ ] 5.14.1 Derivation fix — Table: N priority columns, overflow accessible via expand (never silently dropped)  
- [ ] 5.14.2 Wire `deriveMenuItems()` — currently dead code; `_admin` menu built inline in Sidebar  
- [ ] 5.14.3 Derivation: Form mode heuristic — >12 fields OR has child with `storage: table` → `separate_page`; >5 fields → `drawer`; else → `modal`  
- [ ] 5.14.4 Pola UI lifecycle tambahan — `two_step_manual` dan `one_step_create_submit` via hint `ui:` (`06-page-kinds.md` §2.1); catatan: enum `lifecycle` di EntitySchema tetap 2 nilai (`plain_crud|two_step_autosave`, `04-spec-resolution-api.md` §2) — ini pola UI, bukan nilai enum baru  

### 5.15 Dead code cleanup
- [ ] 5.15.1 Remove `engine/registry.tsx` — replaced by hardcoded `lazy()` map in router  
- [ ] 5.15.2 Wire `OverlayHost` — connect to Form.render modal/drawer and other overlay needs  

### 5.16 VisualSpecKind/Renderer registry & resolution
- [ ] 5.16.1 Renderer resolution engine — pilih Renderer via `(implements, stack_family)`; hanya `official` auto-select; tanpa official → `forma apply` error + sarankan kandidat `verified`/`community`; override via map `renderers:` di App manifest + field `renderer:` per-instance; Renderer non-official masuk consent footprint (`03-renderer-kind.md` §3)  
- [ ] 5.16.2 Slot-tier validation at apply — `accepts_slots` hanya sah dari `tier: page|app`, `implements_slot` hanya dari `tier: component`; kombinasi lain ditolak (`02-visual-spec-kind.md` §4–§5)  
- [ ] 5.16.3 `stack_family` compatibility check — App shell + Page shell-integrated + Component wajib satu family; mismatch = compile-time error; Page independen tidak dicek (`01-visual-hierarchy.md` §3)  

---

## Fase 6: Auth & Authorization

**Goal**: Login, JWT, permission model, roles, API keys, sessions, field security. Prod requirement.

### 6.1 Login & token
- [ ] 6.1.1 Login endpoint — `POST /api/v1/auth/login`, credential verification (password hash), JWT issuance (access + refresh)  
- [ ] 6.1.2 Token claims — `sub`, `workspace`, `roles`, `permissions`, `exp`, `iat`  
- [ ] 6.1.3 Token refresh — rotate (invalidate old, issue new)  
- [ ] 6.1.4 Auth per-App via `auth_config_ref` — App me-resolve strategy autentikasi dari yang terpasang (`basic-auth` minimum untuk single-server; `sso` OIDC/SAML, `social-sso`, `passwordless`, `passkey` = set terbuka) (`platform/02-workspace-app-module.md` §3)  

### 6.2 Permission model
- [ ] 6.2.1 Resource + action permission — format `{module}.{entity}.{action}`  
- [ ] 6.2.2 Wildcard support — `{module}.{entity}.*`, `*` (super-wildcard), `public`  
- [ ] 6.2.3 Wire permission check to every API handler — both surfaces  
- [ ] 6.2.4 Permission resolution — role → permissions list; cache per session  
- [ ] 6.2.5 Consent footprint — aggregate `required_permission` + `uses` presented to workspace owner at install; cross-module write = high-risk consent  
- [ ] 6.2.6 Attribute-based authorization — pemeriksaan atribut App/user/membership melengkapi RBAC; pola multi-cabang = `scope_field` pada natural key + atribut membership (mis. kode cabang) (`platform/02-workspace-app-module.md` §3)  

### 6.3 Roles & membership
- [ ] 6.3.1 `role` Entity — collection of permissions  
- [ ] 6.3.2 `app-membership` Entity — populasi user per App + atribut membership (mis. kode cabang); TERPISAH dari `role-assignment` (penetapan role ke user dalam konteks App) (`platform/02` §9)  
- [ ] 6.3.3 Admin delegation chain — workspace owner → app admin → module staff  
- [ ] 6.3.4 4 symmetric owner roles — Workspace Owner, App Owner, Module Owner, Cloud Owner  
- [ ] 6.3.5 `forma.core` resource set lengkap — `workspace`, `user`, `app-membership`, `role`, `role-assignment`, `api-key`, `session`, `job`, `audit-log`, `setting`; namespace selalu ada, dapat direferensikan tanpa `depends_on` (`platform/02` §9)  

### 6.4 API keys
- [ ] 6.4.1 `api_key` Entity — create (return key once), list (masked), revoke, expiry  
- [ ] 6.4.2 Scope — per workspace or per app  
- [ ] 6.4.3 API key auth middleware — header `X-Forma-Key` (`01-core-basic.md` §8.2; surface external TIDAK menerima session cookie)  

### 6.5 Session management
- [ ] 6.5.1 `session` Entity — session_id, user, workspace, IP, user-agent, created/expires/last_active  
- [ ] 6.5.2 Refresh token rotation — invalidate old, issue new  
- [ ] 6.5.3 Concurrent session limit — configurable per user  
- [ ] 6.5.4 Global revoke — logout all devices  
- [ ] 6.5.5 Session expiry + cleanup job  

### 6.6 Auth middleware pipeline
- [ ] 6.6.1 Auth method detection — Bearer JWT vs `X-Forma-Key` API key vs session cookie (session cookie hanya surface `/_ui`)  
- [ ] 6.6.2 Token validation → identity extraction → permission loading → workspace context  
- [ ] 6.6.3 Rate limiting per auth method  
- [ ] 6.6.4 Audit log every auth attempt (success + failure)  

### 6.7 Field-level security
- [ ] 6.7.1 `classification` label — tag field `pii|financial|internal`; log/export auto-tag  
- [ ] 6.7.2 `required_permission` (field-level) — user without permission → field excluded from response  
- [ ] 6.7.3 `exclude` — per-surface field exclusion (`public_api`, `audit_log`, `webhook`, `ui` — `05-field-types.md` §5.3)  
- [ ] 6.7.4 `encrypted: true` — AES-256-GCM at-rest encryption for field  
- [ ] 6.7.5 `masked: true` — auto-mask in JSON response and structured log (`***`)  
- [ ] 6.7.6 `computed` — server-derived, never client-writable; recompute on every create/update  

### 6.8 `ctx.secrets`
- [ ] 6.8.1 `ctx.secrets.get("key")` — only path for `secret: true` Config keys  
- [ ] 6.8.2 `uses: { secrets: [key, ...] }` — must declare access; undeclared → blocked  
- [ ] 6.8.3 Secret never appears in logs at any level  
- [ ] 6.8.4 Every secret read audited — who read what secret, when  

### 6.9 RichText sanitization
- [ ] 6.9.1 Server-side HTML sanitize — strip script/markup before persist; client HTML never trusted raw  

---

## Fase 7: Engine Extended — Missing Kind Runtimes

**Goal**: Service, Config, Subscription, Workflow, Webhook, Integrator, Hook engine, Validation L4–L6, State machine, Denormalisasi, Period closing, Rate limiter, Async job.

### 7.1 `kind: Service` runtime
- [ ] 7.1.1 Service registry — register by `{module}.{name}`  
- [ ] 7.1.2 Resolve `impl.native` — scan `impl/**/*.go`, `ref: "{Type}.{Method}"`, must be unique in module  
- [ ] 7.1.3 Resolve `impl.script` / `impl.script_ref` / `impl.compiled` / `impl.sidecar` — permission enforcement seragam untuk KELIMA jenis impl (`01-core-basic.md` §5)  
- [ ] 7.1.4 `call: async` — fire-and-forget (no job_id, no progress, no result)  

### 7.2 `kind: Config` runtime
- [ ] 7.2.1 Config registry — load Config manifests, resolve per environment  
- [ ] 7.2.2 `ctx.config.get("key")` — Starlark access; non-secret keys only  
- [ ] 7.2.3 `settings.*` namespace — global settings: currency, locale, timezone, date_format, fiscal_year_start  
- [ ] 7.2.4 Global settings defaults — spec MUST provide acceptable defaults for every setting; components MUST NOT guess  

### 7.3 `kind: Subscription` engine
- [ ] 7.3.1 Tier 1 (outbox) — event → match Subscription → call handler; transactional  
- [ ] 7.3.2 Tier 2 (streaming) — Redis/Kafka; at-least-once, positioned replay, filter/transform Starlark  
- [ ] 7.3.3 `emits:` custom event emission — action declares `emits: <event-name>` → event emitted on action success  
- [ ] 7.3.4 Dynamic subscriptions — runtime-created subscriptions as data (not manifest); live in `forma.core`  
- [ ] 7.3.5 Delivery channels — `webhook` (outbound, HMAC signed, retry), `notification` (bridge to `forma/notify`), `pubsub` (non-durable, at-most-once)  

### 7.4 `kind: Workflow` engine
- [ ] 7.4.1 Approval state machine — attach to Entity transition without modifying Entity  
- [ ] 7.4.2 Multi-approver modes — `all` (all eligible must approve), `any` (quorum from pool), `sequential` (chain order)  
- [ ] 7.4.3 `when` condition — FormaExpr on `resource`; step skipped if false  
- [ ] 7.4.4 `escalation` — timeout (`after`), notify_roles, reassign_roles  
- [ ] 7.4.5 Requester can never approve own request  
- [ ] 7.4.6 Approval = signed statement recorded in audit trail  

### 7.5 State machine engine (basic)
- [ ] 7.5.1 Transition validation — declared transitions only; undeclared → `STATE_TRANSITION_ERROR`  
- [ ] 7.5.2 Starlark inline guards — guard on transition  
- [ ] 7.5.3 Builtin aggregates — `sum_line(field)`, `len(resource.items)` for guard expressions  
- [ ] 7.5.4 Satukan dua implementasi state machine — `entity.StateMachineEngine` (lengkap, ber-guard) tidak dipanggil dari `HandleCustomAction`; enforcement yang jalan versi sederhana di `db.EntityStore.Update` (`runtimes/02` §7, `runtimes/05` §5)  

### 7.6 `kind: Webhook` engine
- [ ] 7.6.1 Inbound endpoint — route registration, method validation  
- [ ] 7.6.2 Signature verification — HMAC (strategy: `signature`, algorithm, header, payload) before handler  
- [ ] 7.6.3 Token auth — strategy: `token`  
- [ ] 7.6.4 Verification failure → rejected BEFORE handler runs  

### 7.7 `kind: Integrator` engine
- [ ] 7.7.1 Listen → call bridge — `listen.resource`+`event` triggers `call.resource`+`action`  
- [ ] 7.7.2 Mandatory symmetric cancel handler — every Integrator MUST provide cancel handler  
- [ ] 7.7.3 Target action must be `idempotent: true` for cross-boundary calls  
- [ ] 7.7.4 Saga compensate — cross-boundary call registers `compensate` to Saga log; `FORMA.SAGA.*` errors  

### 7.8 Hook engine
- [ ] 7.8.1 5 hook points — `before`, `after`, `on_error`, `before_deliver`, `after_deliver`  
- [ ] 7.8.2 `before` — may modify action params or call `fail()` to abort  
- [ ] 7.8.3 `after` — post-action side effects  
- [ ] 7.8.4 `on_error` — compensation/cleanup  
- [ ] 7.8.5 `before_deliver` — may suppress delivery or enrich payload  
- [ ] 7.8.6 Priority ordering — consistent with event priority (smaller first, kelipatan 10)  
- [ ] 7.8.7 Cross-module hooks — must declare `uses`; appear in consent footprint  

### 7.9 Validation levels L4–L6
- [ ] 7.9.1 L4 `business_rules` — single-record business constraints via script  
- [ ] 7.9.2 L5 `cross_validate` — multi-field/child-record validation within same record  
- [ ] 7.9.3 L6 `consistency` — cross-entity consistency (e.g., aggregate balance vs GL); requires `uses: db`  
- [ ] 7.9.4 Sequential evaluation — L1–L3 → L4 → L5 → L6; stop at first failure  
- [ ] 7.9.5 Error response with `details: [{level, field?, message}]`  
- [ ] 7.9.6 Katalog rule L1–L3 lengkap server-side — himpunan tertutup ~20 rule: `required, min_length, max_length, length, pattern, email, url, min, max, positive, precision, in, future, past, after:<field>, before:<field>, min_items, max_items, unique, exists, script` (`05-field-types.md` §3)  

### 7.10 Denormalisasi finansial
- [ ] 7.10.1 Master financial fields snapshot to transaction on `create`/`submit` — not live-join  
- [ ] 7.10.2 Old transactions unaffected by master value changes  

### 7.11 Period closing
- [ ] 7.11.1 `period-closing` as Entity — gets lifecycle, reference guard, audit trail for free  
- [ ] 7.11.2 `submit` → finalize summary period; `cancel` (reopen) → unfinalize  
- [ ] 7.11.3 Reopen requires elevated permission + recorded reason → `FORMA.PERIOD.REOPEN_DENIED`  
- [ ] 7.11.4 Business calendar day resolution — `today`/`current` from EOD, not system clock  
- [ ] 7.11.5 `FORMA.PERIOD.CLOSED` enforcement for create/update/submit/amend in closed period  

### 7.12 Rate limiter
- [ ] 7.12.1 Per-resource rate limit — `max`, `per`, `scope` (tenant|user|ip|global), `strategy` (sliding_window|token_bucket)  
- [ ] 7.12.2 Per-action override — overrides resource default  
- [ ] 7.12.3 `429` response before handler runs  

### 7.13 Async job tracker
- [ ] 7.13.1 `call: async` (tracked) → `202` with `job_id`  
- [ ] 7.13.2 Progress via WebSocket `jobs` channel — `progress`/`completed`/`failed` events  
- [ ] 7.13.3 `ctx.job.progress(pct, message)` from handler  
- [ ] 7.13.4 Callback webhook delivery — HMAC-signed, durable retry  

### 7.14 Starlark sandbox
- [ ] 7.14.1 Hard limits enforcement — wall-clock 5000ms, memory 64MB, iterations 100K, max 50 DB queries, max 1000 records read  
- [ ] 7.14.2 No network/filesystem/subprocess access  
- [ ] 7.14.3 Exceeding any limit → abort with error, no partial results  
- [ ] 7.14.4 Kontrak API script runtime — `resource.field`/`resource.set/save/new`, `<Entity>.query()`, `<resource>.load/call`, `ok()`/`fail()` (fail = rollback transaksi) (`06-script-runtime.md` §2/§4/§6)  

### 7.15 Sidecar multi-runtime
- [ ] 7.15.1 Read `spec.runtime` per Module manifest — go, node, php, python, ruby, java, dotnet, rust  
- [ ] 7.15.2 Spawn one sidecar process per unique runtime  
- [ ] 7.15.3 Sidecar protocol — entity CRUD via `POST /ctx/entity/{op}` (get, set, update, increment, decrement)  

### 7.16 Money type
- [ ] 7.16.1 Money as first-class type — pair of exact amount (decimal) + currency code (ISO-4217)  
- [ ] 7.16.2 Currency resolution order — explicit field → `settings.currency` → error (never guess)  
- [ ] 7.16.3 Banker's rounding default  
- [ ] 7.16.4 Non-default currency MUST declare `decimal_places`  

### 7.17 File storage
- [ ] 7.17.1 File upload route — `POST /:resource/:id/{field}` convention  
- [ ] 7.17.2 `storage` spec enforcement — `allowed_types`, `max_size_mb`, `max_count`, `visibility` (public|private|signed)  
- [ ] 7.17.3 Transform — server-side resize/thumbnail per `transform` spec  

---

## Fase 8: Production Self-Hosting Single Server

**Goal**: `forma serve --mode=production` — production-grade, single-server, no Control Plane.

### 8.1 Production mode
- [ ] 8.1.1 `forma serve --mode=production` — disable dev shortcuts: no auto-approve, no self-signed, no dev auth  
- [ ] 8.1.2 Production JWT — RS256/ES256 (test + wire to config)  
- [ ] 8.1.3 HTTPS — TLS configuration  
- [ ] 8.1.4 Production datastore — Postgres (not SQLite)  
- [ ] 8.1.5 CORS origin allow-list — production TIDAK boleh `Access-Control-Allow-Origin: *` (saat ini hardcoded, `runtimes/05-engine-api-layer.md` §2.2)  
- [ ] 8.1.6 Peran DB least-privilege — `forma_ops_backup` (REPLICATION-only), `forma_ops_ddl` (DDL-only, NOSUPERUSER), tanpa superuser manusia (`platform/06-datastore.md` §8)  

### 8.2 Observability
- [ ] 8.2.1 Structured JSON-lines logging — 12 mandatory fields: timestamp, level, request_id, workspace, module, entity, action, actor, duration_ms, error_code, trace_id, environment  
- [ ] 8.2.2 PII discipline — info/warn/error MUST NOT contain business data; debug gated by operator control  
- [ ] 8.2.3 Request ID — issue at boundary, propagate to Starlark (`ctx.request_id`), sidecar (header), ctx.* calls  
- [ ] 8.2.4 Prometheus `/metrics` endpoint (separate admin listener) — 12 mandatory metrics (`09-observability.md` §3.1): http_requests_total, http_request_errors_total, http_request_duration_seconds, action_duration_seconds, action_errors_total, outbox_pending, outbox_lag_seconds, ws_connections, db_pool_open/idle/wait_total, snapshot_age_seconds  
- [ ] 8.2.5 Cardinality discipline — labels limited to bounded dimensions; no entity_id, request_id, actor, raw URL as labels  
- [ ] 8.2.6 Health endpoint `GET /health` — `{status, reasons, checked_at}` with controlled vocabulary: healthy/degraded/unhealthy; reasons: snapshot_stale, datastore_unreachable, db_pool_exhausted, outbox_backlog, control_plane_unreachable (`09-observability.md` §5); endpoint sama melayani liveness+readiness (ready hanya saat healthy/degraded)  
- [ ] 8.2.7 OpenTelemetry tracing — W3C Trace Context propagation to sidecar (wire contract)  

### 8.3 Backup automation
- [ ] 8.3.1 Scheduled backup — periodic full + incremental  
- [ ] 8.3.2 Restore procedure — documented, tested  

---

## Fase 9: Final Audit & Cleanup

### 9.1 Full audit — code vs `docs/spec/`
- [ ] 9.1.1 Systematic comparison of every spec file against implementation; catalog all deviations  

### 9.2 Integration test
- [ ] 9.2.1 Order-to-Cash end-to-end — order → invoice → payment → general ledger; all flows automated  

### 9.3 Code generation

---

## Fase 10: Developer Experience

### 10.1 Spec hot-reload ✅
- [x] 10.1.1 `App.ReloadSpec()` — rebuild semua registri dari spec directory, atomic swap
- [x] 10.1.2 `watchSpecForChanges()` — fsnotify watcher di `forma dev`, debounce 300ms
- [x] 10.1.3 Native Go handlers preserved across reload via `nativeHandlers` map
- [x] 10.1.4 WebSocket connections preserved via WSHub transfer
- [x] 10.1.5 Auto-watch subdirektori baru
- [x] 10.1.6 ETag-aware Meta API — reload otomatis mengubah ETag, frontend fetch bundle baru
- [ ] 9.3.1 `make generate` — generate TypeScript types from `pkg/spec/` → `renderers/web/src/generated/types.ts`  
- [ ] 9.3.2 Validate generated types against manual `types/manifest.ts`  

### 9.4 Developer guide
- [ ] 9.4.1 "Buat App Pertama Anda dengan Forma" — getting started guide  

### 9.5 Retirement
- [ ] 9.5.1 Final audit `docs_old/` — verify all content migrated; archive or delete  

---

## Fase 10: `forma consult` — AI Business Consultant & Spec Author

**Goal**: AI membantu discovery kebutuhan bisnis + menulis spec Forma yang valid, lewat grounding
(tool nyata) dan validasi wajib server-side — bukan bergantung kedisiplinan LLM.
**Depends on**: Module Vendoring §6/§2.1 (`vendors/`, `forma.lock`, alias — untuk
`list_installed_modules()` yang akurat) dan Marketplace (`ai_index`, trust tier) — keduanya masih
di tabel Deferred di bawah, jadi Fase ini realistis baru mulai setelah salah satunya landing.
**Sumber**: `docs/ai/` (README + 01–06), `docs/cli-tools/05-forma-consult.md` (referensi verb).

### 10.1 `forma mcp-serve` — subcommand Go baru
- [ ] 10.1.1 Subcommand `forma mcp-serve` — expose `forma-local-mcp` lewat stdio, pembungkus tipis `forma-core` (`docs/ai/03-forma-local-mcp.md`)
- [ ] 10.1.2 Tool read-only: `list_kind_schemas(kind)`, `read_workspace_manifest()`, `list_installed_modules()`, `read_module_spec(module,kind,name)` (03 §1); untuk modul `vendors/` hanya ekstraksi field metadata, bukan dump mentah spec (02 §2, 04 §3.1)
- [ ] 10.1.3 Tool `propose_spec_file(path,content)` — tulis draft ke sesi + jalankan `validate_spec` otomatis (03 §2)
- [ ] 10.1.4 Tool `apply_draft(session,file)` — pindahkan draft ke lokasi asli, guard read-only `vendors/` (03 §4)
- [ ] 10.1.5 Tool `validate_spec(yaml)` / `check_naming_conflict(name)` — reuse package sama dengan `forma apply --dry-run`/boot `forma-server`, bukan reimplementasi (03 §3)
- [ ] 10.1.6 Tool `restart_server()` / `get_server_status()` / `stop_server()` — kontrol proses `forma dev` lokal (03 §5)
- [ ] 10.1.7 Tool `list_skills()` / `read_skill(name)` — index dan isi Forma Skill (03 §1, 06)

### 10.2 `forma-consult` client (TypeScript + Vercel AI SDK)
- [ ] 10.2.1 Scaffold project TypeScript, compile jadi binary standalone via `bun build --compile` (`docs/ai/01-architecture.md` §2)
- [ ] 10.2.2 Tool-use loop (`ToolLoopAgent`) — spawn `forma mcp-serve` sebagai child process stdio (01 §3)
- [ ] 10.2.3 LLM Provider Layer — BYOK, provider adapter (Anthropic/OpenAI/dst.), minimum capability bar tool-calling + context window (`docs/ai/05-llm-provider-layer.md`)
- [ ] 10.2.4 Credential storage — interface `CredentialStore`, `zalando/go-keyring` tiered ke environment variable (05 §3)
- [ ] 10.2.5 REPL — kelola sesi, render diff (unified diff teks cukup untuk versi awal) (`docs/ai/02-forma-consult.md`)
- [ ] 10.2.6 Auto-invoke deterministik saat sesi mulai (`read_workspace_manifest`+`list_installed_modules`+`list_skills`) — bukan bergantung inisiatif LLM (01 §5)
- [ ] 10.2.7 Kompresi riwayat sesi panjang — distilasi turn lama jadi ringkasan terstruktur, transcript penuh tetap di disk, jaga pasangan `tool_use`/`tool_result` (01 §6)

### 10.3 Validation Gate
- [ ] 10.3.1 `propose_spec_file` composite tool — validasi wajib server-side, proteksi sama untuk client built-in maupun eksternal (03 §2)
- [ ] 10.3.2 Scope structural-only (schema, referensi `depends`/Entity Extension/shadow-copy, bentrok nama) — bukan validasi data runtime (03 §3)
- [ ] 10.3.3 Jalur online eksplisit terpisah untuk verifikasi signature/trust-tier vendor module (03 §3)

### 10.4 Session storage & diff
- [ ] 10.4.1 `.forma/consult/{session}/` — `transcript.md`, `discovery-summary.md`, `draft/`, `undo/` (02 §3)
- [ ] 10.4.2 `forma consult diff` — unified diff `draft/` vs `modules/`/`vendors/` project asli (02 §4)
- [ ] 10.4.3 Accept/reject per file → `apply_draft` (02 §4)

### 10.5 `forma-remote-mcp` (Forma Cloud, hosted)
- [ ] 10.5.1 Streamable HTTP server — `list_business_templates()`, `search_modules_registry(query)`, `get_module_detail(name)` (`docs/ai/04-forma-remote-mcp.md`)
- [ ] 10.5.2 Katalog industry template awal, 100% Forma-authored — pattern YAML + probing questions (04 §1)
- [ ] 10.5.3 pgvector embedding untuk `search_modules_registry` — model multilingual (Voyage AI/BGE-M3), hybrid dengan `aliases:` eksplisit (04 §2)
- [ ] 10.5.4 `ai_index`/`skills_for_ai` untrusted-input handling — wajib selesai sebelum trust tier `community` dibuka untuk field ini (04 §3.1)

### 10.6 Forma Skill
- [ ] 10.6.1 Format YAML frontmatter + Markdown body — `name`, `description`, `applies_to_kind`, `min_core_spec_version` (`docs/ai/06-forma-skill.md` §2)
- [ ] 10.6.2 Skill pertama: entity-authoring, form-layout, entity-extension-authoring, module-vendoring (06 §2, §4)
- [ ] 10.6.3 Bundling bersama instalasi `forma` (ikut siklus rilis, dicek vs Core Spec lokal), dibaca lewat `list_skills()`/`read_skill()` (06 §2–§3)
- [ ] 10.6.4 Re-cek skill relevan sebagai bagian composite `propose_spec_file` — pemicu deterministik, bukan inisiatif LLM (06 §3)

### 10.7 Operational safety
- [ ] 10.7.1 Snapshot & undo — auto-backup file-level di `.forma/consult/{session}/undo/` sebelum `apply_draft` menimpa (02 §4)
- [ ] 10.7.2 Guard read-only `vendors/` ditegakkan di semua tool tulis, bukan konvensi dokumentasi (03 §4)

---

## Deferred (Cloud Phase)

| Area | Reason |
|---|---|
| `forma-ctl` (all modes: region, cluster, standalone) | Control Plane — cloud deployment phase |
| K8s Operator (`forma-operator`) | Production infrastructure |
| Marketplace (pricing, metering, licensing) | Business features |
| Control Plane (Environment, Policy/OPA, transparency log, key model, contracts) | Governance |
| Two-stage deployment pipeline (register→deploy, snapshot, evidence) | Requires Control Plane |
| `forma promote/archive/saga/module/sign/script/freeze/rollback/lock/workspace/suspend` | CLI — depend on Control Plane |
| Module vendoring & activation — `vendors/`/`overrides/` folders, `forma.lock`, install-time alias on name conflict, marker-based activation (`--use`), shadow copy (`forma override adopt/diff`) for `Form`/`Menu`/`VisualSpecKind` instances | Design agreed (`docs/spec/platform/08-project-layout.md` §6, `docs/spec/platform/02-workspace-app-module.md` §2.1), depends on `forma module install`/Marketplace maturity — open questions in §6.5 need resolving first |
| `forma consult` task breakdown — see **Fase 10** above | Zero implementation; realistically starts after Module vendoring or Marketplace (below) lands |
| Conformance test-suite VisualSpecKind/Renderer/PersistBackend (fixture, trust tier `verified`/`official`) | Terkait Marketplace/distribusi — `frontend/02` §6, `frontend/03` §5, `backend/04` §7 |
| Print: thermal/dotmatrix | Niche — PDF sufficient |
| gRPC + mTLS transport | Cloud deployment |
| Platform signing (HSM/KMS) | Cloud deployment |
| Generic Docker image (`formahub/forma-resource`) | Cloud deployment |
| Scale-to-zero | Cloud deployment |
| Unmanaged client codegen (Dart, Flutter) | Future SDK |



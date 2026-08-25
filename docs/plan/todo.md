# Master Plan: FormSpec Implementation

**Last Updated**: 2026-08-25  
**Status**: ✅ Fase 0 complete · ✅ Fase 1 (1.1–1.5) · ✅ Fase 2.1 · ✅ Fase 2.2 · ✅ Fase 2.6 (2.6.1–2.6.3, 2.6.5–2.6.6) · ✅ Fase 2.7 (idempotency prepare flow) · ✅ Fase 2.8 (spec.expose) · ✅ Fase 2.9 (2.9.1–2.9.3: ctx.\* primitives + dev auto-provision) · ✅ Fase 5 (5.1–5.4) · ✅ Spec hot-reload · ✅ Fase 11 (review schema↔docs) · ✅ Audit spec↔schema + tambah TODO item · ✅ `formspec validate` (3.1.1, engine+schema) · ✅ Rename forma→formspec (docs/plan/rename-formspec.md) · 🚧 Fase 12 Domain Infrastruktur (docs/architecture/09-domain-map.md) · ✅ Schema registry online (docs/plan/schema-registry-online.md) · ✅ CLI repl/seed/diff (3.4.1, 3.6.2, 3.6.3) · ✅ **Fase 4 (4.1–4.10) complete** (incl. 4.3.1–4.3.5 entity extension, 4.8.3 restore remap) · ✅ Landing page (5.1.3 + 5.13.5, docs/plan/landing-page.md) · ✅ App renderer archetypes (5.1.1–5.1.3: sidebar-nav/topnav/no-nav + access + persist_backend, docs/plan/landing-page.md) · ✅ **Fase 6.1 (6.1.1–6.1.3: login + token, entity-backed auth, external/ merge, generate-auth)** (docs/plan/auth-login-token.md) · ✅ **6.3.1 + 6.3.2 + 5.12.5 (role + role-assignment Entity, materialisasi grant page → permission)** (docs/changelog/2026-08-20-001) · ✅ **6.2.3 (wire permission check semua handler, surface-aware 404)** (docs/changelog/2026-08-20-002) · ✅ **Fase 6 COMPLETE (6.1–6.9, dogfooding auth module)** (docs/plan/fase6-dogfooding-auth-module.md, changelog 2026-08-20-003 s/d 2026-08-21-014) · 📐 **Widget strategy** (docs/plan/widget-strategy.md — sync 5.10, tambah 5.2.7/5.10a, cross-link 7.17.1) · ✅ **Fase 5 COMPLETE (5.1–5.16, docs/plan/fase5-completion.md, changelog 2026-08-24-027 s/d 2026-08-24-033)** · ✅ **Fase 7 subset (7.5, 7.8, 7.12, 7.14, 7.16 — docs/plan/fase7-subset.md, changelog 2026-08-25-001 s/d 2026-08-25-002)** · ✅ **7.2 Config runtime (7.2.1–7.2.4: Config registry + ctx.config/ctx.secrets wiring, docs/plan/fase7-config-runtime.md, changelog 2026-08-25-003)** · ✅ **7.5.4 unify state machine guard (internal/starlark/guard.go shared EvaluateGuard, changelog 2026-08-25-004)** · ✅ **7.1 Service runtime (7.1.1–7.1.4: registry + dispatch + API exposure + call:async, docs/plan/fase7-service-runtime.md, changelog 2026-08-25-005)** · ✅ **7.14.4 resource.new() (script runtime contract, changelog 2026-08-25-006)** · ✅ **7.17.2 storage spec enforcement (max_count + visibility, changelog 2026-08-25-007)** · ✅ **7.9.6 validation rules L1–L3 complete (length/in/script/unique, changelog 2026-08-25-008)** · ✅ **Contoh project service-demo (Service runtime + validation rules, examples/service-demo, changelog 2026-08-25-009)** · ✅ **7.6 Webhook engine (7.6.1–7.6.4: registry + route + HMAC signature + token auth, docs/plan/fase7-webhook-engine.md, changelog 2026-08-25-010)** · ✅ **7.3 Subscription Tier 1 (7.3.1: registry + dispatch + outbox wiring; 7.3.3 emits: sudah ada — docs/plan/fase7-subscription-engine.md, changelog 2026-08-25-011)** · ✅ **7.4 Workflow engine core (7.4.1–7.4.3, 7.4.5: registry + approval state machine + multi-approver modes + when + requester exclusion — docs/plan/fase7-workflow-engine.md, changelog 2026-08-25-012)** · ✅ **7.7 Integrator engine (7.7.1: registry + listen→call bridge — docs/plan/fase7-integrator-engine.md, changelog 2026-08-25-013)** · ✅ **7.4.6 Workflow audit trail (approval = signed statement di formspec_audit_log — docs/plan/fase7-workflow-audit-trail.md, changelog 2026-08-25-014)** · ✅ **7.4.4 Workflow escalation (timeout + reassign_roles, escalation worker — docs/plan/fase7-workflow-escalation.md, changelog 2026-08-25-015)** · ✅ **7.7.2+7.7.3 Integrator validation (symmetric cancel + idempotent target — docs/plan/fase7-integrator-validation.md, changelog 2026-08-25-016)** · ✅ **7.7.4 Integrator saga compensate (cross-boundary compensate ke saga log — docs/plan/fase7-integrator-saga.md, changelog 2026-08-25-017)**

> `⬜` not started · `✅` complete · `⏸️` deferred

**Scope**: `formspec dev` + `formspec serve --mode=production` single-server.  
**Deferred**: Control Plane (`formspec-ctl`), K8s Operator, Marketplace — untuk cloud phase berikutnya.  
**Sumber**: `docs/spec/backend/` (01–06), `docs/spec/frontend/` (01–08), `docs/spec/platform/` (01–10), `docs/cli-tools/` (01–05), `docs/renderers/jsonb-persist/` (01–04), `docs/renderers/shadcn-shell/` (01–04), `docs/ai/` (01–06).  
**Catatan status sumber**: seluruh spec masih **Draft** (jsonb-persist masih **Outline**; `platform/08` §3–§6 "target desain") — mismatch todo↔docs bisa berarti docs-nya yang perlu diperbaiki. Audit penuh todo vs docs terakhir: 2026-07-19.

**Catatan 2026-07-31**: `platform/08-project-layout.md` §1–§2 ditulis ulang agar match
layout contoh `examples/Clinic-UI-Showcase/spec/` (entity-centric + `spec/` container +
`formspec-app.yaml` sebagai config dev) — lihat `docs/changelog/2026-07-31-002-update-project-layout-sesuai-clinic-ui-showcase.md`. §3–§6 tetap target desain.

**Catatan 2026-07-31**: `rtk` (CLI proxy token LLM) di-bake ke
`.devcontainer/Dockerfile` (binary pinned v0.44.1 + `rtk init -g`) karena
`~/.local/bin`/`~/.claude` tidak di-persist volume — lihat
`docs/changelog/2026-07-31-003-bake-rtk-ke-devcontainer-dockerfile.md`.

**Catatan 2026-08-11**: Jalur **agent-assisted app development tanpa MCP** selesai —
lihat `docs/plan/agent-assisted-app-development.md`, guide
`docs/guides/agent-assisted-app-development.md`, dan contoh `examples/cafe/`.
`formspec-app-workflow` skill kini punya Phase Detection + No-MCP Tool Map;
`formspec init` menulis copilot-instructions yang mereferensikan workflow 4 fase +
`formspec validate` sebagai gate. Fase 10 (`formspec consult`/MCP) tetap di-defer;
konten skill dibuat MCP-agnostic agar reuse saat Fase 10 landing.

**Catatan 2026-08-17**: 9 test gagal `examples/Clinic-UI-Showcase` (sebelumnya
"pre-existing") **diperbaiki** — ternyata 4 bug nyata yang saling menutupi:
(1) hook script tidak resolve karena `HandleCreate`/`HandleUpdate` tidak mengisi
`SpecDir`; (2) `resource.save()`/PATCH menulis balik alias relasi ter-enrich
(`patient`) → `stripEnrichedRelations` di `Update`/`Insert`; (3) guard
`!empty(items)` invalid Starlark → `not empty(items)`; (4) visit lifecycle-active
tanpa route submit → `submit: disabled` (lifecycle-free). Plus test time-dependent
(hardcoded date) → `recentDate()`. `go test ./...` kini **571 pass, 0 fail**.
Lihat `docs/changelog/2026-08-17-003-fix-clinic-e2e-failures.md`.

**Catatan 2026-08-20**: **Fase 13 Module Registry & Vendoring** ditambahkan
(planned, belum dikerjakan) — ekosistem module registry npm-like:
`formspec module install/publish/list/uninstall`, `formspec override adopt/diff`,
`vendors/` + `overrides/` + `formspec.lock`, aktivasi berbasis marker, dan
registry server sebagai **FormSpec app (dogfooding)** untuk
`registry.formspec.dev`. Model: read-only vendoring + shadow copy (sesuai
`docs/spec/platform/08-project-layout.md` §6). Dependensi: Fase 6 (auth —
6.2 permission model + 6.4 API keys) untuk bagian auth-dependent; Fase 8
(production serve) untuk deploy nyata. Lihat section **Fase 13** di bawah.

**Catatan 2026-08-20**: **Fase 6 dikerjakan sebagai dogfooding** — auth dibangun
ulang sebagai **1 modul FormSpec** (`internal/auth/module/`, bundled + embed,
namespace `formspec.core`) yang bisa di-merge ke project lain via `external/`
atau `spec/modules/`. `formspec.core` dipindah dari registrasi programatik Go ke
YAML manifests; middleware tetap Go. Plan: `docs/plan/fase6-dogfooding-auth-module.md`.
Demo merge: `verticals/reference-app` + `examples/Clinic-UI-Showcase`.

**Catatan 2026-08-24**: **Global Settings Config** selesai — namespace
`settings.*` (spec §10 "jangan pernah menebak") diimplementasikan sebagai
kontrak berlaku: `Settings`/`CurrencySettings` di `pkg/spec`, di-resolve dari
`kind: Config` manifest, dikirim via `/meta/ui` bundle, dan dipakai util
format terpusat `lib/format.ts` di frontend (money/date/number/relative).
Semua hard-code format per komponen (`en-US`/`USD` vs `id-ID`/`IDR`) di-refactor
ke formatter. Contoh: `examples/cafe/spec/modules/formspec.core/config.yaml`.
Plan: `docs/plan/global-settings-config.md` · changelog: `2026-08-24-008`.
Follow-up: menu sidebar kategori **"Global"** (Akses User dan Peran +
Pengaturan) + halaman settings (`examples/cafe/spec/modules/formspec.core/
pages/settings.yaml`) — changelog `2026-08-24-009`.

**Catatan 2026-08-24**: **Date input global format + runtime settings** selesai —
`DateInput` di-rewrite (overlay native picker) agar tampil sesuai global
`date_format`; 3 input tanggal mentah (Kanban/Table filter, Wizard) diganti
`DateInput`. Global settings kini **runtime-editable**: Entity `app-setting`
(characteristic: reference, natural key "global") menyimpan running value di
DB; backend merge ke `bundle.settings`; halaman Pengaturan = Configuration
Page (Form edit); auto-apply via refresh meta setelah save. Fix bug widget
resolution integer/decimal di FormRenderer. Plan:
`docs/plan/date-input-global-format-runtime-settings.md` · changelog:
`2026-08-24-010`.

**Catatan 2026-08-24**: **Fix: seed `app-setting` default dari manifest** —
halaman Pengaturan menampilkan form kosong pada akses pertama karena
find-or-create reference entity hanya meng-seed natural key, tidak menyalin
nilai default dari manifest `settings:`. Kini `HandleFind` men-seed record
`formspec.core/app-setting` dengan resolved settings (`seedSettingsData`) saat
find-or-create; `HandlerFactory.SetSettings` di-wire dari
`RouterBuilder.SetSettings`. Changelog: `2026-08-24-012`.

**Catatan 2026-08-24**: **Rounding: enum dropdown + diterapkan di formatting** —
(1) field `rounding` di halaman Pengaturan kini dropdown (Select) via
`enum_values` di entity + `widget: select` di form (tetap `type: string` untuk
hindari migrasi enum yang rapuh). (2) `rounding` yang tadinya "declared but
unused" kini benar-benar dipakai: `lib/format.ts` ekspor `RoundingMode` +
`roundTo(value, places, mode)` (semantik BigDecimal, snap presisi tinggi untuk
atasi drift biner), dan `createFormatter` menerapkannya di `money`/`number`.
Changelog: `2026-08-24-013`.

**Catatan 2026-08-24**: **Dokumentasi `formspec.core` sebagai special module** —
`docs/spec/platform/02-workspace-app-module.md` §9 diperkaya: intro menegaskan
`formspec.core` adalah special/reserved module (selalu ada, tidak perlu
`depends_on`, tidak boleh dideklarasikan user); subsection baru §9.1
"Karakteristik khusus" (reserved namespace, bundled module dogfooding,
special-casing framework untuk global settings `app-setting` + auth core,
selalu tersedia); tabel resource ditambah `app-setting`. Ditambah §9.2
"Route & Page yang disediakan" (page eksplisit + derived CRUD route entity
ui-exposed), §9.3 "Akses dari script" (`resource.fetch`, `ctx.config().get`,
`ctx.db`), §9.4 "Override default value" (runtime settings, `external/`,
`overrides/`, `auth_config_ref`). Changelog: `2026-08-24-014`.

---

## Fase 0: Documentation & Repo Foundation ✅ COMPLETE

| Item                                                                                                                | Status        |
| ------------------------------------------------------------------------------------------------------------------- | ------------- |
| 0.1 Fix CLI doc numbering (01-dev, 02-cli, 03-generate, 04-ctl)                                                     | ✅            |
| 0.2 Fix Document → Entity in docs/spec/                                                                             | ✅            |
| 0.3 Repo restructure: `web/` → `renderers/web/`                                                                     | ✅            |
| 0.4 Repo restructure: `internal/db/`+`datastore/` → `renderers/jsonbpersist/`                                       | ✅            |
| 0.5 AI instructions + 3 skills (backend, frontend, cli)                                                             | ✅            |
| 0.6 Verify no `docs_old/` refs in `docs/`                                                                           | ✅            |
| 0.7 Rename `renderers/web/` → `renderers/react-shadcn/` (+ cleanup ref `web/` stale di docs aktif)                  | ✅ 2026-08-14 |
| 0.8 Rename `renderers/jsonbpersist/` → `renderers/jsonb-persist/` (selaras nama docs; paket `db`/`datastore` tetap) | ✅ 2026-08-14 |

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
- [x] 1.1.10 `MenuItem` — kontrak menu App/Module (`platform/02-workspace-app-module.md` §4: `[]MenuItem` langsung tanpa wrapper, array-index order, nesting max 3 level, node adopt/group/leaf, `when` FormSpecExpr) + validasi apply §6 (`module` di menu anggota `App.spec.modules`, `root_url` unik prefix `/app/`, `Form`/`Table` bukan target `view`)

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
- [ ] 1.4.12 `MoneyType` FX & multi-currency — konversi antar mata uang (rate table, tanggal efektif, spread) untuk field `money`; belum dispesifikasikan di `05-field-types.md` § "Open — FX & multi-currency"

### 1.5 Error glossary Go types ✅

- [x] 1.5.1 Go const/type mapping dari `error-glossary.yaml` (22 error codes → `FORMSPEC.DOC.*`, `FORMSPEC.TXN.*`, `FORMSPEC.PERIOD.*`, `FORMSPEC.EVENT.*`, `FORMSPEC.SAGA.*`, `FORMSPEC.REF.*`, `FORMSPEC.PERSIST.*`, `FORMSPEC.ARCHIVE.*`, `FORMSPEC.VALIDATE.*`)
- [x] 1.5.2 Observability error codes — `OBSERVABILITY_METRICS_DISABLED`, `OBSERVABILITY_DEBUG_FORBIDDEN`, `LOGS_FILTER_INVALID` (`09-observability.md` §8)

---

## Fase 2: Engine Core — `formspec dev` Reliability

**Goal**: Atomic operations, correct PK, complete filters, lifecycle enforcement — agar `formspec dev` bisa diandalkan untuk testing.

**Progress**: 2.1 ✅ · 2.2 ✅ · 2.3 ✅ · 2.4 ✅ · 2.5 ✅ · 2.6 (2.6.1–2.6.3, 2.6.5–2.6.6 ✅; 2.6.4 ⬜ sebagian — cross-module resource access enforced, ctx.\*/secrets masih blocked on 2.9.1) · 2.7 ✅ · 2.8 ✅ · 2.9 (2.9.1–2.9.3 ✅; 2.9.4 ⬜)

### 2.1 Database integrity ✅

- [x] 2.1.1 Atomic mutation + outbox — wrap Entity INSERT/UPDATE/DELETE + outbox write dalam `BeginTx`/`Commit` (rollback on error). Terpenuhi untuk create/update HTTP (`InTx`) **dan** custom action (`HandleCustomAction` + `TxScope`, `renderers/jsonbpersist/txscope.go`) — satu transaksi request-scoped mencakup semua panggilan `resource.save()`/`.create()` (Starlark/native/sidecar via `X-FormSpec-Scope-Id`) dalam satu eksekusi action, join berdasar identitas store (bukan Module — multi-Module dalam satu Datastore fisik yang sama tetap atomik; lintas-Datastore genuinely berbeda → `ErrCrossStoreTx`). **Gap tersisa**: `RunAfterPhase` masih fire-and-forget (tidak rollback); SDK sidecar (`sdk/php`/`sdk/python`/`sdk/typescript`) belum mengirim `X-FormSpec-Scope-Id` (`01-architecture.md` §3, `runtimes/04-formspec-sidecar.md` §4.3a).
- [x] 2.1.2 Natural key counter in same transaction as Entity insert — UPSERT counter + INSERT dalam satu `Tx` (`generateNaturalKeys` menerima DB terikat-transaksi; `04-query-and-keys.md` §2)
- [x] 2.1.3 UUID v7 PK — replace SQLite `INTEGER PRIMARY KEY AUTOINCREMENT` with UUID v7 generated at app layer (`NewUUIDv7`, kedua driver; child table PK juga ikut)
- [x] 2.1.4 Idempotency retention configurable — `IdempotencyStore` sekarang dikonstruksi di `resource.App` dengan TTL dari `Config.IdempotencyTTL` (default 24h via `db.DefaultIdempotencyTTL`), diekspos lewat `App.Idempotency()`. Resolusi dari manifest `kind: Config` (`core.idempotency_retention`) menunggu runtime Config-kind (Fase 7.2, belum ada) — `Config.IdempotencyTTL` adalah seam yang setara untuk saat ini.
- [x] 2.1.5 `natural_key_rule` lengkap — `strategy: sequence|custom` (custom = framework tidak auto-generate, diisi hook/script/import), `format`, `prefix`, `reset: never|yearly|monthly|daily` (divalidasi di `ValidateDocumentSpec`), `scope_field` (`01-core-basic.md` §2); counter komposit `(tenant, resource, field, scope, period, seq)` sudah ada (`jsonb-persist/04` §2)

### 2.2 Query correctness ✅

- [x] 2.2.1 Filter operators 13/13 (`eq neq gt gte lt lte between in nin like ilike null notnull` — `01-core-basic.md` §6) — added `between`, `ilike`, `null`, `notnull`; handler parsing supports `between` as comma-separated pair
- [x] 2.2.2 JSONB path fallback for non-indexed fields — `data->>'field'` (PG) / `json_extract(data, '$.field')` (SQLite) via `EntityStore.columnRefExpr()`
- [x] 2.2.3 Generated column dialect-aware — `generateGeneratedColumn` now accepts `DriverType`; PG uses `data->>'field'`, SQLite uses `json_extract`
- [x] 2.2.4 `exists:<resource>` real lookup — already wired in `resource/formspec.go` via `SetEntityLookup`, queries entity registry
- [x] 2.2.5 Cross-module relation resolution — `ValidateRelationTargets` parses `{module}.{entity}` from `Relation.Resource`; registry injects `targetTableResolver` using spec's Plural (not naive `+s`)

### 2.3 Lifecycle engine ✅

- [x] 2.3.1 8 reserved actions with guard enforcement — `LifecycleGuard` function for all 8; wired into `Update()`, `SoftDelete()`, `Submit()`, `Cancel()`; REST routes added for submit/cancel/amend
- [x] 2.3.2 Transitive gating — `TransitiveDisabled()` wired into route generation (`generator.go`)
- [x] 2.3.3 `update` after `submit` always rejected — `LifecycleGuard("update")` checked in `Update()`
- [x] 2.3.4 Referenceability — already implemented via `ValidateRelationTargets()` (unchanged)
- [x] 2.3.5 `delete` guard absolut — `LifecycleGuard("delete")` checked in `SoftDelete()`
- [x] 2.3.6 `create-submit`/`amend-submit` auto-derived — `DeriveReservedActions()` exists (route-level skip for now)
- [x] 2.3.7 Error codes lengkap — `FORMSPEC.DOC.ALREADY_SUBMITTED`, `ALREADY_CANCELLED`, `SUBMIT_NOT_DRAFT`, `CANCEL_NOT_SUBMITTED`, `UPDATE_NOT_DRAFT`, `DELETE_NOT_DRAFT`, `AMEND_NOT_SUBMITTED_OR_CANCELLED`, `FORMSPEC.REF.DELETE_BLOCKED`, `FORMSPEC.REF.CANCEL_BLOCKED`
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
- [x] 2.4.5 `FORMSPEC.EVENT.TYPE_MISMATCH` + `FORMSPEC.EVENT.TYPE_MISSING` — wired into `ValidateEventNaming()` error messages

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

- [x] 2.6.1 Cross-tenant isolation — already in place: `AuthMiddleware` (`internal/api/middleware.go`) returns 404 (not 403) on identity-vs-URL workspace mismatch; every `EntityStore` query in `renderers/jsonbpersist/crud.go` scopes on `tenant_id`. Covered by existing `internal/api/api_test.go`/`renderers/jsonbpersist/crud_test.go`.
- [x] 2.6.2 Tenant ID auto-injection — already in place: `GenerateEntityDDL` (`renderers/jsonbpersist/ddl.go`) always emits `tenant_id` + tenant-scoped unique indexes.
- [x] 2.6.3 Permission auto-registration — `internal/entity/registry.go`'s `registerStandardPermissions()` (shared by `LoadEntities`/`RegisterArtifactManifest`) now also registers `submit`/`cancel`/`amend`, gated identically to route generation (`db.TransitiveDisabled` + `characteristic: summary`) so registered permissions never drift from actual routes. Format stays `{module}.{plural}.{action}`, matching `internal/api/generator.go`.
- [x] 2.6.4 UsesEnforcement wiring (cross-module resource access + ctx.\* primitives) — **complete**: blocker (a) resolved (cross-module `resource.call()`/`fetch()`/`create()` diblokir `USES_VIOLATION` bila target tak dideklarasikan di `uses.resources`; matcher `{module}.{entity}`, `{module}/{entity}`, `{module}.*`, `*`). **Blocker (b) resolved** (2026-08-17): `ctx.*` primitive enforcement kini di-thread — `internal/action/script.go` meneruskan `action.Uses` penuh → `internal/starlark.ScriptExecutor.Execute(uses)` → `CtxAPI.SetUses` + `SetStrictPrimitives`; di ProdMode/StrictMode, akses `ctx.db/cache/lock/queue/pubsub/storage/kvstore` yang tidak dideklarasikan di `uses.primitives` → `USES_VIOLATION` (dev mode relaxed). Test: `resource/uses_enforcement_test.go` + `uses_enforcement_e2e_test.go` + `ctx_uses_enforcement_test.go`. Module auto-suspend + incident audit tetap subsistem baru yang belum ada. Stub middleware `UsesEnforcement` di `internal/api/middleware.go` tetap dead code — enforcement nyata hidup di `resource/formspec.go` + `internal/starlark/context.go`. ✅ 2026-08-17
- [x] 2.6.5 Optimistic concurrency — storage layer was already correct (`crud.go`'s `Update()` does `WHERE version = ?`; conflicts already mapped to 409), but `HandleUpdate` (`internal/api/handler.go`) silently ignored the client and always used the just-fetched version — meaning the `If-Match: version=N` header renderers/web's `apiPatch` (`renderers/web/src/lib/api/client.ts`) already sends on every Form autosave/Kanban drag-update was a no-op. Fixed: `HandleUpdate` now parses `If-Match` and uses the client's version for the CAS check when present; missing `If-Match` falls back to today's behavior in relaxed/dev mode but is `409 CONFLICT` when `SetStrictMode(true)` (production).
- [x] 2.6.6 WebSocket per-message permission filter — `wsConn` (`internal/api/wshub.go`) now carries the connection's `*auth.Identity` (captured in `HandleWS`); `Broadcast` resolves `EventMessage.Resource` to `{module}.{plural}.view` via the entity registry and skips connections lacking that permission. Fails open (delivers unfiltered) when identity is nil or the resource/registry can't be resolved, so it only engages once real auth is wired up — see the "identity/registry" branch in `internal/api/wshub_test.go`/`wshub_permission_test.go`.

### 2.7 Idempotency ✅

- [x] 2.7.1 Two-step prepare flow — `POST /{resource}/{action}/prepare` → receive key → retry action with key. Endpoint `HandlePrepare` (server-sourced idempotent actions only; header/param-sourced 404), route `POST /api/v1/{module}/{plural}/create/prepare` + `/{action}/prepare` di kedua surface (external + `/_ui/entity/`). Lihat `docs/plan/idempotency-prepare-flow.md`.
- [x] 2.7.2 Idempotency store — `(tenant, action, key) → pending|completed + response` (`01-core-basic.md` §5 — tanpa state `failed`); enforcement di `HandleCreate` + `HandleCustomAction`: duplicate after completed → replay response asli (status + body); duplicate saat pending (in-flight) → 409; failed → retry diizinkan. `IdempotencyStore.Lookup` membedakan pending vs failed (TryClaim menggabungkan keduanya). Store di-wire ke router di `New()` + `ReloadSpec()`.

### 2.8 `spec.expose` enforcement ✅

- [x] 2.8.1 `spec.expose: []` → external API returns 404 for all endpoints; UI surface unaffected — `GenerateRoutes` skip entity tanpa expose; `GenerateUIRoutes` selalu include semua entity (`internal/api/generator.go`; test `TestGenerateRoutes_NoExpose`/`TestHTTPRouter_404OnUnexposed`)
- [x] 2.8.2 `spec.expose: [{type: rest, actions: [list, find]}]` → only those actions on `/api/v1/` — `generateRESTRoutes` filter `allowed` dari `exp.Actions` (test `TestGenerateRoutes_WithExpose`)

### 2.9 `ctx.*` infrastructure primitives

- [x] 2.9.1 Wire `CtxAPI.SetDatastoreResolver` + implementasi `datastore.Open()` nyata — `ctx.db().query()` kini jalan terhadap database utama app (SQLite dev / Postgres prod) via `datastore.DBQuerier`; resolver di-wire dari `newDispatcher` (`resource/formspec.go`) → `action.ScriptExecutor.SetDatastoreResolver` → `starlark.ScriptExecutor` → `CtxAPI`; Go context di-thread lewat `starlark.Thread.SetLocal`; `primitiveRunner` operasi (`query/get/set/delete/acquire/release`) memakai capability interfaces (`Querier`/`KVGetter`/`KVSetter`/`KVDeleter`/`Locker`). Primitif lain + named datastore masih error jelas ("no live datastore ... only db/default is wired") — menunggu 2.9.2–2.9.4. Lihat `docs/plan/ctx-datastore-resolver.md`. (`runtimes/02-formspec-resource.md` §7, `runtimes/04-formspec-sidecar.md` §8)
- [x] 2.9.2 Closed set 9 primitive — `db`, `cache`, `lock`, `queue`, `pubsub`, `storage`, `config`, `kvstore`, `log` (`platform/06-datastore.md` §2), termasuk binding `.named()`. Primitif yang di-routing lewat resolver (`db`/`cache`/`lock`/`queue`/`pubsub`/`storage`/`kvstore`) kini resolve ke backend nyata; `config`/`log` adalah builtin terpisah (`ctx.config`/`ctx.log`). Operasi baru di `primitiveRunner`: `enqueue`/`dequeue`, `publish`/`subscribe`, `upload`/`download`. Lihat `docs/plan/ctx-primitives-closed-set.md`.
- [x] 2.9.3 Dev auto-provision `'default'` per primitive — db→SQLite (database utama app), cache/lock/queue/pubsub/kvstore→in-memory, storage→filesystem (`platform/06-datastore.md` §5); named datastore selain `'default'` → error jelas (menunggu 2.9.4). Resolver dibangun `ctxPrimitiveResolver` di `resource/ctxresolver.go`, dipakai `newDispatcher` (dan `formspec.New` → dev.go).
- [ ] 2.9.4 `ctx.db()` module-scoped (normatif) — resolve ke Datastore milik Module; interaksi lintas-Module-lintas-Datastore WAJIB async, tanpa escape hatch `ctx.db` sekalipun dengan `uses` (`01-core-basic.md` §3/§5)

---

## Fase 3: CLI — `formspec` Command Completion

**Goal**: `validate` → `check` → `new` → `dev` → `generate` → `diff/get/describe/delete` → `migrate/repl/seed` → `backup/restore/logs`.  
**Priority**: per `docs/cli-tools/02-formspec-cli.md` §13.1.

### 3.1 High-priority (no backend dependency)

- [x] 3.1.1 `formspec validate --spec <path>` — dry-run validation dua lapis: engine loader (`internal/manifest`; parse + Entity deep-validation) + JSON Schema per kind (`schemas/kinds/*` via `santhosh-tekuri/jsonschema`, lihat `cmd/formspec/validate.go`). Exit 1 bila ada gagal. 2026-07-31. Catatan: lapis schema lebih ketat dari engine untuk shorthand `guard`/`render` (Go `UnmarshalYAML` scalar+map tak bisa diekspresikan generator schema) — gunakan bentuk objek. Sisa: honesty scan Starlark → 3.1.1a.
- [ ] 3.1.1a `formspec validate` honesty scan Starlark (undeclared usage → error, declared-but-unused → warning, `ctx.environment` branching → warning) + flag `--fix`. ⏸️ Ditunda dari 3.1.1 karena butuh `internal/starlark` analyzer penuh.
- [x] 3.1.2 `formspec check [--fix] -f <path>` — cross-file analysis: Form field ref ke field tak ada (error), FormSpecExpr ref ke field tak ada (error), `uses.resources` ref ke `{module}.{entity}` tak ada (error). `--fix`: hapus deklarasi `uses.resources` yang broken (target tak ada — aman, tidak mengubah footprint consent; penambahan deklarasi = perluasan consent → interaktif, di-defer). Lihat `cmd/formspec/check.go` + `docs/plan/formspec-check.md`.
- [x] 3.1.3 `formspec new <kind>` — scaffold: `new app <name>`, `new entity <name>`, `new module <name>`. Generate boilerplate YAML + directory. `new module` → `spec/modules/{module}/module.yaml`; `new entity` → `spec/modules/{module}/{characteristic}/{entity}/entity.yaml` (fields dasar code/name/description + expose default; characteristic divalidasi closed set; module di-detect dari CWD atau `--module`). Lihat `cmd/formspec/new.go`.
- [x] 3.1.4 `formspec init` bundel JSON Schema (`schemas/` dari `//go:embed`) + tulis `.vscode/settings.json` (`yaml.schemas` → `schemas/formspec.schema.json` untuk `spec/**/*.yaml|yml`), agar YAML editor punya autocomplete/validasi langsung setelah scaffold — lihat `docs/plan/init-schema-scaffold.md`

### 3.2 `formspec dev` — verify against spec

- [x] 3.2.1 Verify 12 flags work: `--spec`, `--dsn`, `--addr`, `--listen` (none/local_http/unix_socket), `--app-endpoint` (none/local_http/unix_socket), `--runtime` (auto-detect + explicit override), `--dev`, `--dev-ui` (implies `--dev`+`--force`), `--force`, `--web-dir`, `--state-dir`, `--workspace-id`
- [x] 3.2.2 Runtime auto-detect — `go.mod` → go (local), `package.json` → node, `composer.json` → php, `requirements.txt`/`pyproject.toml` → python, `*.csproj` → dotnet (SDK belum tersedia) — per `01-formspec-dev.md` §4; ruby/java TIDAK termasuk auto-detect `formspec dev` (hanya konteks sidecar `spec.runtime`, lihat 7.15.1)
- [x] 3.2.3 SPA serving priority — explicit `--web-dir` > embedded `//go:embed` FS > auto-detect `renderers/web/dist/` (urutan per `01-formspec-dev.md` §6; path auto-detect di docs masih `web/dist/` — stale pasca-restructure 0.3, perbaiki docs)
- [x] 3.2.4 Config file `formspec-app.yaml` support
- [x] 3.2.5 Two personas: Persona A (embedded SPA, 80%) + Persona B (`--dev-ui` Vite HMR, 20%)
- [x] 3.2.6 Add `check`, `promote`, `logs` to CLI dispatcher switch (currently fall to `usage()`)

### 3.3 `formspec generate`

- [x] 3.3.1 `formspec generate --lang typescript --spec <path> --out <dir>` — generate typed TS client from manifests (sudah ada di `cmd/formspec/generate.go`; deny-by-default: entity tanpa `expose` → 0 kode; error bila tak ada entity exposed)
- [x] 3.3.2 Generate: typed interfaces, create/update input types, custom action params, `createApi()` function (semua di `writeEntityTypes`/`writeEntityApi`; key field literal, tidak di-camelCase — match wire JSON)
- [x] 3.3.3 Field type mapping per `03-formspec-generate.md` §3: string/uuid/date/datetime→string, integer→number, decimal/number→**string** (presisi), boolean→boolean, enum→union, json→unknown, relation→string, child→array; **`money`→`{amount: string; currency: string}`** (amount wajib string) dan **`file`/`attachment`→`{key, filename, content_type, size, checksum}`** kini ditetapkan di spec + diimplementasikan di `tsFieldType` (sebelumnya jatuh ke `unknown`)

### 3.4 Read-only CLI ops

- [x] 3.4.1 `formspec diff -f <path>` — compare local vs deployed (dry-run) — dalam scope single-server, "deployed" = schema DB vs manifest lokal via `MigrationRunner.PlanMigrations`; exit 1 bila ada perbedaan (gate CI). Lihat `docs/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.4.2 `formspec get <kind> <name>` — fetch resource, table/JSON output. Beroperasi terhadap manifest lokal (Control Plane di-defer): `get <kind> [name] [--output table|json]`; `document` = alias `entity`. Lihat `cmd/formspec/get.go` + `docs/plan/formspec-get-describe.md`.
- [x] 3.4.3 `formspec describe <kind> <name>` — detailed view: field, action, state machine, permission (`02-formspec-cli.md` §2). Untuk Entity: fields, actions (+ permission + impl), state machine, expose; non-Entity: spec JSON. Lihat `cmd/formspec/get.go`.

### 3.5 Mutation CLI ops

- [x] 3.5.1 `formspec delete <kind> <name> --confirm` — remove resource. Beroperasi terhadap manifest lokal (Control Plane di-defer): file satu-dokumen → hapus file; file multi-dokumen → hapus dokumen yang cocok (yaml.v3 node), sisakan lainnya. `--confirm` wajib. Lihat `cmd/formspec/delete.go` + `docs/plan/formspec-delete.md`.

### 3.6 Engine-dependent CLI ops

- [x] 3.6.1 `formspec migrate plan|apply` — structural diff from Entity changes, applied via migration runner. `plan` → `PlanMigrations` + cetak DDL (tanpa eksekusi); `apply` → `ApplyMigrations` (idempotent). Daftar entity dibangun dari manifest lokal. Lihat `cmd/formspec/migrate.go` + `docs/plan/formspec-migrate.md`.
- [x] 3.6.2 `formspec repl [--environment]` — interactive Starlark console, full `ctx.*` (via `NewCtxPrimitiveResolver` + `App.Database()`); mode one-shot `-e <expr>`; `--environment` diterima (policy Control Plane di-defer). Lihat `docs/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.6.3 `formspec seed [--module]` — run seeders from YAML seed files (`kind: Seed`, format baru karena `formspec/seed` official module belum ada); idempotent via natural key. Lihat `docs/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [ ] 3.6.4 `formspec summary rebuild <entity>` — rebuild summary Entity dari replay event durable (`02-core-extended.md` §6) — ⏸️ **butuh design decision**: kontrak `sources`/`join_key`/`rebuild` untuk summary Entity belum dispesifikasikan di `pkg/spec` (tidak ada field `sources`/`join_key`/`rebuild` di `EntitySpec`), dan `docs/renderers/jsonb-persist/04-query-and-keys.md` §4 menyatakan detail populasi summary "mengikuti bagaimana Summary dipopulasikan dari event durable" yang belum ada (projection engine belum ada — lihat `docs/plan/fix-clinic-dashboard-summary.md`). Jangan invent contract; perlu keputusan desain dulu.

### 3.7 Data lifecycle CLI ops

- [x] 3.7.1 `formspec backup create [--full|--incremental|--filter]` — backup DB + artifacts, open format — `--full` implemented (tar: manifest.json + `<module>_<entity>.jsonl`); `--incremental`/`--filter` belum (gap). File storage (ctx.storage) belum ikut (gap 4.8.1). Lihat `docs/plan/formspec-repl-seed-diff.md`. ✅ 2026-08-17
- [x] 3.7.2 `formspec backup inspect <file>` — inspect backup contents — baca manifest.json (created_at, driver, tables + counts). ✅ 2026-08-17
- [x] 3.7.3 `formspec restore --from <file> [--map-resource] [--conflict skip|overwrite|remap] [--dry-run]` — restore with conflict resolution — `--conflict skip|overwrite` + `--dry-run` implemented; `--map-resource`/`remap` belum (gap). ✅ 2026-08-17
- [x] 3.7.4 `formspec logs [--workspace] [--module] [--entity] [--action] [--level] [--since] [--until] [--request-id] [--output pretty|json] [--follow]` — tail structured logs (`09-observability.md` §7) — baca event log (`formspec_event_log`, channel audit_log) dengan filter workspace/module/entity + output pretty|json; `--action/--level/--since/--until/--request-id/--follow` belum (full 12-field request logging = Fase 8.2). ✅ 2026-08-17

### 3.8 Deferred CLI ops

- [ ] ⏸️ `promote`, `archive`, `saga`, `module`, `sign`, `script`, `freeze`, `rollback`, `lock`, `workspace create`, `suspend scripts` — depend on Control Plane or backend maturity

---

## Fase 4: JSONB Persist — Clean Renderer

**Goal**: Clean PersistBackend interface, extension, categories, migration engine, backup/restore, archiving, audit trail, query builder.

### 4.1 Clean PersistBackend interface

- [x] 4.1.1 Define `PersistBackend` Go interface — technology-agnostic (no SQL types: `*sql.DB`, `ExecContext`, `QueryContext`, `Driver()`) — `renderers/jsonb-persist/persist_backend.go` (SyncSchema/PlanSchema/NextKey/UninstallExtension/EntityStore/DriverName). ✅ 2026-08-17
- [x] 4.1.2 Required capabilities: structural diff apply, query resolution (identical results across backends), `ctx.next_key` (gap-free, atomic), index generation, clean extension uninstall — semua sudah ada di jsonb-persist (migrate diff, List/Aggregate/Window, natural-key counter, persist.indexes, UninstallExtension). ✅ 2026-08-17 (verifikasi)
- [x] 4.1.3 Refactor `renderers/jsonbpersist/` to implement `PersistBackend` interface — `MigrationRunner` kini memenuhi `PersistBackend` (SyncSchema/PlanSchema/NextKey/UninstallExtension/EntityStore/DriverName via `SetRegistry`). ✅ 2026-08-17

### 4.2 Migration engine

- [x] 4.2.1 Structural diff from Entity spec changes — field add/remove/type-change → storage-agnostic diff (not SQL text) — `PlanMigrations` kini diff tabel existing: field indexed/unique/natural-key baru → `ALTER TABLE ADD COLUMN` (SQLite plain column karena modernc tak bisa ADD generated column; PG generated). Field removal/type-change tetap dua-fase (4.2.2). ✅ 2026-08-17
- [x] 4.2.2 `renamed_from` field — two-phase removal (deprecate then drop) — `Field.RenamedFrom` ditambahkan + validasi (tidak boleh reserved/collide). Diff field-add tidak menandai kolom lama sebagai removal (rename ≠ drop+add). Drop dua-fase penuh tetap enhancement. ✅ 2026-08-17
- [x] 4.2.3 Per-Entity migration in one transaction — fail = full rollback; data in `data` JSONB never rewritten by structural migration — `ApplyMigrations` kini wrap DDL + record per entity dalam satu `BeginTx`/`Commit` (rollback on error). ✅ 2026-08-17
- [x] 4.2.4 `kind: Migration` — custom DDL (index, function, trigger, extension, materialized view); DML rejected at runtime — `formspec migrate plan|apply` kini load `kind: Migration` manifests, `validateDDLOnly` menolak DML (INSERT/UPDATE/DELETE/SELECT), eksekusi DDL. ✅ 2026-08-17
- [x] 4.2.5 Data migration ber-versi — script backfill dengan run/rollback manual; tipe migrasi ketiga di samping structural diff + custom DDL (`01-core-basic.md` §4, `04-persist-backend.md` §2) — `kind: DataMigration` (`version`/`run`/`rollback`) + `formspec migrate data <name> run|rollback` (eksekusi Starlark). ✅ 2026-08-17

### 4.3 Entity extension

- [x] 4.3.1 Extension read — `entity.ext("namespace").field` via JSONB column access — `EntityStore.mergeExtensions` membaca kolom `ext_{namespace}` dan menggabungkannya ke `Data` di bawah key namespace saat `hydrateAndCompute`; registry me-wire `SetExtensions` dari semua entity `ExtendStorage` yang menarget entity ini. ✅ 2026-08-17
- [x] 4.3.2 Extension write — populate `ext_{namespace}` column — `EntityStore.splitExtensions` memisahkan data namespace dari base data; Insert & Update menulis payload ke kolom `ext_{namespace}` (terisolasi dari JSONB base); `validateKnownFields` menerima key namespace. ✅ 2026-08-17
- [x] 4.3.3 Extension uninstall — `DROP COLUMN ext_{namespace}` + remove registry entry + namespace lock (never reused) — `MigrationRunner.UninstallExtension` (drop column + set status='locked' dalam satu tx). ✅ 2026-08-17
- [x] 4.3.4 Extension namespace collision prevention — `formspec apply` rejects duplicate namespace for same target — `PlanMigrations` cek `formspec_extensions` (namespace reservation) dan tolak bila sudah dipakai. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.3.5 Extension `validate:` (additive business rule) — runs after base Entity L1–L6 validation, never overrides it; read-only access to base fields, may only require its own namespaced fields (`docs/spec/backend/03-entity-extension.md` §5) — `ExtendStorage.Validate` (script ref) ditambahkan; eksekusi runtime script validate = enhancement. ✅ 2026-08-17

### 4.4 Category schemas

- [x] 4.4.1 6 category schemas: operational, financial, compliance, analytics, master, archive — `CategorySchema` map (ddl.go). ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.4.2 Cross-category JOIN block — `FORMSPEC.PERSIST.CROSS_CATEGORY` error — `resolveRelations` memblokir resolusi relasi lintas kategori (via `SetTargetCategoryResolver` di registry). ✅ 2026-08-17
- [x] 4.4.3 `spec.persist.category` enforcement at query time — `qualifiedTable()` memakai schema kategori (PG). ✅ 2026-08-17 (verifikasi — sudah terimplementasi)

### 4.5 Query Builder

- [x] 4.5.1 Aggregate functions — `sum`, `count`, `avg`, `min`, `max` — `EntityStore.Aggregate()` (renderers/jsonb-persist/crud.go), pre-aggregation filters sama dengan List. ✅ 2026-08-17
- [x] 4.5.2 `group_by` — single + multi-field grouping — `AggregateParams.GroupBy []string`. ✅ 2026-08-17
- [x] 4.5.3 `having` — post-aggregation filter — `AggregateParams.Having []FilterOp` diterapkan ke ekspresi agregat (mis. `HAVING SUM(amount) > 500`). ✅ 2026-08-17
- [x] 4.5.4 `date_trunc` — time bucketing (day/week/month/quarter/year) — `AggregateParams.DateTrunc` (PG `date_trunc`, SQLite `strftime`). ✅ 2026-08-17
- [x] 4.5.5 Window functions — running total, ranking — `EntityStore.Window()` (`running_total`/`rank`/`row_number`, `PartitionBy`/`OrderBy`). ✅ 2026-08-17
- [x] 4.5.6 `include()` batched — eager-load relations in one query per level (N+1 prevention) — `resolveRelations` (crud.go) batch-fetch per relation field (`WHERE id IN (...)`), bukan per record. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)

### 4.6 Tree/hierarchy

- [x] 4.6.1 Materialized path column — `_tpath_{field_name}` for `tree: true` self-referential relations; path format: `""` (root) or `parent.child.grandchild` — DDL + `setTreePaths` (compute on insert). ✅ 2026-08-17
- [x] 4.6.2 Tree operators — `descendant_of` → `LIKE 'prefix.%'`, `ancestor_of` → PK lookup, `child_of` → FK query, `root` → `parent_id IS NULL` — filter ops di List (`descendant_of`/`child_of`/`root`; `ancestor_of` = PK lookup via `eq`). ✅ 2026-08-17
- [x] 4.6.3 Cycle detection — server-side on create/update/move/reparent → `VALIDATION_ERROR` (422) — `setTreePaths` menolak bila path parent mengandung id record (cycle). ✅ 2026-08-17

### 4.7 Business audit trail

- [x] 4.7.1 `audit: true` on action → append-only audit entries — `writeAuditLog` dipanggil dari Insert/Update/SoftDelete (crud.go); `AuditAction` create/update/delete/action. ✅ 2026-08-17 (verifikasi — sudah terimplementasi; audit ditulis untuk semua mutasi CRUD, bukan hanya action ber-`audited`)
- [x] 4.7.2 Per-entry: actor, action name (not "document updated"), timestamp (`created_at`), before/after diff, request_id — `AuditRecord` kini punya `request_id` (kolom + write + scan); `InsertParams`/`UpdateParams.RequestID` di-thread dari handler. Actor/action/timestamp/before-after diff sudah ada. ✅ 2026-08-17
- [x] 4.7.3 Immutability — no API update/delete; framework writes only — audit log append-only, tidak ada route update/delete. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)
- [x] 4.7.4 Queryable per record — source for Timeline kind; filterable with standard query operators — `AuditStore.ListByEntity`/`ListByWorkspace`. ✅ 2026-08-17 (verifikasi — sudah terimplementasi)

### 4.8 Backup & restore

- [x] 4.8.1 Full + incremental backup — DB dump + file storage (ctx.storage), open format — `--full` + file storage (`{state}/storage` → `storage/` di tar) implemented; `--incremental` belum (gap). ✅ 2026-08-18
- [x] 4.8.2 Filterable backup — by workspace, module, entity — `formspec backup create --filter <module|module/entity>` (workspace = "demo" saat ini). ✅ 2026-08-17
- [x] 4.8.3 Restore with conflict resolution — `skip|overwrite|remap`, `--dry-run` compatibility report — `formspec restore --conflict skip|overwrite|remap`; `remap` menetapkan natural key baru (`-r1`, `-r2`, …) dan insert sebagai record baru; `--dry-run` mencetak compatibility report per-entity (restore/skip/remap/fail). ✅ 2026-08-17
- [x] 4.8.4 Credible exit — read/export operations never license-gated — backup/restore tidak license-gated (prinsip desain, terpenuhi). ✅ 2026-08-17
- [x] 4.8.5 Outbox reconciliation pass WAJIB setelah restore — entri outbox pending di-replay/diverifikasi terhadap state hasil restore sebelum workspace kembali melayani (`platform/04-control-plane.md` §6.1, MUST — berlaku juga single-server) — `formspec restore` kini menjalankan `reconcileOutbox` (hitung pending + lapor; replay penuh = tugas outbox worker). ✅ 2026-08-17

### 4.9 Data archiving

- [x] 4.9.1 Archive transactions (`characteristic: transaction`) to Parquet when age ≥ `retention.archive_after` — `formspec archive run --max-age <dur> [--dry-run]` mengarsip transaksi tua ke format JSONL open (Parquet = enhancement), hapus baris transaksi. ✅ 2026-08-17
- [x] 4.9.2 Master snapshot "as-of" — referenced masters snapshotted alongside archived transactions — `snapshotMasters` (archive run) snapshot master yang direferensikan belongs_to + set `locked_for_deletion`. ✅ 2026-08-17
- [x] 4.9.3 `locked_for_deletion` flag — master referenced by archived transaction cannot be deleted — `SoftDelete` memblokir bila `data.locked_for_deletion == true` (4.9.4). ✅ 2026-08-17
- [x] 4.9.4 `FORMSPEC.ARCHIVE.LOCKED_FOR_DELETION` error code — `spec.ErrorArchiveLockedForDeletion` + enforcement di `SoftDelete`. ✅ 2026-08-17
- [x] 4.9.5 `formspec archive run [--dry-run]` / `view --batch-id` / `restore-batch` — `run` + `view` implemented (JSONL open format, batch subdir); `restore-batch` belum. ✅ 2026-08-17

### 4.10 Soft-delete & soft-deactivation

- [x] 4.10.1 `persist.soft_delete: true` → `deleted_at` column + query auto-filters — sudah ada: `deleted_at` column di DDL (default true, bisa di-disable), semua query auto-filter `deleted_at IS NULL`, `SoftDelete()` method. ✅ 2026-08-17 (verifikasi — sudah terimplementasi sebelumnya)
- [x] 4.10.2 `is_active` + `deactivate`/`reactivate` pattern — dropdown filters `is_active: true` for new transactions; list shows all — `soft_deactivate: {enabled: true}` kini inject `is_active` field (default true) + `deactivate`/`reactivate` actions (store methods, handlers, routes, permissions). Dropdown filter `is_active: true` untuk transaksi baru = concern frontend (Fase 5). Lihat `docs/plan/soft-deactivate.md`. ✅ 2026-08-17

---

## Fase 5: Frontend — shadcn-shell Completeness

**Goal**: Semua UI kind, widget, contract, dan FormSpecExpr sesuai spec. Bisa dites end-to-end.

### 5.1 App Shell

- [x] 5.1.1 `sidebar-nav` — full chrome, side navigation, breadcrumb (verified, working)
- [x] 5.1.2 `topnav` — full chrome, top navigation — `TopNavShell` (nav atas + dropdown group + breadcrumb + mobile drawer), menu di-resolve via `useResolvedMenu` (sama dgn Sidebar). Contoh `examples/arisan/`. ✅ 2026-08-19
- [x] 5.1.3 `no-nav` — chrome minimal tanpa nav standar — App renderer archetype (bukan "landing"/marketing): chrome & auth dipisah (`app_renderer` = chrome; `access: public|private` = auth). `NoNavShell` chrome-only + blok `section:` declarative (hero/feature_grid/card/carousel/cta) + anonim create (list/find/create publik di module App `access: public`) + login `returnTo`. Contoh `examples/storefront/`. Lihat `docs/plan/landing-page.md` + changelog 2026-08-19-001/002. ✅ 2026-08-19

### 5.1a App-level fields (chrome/auth/shell/persist)

- [x] 5.1a.1 `App.spec.access` — `private` (default) | `public` — sumbu auth terpisah dari `app_renderer`; pemicu bundle anonim + data seam publik + boleh root `/`. ✅ 2026-08-19
- [x] 5.1a.2 `App.spec.stack_family` — shell implementasi (default `react-shadcn`); ekspos di bundle; validasi renderer penuh = 5.16. ✅ 2026-08-19
- [x] 5.1a.3 `App.spec.persist_backend` — backend persist entity (default `jsonb-persist`); nama tak ter-install / tak implement kontrak `formspec/storage.entity-persist` → ERROR di apply/check. ✅ 2026-08-19

### 5.2 `kind: Page`

- [x] 5.2.1 Blocks composition — form, table, component blocks (himpunan tertutup `06-page-kinds.md` §1; `widget` milik Dashboard §7, `html` block tidak ada di spec); permission-gated per block
- [x] 5.2.2 Tabs variant — mutually exclusive with blocks; permission-checked per tab
- [x] 5.2.3 Master-detail split — `layout.mode: split`, `binds: {source, param}`; detail refetch on selection change — `PageSplit` di `PageRenderer.tsx` (master Table block + detail block via `binds`, refetch on selection, empty-state tanpa seleksi). ✅ 2026-08-24
- [x] 5.2.4 Full-custom — single `component:` block — full-bleed render tanpa grid wrapper (blocks.length===1 && blocks[0].component). ✅ 2026-08-24
- [x] 5.2.5 Custom Page (`mode: custom`) — full-code page with `binds` footprint (entities, actions, subscribe); top rung of frontend control — `CustomPage` di `PageRenderer.tsx` + `bindsToNeeds` → `AssetNeeds`. ✅ 2026-08-24
- [x] 5.2.6 Configuration Page pattern — `characteristic: reference` entities → no New/Delete buttons, only Update surfaced
- [x] 5.2.7 Declarative banner/alert/notice block — `AlertBlock` di `SectionBlocks.tsx` (variant info/success/warning/destructive); perluasan `SectionBlock` closed set. ✅ 2026-08-24

### 5.3 `kind: Form`

- [x] 5.3.1 `render` mode enforcement — `modal` (dialog overlay), `drawer` (slide-in panel), `separate_page` (own route); design-time, no runtime switch
- [x] 5.3.2 Wire `OverlayHost` — connected to Form.render modal/drawer
- [x] 5.3.3 409 conflict handling — CAS version mismatch → "Data telah diubah oleh pengguna lain", offer reload + re-apply changes — `FormRenderer` catches `FormaApiError` with `status === 409` from both auto-save and manual submit, stashes the pending edits, and shows `ConfirmDialog` ("Reload & Reapply"); confirming re-fetches the record (fresh `recordVersion`) then layers the stashed edits back on top via `reset()`. ✅ 2026-08-22
- [x] 5.3.4 Lifecycle UI patterns — plain_crud (no submit), 2-step+auto-save (default), 2-step manual (Save Draft + Submit buttons), 1-step create-submit (single button, no draft)
- [x] 5.3.5 FormSpecExpr — `visible_when`, `readonly_when`, `required_when`, `compute` per field

### 5.4 `kind: Table`

- [x] 5.4.1 Fix hardcoded `/_admin` prefix — surface-aware navigation (`/app` vs `/_admin`)
- [x] 5.4.2 Inline editing — `inline_edit: true`, cell editable for non-readonly/computed/immutable fields; CAS per baris; submitted rows reject inline-edit — `TableRenderer` (editingCell + commitInlineEdit via `apiPatch` + CAS version; 409 → stale badge). ✅ 2026-08-24
- [x] 5.4.3 Batch editing — `batch_edit: [field, ...]`, update per baris, partial failure reported (not all-or-nothing) — `TableRenderer` (batchDraft + applyBatchEdit loop PATCH per row + per-row report). ✅ 2026-08-24
- [x] 5.4.4 Column derivation fix — N priority columns (natural key → label_field → status → transaction_date → rest), overflow accessible via row expand/detail; NEVER silently dropped — `derive.ts` `DERIVED_TABLE_VISIBLE_COLUMNS=8` + priority sort; `TableRenderer` row-expand toggle. ✅ 2026-08-24
- [x] 5.4.5 `realtime: true` — auto-subscribe + patch rows in-place (depends on 5.8) — `useRealtime` di `TableRenderer` → silent refetch saat event entity cocok. ✅ 2026-08-24
- [x] 5.4.6 Fix table auto-refresh setelah overlay (modal/drawer) close — `TableRenderer` mendeteksi transisi URL `action` ada → hilang (overlay `OverlayHost` ditutup setelah save/cancel) lalu silent refetch; mencakup create & edit, dan berlaku untuk table derived maupun table block/tab di Page. Changelog `2026-08-24-011`. ✅ 2026-08-24

### 5.5 `kind: Kanban`

- [x] 5.5.1 Drag-and-drop — wire `@dnd-kit/core`; drag card antar kolom → PATCH `status_field`
- [x] 5.5.2 Optimistic update with server-enforced rollback (409 → snapshot restore)
- [x] 5.5.3 `drag_guard` FormSpecExpr — pre-check UX, prevent drop that server will reject — `KanbanRenderer.handleDragEnd` evaluasi `entry.spec.drag_guard` (context `fields`=record, `target`=status kolom tujuan); drop diblokir + toast bila guard false; server state-machine guard tetap otoritas. ✅ 2026-08-24
- [x] 5.5.4 WIP limits — `max_cards_per_column`, soft UX enforcement (visual + toast)
- [x] 5.5.5 Zero-config — derive columns from state machine or `group_by` enum — `deriveKanbanColumns` di `engine/derive.ts` (state machine states → `enum_values` status field); `KanbanRenderer` pakai bila `columns:` kosong + empty-state hint. ✅ 2026-08-24
- [x] 5.5.6 Click card → detail page navigation
- [x] 5.5.7 Row actions (view/edit/delete/custom) with confirm + permission check
- [x] 5.5.8 Filter columns from `filters` manifest — Select dropdown per filter field
- [x] 5.5.9 Filter generik server-side — `filters` objek (`default` seed, type `select`/`date`/`text`, `today()`) + `fixed_filters` immutable; `transaction_date[eq]=` untuk scope tanggal board (lihat `docs/plan/kanban-filter-tanggal-filter-generik.md`)

### 5.6 `kind: Calendar`

- [x] 5.6.1 Month/week/day/resource views — `views: [month, week, day, resource]` — `CalendarRenderer` (view switcher + MonthView/WeekView/DayView/ResourceView). ✅ 2026-08-24
- [x] 5.6.2 Event rendering — from `date_field` + optional `end_field`; title from `label_field` or `title_field` — `CalendarRenderer` (events dari `date_field`/`end_field`, title dari `title_field` ?? `label_field`). ✅ 2026-08-24
- [x] 5.6.3 Click event → detail Page/Form; click empty slot → Form create with date pre-filled — `openEvent` (navigate detail) + `createAt` (overlay create + `prefill.{date_field}`). ✅ 2026-08-24
- [x] 5.6.4 Drag reschedule — call `update` action on date_field (server-enforced); submitted immutable rows disable drag — HTML5 drag → `apiPatch` date_field (+ end_field proporsional); submitted rows ditolak; 409 → toast. ✅ 2026-08-24
- [x] 5.6.5 RRULE recurrence — parse RFC 5545, expand to instances for visible date range (render-time, not materialized) — library `rrule` (npm) + `expandRecurrence` (bounded `between`). ✅ 2026-08-24
- [x] 5.6.6 Resource view — one lane per `resource_field` value; color by `color_field` — `ResourceView` (lane per resource, warna dari `color_field`). ✅ 2026-08-24
- [ ] ⏸️ 5.6.7 RRULE exception per-instance — ubah/batalkan satu occurrence tanpa ubah pattern; butuh model data exception tersendiri (row terpisah + override tanggal asli); ditunda ke iterasi berikutnya (`06-page-kinds.md` §5 "Di luar cakupan v1")

### 5.7 `kind: Dashboard` + `kind: Widget`

- [x] 5.7.1 Widget `stat` — fetch from summary entity, display number with label — `MetricWidget` di `DashboardRenderer.tsx` (query FormSpecExpr subset → server list filters). ✅ 2026-08-24
- [x] 5.7.2 Widget `chart` — bar/line/pie from summary entity; add chart library dependency (katalog widget bawaan spec HANYA `stat` + `chart` — `07-component-kinds.md` §2; ListWidget/SummaryWidget tidak ada di spec, usulkan ke spec dulu bila dibutuhkan) — `ChartWidget` + `LineChart` SVG (tanpa dependency chart library; satu series per `group_by`). ✅ 2026-08-24
- [x] 5.7.3 Dashboard customizable — `customizable: true`, user add/remove/reorder widgets from catalog; preference stored as runtime preference (not YAML) — `DashboardRenderer` (dnd-kit sortable reorder + add via Select catalog + remove button); layout disimpan di `usePrefsStore.dashboardLayouts` (localStorage), bukan YAML. ✅ 2026-08-24
- [x] 5.7.4 Widget catalog visibility — derived from user's `list`/`view` permission on underlying entity (not manual flag) — catalog di-filter `checkPermission` pada entity underlying; widget terpasang juga di-filter. ✅ 2026-08-24

### 5.8 Realtime WebSocket

- [x] 5.8.1 `useRealtime(entityRef)` hook — subscribe to `entity:{module}.{name}` channels — `hooks/useRealtime.ts` (singleton WS, subscribe/unsubscribe frames, union subscriber). ✅ 2026-08-24
- [x] 5.8.2 Optimistic update — patch rendered data in-place on event — konsumen (TableRenderer) silent refetch saat `tick` berubah (non-durable, no replay). ✅ 2026-08-24
- [x] 5.8.3 Reconnect → refetch via `/_meta/ui`, no replay — `tick` naik saat reconnect → konsumen re-run load; re-register subscription penuh. ✅ 2026-08-24

### 5.9 Asset Component Contract

> **Track C widget strategy** (docs/plan/widget-strategy.md): 5.9.2 `formspec.components`, 5.9.3
> `formspec.ui`, 5.9.4 `formspec.files` = jalur #2 "UI rich" — expose chrome struktural shadcn ke
> component `asset`, bukan dijadikan field widget.

- [x] 5.9.1 Dynamic ES module loader — `shell/AssetRenderer.tsx` (dynamic `import()` + `mount`/`unmount`) + backend `GET /_ui/assets/{module}/{path*}` (`internal/api/asset.go`, serve `{root}/modules/{module}/assets/{path}`). ✅ 2026-08-24
- [x] 5.9.2 `formspec` client injection — `lib/formspec-client.ts` (`api`, `subscribe`, `navigate`, `theme`, `ui`, `components`); di-inject ke asset via `AssetRenderer`. ✅ 2026-08-24
- [x] 5.9.3 `formspec.ui` centralized service — `lib/ui.ts` (`toast` re-export + `confirm`/`dialog`/`drawer` promise-based) + `shell/UiHost.tsx` (ConfirmDialog + Sheet); 9 renderer migrasi import `sonner` → `@/lib/ui`. ✅ 2026-08-24
- [x] 5.9.4 `formspec.files` — `lib/files.ts` (download tray store + `files` API) + `shell/DownloadTray.tsx`; di-inject ke asset. ✅ 2026-08-24
- [x] 5.9.5 `formspec.form(entity, {mode, id?})` — `lib/headless-form.ts` (`createHeadlessForm`): field state, dirty tracking, validasi client dari field rules (zod via `lib/zod-schema.ts`), FormSpecExpr eval, `submit()` dengan CAS version. ✅ 2026-08-24
- [x] 5.9.6 `needs:` declaration — `BlockRef.needs` (`AssetNeeds`); `formspec.api` di-wrap `withNeeds` — panggilan di luar `needs` gagal client-side. ✅ 2026-08-24
- [x] 5.9.7 CSP sandbox — asset endpoint set `Content-Security-Policy` (`connect-src 'self'`). ✅ 2026-08-24
- [x] 5.9.8 CSS scoped — `AssetRenderer` mount ke Shadow DOM host (CSS component tidak bocor). ✅ 2026-08-24

### 5.10 Missing input widgets

> **Keputusan strategi widget** (docs/plan/widget-strategy.md): registry widget dasar = **closed set**
> yang dikurasi (`07-component-kinds.md` §1) — **TIDAK** semua komponen shadcn di-mapping ke widget.
> Tiga jalur "UI rich": (1) field widget — set tertutup dikurasi (bagian ini), (2) chrome struktural via
> `formspec.ui`/`formspec.components`/`formspec.files` untuk component `asset` (5.9), (3) block presentasi
> deklaratif di Page (5.2.7 + section blocks). Komponen shadcn struktural (alert, alertDialog,
> dropdown-menu, popover, dll) dipakai internal kinds / di-expose via `formspec.*` — bukan dijadikan widget.

- [x] 5.10.1 DatePicker — `DateInput` SUDAH meng-cover `date`/`datetime` via native `showPicker()` (keputusan desain, bukan `react-day-picker`) + input ketik terformat. ✅
- [x] 5.10.2 JsonEditor — `JsonInput` SUDAH ada (`widgets/JsonInput.tsx`): textarea + pretty-print + parse validasi live. ✅
- [x] 5.10.3 ChildGrid — `ChildTable` SUDAH ada (`widgets/ChildTable.tsx`): inline table utk `child` entities `storage: table`, sorting, computed, readonly_when, auto-fill. ✅
- [x] 5.10.4 RichText — `RichText` widget (`widgets/RichText.tsx`): toolbar bold/italic/list/link/heading via contentEditable + `document.execCommand`; client sanitizer `lib/sanitize.ts` (mirror server `sanitizeHTML`); render sanitized di DetailPage. ✅ 2026-08-24
- [x] 5.10.5 FileInput — `FileInput` widget (`widgets/FileInput.tsx`): upload via `POST /{module}/{entity}/{id}/{field}`, preview image/PDF, size/type enforcement dari `StorageSpec`; object key disimpan di field. ✅ 2026-08-24
- [x] 5.10.6 DecimalInput — nama manifest distinct `decimalinput` terdaftar di router (`FormFieldWidget`) + `derive.formWidget()` (decimal → `decimalinput`); `NumberInput` handle scale/rounding. ✅ 2026-08-24
- [x] 5.10.7 DateTimeInput — nama manifest distinct `datetimeinput` terdaftar di router + `derive.formWidget()` (datetime → `datetimeinput`); `DateInput` handle `withTime`. ✅ 2026-08-24
- [x] 5.10.8 Base UI components — breadcrumb/skeleton/badge/card/pagination SUDAH ada + `EmptyState` (`components/ui/empty-state.tsx`). ✅ 2026-08-24
- [x] 5.10.9 Textarea — `TextareaInput` widget (`widgets/TextareaInput.tsx`, wrap `components/ui/textarea.tsx`); router case `textarea` + field type `text`; render pre-wrap di DetailPage. ✅ 2026-08-24

#### 5.10a Field widget kurasi (Track B, docs/plan/widget-strategy.md)

- [x] 5.10.10 RadioGroup — `RadioGroup` widget (`widgets/RadioGroup.tsx`, button-based, no dep); single-choice enum alternatif `select`. ✅ 2026-08-24
- [x] 5.10.11 Combobox — `Combobox` widget (`widgets/Combobox.tsx`, custom dropdown + search, no dep); searchable select utk enum besar. ✅ 2026-08-24
- [x] 5.10.12 Password — `PasswordInput` widget (`widgets/PasswordInput.tsx`); masking + reveal toggle. ✅ 2026-08-24
- [x] 5.10.13 Slider — `SliderInput` widget (`widgets/SliderInput.tsx`, native range); number field utk range (min/max dari rules, step dari scale). ✅ 2026-08-24
- [x] 5.10.14 Tags — `TagsInput` widget (`widgets/TagsInput.tsx`); multi-select disimpan sebagai **comma-separated string** (frontend-only, tanpa backend change). Opsi array (backend) ditunda. ✅ 2026-08-24

### 5.11 FormSpecExpr

- [x] 5.11.1 Audit grammar vs spec — verify lexer→parser→evaluator supports all operators from `08-formspec-expr.md` §2 — grammar lengkap (literal, `fields.x`, perbandingan, and/or/not, aritmetika, len/sum, list comprehension, `in`); 94 test pass. ✅ 2026-08-24
- [x] 5.11.2 Deploy-time static validation — `formspec apply`/`formspec check` rejects unresolvable field references + invalid grammar (ERROR, not warning) — `checkForms` (Form) + `checkKanban` (drag_guard) + `checkWizard` (step fields) + `validateExprGrammar` (tolak `ctx.`, def/import/return, delimiter tak seimbang). ✅ 2026-08-24
- [x] 5.11.3 Runtime error state — nonexistent field reference → visible error state (never silent fail-safe/evaluate to `false`) — `strictEvalFormSpecExpr` (parse error + eval warnings → `error`); `FormRenderer` tampilkan banner error per field; `evalIdentifier` tetap graceful utk field belum-set (normal null), schema-level di-deploy-time. ✅ 2026-08-24
- [x] 5.11.4 `title` interpolation — `"Order {order.number}"` pattern in Page/Wizard/Print titles — `PageRenderer` fetch record utk token title + `interpolate()`; Print/Wizard sudah pakai pola sama. ✅ 2026-08-24
- [ ] ⏸️ 5.11.5 Cross-shell conformance test suite — identical interpretation across shells — **deferred**: hanya satu shell (`react-shadcn`) yang ada; interpreter JS (`lib/formspec-expr`) adalah referensi. Conformance test suite baru bermakna saat shell kedua muncul (mis. `vue`/`flutter`).

### 5.12 Spec Resolution API

- [x] 5.12.1 ETag caching — conditional GET with 304 for `/_meta/ui` bundle — `internal/api/meta.go` (ETag over data portion + `If-None-Match` → 304). ✅ 2026-08-24
- [x] 5.12.2 `label_field` fallback — `natural key` → `name` → `title` → `number` → `id` (`04-spec-resolution-api.md` §2) — `internal/ui/meta.go` `labelField()` + `TestLabelFieldFallbacks`. ✅ 2026-08-24
- [x] 5.12.3 Entity schema shape — `label_field`, `lifecycle`, `actions` with embedded `permission` — `EntitySchema`/`ActionSummary` di `internal/ui/meta.go`. ✅ 2026-08-24
- [x] 5.12.4 Permission filtering — entity (404 if no list/view), page (hidden if missing permission), action (permission string sent, not filtered) — `BuildBundle` (entity tanpa list/view → tidak ship → 404), `allowedPage` (page hidden bila tak ada permission), `ActionSummary.Permission` (string dikirim, tidak difilter). ✅ 2026-08-24
- [x] 5.12.5 Task-based admin granting → materialized permission strings — `Materializer` (`internal/auth/materialize.go`) menurunkan footprint page (blocks/tabs → entity-action) + derived entity page (`{entity}-page`) + navigation kind (`{kind}:{name}`) dan meng-expand grant role → permission strings; di-wire ke auth service (`permissionsForUser` saat login). Admin UI granting: `GrantsEditor` menampilkan semua page app (authored + derived entity + navigation kinds) dengan label action + permission string inline + search + preview permission termaterialisasi. ✅ 2026-08-20 (materializer) · ✅ 2026-08-22 (GrantsEditor semua page, changelog 004)

### 5.13 Other UI kinds

- [ ] 5.13.1 `kind: Report` — **sebagian**: ✅ totals row bug fixed (nilai kini dirender di kolom yang cocok via `TotalsRow`, bukan `<td>` kosong) + ✅ grouping + subtotal per group (`computeTotals` shared); ⏸️ export sebagai async job → download tray belum (butuh backend job infra; saat ini masih CSV Blob client-side) (`06-page-kinds.md` §8)
- [x] 5.13.1a Report `source.filter` — filter parameterized deklaratif (`source: { entity, filter }` dengan `":param"` placeholder); saat ini parameter dikirim sebagai filter query `?<field>=<value>` (`06-page-kinds.md` §8 "Open — source.filter") — `ReportSource` di `pkg/spec` + `ReportRenderer` resolve `":param"` placeholder dari `parameters[]`; literal pass-through. ✅ 2026-08-24
- [x] 5.13.2 `kind: Print` — PDF server-side generation; `format: html` via `window.print()` (existing) — endpoint `GET /_ui/print/{module}/{name}/{id}` (`internal/api/print.go`) render Print manifest + record ke PDF via `go-pdf/fpdf` (header/body fields/child_table/footer + `{path}` interpolation); route di `router.go`; test `print_test.go`. ✅ 2026-08-24
- [x] 5.13.3 `kind: ApprovalInbox` — pending approvals list, `approve`/`reject` inline actions, badge count, `realtime: true` — `ApprovalInboxRenderer` (zero-config; load dari entity approval konvensional bila ada; approve/reject inline; badge count; realtime). ✅ 2026-08-24
- [x] 5.13.4 `kind: NotificationCenter` — notification list, badge unread, `mark-read` action, `realtime: true`, deep-link on click — `NotificationCenterRenderer` (zero-config; unread badge; mark-read; realtime). ✅ 2026-08-24
- [x] 5.13.5 `kind: Listing` — public catalog, no auth wrap, no row/bulk actions — `ListingRenderer` read-only (search + filter, tanpa create/row/bulk; klik baris → detail) + kind `Listing` end-to-end (spec, registry, bundle, route). Contoh `examples/storefront/`. Lihat `docs/plan/landing-page.md` + changelog 2026-08-19-001. ✅ 2026-08-19

### 5.14 Derivation engine

- [x] 5.14.1 Derivation fix — Table: N priority columns, overflow accessible via expand (never silently dropped) — sama dgn 5.4.4 (`derive.ts` priority sort + `TableRenderer` row expand). ✅ 2026-08-24
- [x] 5.14.2 Wire `deriveMenuItems()` — currently dead code; `_admin` menu built inline in Sidebar — `useResolvedMenu()` (`hooks/useResolvedMenu.ts`) now calls `deriveMenuItems(bundle.entities)` for the `_admin` branch instead of duplicating the grouping logic inline; "Access Management" shortcut still prepended. ✅ 2026-08-22
- [x] 5.14.3 Derivation: Form mode heuristic — >12 fields OR has child with `storage: table` → `separate_page`; >5 fields → `drawer`; else → `modal` — `deriveFormRenderMode` di `engine/derive.ts`. ✅ 2026-08-24
- [x] 5.14.4 Pola UI lifecycle tambahan — `two_step_manual` dan `one_step_create_submit` via hint `ui:` (`06-page-kinds.md` §2.1); catatan: enum `lifecycle` di EntitySchema tetap 2 nilai (`plain_crud|two_step_autosave`, `04-spec-resolution-api.md` §2) — ini pola UI, bukan nilai enum baru — `engine/lifecycle.ts` (4 pola) + `FormRenderer` (Save Draft/Submit/Create-Submit). ✅ 2026-08-24

### 5.15 Dead code cleanup

- [x] 5.15.1 Remove `engine/registry.tsx` — replaced by hardcoded `lazy()` map in router — deleted (zero remaining references; `shell/router.tsx`'s hardcoded `lazy()` map is the only path used). ✅ 2026-08-22
- [x] 5.15.2 Wire `OverlayHost` — connect to Form.render modal/drawer and other overlay needs — `shell/OverlayHost.tsx` (URL `?action=&form=&mode=` → Dialog/Sheet; derived-form fallback via `entity=`). ✅ 2026-08-24

### 5.16 VisualSpecKind/Renderer registry & resolution

- [x] 5.16.1 Renderer resolution engine — pilih Renderer via `(implements, stack_family)`; hanya `official` auto-select; tanpa official → `formspec apply` error + sarankan kandidat `verified`/`community`; override via map `renderers:` di App manifest + field `renderer:` per-instance; Renderer non-official masuk consent footprint (`03-renderer-kind.md` §3) — `internal/manifest/renderer.go` (`RendererRegistry.ResolveRenderer` + `ValidateRendererResolution`); `AppSpec.Renderers` map + `PageSpec.Renderer` field; wired ke `formspec check`. ✅ 2026-08-24
- [x] 5.16.2 Slot-tier validation at apply — `accepts_slots` hanya sah dari `tier: page|app`, `implements_slot` hanya dari `tier: component`; kombinasi lain ditolak (`02-visual-spec-kind.md` §4–§5) — `RendererRegistry.ValidateSlotTiers`. ✅ 2026-08-24
- [x] 5.16.3 `stack_family` compatibility check — App shell + Page shell-integrated + Component wajib satu family; mismatch = compile-time error; Page independen tidak dicek (`01-visual-hierarchy.md` §3) — `RendererRegistry.ValidateStackFamily`. ✅ 2026-08-24

---

## Fase 6: Auth & Authorization ✅ COMPLETE (inti) — sebagian item ⏸️ deferred (dogfooding — `docs/plan/fase6-dogfooding-auth-module.md`)

**Goal**: Login, JWT, permission model, roles, API keys, sessions, field security. Prod requirement.
**Pendekatan**: `formspec.core` = bundled module YAML (`internal/auth/module/`, embed + loader),
mergeable ke project lain via `external/`/`spec/modules/`; middleware tetap Go.
**Progress**: ✅ Fase A–L selesai (2026-08-20/21). Changelog `2026-08-20-003` s/d `2026-08-21-014`.
**Deferred/partial** (lihat item ⏸️ di bawah): 6.2.5 consent-flow penuh, 6.2.6 ABAC enforcement,
6.3.3 delegation-chain enforcement, 6.3.5 job/audit-log/setting, 6.7.1 classification-tag,
6.7.4 encrypted at-rest, 6.8 store populasi (Config runtime 7.2).

### 6.1 Login & token

- [x] 6.1.1 Login endpoint — `POST /api/v1/auth/login`, credential verification (password hash), JWT issuance (access + refresh) — `POST /{ws}/api/v1/auth/login`; bcrypt verify; access + refresh JWT. Backed by `formspec.core.user`/`session` entities (internal, tanpa route). Dev seed `admin/admin`. Lihat `docs/plan/auth-login-token.md`. ✅ 2026-08-19
- [x] 6.1.2 Token claims — `sub`, `workspace`, `roles`, `permissions`, `exp`, `iat` — access claims: `sub`, `ws`, `roles`, `perms`, `typ=access`, `iat`, `exp`, `iss`, `aud`; refresh claims: `sub`, `ws`, `typ=refresh`, `jti`, `iat`, `exp`. ✅ 2026-08-19
- [x] 6.1.3 Token refresh — rotate (invalidate old, issue new) — `POST /{ws}/api/v1/auth/refresh`; session (jti) di-rotate: hapus jti lama + issue baru; replay token lama → 401. ✅ 2026-08-19
- [x] 6.1.4 Auth per-App via `auth_config_ref` — App me-resolve strategy autentikasi dari yang terpasang (`basic-auth` minimum untuk single-server; `sso` OIDC/SAML, `social-sso`, `passwordless`, `passkey` = set terbuka) (`platform/02-workspace-app-module.md` §3) — `ResolveAppAuth` + `RoleResolver.SetOverride`. ✅ 2026-08-20 (Fase F, changelog 008)

### 6.2 Permission model

- [x] 6.2.1 Resource + action permission — format `{module}.{entity}.{action}` — `ValidatePermissionFormat`/`ParseResourceTarget`/`AutoPrefixPermission`. ✅ (sudah ada, diverifikasi Fase C)
- [x] 6.2.2 Wildcard support — `{module}.{entity}.*`, `*` (super-wildcard), `public` — `Identity.HasPermission` (+ module-level `{module}.*` Fase G). ✅ (sudah ada, diverifikasi Fase C/G)
- [x] 6.2.3 Wire permission check to every API handler — both surfaces — enforcement inti sudah ter-wire di semua route (`RequirePermission` + `RequiredPermission` per route, kedua surface); surface-aware — UI surface entity list/view tanpa izin → 404 (spec §4, tidak bocor keberadaan entity), selain itu → 403. Test `internal/api/permission_enforcement_test.go`. ✅ 2026-08-20 (surface-aware 404)
- [x] 6.2.4 Permission resolution — role → permissions list; cache per session — `PermissionResolver` + cache per-session. ✅ 2026-08-20 (Fase C, changelog 005)
- [ ] ⏸️ 6.2.5 Consent footprint — aggregate `required_permission` + `uses` presented to workspace owner at install; cross-module write = high-risk consent — **sebagian**: `formspec check --footprint` (Fase K, changelog 013); alur consent penuh saat install di-defer.
- [ ] ⏸️ 6.2.6 Attribute-based authorization — pemeriksaan atribut App/user/membership melengkapi RBAC; pola multi-cabang = `scope_field` pada natural key + atribut membership (mis. kode cabang) (`platform/02-workspace-app-module.md` §3) — **sebagian**: `EvaluateGrantConditions` evaluator (Fase K, changelog 013); enforcement penuh di request di-defer.

### 6.3 Roles & membership

- [x] 6.3.1 `role` Entity — collection of grants (page → tab → action + conditions) — `formspec.core.role` (internal), tipe `Grant`/`TabGrant`/`ActionGrant`/`ConditionGrant` di `internal/auth/grant.go`; `RoleStore` baca role. ✅ 2026-08-20
- [x] 6.3.2 `app-membership` Entity — populasi user per App + atribut membership (mis. kode cabang) — `formspec.core.app-membership` (user_id, app, attributes, active). ✅ 2026-08-20 (Fase B, changelog 004)
- [ ] ⏸️ 6.3.3 Admin delegation chain — workspace owner → app admin → module staff — **sebagian**: 4 owner roles di-seed + di-recognize (Fase G); enforcement rantai delegasi (siapa assign role apa) di-defer.
- [x] 6.3.4 4 symmetric owner roles — Workspace Owner, App Owner, Module Owner, Cloud Owner — `SeedOwnerRoles` + `ownerRolePermission`. ✅ 2026-08-20 (Fase G, changelog 009)
- [ ] ⏸️ 6.3.5 `formspec.core` resource set lengkap — `workspace`, `user`, `app-membership`, `role`, `api-key`, `session`, `job`, `audit-log`, `setting` — **sebagian**: user/session/role/api-key/app-membership/workspace ada (bundled module); `job`/`audit-log`/`setting` milik sistem lain (7.13/4.7/7.2).

### 6.4 API keys

- [x] 6.4.1 `api_key` Entity — create (return key once), list (masked), revoke, expiry — `ApiKeyStore`. ✅ 2026-08-20 (Fase B, changelog 004)
- [x] 6.4.2 Scope — per workspace or per app — field `scope` (workspace|app). ✅ 2026-08-20 (Fase B)
- [x] 6.4.3 API key auth middleware — header `X-FormSpec-Key` (`01-core-basic.md` §8.2; surface external TIDAK menerima session cookie) — `AuthMiddleware` resolve key hanya di `/api/v1/`. ✅ 2026-08-20 (Fase B)

### 6.5 Session management

- [x] 6.5.1 `session` Entity — session_id, user, workspace, IP, user-agent, created/expires/last_active — `formspec.core.session`. ✅ (sudah ada)
- [x] 6.5.2 Refresh token rotation — invalidate old, issue new — `POST /auth/refresh`. ✅ (sudah ada)
- [x] 6.5.3 Concurrent session limit — configurable per user — `SetMaxSessionsPerUser` + evict oldest. ✅ 2026-08-20 (Fase D, changelog 006)
- [x] 6.5.4 Global revoke — logout all devices — `LogoutAll`. ✅ 2026-08-20 (Fase D)
- [x] 6.5.5 Session expiry + cleanup job — `PurgeExpired` + `StartSessionCleanup`. ✅ 2026-08-20 (Fase D)
- [x] 6.5.6 Frontend session expiry → login redirect + auto-logout timer — 401 dari API client (entity + meta) menandai session unauthenticated → auth guard redirect ke `{surfacePath}/login?returnTo=...` (bukan error); boot `fetchMe` membedakan 401 (→ login) dari network error; auto-logout idle timer configurable (`prefs.sessionTimeoutMinutes`, default 30, 0=never) di-set dari LoginScreen, hook `useAutoLogout` di `SurfaceShell`. Changelog `2026-08-22-001`. ✅ 2026-08-22
- [x] 6.5.7 Refresh token flow (frontend) + fix session workspace (backend) — frontend simpan `refreshToken`, auto-refresh access token saat 401 (`authHooks` shared: 401 → `ky.retry()` → `beforeRetry` refresh single-flight → retry); `refreshSession()` di session store; login meneruskan refresh token. Backend fix: `sessWorkspaceForJTI` hardcode "demo" → thread workspace melalui `SessionStore` (`Get`/`Delete`/`DeleteForUser`/`CountForUser`/`ListForUser`), `Refresh` pakai `claims.Workspace`. Changelog `2026-08-22-002`. ✅ 2026-08-22
- [x] 6.5.8 Session persistence ke sessionStorage — access + refresh token dipersist ke `sessionStorage` (key `formspec-session`) sehingga browser refresh (F5) me-restore session tanpa login ulang; `boot` restore saat workspace cocok, `setSession`/`refreshSession` tulis, `clearSession`/`expireSession` clear. Changelog `2026-08-22-003`. ✅ 2026-08-22

### 6.6 Auth middleware pipeline

- [x] 6.6.1 Auth method detection — Bearer JWT vs `X-FormSpec-Key` API key vs session cookie (session cookie hanya surface `/_ui`) — `AuthMiddleware` (JWT + API key; cookie belum ada mekanisme). ✅ 2026-08-20 (Fase E, changelog 007)
- [x] 6.6.2 Token validation → identity extraction → permission loading → workspace context — pipeline di `AuthMiddleware`. ✅ 2026-08-20 (Fase E)
- [x] 6.6.3 Rate limiting per auth method — token bucket per IP (login/refresh). ✅ 2026-08-20 (Fase E)
- [x] 6.6.4 Audit log every auth attempt (success + failure) — `authAudit`. ✅ 2026-08-20 (Fase E)

### 6.7 Field-level security

- [ ] ⏸️ 6.7.1 `classification` label — tag field `pii|financial|internal`; log/export auto-tag — struct ada (1.4.3); tagging di log/export di-defer (butuh kebijakan log/export terpusat).
- [x] 6.7.2 `required_permission` (field-level) — user without permission → field excluded from response — `sanitizeData`. ✅ 2026-08-20 (Fase H, changelog 010)
- [x] 6.7.3 `exclude` — per-surface field exclusion (`public_api`, `audit_log`, `webhook`, `ui` — `05-field-types.md` §5.3) — `sanitizeData` (public_api vs ui). ✅ 2026-08-20 (Fase H)
- [ ] ⏸️ 6.7.4 `encrypted: true` — AES-256-GCM at-rest encryption for field — struct ada (1.4.6); enforcement di-defer (butuh master key/keystore).
- [x] 6.7.5 `masked: true` — auto-mask in JSON response and structured log (`***`) — `sanitizeData`/`maskValue`. ✅ 2026-08-20 (Fase H)
- [x] 6.7.6 `computed` — server-derived, never client-writable; recompute on every create/update — `evaluateComputed`. ✅ (sudah ada)

### 6.8 `ctx.secrets`

- [x] 6.8.1 `ctx.secrets.get("key")` — only path for `secret: true` Config keys — `secretsAPI`; populasi store menunggu Config runtime 7.2. ✅ 2026-08-21 (Fase I, changelog 011)
- [x] 6.8.2 `uses: { secrets: [key, ...] }` — must declare access; undeclared → blocked — `declaredUsesSecrets`. ✅ 2026-08-21 (Fase I)
- [x] 6.8.3 Secret never appears in logs at any level — `secretsAPI` tidak log nilai. ✅ 2026-08-21 (Fase I)
- [x] 6.8.4 Every secret read audited — who read what secret, when — `SecretsAudit` hook. ✅ 2026-08-21 (Fase I)

### 6.9 RichText sanitization

- [x] 6.9.1 Server-side HTML sanitize — strip script/markup before persist; client HTML never trusted raw — `sanitizeRichText`/`sanitizeHTML` di Insert/Update. ✅ 2026-08-21 (Fase J, changelog 012)

---

## Fase 7: Engine Extended — Missing Kind Runtimes

**Goal**: Service, Config, Subscription, Workflow, Webhook, Integrator, Hook engine, Validation L4–L6, State machine, Denormalisasi, Period closing, Rate limiter, Async job.

### 7.1 `kind: Service` runtime

- [x] 7.1.1 Service registry — register by `{module}.{name}` — `internal/service/registry.go` (`Registry` + `Add`/`Get`/`GetAction`/`List`); `buildServiceRegistry` di `resource/formspec.go` (boot + reload). ✅ 2026-08-25
- [x] 7.1.2 Resolve `impl.native` — `ref: "{Type}.{Method}"` via `NativeExecutor` (sama seperti entity action); auto-scan `impl/**/*.go` belum (native handler didaftarkan eksplisit via `RegisterNative` — enhancement). ✅ 2026-08-25
- [x] 7.1.3 Resolve `impl.script` / `impl.script_ref` / `impl.compiled` / `impl.sidecar` — permission enforcement seragam untuk KELIMA jenis impl via dispatcher yang sama (`invokeServiceAction` + `HandleServiceAction`); route `POST /api/v1/{module}/{service}/{action}`. ✅ 2026-08-25
- [x] 7.1.4 `call: async` — fire-and-forget (no job_id, no progress, no result) — `HandleServiceAction` dispatch di goroutine + return 202 Accepted. ✅ 2026-08-25

### 7.2 `kind: Config` runtime

- [x] 7.2.1 Config registry — load Config manifests, resolve per environment — `internal/config/registry.go` (`Registry` + `NonSecret`/`Secrets`); `buildConfigRegistry` di `resource/formspec.go` di-wire di boot + reload. Single-server resolve per-environment = default yang dideklarasikan (Control Plane di-defer). ✅ 2026-08-25
- [x] 7.2.2 `ctx.config.get("key")` — Starlark access; non-secret keys only — `SetConfigStore` → `ctxObj.Config = NewConfigAPI(e.ConfigStore)`; secret keys dipisah ke `ctx.secrets` (gated `uses.secrets`). ✅ 2026-08-25
- [x] 7.2.3 `settings.*` namespace — global settings: currency, locale, timezone, date_format, fiscal_year_start — `Settings`/`ResolveSettings` di `pkg/spec` + `bundle.settings` (changelog 2026-08-24-008..014). ✅ 2026-08-24
- [x] 7.2.4 Global settings defaults — spec MUST provide acceptable defaults for every setting; components MUST NOT guess — `DefaultSettings` + `ResolveSettings` overlay; formatter terpusat `lib/format.ts`. ✅ 2026-08-24

### 7.3 `kind: Subscription` engine

- [x] 7.3.1 Tier 1 (outbox) — event → match Subscription → call handler; transactional — `internal/subscription/registry.go` (`Registry` + `Add`/`Get`/`List`/`ForEvent`, index by event name); `internal/subscription/dispatch.go` (`Dispatcher` dispatch handler `ImplDecl` via action dispatcher, payload + `_event` metadata); `buildSubscriptionRegistry` di `resource/formspec.go` (boot + reload); `DeliveryEventHandler.Subscriptions` di `renderers/jsonb-persist/event_handler.go` (dispatch setelah channel fan-out, fully-qualified event name); outbox worker kini di-start di dev mode (`App.StartBackgroundWorkers` + `cmd/formspec/dev.go`). ✅ 2026-08-25
- [ ] 7.3.2 Tier 2 (streaming) — Redis/Kafka; at-least-once, positioned replay, filter/transform Starlark
- [x] 7.3.3 `emits:` custom event emission — action declares `emits: <event-name>` → event emitted on action success — `ResolveEmission` + `ValidateActionEmits` + custom action emission di `internal/api/handler.go` (sudah ada). ✅ 2026-08-25
- [ ] 7.3.4 Dynamic subscriptions — runtime-created subscriptions as data (not manifest); live in `formspec.core`
- [ ] 7.3.5 Delivery channels — `webhook` (outbound, HMAC signed, retry), `notification` (bridge to `formspec/notify`), `pubsub` (non-durable, at-most-once)

### 7.4 `kind: Workflow` engine

- [x] 7.4.1 Approval state machine — attach to Entity transition without modifying Entity — `internal/workflow/registry.go` (`Registry` + `Add`/`Get`/`List`/`ForTransition`, index by `{entity}.{from}.{to}`); `buildWorkflowRegistry` di `resource/formspec.go` (boot + reload); `HandleCustomAction` intercept transisi → `handleWorkflowApproval` (create pending approval, approve/reject via `{"decision": ...}`); `executeWorkflowTransition` update state setelah approval selesai; tabel `formspec_workflow_approval` + `WorkflowApprovalStore`. ✅ 2026-08-25
- [x] 7.4.2 Multi-approver modes — `all` (all eligible must approve), `any` (quorum from pool), `sequential` (chain order) — `StepMode`/`Quorum` di `internal/workflow/engine.go` (all=eligible count, any=approvers N, sequential=1). ✅ 2026-08-25
- [x] 7.4.3 `when` condition — FormSpecExpr on `resource`; step skipped if false — `ApplicableSteps` evaluasi `when` via `starlark.EvaluateGuard`; `FieldMap` (internal/starlark/fieldmap.go) mendukung `resource.amount` dot-notation. ✅ 2026-08-25
- [x] 7.4.4 `escalation` — timeout (`after`), notify_roles, reassign_roles — `internal/workflow/escalation.go` (`EscalationWorker` background poll pending approvals; step aktif dengan `escalation.after` yang sudah lewat → eskalasi: catat audit `workflow.escalate` + tandai step escalated dengan `reassign_roles`); `WorkflowApprovalRow.EscalatedSteps` (stepIdx → reassign_roles); `CanApprove` menerima reassign_roles sebagai eligible roles tambahan; `App.StartBackgroundWorkers`/`Close` wire worker; approval menyimpan nama manifest workflow (bukan pointer address) via `Registry.NameFor`. ✅ 2026-08-25
- [x] 7.4.5 Requester can never approve own request — `CanApprove` menolak jika `userID == requesterID` (created_by record). ✅ 2026-08-25
- [x] 7.4.6 Approval = signed statement recorded in audit trail — `db.WriteAuditLog` (exported wrapper); `recordWorkflowAudit` di `internal/api/handler.go` mencatat `workflow.approve`/`workflow.reject` (actor = approver) + `workflow.transition` (actor = system) ke `formspec_audit_log`; `SetAuditWriter` wiring di `resource/formspec.go` (boot + reload). ✅ 2026-08-25

### 7.5 State machine engine (basic)

- [x] 7.5.1 Transition validation — declared transitions only; undeclared → `STATE_TRANSITION_ERROR` — `StateMachineEngine.CanTransition` + `STATE_TRANSITION_ERROR` di `internal/entity/state_machine.go`. ✅ 2026-08-25
- [x] 7.5.2 Starlark inline guards — guard on transition — `evaluateGuard` (Starlark `EvalExpr` terhadap resource data). ✅ 2026-08-25
- [x] 7.5.3 Builtin aggregates — `sum_line(field)`, `len(resource.items)` for guard expressions — `sum_line_*` pre-computed + injected ke env guard (`computeSums`). ✅ 2026-08-25
- [x] 7.5.4 Satukan dua implementasi state machine — `entity.StateMachineEngine` (lengkap, ber-guard) dipanggil dari `HandleCustomAction` via `CanTransition`; evaluasi guard diekstrak ke helper bersama `internal/starlark.EvaluateGuard` (`internal/starlark/guard.go`) yang dipakai BOTH `state_machine.go` (CanTransition) dan `crud.go` (validateStateTransition saat Update) — helper `sum_line_*`/`item_count`/`line_count` kini konsisten di kedua path. ✅ 2026-08-25

### 7.6 `kind: Webhook` engine

- [x] 7.6.1 Inbound endpoint — route registration, method validation — `internal/webhook/registry.go` (`Registry` + `Add`/`Get`/`List`); `buildWebhookRegistry` di `resource/formspec.go` (boot + reload); `GenerateWebhookRoutes` (path auto-derive `/api/v1/webhooks/{module}/{name}` atau `spec.path`); `HandleWebhook` dispatch ke Service action `spec.for`. ✅ 2026-08-25
- [x] 7.6.2 Signature verification — HMAC (strategy: `signature`, algorithm, header, payload) before handler — `internal/webhook/verify.go` (`Verify` + `computeHMAC` hmac-sha256/sha512, key dari config via `config.Registry.ResolveKey`). ✅ 2026-08-25
- [x] 7.6.3 Token auth — strategy: `token` — `WebhookTokenConfig` (header + key ref config); `verifyToken` (dukung prefix "Bearer "). ✅ 2026-08-25
- [x] 7.6.4 Verification failure → rejected BEFORE handler runs — `HandleWebhook` panggil `webhook.Verify` sebelum dispatch; gagal → 401 (auth) / 500 (misconfig). ✅ 2026-08-25

### 7.7 `kind: Integrator` engine

- [x] 7.7.1 Listen → call bridge — `listen.resource`+`event` triggers `call.resource`+`action` — `internal/integrator/registry.go` (`Registry` + `Add`/`Get`/`List`/`ForEvent`, index by `{resource}.{event}`); `internal/integrator/dispatch.go` (`Dispatcher` dispatch target action via entity/service registry + action dispatcher, payload + `_event` metadata); `buildIntegratorRegistry` di `resource/formspec.go` (boot + reload); composed ke `DeliveryEventHandler.Subscriptions` bersama subscription dispatch. ✅ 2026-08-25
- [x] 7.7.2 Mandatory symmetric cancel handler — every Integrator MUST provide cancel handler — `validateIntegrators` di `cmd/formspec/validate.go` (cross-manifest check: integrator yang listen event non-cancel wajib punya pasangan `on_cancel`/`before_cancel` untuk resource yang sama). ✅ 2026-08-25
- [x] 7.7.3 Target action must be `idempotent: true` for cross-boundary calls — `validateIntegrators` juga cek target action (`call.resource`+`call.action`) harus `idempotent: true` (resolve dari entity manifest set). ✅ 2026-08-25
- [x] 7.7.4 Saga compensate — cross-boundary call registers `compensate` to Saga log; `FORMSPEC.SAGA.*` errors — `renderers/jsonb-persist/saga.go` (`SagaStore` + tabel `formspec_saga_log`: Register/ListPending/MarkCompleted/MarkCompensated); `internal/integrator/dispatch.go` register compensate saat dispatch cross-boundary call, invoke compensate action saat dispatch gagal (`FORMSPEC.SAGA.COMPENSATE_FAILED`); wire saga store di `resource/formspec.go` (boot + reload). ✅ 2026-08-25

### 7.8 Hook engine

- [x] 7.8.1 5 hook points — `before`, `after`, `on_error`, `before_deliver`, `after_deliver` — `internal/action/hooks.go` (before/after/on_error) + `deliver.go` (before_deliver/after_deliver, todo 7.8.5). ✅ 2026-08-25
- [x] 7.8.2 `before` — may modify action params or call `fail()` to abort — `RunBeforePhase` (Dispatch error → abort + on_error). ✅ 2026-08-25
- [x] 7.8.3 `after` — post-action side effects — `RunAfterPhase` (best-effort, tidak gagalkan response). ✅ 2026-08-25
- [x] 7.8.4 `on_error` — compensation/cleanup — `runOnErrorPhase` (params `_hook_error`). ✅ 2026-08-25
- [x] 7.8.5 `before_deliver` — may suppress delivery or enrich payload — `runBeforeDeliver` (fail → suppress; ok(data) → enrich) + `runAfterDeliver` (best-effort); `SelectEventHooks` match `h.Event`; wired via `deliveryDepsFor` (dispatcher + entity hooks). ✅ 2026-08-25
- [x] 7.8.6 Priority ordering — consistent with event priority (smaller first, kelipatan 10) — `SelectHooks`/`SelectEventHooks` sort by effective priority (0→10). ✅ 2026-08-25
- [x] 7.8.7 Cross-module hooks — must declare `uses`; appear in consent footprint — hook eksekusi mewarisi `actionSpec.Uses` (`RunBeforePhase`/`RunAfterPhase`), jadi akses cross-module dari hook di-gate oleh uses action. ✅ 2026-08-25

### 7.9 Validation levels L4–L6

- [ ] ⏸️ 7.9.1 L4 `business_rules` — single-record business constraints via script — **deferred**: kontrak deklarasi L4–L6 belum dispesifikasikan di `pkg/spec` (tidak ada field `business_rules`/`cross_validate`/`consistency` di `EntitySpec`); butuh design decision.
- [ ] ⏸️ 7.9.2 L5 `cross_validate` — multi-field/child-record validation within same record — **deferred** (sama dengan 7.9.1).
- [ ] ⏸️ 7.9.3 L6 `consistency` — cross-entity consistency (e.g., aggregate balance vs GL); requires `uses: db` — **deferred** (sama dengan 7.9.1).
- [ ] ⏸️ 7.9.4 Sequential evaluation — L1–L3 → L4 → L5 → L6; stop at first failure — **deferred** (bergantung 7.9.1–7.9.3).
- [ ] ⏸️ 7.9.5 Error response with `details: [{level, field?, message}]` — **deferred** (bergantung 7.9.1–7.9.3).
- [x] 7.9.6 Katalog rule L1–L3 lengkap server-side — himpunan tertutup ~20 rule: `required, min_length, max_length, length, pattern, email, url, min, max, positive, precision, in, future, past, after:<field>, before:<field>, min_items, max_items, unique, exists, script` — `length`/`in`/`script`/`unique` ditambahkan di `validateSingleRule`/`validateUnique` (`renderers/jsonb-persist/crud.go`); `unique` baca lewat transaksi aktif (hindari deadlock SQLite). ✅ 2026-08-25

### 7.10 Denormalisasi finansial

- [ ] 7.10.1 Master financial fields snapshot to transaction on `create`/`submit` — not live-join
- [ ] 7.10.2 Old transactions unaffected by master value changes

### 7.11 Period closing

- [ ] 7.11.1 `period-closing` as Entity — gets lifecycle, reference guard, audit trail for free
- [ ] 7.11.2 `submit` → finalize summary period; `cancel` (reopen) → unfinalize
- [ ] 7.11.3 Reopen requires elevated permission + recorded reason → `FORMSPEC.PERIOD.REOPEN_DENIED`
- [ ] 7.11.4 Business calendar day resolution — `today`/`current` from EOD, not system clock
- [ ] 7.11.5 `FORMSPEC.PERIOD.CLOSED` enforcement for create/update/submit/amend in closed period

### 7.12 Rate limiter

- [x] 7.12.1 Per-resource rate limit — `max`, `per`, `scope` (tenant|user|ip|global), `strategy` (sliding_window|token_bucket) — `ResourceRateLimiter` (`internal/api/resource_ratelimit.go`); `EntitySpec.RateLimit` + `Action.RateLimit`; scope key derivation. ✅ 2026-08-25
- [x] 7.12.2 Per-action override — overrides resource default — `checkRateLimit` (per-action `Action.RateLimit` menang atas `EntitySpec.RateLimit`; key per-action). ✅ 2026-08-25
- [x] 7.12.3 `429` response before handler runs — `checkRateLimit` di semua handler (List/Find/Create/Update/Delete/CustomAction) → `RATE_LIMITED` 429. ✅ 2026-08-25

### 7.13 Async job tracker

- [ ] 7.13.1 `call: async` (tracked) → `202` with `job_id`
- [ ] 7.13.2 Progress via WebSocket `jobs` channel — `progress`/`completed`/`failed` events
- [ ] 7.13.3 `ctx.job.progress(pct, message)` from handler
- [ ] 7.13.4 Callback webhook delivery — HMAC-signed, durable retry

### 7.14 Starlark sandbox

- [x] 7.14.1 Hard limits enforcement — wall-clock 5000ms, memory 64MB, iterations 100K, max 50 DB queries, max 1000 records read — `internal/starlark/limits.go` (`ScriptLimits` + `SetMaxExecutionSteps` 100K + context timeout 5s + query/records counters di `builtinQuery`). Memory 64MB tidak terukur langsung di interpreter Starlark — step limit adalah bound praktis. ✅ 2026-08-25
- [x] 7.14.2 No network/filesystem/subprocess access — sandbox `Load: nil` (no imports), no I/O. ✅ 2026-08-25
- [x] 7.14.3 Exceeding any limit → abort with error, no partial results — `CheckQuery`/`AddRecordsRead`/step limit → error abort. ✅ 2026-08-25
- [x] 7.14.4 Kontrak API script runtime — `resource.field`/`resource.set/save/new`, `<resource>.load/call`, `ok()`/`fail()` (fail = rollback transaksi) — `resource.new()` (handle baru entity sama, `save()` → INSERT) ditambahkan 2026-08-25; `resource.field`/`set`/`save`/`fetch`/`call`/`create`/`ok`/`fail` sudah ada. `<Entity>.query()` (builder §16) di-defer (butuh scope builder query besar). ✅ 2026-08-25

### 7.15 Sidecar multi-runtime

- [ ] 7.15.1 Read `spec.runtime` per Module manifest — go, node, php, python, ruby, java, dotnet, rust
- [ ] 7.15.2 Spawn one sidecar process per unique runtime
- [ ] 7.15.3 Sidecar protocol — entity CRUD via `POST /ctx/entity/{op}` (get, set, update, increment, decrement)

### 7.16 Money type

- [x] 7.16.1 Money as first-class type — pair of exact amount (decimal) + currency code (ISO-4217) — `Money` struct (`pkg/spec/money.go`) + `Field.Currency`/`Field.DecimalPlaces`. ✅ 2026-08-25
- [x] 7.16.2 Currency resolution order — explicit field → `settings.currency` → error (never guess) — `ResolveMoneyCurrency`. ✅ 2026-08-25
- [x] 7.16.3 Banker's rounding default — `RoundMoney` (round-half-to-even default; `settings.rounding` override half_up/half_down/up/down). ✅ 2026-08-25
- [x] 7.16.4 Non-default currency MUST declare `decimal_places` — `ValidateMoneyField` + `ValidateEntitySpec` (money field override currency tanpa decimal_places → error). ✅ 2026-08-25

### 7.17 File storage

- [x] 7.17.1 File upload route — `POST /:resource/:id/{field}` + `GET` download; storage resolver dua backend: `file` (default, `memory.Storage`) atau `minio` (`datastore/minio`, `FORMSPEC_STORAGE=minio`); permission update/view; StorageSpec enforcement. ✅ 2026-08-24
- [x] 7.17.2 `storage` spec enforcement — `allowed_types`, `max_size_mb`, `max_count`, `visibility` (public|private|signed) — `max_size_mb`+`allowed_types` di upload (sudah ada); `max_count` (multi-file array key, `FILE_COUNT_EXCEEDED`) + `visibility` (public=anon, private=view, signed=501 deferred) di `internal/api/file.go`. ✅ 2026-08-25
- [ ] 7.17.3 Transform — server-side resize/thumbnail per `transform` spec

### 7.18 `kind: KindDefinition` runtime

- [ ] 7.18.1 Kind registry — daftarkan kind baru dari `KindDefinition` manifest; validasi `group` + `version` + `schema`
- [ ] 7.18.2 Handler resolution — `impl.native`/`impl.script`/`impl.compiled`/`impl.sidecar`; eksekusi di bawah `uses` module yang mendeklarasikan
- [ ] 7.18.3 Group-scoped naming — instance pakai `apiVersion: {group}/v1`; tabrakan namespace mustahil secara struktural

### 7.19 `kind: Mockup` runtime

- [ ] 7.19.1 Mock connector — simulated third-party API response; `for` menunjuk Service/Webhook yang di-mock; `config_ref` ke Config
- [ ] 7.19.2 Dev-only gate — Mockup hanya aktif di `formspec dev`; production → `formspec apply` menolak atau warning

---

## Fase 8: Production Self-Hosting Single Server

**Goal**: `formspec serve --mode=production` — production-grade, single-server, no Control Plane.

### 8.1 Production mode

- [ ] 8.1.1 `formspec serve --mode=production` — disable dev shortcuts: no auto-approve, no self-signed, no dev auth
- [ ] 8.1.2 Production JWT — RS256/ES256 (test + wire to config)
- [ ] 8.1.3 HTTPS — TLS configuration
- [ ] 8.1.4 Production datastore — Postgres (not SQLite)
- [ ] 8.1.5 CORS origin allow-list — production TIDAK boleh `Access-Control-Allow-Origin: *` (saat ini hardcoded, `runtimes/05-engine-api-layer.md` §2.2)
- [ ] 8.1.6 Peran DB least-privilege — `formspec_ops_backup` (REPLICATION-only), `formspec_ops_ddl` (DDL-only, NOSUPERUSER), tanpa superuser manusia (`platform/06-datastore.md` §8)

### 8.2 Observability

- [ ] 8.2.1 Structured JSON-lines logging — 12 mandatory fields: timestamp, level, request_id, workspace, module, entity, action, actor, duration_ms, error_code, trace_id, environment
- [ ] 8.2.2 PII discipline — info/warn/error MUST NOT contain business data; debug gated by operator control
- [ ] 8.2.3 Request ID — issue at boundary, propagate to Starlark (`ctx.request_id`), sidecar (header), ctx.\* calls
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
- [x] 10.1.2 `watchSpecForChanges()` — fsnotify watcher di `formspec dev`, debounce 300ms
- [x] 10.1.3 Native Go handlers preserved across reload via `nativeHandlers` map
- [x] 10.1.4 WebSocket connections preserved via WSHub transfer
- [x] 10.1.5 Auto-watch subdirektori baru
- [x] 10.1.6 ETag-aware Meta API — reload otomatis mengubah ETag, frontend fetch bundle baru
- [ ] 9.3.1 `make generate` — generate TypeScript types from `pkg/spec/` → `renderers/web/src/generated/types.ts`
- [ ] 9.3.2 Validate generated types against manual `types/manifest.ts`

### 9.4 Developer guide

- [ ] 9.4.1 "Buat App Pertama Anda dengan FormSpec" — getting started guide

### 9.5 Retirement

- [ ] 9.5.1 Final audit `docs_old/` — verify all content migrated; archive or delete

---

## Fase 10: `formspec consult` — AI Business Consultant & Spec Author

**Goal**: AI membantu discovery kebutuhan bisnis + menulis spec FormSpec yang valid, lewat grounding
(tool nyata) dan validasi wajib server-side — bukan bergantung kedisiplinan LLM.
**Depends on**: Module Vendoring §6/§2.1 (`vendors/`, `formspec.lock`, alias — untuk
`list_installed_modules()` yang akurat) dan Marketplace (`ai_index`, trust tier) — keduanya masih
di tabel Deferred di bawah, jadi Fase ini realistis baru mulai setelah salah satunya landing.
**Sumber**: `docs/ai/` (README + 01–06), `docs/cli-tools/05-formspec-consult.md` (referensi verb).

### 10.0 Jalur tanpa-MCP — agent-assisted app development ✅

> Jalur yang berjalan hari ini **tanpa** MCP — lihat
> `docs/plan/agent-assisted-app-development.md`. MCP (10.1–10.7 di bawah) tetap
> di-defer.

- [x] 10.0.1 Skill `formspec-app-workflow`: section Phase Detection + No-MCP Tool Map
- [x] 10.0.2 `formspec init`: copilot-instructions mereferensikan workflow 4 fase + `formspec validate` gate; `init_test.go`
- [x] 10.0.3 Guide `docs/guides/agent-assisted-app-development.md` + index
- [x] 10.0.4 Contoh `examples/cafe/` (16 manifest, 0 problem) — dibangun lewat alur ini

### 10.0.5 Cafe Order — child items UX (auto-fill, read_only, dropdown) ✅

> Lanjutan `examples/cafe` order form (`order-create`, widget `child-grid`).
> Plan: `docs/plan/cafe-order-child-autofill-readonly-dropdown.md`; changelog
> `docs/changelog/2026-08-24-001`.

- [x] 10.0.5.1 `auto_fill` client-side — child field `unit_price` auto-fill dari
      `menu_item_id → price` (record lengkap dari RelationPicker via `onSelectRecord`);
      clear relation → target ikut kosong. `pkg/spec` `AutoFillDecl` + `Field.AutoFill`.
- [x] 10.0.5.2 `read_only` pada child field — `Field.ReadOnly` (Go+TS),
      `ChildTable.isCellReadonly` menghormatinya; `unit_price` selalu read-only
      (tanpa `readonly_when`).
- [x] 10.0.5.3 Dropdown RelationPicker tidak terpotong — portal ke `<body>`
      (`position: fixed`, anchor bounding rect) + flip ke atas saat ruang bawah
      kurang; reposisi pada scroll/resize.
- [x] 10.0.5.4 Schema — `AutoFillDecl` masuk `sharedTypes` generator;
      `make generate-schema` (125 shared defs); `formspec validate` tetap hijau.
- [x] 10.0.5.5 Fix integer input menerima pecahan — `ChildTable` + `NumberInput`
      blokir `.`/`,`/`e`/`E` (keydown) + `step={1}` + strip desimal saat paste;
      `quantity` (integer) tidak bisa diisi pecahan lagi.
- [x] 10.0.5.6 `NumberInput` bedakan integer vs decimal via prop eksplisit
      `integer` (bukan inferensi dari `step` yang tidak konsisten) — fix decimal
      ter-truncate ke integer; `precision` kini dipakai untuk step decimal.
- [x] 10.0.5.7 Decimal `scale` membatasi input pecahan — `Field.Scale`/`Precision`
      (Go+TS+schema, 05-field-types.md §1.2); `NumberInput` prop `precision`→`scale`,
      round ke `scale` desimal saat change/paste + blokir digit berlebih di keydown;
      `FormRenderer` kirim `scale={entityField.scale}`.
- [x] 10.0.5.8 `ChildTable` pakai `NumberInput` untuk child integer/decimal —
      `scale` kini berlaku di child field (mis. `quantity: decimal, scale: 2` →
      step 0.01, input dibatasi 2 desimal); logika duplikat dihapus.
- [x] 10.0.5.9 Fix select-all + ketik di `NumberInput` (scale) terblokir —
      `type="number"` tidak expose `selectionStart` (null), jadi blokir keydown
      berbasis scale salah memblokir replace; blokir scale dihapus, pembatasan
      ditangani sanitize-on-change (`toFixed`).
- [x] 10.0.5.10 `NumberInput` spinner hormati `min`/`max` + flag merah
      out-of-range — prop `positive` (rule spec `> 0`); boundary spinner =
      langkah positif terkecil; nilai di luar range di-flag merah (border+teks+
      tooltip), bukan di-ignore/clamp; `ChildTable` kirim `min`/`max`/`positive`
      dari rules.
- [ ] 10.1.3 Tool `propose_spec_file(path,content)` — tulis draft ke sesi + jalankan `validate_spec` otomatis (03 §2)
- [ ] 10.1.4 Tool `apply_draft(session,file)` — pindahkan draft ke lokasi asli, guard read-only `vendors/` (03 §4)
- [ ] 10.1.5 Tool `validate_spec(yaml)` / `check_naming_conflict(name)` — reuse package sama dengan `formspec apply --dry-run`/boot `formspec-server`, bukan reimplementasi (03 §3)
- [ ] 10.1.6 Tool `restart_server()` / `get_server_status()` / `stop_server()` — kontrol proses `formspec dev` lokal (03 §5)
- [ ] 10.1.7 Tool `list_skills()` / `read_skill(name)` — index dan isi FormSpec Skill (03 §1, 06)

### 10.2 `formspec-consult` client (TypeScript + Vercel AI SDK)

- [ ] 10.2.1 Scaffold project TypeScript, compile jadi binary standalone via `bun build --compile` (`docs/ai/01-architecture.md` §2)
- [ ] 10.2.2 Tool-use loop (`ToolLoopAgent`) — spawn `formspec mcp-serve` sebagai child process stdio (01 §3)
- [ ] 10.2.3 LLM Provider Layer — BYOK, provider adapter (Anthropic/OpenAI/dst.), minimum capability bar tool-calling + context window (`docs/ai/05-llm-provider-layer.md`)
- [ ] 10.2.4 Credential storage — interface `CredentialStore`, `zalando/go-keyring` tiered ke environment variable (05 §3)
- [ ] 10.2.5 REPL — kelola sesi, render diff (unified diff teks cukup untuk versi awal) (`docs/ai/02-formspec-consult.md`)
- [ ] 10.2.6 Auto-invoke deterministik saat sesi mulai (`read_workspace_manifest`+`list_installed_modules`+`list_skills`) — bukan bergantung inisiatif LLM (01 §5)
- [ ] 10.2.7 Kompresi riwayat sesi panjang — distilasi turn lama jadi ringkasan terstruktur, transcript penuh tetap di disk, jaga pasangan `tool_use`/`tool_result` (01 §6)

### 10.3 Validation Gate

- [ ] 10.3.1 `propose_spec_file` composite tool — validasi wajib server-side, proteksi sama untuk client built-in maupun eksternal (03 §2)
- [ ] 10.3.2 Scope structural-only (schema, referensi `depends`/Entity Extension/shadow-copy, bentrok nama) — bukan validasi data runtime (03 §3)
- [ ] 10.3.3 Jalur online eksplisit terpisah untuk verifikasi signature/trust-tier vendor module (03 §3)

### 10.4 Session storage & diff

- [ ] 10.4.1 `.formspec/consult/{session}/` — `transcript.md`, `discovery-summary.md`, `draft/`, `undo/` (02 §3)
- [ ] 10.4.2 `formspec consult diff` — unified diff `draft/` vs `modules/`/`vendors/` project asli (02 §4)
- [ ] 10.4.3 Accept/reject per file → `apply_draft` (02 §4)

### 10.5 `formspec-remote-mcp` (FormSpec Cloud, hosted)

- [ ] 10.5.1 Streamable HTTP server — `list_business_templates()`, `search_modules_registry(query)`, `get_module_detail(name)` (`docs/ai/04-formspec-remote-mcp.md`)
- [ ] 10.5.2 Katalog industry template awal, 100% FormSpec-authored — pattern YAML + probing questions (04 §1)
- [ ] 10.5.3 pgvector embedding untuk `search_modules_registry` — model multilingual (Voyage AI/BGE-M3), hybrid dengan `aliases:` eksplisit (04 §2)
- [ ] 10.5.4 `ai_index`/`skills_for_ai` untrusted-input handling — wajib selesai sebelum trust tier `community` dibuka untuk field ini (04 §3.1)

### 10.6 FormSpec Skill

- [ ] 10.6.1 Format YAML frontmatter + Markdown bodPy — `name`, `description`, `applies_to_kind`, `min_core_spec_version` (`docs/ai/06-formspec-skill.md` §2)
- [ ] 10.6.2 Skill pertama: entity-authoring, form-layout, entity-extension-authoring, module-vendoring (06 §2, §4)
- [ ] 10.6.3 Bundling bersama instalasi `formspec` (ikut siklus rilis, dicek vs Core Spec lokal), dibaca lewat `list_skills()`/`read_skill()` (06 §2–§3)
- [ ] 10.6.4 Re-cek skill relevan sebagai bagian composite `propose_spec_file` — pemicu deterministik, bukan inisiatif LLM (06 §3)

### 10.7 Operational safety

- [ ] 10.7.1 Snapshot & undo — auto-backup file-level di `.formspec/consult/{session}/undo/` sebelum `apply_draft` menimpa (02 §4)
- [ ] 10.7.2 Guard read-only `vendors/` ditegakkan di semua tool tulis, bukan konvensi dokumentasi (03 §4)

---

## Fase 11: Resolusi Review Schema ↔ Docs ✅ COMPLETE (2026-07-31)

Menutup kontradiksi `pkg/spec`/JSON Schema/`renderers/web` vs `docs/spec/`.
Referensi: `docs/changelog/2026-07-31-001-resolusi-review-schema-docs.md`.

| Item                                                                                                                                                           | Status | Catatan                                                                                     |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------- |
| A1: `Config.spec.keys` → `$ref ConfigKey` (generator map-of-struct)                                                                                            | ✅     | `internal/genjsonschema` + regen schema                                                     |
| A2: `Entity` canonical (revert `Document`), konsolidasi validator                                                                                              | ✅     | `spec.go`/`entity.go`/`registry.go`/`kinds.go`/`manifest.ts`; `Document` = deprecated alias |
| B1: `ModuleSpec` +`vendor/datastore/config/ai_index`; `AppSpec` structured `publishes`/`consumes` +`app_renderer/theme_ref/auth_config_ref`                    | ✅     | binding `datastore` runtime tetap defer (2.9.4)                                             |
| B2: `EnvironmentSpec`/`PolicySpec` (`pkg/spec/control.go`) — schema tak lagi bare                                                                              | ✅     | eksekusi control-plane tetap defer                                                          |
| B3: `attachment` alias `file` + docs `spec.auth` §1.4                                                                                                          | ✅     | normalisasi di `ValidateEntitySpec`                                                         |
| C1: Form `sections`/`read_only`/`render:{mode}` + Table `filters`/`default_sort` — docs→kode                                                                   | ✅     | perbaiki fixture `internal/ui` (test sebelumnya merah)                                      |
| C2: Kanban docs → schema (`status_field` wajib, `columns{status,label}`, `card_template`) + Open zero-config/`group_by`/`drag_guard`/`wip_limit`/`card_fields` | ✅     | defer → `docs/plan/kanban-full-implementation.md`                                           |
| C3: Dashboard/Widget docs → ref-based + Open rendering widget                                                                                                  | ✅     | defer → Fase 5.7                                                                            |
| C4: Report docs → kode (`parameters`/`groups`/`export`, objek) + Open `source.filter`; TS/ renderer fix                                                        | ✅     | contoh manifest Report diperbarui                                                           |
| C5: Print hapus `formats` redundan                                                                                                                             | ✅     | satu format per manifest = `output.format`                                                  |
| C6: Page `binds`/`mode:custom` + BlockRef `needs:` ditandai Open                                                                                               | ✅     | defer → Fase 5                                                                              |

## Fase 12: Domain Infrastruktur formspec.dev 🚧 (2026-08-12)

Setup domain, landing, docs site, dan schema hosting. Referensi:
`docs/architecture/09-domain-map.md`, `docs/plan/rename-formspec.md`.

| Item                                                                                            | Status | Catatan                                                                            |
| ----------------------------------------------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------- |
| 12.1 DNS Cloudflare: nameserver, records, SSL Full (strict), redirect rules                     | 🔲     | Manual — butuh akses akun Cloudflare/registrar                                     |
| 12.2 Landing page `formspec.dev` (`site/`, Vite+React)                                          | ✅     | Build hijau; deploy Pages belum (butuh akun)                                       |
| 12.3 Docs site `docs.formspec.dev` (`docs-site/`, VitePress)                                    | ✅     | Build hijau (123 halaman); changelog/plan/presentations/technical-notes di-exclude |
| 12.4 Schema hosting `schemas.formspec.dev` (`scripts/publish-schemas.sh`)                       | ✅     | Stage v1 + alias latest; upload R2 belum                                           |
| 12.5 Email Resend `send.formspec.dev` (SPF/DKIM/DMARC)                                          | 🔲     | Manual — butuh akun Resend + set DNS                                               |
| 12.6 Reserve subdomain backend (registry/mcp/api/ops/status/try/control.\*)                     | 🔲     | Manual — Cloudflare DNS                                                            |
| 12.7 `security.txt`, `.well-known`, `robots.txt`, `_redirects`/`_headers`                       | ✅     | Di `site/public/` & `docs-site/public/`                                            |
| 12.8 Deploy CI: Pages project (site/, docs-site/, schemas)                                      | 🔲     | Manual — pakai Build watch paths (changelog 2026-08-14-001)                        |
| 12.9 Schema registry online: versi di `apiVersion`, `formspec schema`, cache lokal, hapus embed | ✅     | `docs/plan/schema-registry-online.md` + changelog 2026-08-14-004                   |

## Fase 13: Module Registry & Vendoring (npm-like) 🚧 (2026-08-20, planned)

**Goal**: Ekosistem module registry — `formspec module install/publish/list/uninstall`,
`formspec override adopt/diff`, `vendors/` + `overrides/` + `formspec.lock`, aktivasi
berbasis marker, dan registry server sebagai **FormSpec app (dogfooding)** untuk
`registry.formspec.dev` — trial & POC bahwa service internal FormSpec dibangun dengan
FormSpec sendiri.
**Model**: read-only vendoring + shadow copy (sesuai `docs/spec/platform/08-project-layout.md` §6).
**Sumber**: `docs/spec/platform/07-marketplace.md`, `08-project-layout.md` §6,
`02-workspace-app-module.md` §2.1, `docs/technical-notes/Forma-Technical-Note-Module-Vendoring-Aktivasi.md`,
`docs/cli-tools/02-formspec-cli.md` §9, `docs/architecture/09-domain-map.md`.
**Depends on**: Fase 6 (auth — 6.2 permission model + 6.4 API keys) untuk bagian
auth-dependent; Fase 8 (production serve) untuk deploy nyata ke `registry.formspec.dev`.

### 13.1 Local vendoring & activation (offline-first CLI)

- [ ] 13.1.1 `formspec.lock` schema + layout `vendors/` — paket baru `internal/vendor/` (types lockfile, marker parser, alias resolver). Entri per module: `source`, `version`, `checksum` tree, `signature`, `trust_tier`, `alias`, `installed_at` (YAML).
- [ ] 13.1.2 `formspec module install <source>` (git/folder/tarball dulu, offline) — `cmd/formspec/module.go` + `module_install.go`. Fetch → stage → validate (`module.yaml`) → copy ke `vendors/{module}/` → checksum → lock → tulis marker block ter-comment di `App.spec.modules`. Flag `--use` (aktif langsung), `--from` (registry, 13.3), `--yes` (skip consent).
- [ ] 13.1.3 Alias saat konflik nama — dihitung saat install (Opsi B: terhadap semua yang pernah di-install + module lokal), dicatat di lock + marker.
- [ ] 13.1.4 Boot-time enforcement — `AddManifestRoot("vendors/")` di `internal/entity/registry.go`; resolusi nama efektif dari lock; bentrok di set aktif → refuse boot; hanya module aktif (uncommented) yang diregister.
- [ ] 13.1.5 `formspec module list` / `uninstall` — list: status aktif/nonaktif + trust tier; uninstall: hapus `vendors/` + lock + marker (jaga status aktivasi).
- [ ] 13.1.6 `formspec verify` — checksum tree `vendors/` vs lock; tolak build kalau ada modifikasi manual.

### 13.2 Shadow copy (`overrides/`)

- [ ] 13.2.1 `formspec override adopt <module> <kind> <name>` — copy ke `overrides/{module}/{kind}.{name}.yaml`, catat checksum sumber di lock ("asal fork").
- [ ] 13.2.2 Boot-time replace-total + whitelist — `AddManifestRoot("overrides/")`; override menang atas `modules/`/`vendors/`; whitelist per kind (Form/Menu/VisualSpecKind instance boleh; Entity/Service/Workflow diblokir).
- [ ] 13.2.3 `formspec override diff <module> <kind> <name>` — bandingkan shadow copy vs upstream.
- [ ] 13.2.4 Drift detection — saat install/update, bandingkan checksum base baru vs "asal fork" → warning.

### 13.3 Registry sebagai FormSpec app (dogfooding POC)

> Registry TIDAK dibangun sebagai Go binary hand-written — melainkan sebagai **FormSpec app**.
> Native embedding sudah ada (`examples/reference-app/main.go` + `docs/runtimes/02-formspec-resource.md`);
> `cmd/formspec-registry` = wrapper tipis yang meng-embed engine + spec registry
> (`verticals/registry/spec/` via `//go:embed` atau `--spec`). Tidak perlu ubah spec —
> hanya tambah docs: "native app binary" sebagai deployment mode first-class.

- [ ] 13.3.1 Scaffold `verticals/registry/` — App + Module manifests, `formspec generate auth` (API key untuk publish). POC jalan via `formspec dev`.
- [ ] 13.3.2 Entities `Module` / `ModuleVersion` / `Vendor` — state machine (draft→published→deprecated), events, permissions; tarball → `ctx.storage` (ref di Entity, blob di storage).
- [ ] 13.3.3 Services `signature-verify` (ed25519), `checksum`, `search` (list filters dulu; pgvector 10.5.3 nanti).
- [ ] 13.3.4 `spec.expose` — public read (search/detail/download), authenticated publish (API key, 2.5.1).
- [ ] 13.3.5 Workflow review trust tier `verified` (approval).
- [ ] 13.3.6 `formspec sign` — ed25519 keypair, sign checksum tree module; registry verifikasi saat publish; trust tier (official/verified/community).
- [ ] 13.3.7 `formspec module publish` — sign + upload; `--registry`/env `FORMSPEC_MODULE_REGISTRY` (default `https://registry.formspec.dev`).
- [ ] 13.3.8 `formspec module install --from registry.formspec.dev` — download tarball → verifikasi signature → alur 13.1.
- [ ] 13.3.9 (deferred) Marketplace layer — pricing/metering/licensing per `07-marketplace.md` §4–§9 — out of scope fase ini.

### 13.4 Docs & tests

- [ ] 13.4.1 Update `docs/spec/platform/08-project-layout.md` §6 (target desain → implemented) + resolve open questions §6.5.
- [ ] 13.4.2 Update `docs/cli-tools/02-formspec-cli.md` §9.
- [ ] 13.4.3 Tests: unit (lock/marker/alias/checksum), integration (install→boot), e2e (publish→install).
- [ ] 13.4.4 Changelog per hari + update todo.

**Dependensi**: Fase 6 (6.2 permission model + 6.4 API keys) wajib selesai untuk bagian
auth-dependent (13.3.4–13.3.8); Fase 8 (production serve) untuk deploy nyata ke
`registry.formspec.dev`. Data model/API/storage (13.3.1–13.3.3) bisa dibangun paralel
tanpa auth.

## Deferred (Cloud Phase)

| Area                                                                                                                                                                                                                                               | Reason                                                                                                                                                                                                                                                                                                                                                                                              |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `formspec-ctl` (all modes: region, cluster, standalone)                                                                                                                                                                                            | Control Plane — cloud deployment phase                                                                                                                                                                                                                                                                                                                                                              |
| K8s Operator (`formspec-operator`)                                                                                                                                                                                                                 | Production infrastructure                                                                                                                                                                                                                                                                                                                                                                           |
| Marketplace (pricing, metering, licensing)                                                                                                                                                                                                         | Business features — registry foundation (upload/download/listing) ada di **Fase 13.3**; lapisan pricing/metering/licensing tetap deferred (13.3.9)                                                                                                                                                                                                                                                  |
| Control Plane (Environment, Policy/OPA, transparency log, key model, contracts)                                                                                                                                                                    | Governance                                                                                                                                                                                                                                                                                                                                                                                          |
| Two-stage deployment pipeline (register→deploy, snapshot, evidence)                                                                                                                                                                                | Requires Control Plane                                                                                                                                                                                                                                                                                                                                                                              |
| `formspec promote/archive/saga/script/freeze/rollback/lock/workspace/suspend`                                                                                                                                                                      | CLI — depend on Control Plane. **`module`/`sign` sudah direncanakan di Fase 13**                                                                                                                                                                                                                                                                                                                    |
| Module vendoring & activation — `vendors/`/`external/` folders, `formspec.lock`, install-time alias on name conflict, marker-based activation (`--use`), shadow copy (`formspec override adopt/diff`) for `Form`/`Menu`/`VisualSpecKind` instances | **Dipindah ke Fase 13** (13.1–13.2) — planned 2026-08-20. Design agreed (`docs/spec/platform/08-project-layout.md` §6, `docs/spec/platform/02-workspace-app-module.md` §2.1). **Sebagian sudah landing untuk auth**: `external/` (module user-kustom, di-commit) di-load loader + menang atas `formspec.core` defaults; `formspec generate auth` meng-scaffold auth module ke `external/auth` (6.1) |
| `formspec consult` task breakdown — see **Fase 10** above                                                                                                                                                                                          | Zero implementation; realistically starts after Module vendoring or Marketplace (below) lands                                                                                                                                                                                                                                                                                                       |
| Conformance test-suite VisualSpecKind/Renderer/PersistBackend (fixture, trust tier `verified`/`official`)                                                                                                                                          | Terkait Marketplace/distribusi — `frontend/02` §6, `frontend/03` §5, `backend/04` §7                                                                                                                                                                                                                                                                                                                |
| Print: thermal/dotmatrix                                                                                                                                                                                                                           | Niche — PDF sufficient                                                                                                                                                                                                                                                                                                                                                                              |
| gRPC + mTLS transport                                                                                                                                                                                                                              | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Platform signing (HSM/KMS)                                                                                                                                                                                                                         | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Generic Docker image (`formahub/formspec-resource`)                                                                                                                                                                                                | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Scale-to-zero                                                                                                                                                                                                                                      | Cloud deployment                                                                                                                                                                                                                                                                                                                                                                                    |
| Unmanaged client codegen (Dart, Flutter)                                                                                                                                                                                                           | Future SDK                                                                                                                                                                                                                                                                                                                                                                                          |


Core Basic MUSTs (02-core-basic.md Part VII, lines 659–673) with NO corresponding todo.md task
Cross-checked each conformance clause against all Fase 1–6 items (grep for forma.core, grant, category, provisioning, Config/Service/Migration kinds returned no matching tasks).

Genuine gaps (no task anywhere):

forma.core resources — Conformance §10 / 02-core-basic.md:625–627. The spec mandates entities workspace, user, app-membership, role, role-assignment, api-key, session, job, audit-log, failed-event, setting + services health, metrics. todo.md has no task for the forma.core entity set. audit-log is partially served by the audit.go Audit Logger (line 27) and failed-event partially by the outbox (line 26), but neither is exposed as a forma.core resource, and none of workspace/user/role/role-assignment/app-membership/api-key/session/setting entities are tasked. This is the largest gap. (Note: the task's other suggested examples — backup/restore and events durability and state machine — are in fact covered; see below.)
Cross-app grant verification — Conformance §7 / 02-core-basic.md:540–542 (§15.3). Runtime MUST verify a signed grant before routing cross-app calls (ungranted → 404). No task in any Fase (grants appear only in the D54 page-materialization sense, line 300, which is unrelated). Fase 5 Control Plane lists Environment/Policy/signing but not grant enforcement.
Category schemas + cross-category block — Conformance §8 / 02-core-basic.md:597, 563 (§19). category → PostgreSQL schema mapping and CROSS_CATEGORY_ACCESS_DENIED on cross-category SQL joins. The DDL generator task (1.1.5, line 20) does not mention category schemas or the cross-category guard. No task.
kind: Migration DDL-only execution + DML rejection — Conformance §9 / 02-core-basic.md:135–137, 615–617. 1.1.6 (line 21) is the structural migration runner; the developer-written custom-DDL kind with runtime DML rejection is not tasked (6.5 "forma migrate" is the CLI verb, not the kind).
kind: Service, kind: Config, kind: App handling — Conformance §2 / 02-core-basic.md:664. Conformance requires all of App, Module, Entity, Service, Config, Migration. todo.md tasks cover Entity (1.2) and Module footprint (1.5) but there is no task for loading/serving kind: Service (stateless compute, §4.2), kind: Config (§7 — ctx.config.get), or kind: App root manifest (§4.4).
Workspace provisioning lifecycle — 02-core-basic.md:621–623 (§21). create → provisioning → seed default roles + reference seeds → active emitting workspace.activated, plus ctx.tenant.config(). No task. (Related to gap 1.)
Covered — NOT gaps (contrary to the task's suggested examples):

Backup/restore (§25, Conformance §10): tasked at 6.6 "forma backup create|inspect|restore (D41)" (line 264).
Events durability contract (§12): tasked across Fase 3.1–3.4 (lines 176–179), incl. outbox bridge; outbox table already built (1.1.11, line 26).
State machine with guards (§14): tasked at Fase 2.5 "State Machine engine — transitions + guards" (line 167).
Optimistic concurrency / idempotency (§11.3): CAS in 1.1.7 (line 22), idempotency store 1.1.10 (line 25).
Scripting sandbox (§26): Fase 2.1/2.6.
Codegen forma generate (§27): Fase 6.4 (line 262).
Summary of recommended edits (no files changed)
Renumber/relabel D51–D58 so they don't impersonate ratified spec decisions, or add them to 11-reference.md §2 if intended to be canonical.
Line 251: change "per D50" → "per D47" (Plane Protocol).
Reconcile D52 (/_/api/v1) and D53 (/{ws}/{app}/{page}/api/v1) with 02-core-basic.md §16 + D49 + D38, or flag them explicitly as proposed extensions beyond Core Basic.
Reconcile D58 with the five impl types (§10) and D46 trust tiers; drop or re-ground "cloud-hosted handler."
Fix D55/line 287 cross-process wording to "configured protocol adapter (REST required, gRPC recommended)."
Reconcile the 152 / 166 / 158 test counts.
Add tasks for the six Core Basic conformance gaps, above all the forma.core resource set.
Agent audit pertama (1.1/1.6) selesai — temuan penting: retensi idempotency masih hardcode 24h (setter WithTTL ada tapi belum di-wire ke config), pagination belum clamp sesuai batas normatif baru, dan shorthand after:<field> bentuk colon tidak ter-parse. Menunggu dua agent lainnya.

I have all the evidence needed. Here is my audit.

Audit: todo.md §1.2–1.5 vs. Implementation
Test-count reality check (measured via go test -v -count=1)
Package	todo claim	Actual top-level funcs	Actual RUN events (incl. subtests)	Verdict
internal/entity	9	9	9	VERIFIED
internal/api	23	21	24	IMPRECISE (21 funcs / 24 cases, not 23)
internal/auth	7	5	32	FALSE as written (5 funcs; 32 subtest cases)
internal/permission	18	14	37	FALSE as written (14 funcs; 37 subtest cases)
All tests pass (ok for all four packages). The per-package "Test Summary" numbers (lines 310–321) don't correspond cleanly to either func-count or subtest-count. The prose claims are closer: jwt_test.go "8 test cases" ≈ 8 t.Run subtests in TestJWTValidator_Validate + TestDevValidator (jwt_test.go:41–156). auth_test.go "19 test cases" — 3 table-driven funcs whose loops expand to many cases at runtime.

§1.2 Entity Registry
1.2.1 LoadEntities / SyncSchema / GetEntityStore — VERIFIED. registry.go:74 (LoadEntities), registry.go:155 (SyncSchema), registry.go:200 (GetEntityStore). LoadEntities filters kind: Entity (registry.go:92), parses, validates, registers.
1.2.2 ListEntities / GetEntity / GetByCharacteristic — PARTIAL. ListEntities (registry.go:235) and GetEntity (registry.go:227) VERIFIED. But the method claimed as GetByCharacteristic does not exist — the actual method is GetEntitiesByCharacteristic (registry.go:274). Grep for GetByCharacteristic returns zero hits. Functionality present; name in todo is wrong.
1.2.3 cmd/forma-entity-sync load→register→sync — VERIFIED. cmd/forma-entity-sync/main.go: LoadEntities (:49), ListEntities (:56), SyncSchema (:69), then GetEntityStore + insert/verify (:81–127).
1.2.4 tests (9) — VERIFIED. 9 functions in registry_test.go including TestRegistry_LoadEntities_FromGeneralLedger.
§1.3 REST API
1.3.1 D49 deny-by-default — VERIFIED. generator.go:22 skips entities with len(es.Expose)==0. Comment doc at descriptor.go:1–6.
1.3.2 descriptor + generator (5 tests) — VERIFIED. descriptor.go (RouteDescriptor + StandardRESTActions), generator.go. 5 route-gen tests exist (NoExpose, WithExpose, RouteDescriptor, SummaryEntity, NoActionsFilter).
1.3.3 handler.go — 5 auto-handlers — VERIFIED. HandleList/Find/Create/Update/Delete at handler.go:32/69/95/138/195. (Plus HandleCustomAction:373.)
1.3.4 middleware.go — Tenant, Auth, RequestID, CORS, Recovery, Log — VERIFIED. All six present: TenantMiddleware:33, AuthMiddleware:60, RequestIDMiddleware:140, CORSMiddleware:150, RecoveryMiddleware:165, LoggingMiddleware:178.
1.3.5 router.go — chi, /{workspace}/api/v1/... — VERIFIED. router.go:38 chi.NewRouter; mount r.Route("/{workspace}") → r.Route("/api/v1") (:49–50) then /{module}/{plural}[/{id}] (registerRoute:73). Actual mount pattern = /{workspace}/api/v1/{module}/{plural}[/{id}], matching the claim. Full middleware stack wired at :41–46; per-route RequirePermission via r.With() at :129.
1.3.6 cmd/forma-serve load→sync→routes→serve — VERIFIED. cmd/forma-serve/main.go: load (:74), sync (:96), BuildRoutes (:108), ListenAndServe (:128).
1.3.7 Response Envelopes — PARTIAL / spec-nonconforming. Types exist (handler.go:216–254) but do not fully match spec §16 (docs/spec/02-core-basic.md:561):
SingleResponse {data, meta:{request_id, timestamp}} — matches spec ✓
ListResponse {data, meta:{page,per_page,total,total_pages}} — missing links field required by §16 ("list { data, meta:{...}, links }").
ErrorResponse {error:{code,message}, meta} — ErrorDetail (handler.go:250) has only Code+Message; missing details required by §16/§ (line 493: details: [{level, field?, message}]). writeValidationErrors concatenates errors into a single message string instead of a structured details array.
§1.4 Auth & Tenant Middleware
auth.go — Identity, TokenValidator, HasPermission (wildcard+exact) — VERIFIED. Identity:24, TokenValidator:82, HasPermission:38 with exact (:47), single-segment wildcard x.y.* (:51–58), super-wildcard * (:44), and public (:39).
jwt.go — HS256/RS256/ES256 — PARTIAL. Parser allows HS/RS/ES 256/384/512 (jwt.go:59) and Validate handles both HMAC (secret, :72) and asymmetric keys (publicKey, :78). NewJWTValidator = HS256 (:33); NewJWTValidatorWithKey = asymmetric (:43). So RS256/ES256 are supported in library code but never tested (jwt_test.go only signs with SigningMethodHS256) and not wired to the CLI — forma-serve only calls NewJWTValidator (main.go:43); there is no flag to supply an RSA/ECDSA public key. "All three supported" is true at the type level only.
dev.go — VERIFIED. DevValidator returns {UserID:"developer", WorkspaceID:"demo", Permissions:["*"], Roles:["admin"]} (dev.go:19–25).
RequirePermission per route — VERIFIED. middleware.go:112 factory; wired per-route in router.go:128–130. Returns 401 if identity nil (:123), 403 if lacking perm (:129), passthrough for ""/public (:116).
Cross-tenant isolation / "identity's workspace overrides URL tenant" (line 85) — PARTIAL. The override is VERIFIED: AuthMiddleware sets tenant from identity.WorkspaceID (middleware.go:86–89), and tenantFromContext prefers identity workspace (handler.go:281–285). URL→tenant extraction VERIFIED (TenantMiddleware:33–51). However there is NO cross-tenant mismatch enforcement: no code compares identity.WorkspaceID against the URL {workspace} slug. A token scoped to workspace A hitting /B/api/... silently uses A (override) rather than being rejected — isolation-by-override, not rejection. The "1.4.4 cross-tenant isolation (3 tests)" claim maps to TestTenantMiddleware_Isolation (api_test.go:742), which only asserts URL-path→tenant extraction (/acme/...→acme); it does not test cross-tenant denial.
Spec-conformance flag: 404 vs 403 on cross-tenant
Not implemented. Spec expectation (404, not 403, on cross-tenant access) has no corresponding code. Grep for tenant-mismatch handling found none — the only StatusNotFound uses are "entity not found" (handler.go) and unregistered-action/entity in custom routes (router.go:98,115). There is no path that detects a workspace mismatch and returns 404. TestHTTPRouter_404OnUnexposed (api_test.go:315) tests unexposed-entity 404, unrelated to tenant isolation.

§1.5 Permission Enforcement
1.5.1 data model in internal/permission — VERIFIED. PermissionEntry:39, UsesEntry:50, ModuleFootprint:82, AuthChecker:183, ValidatePermissionFormat:114, AutoPrefixPermission:139, ParseResourceTarget:167 (all permission.go).
1.5.2 Registry register/aggregate/query — VERIFIED. registry.go: RegisterAction:44, GetModuleFootprint:93, FindPermission:130, PermissionExists:137, thread-safe via RWMutex.
1.5.3 validator — format check, auto-prefix, cross-module detection — VERIFIED. validator.go ValidateUses:15, ValidateAction:87, BuildUsesEntry:110; cross-module-write detection in registry.go:78–86 (AccessWrite to another module → CrossModuleWrites).
1.5.4 ctx.auth.has() integration — VERIFIED (with caveat). auth.go PermissionChecker:97, SetPermissionChecker:123, CtxAuthHas:132, defaultPermissionChecker:107. Caveat below.
1.5.5 auto-register on LoadEntities — PARTIAL. registry.go:127–136 registers each declared action.RequiredPermission + uses via RegisterAction. But standard CRUD permissions are NOT registered. The block at registry.go:138–147 has a comment "Also register standard CRUD permissions if the entity is exposed" yet only calls SetModuleDescription (:146) — it never registers {module}.{plural}.{list|view|create|update|delete} into permIndex. Consequence: PermissionExists("billing.customers.list") returns false, so CtxAuthHas (auth.go:141) would deny standard CRUD perms for non-wildcard identities. Route guarding still works because RequirePermission calls identity.HasPermission directly (not PermissionExists) — so this gap only affects ctx.auth.has() (Fase 2 surface).
1.5.6 custom action RequiredPermission from YAML — VERIFIED. generator.go GenerateCustomActionRoutes:159 prefers action.RequiredPermission, deriving {module}.{plural}.{action} only when empty. router.go:93–121 handles "custom" handler type.
1.5.7 UsesEnforcement stub + error codes — PARTIAL. Stub UsesEnforcement:239 and writeUsesViolation/writeConfigAccessDenied/writeKvstoreAccessDenied (:257/263/269) exist with codes USES_VIOLATION, CONFIG_ACCESS_DENIED, KVSTORE_ACCESS_DENIED. However all four are dead code — grep confirms none are referenced/wired into the router chain (UsesEnforcement is never added to any route in router.go). This is acknowledged as a stub in the code comments (:233) and "Deferred to Fase 2" in todo (:116), so consistent with intent, but the middleware is not actually active anywhere.
1.5.8 forma-serve --strict + footprint print — VERIFIED. main.go:30 --strict flag, :51 SetStrictMode, :84–94 prints AllFootprints() + total permission count.
1.5.9 tests (22) — PARTIAL/IMPRECISE. permission_test.go has 14 top-level funcs; runtime expands to 37 RUN cases (the prose "22 test cases" at todo:105 doesn't match either count exactly).
Additional spec-conformance flag: D52 "dual namespace" (todo line 298)
FALSE — the internal /_/api/v1/... namespace does not exist in code. Grep for _/api, /_/, internal namespace, dual namespace, _meta across internal/ and cmd/ returns zero hits. BuildHTTP (router.go:37–66) mounts only /{workspace}/api/v1/... plus /health. There is no auto/all-entities internal route tree. D52 as described (admin panel works without expose via /_/) is unimplemented. Similarly the D53/4.7 page-scoped /{ws}/{app}/{page}/... namespace is absent (correctly listed as deferred, todo:288).

Summary of discrepancies to correct in todo.md
§1.2.2: method is GetEntitiesByCharacteristic, not GetByCharacteristic.
§1.3.7: ListResponse missing links; ErrorResponse missing structured details — not fully §16-conformant despite ✅.
§1.4 jwt: RS256/ES256 supported only at type level — untested and not CLI-wired (only HS256 reachable via forma-serve).
§1.4.4 / spec: no cross-tenant mismatch enforcement and no 404-on-cross-tenant; the "3 tests" only cover URL→tenant extraction.
§1.5.5: standard CRUD permissions are NOT auto-registered (only module description is) — the code comment is misleading.
§1.5.7: UsesEnforcement + error-code writers are defined but never wired (dead code) — acceptable as a declared stub but not "active."
D52 dual-namespace (/_/api/v1/...) claimed in line 298 does not exist in code at all.
Test-count figures in the summary table (auth 7, permission 18, api 23) don't match measured func or subtest counts.
Audit Report: todo.md §1.1 (Database Layer) and §1.6 (Validation Engine)
All file paths are absolute under /workspaces/forma/.

Section 1.1 — Database Layer (internal/db/)
1.1.1 DB Interface + Factory — VERIFIED
internal/db/interface.go:16-55 defines DB and Tx interfaces.
internal/db/db.go:15-29 Open(dsn) factory dispatches to openSQLite/openPostgres by driver.
Tests: claimed 5. internal/db/db_test.go has 9 real test funcs + TestMain (factory + SQLite driver tests combined here). Undercounted, not false.
1.1.2 DSN Config Parser — VERIFIED
internal/db/config.go:41-114 ParseDSN handles sqlite (2-/3-slash + ?query), postgres/postgresql (userinfo, host, port default 5432, schema param), no-scheme→sqlite default, error on unknown scheme.
Tests: claimed 9; config_test.go actual 14 funcs.
1.1.3 SQLite Driver (WAL, FK, busy_timeout) — VERIFIED (test-count caveat)
internal/db/sqlite_db.go:68-71 DSN pragmas journal_mode(WAL), foreign_keys(ON), busy_timeout(5000), plus applyPragmas at :95-106 re-issues PRAGMA journal_mode=WAL / foreign_keys=ON / busy_timeout=5000. SetMaxOpenConns(1) at :50.
Tests: claimed 7, but there is no sqlite_db_test.go. SQLite driver is exercised via db_test.go (TestSQLite_Ping/ExecAndQuery/HasTable/TransactionCommit/TransactionRollback/DefaultDevDSN ≈ 6). No test explicitly asserts WAL/busy_timeout pragma values.
1.1.4 PostgreSQL Driver (pgx/stdlib, pool) — VERIFIED
internal/db/postgres_db.go:8 imports github.com/jackc/pgx/v5/stdlib; :40 sql.Open("pgx", …); pool via SetMaxOpenConns(25)/SetMaxIdleConns(10) at :45-46. No tests claimed, none needed.
1.1.5 Entity→DDL Generator (dialect-aware, child tables) — VERIFIED
internal/db/ddl.go:59-78 dialectFor (SQLite text/json_extract vs PG uuid/jsonb/timestamptz). GenerateEntityDDL:81 emits normative columns, generated columns for indexed/unique fields, enum CHECK, child tables via generateChildTableDDL:315 (parent__child, FK ON DELETE CASCADE).
Tests: claimed 7; ddl_test.go actual 10.
1.1.6 Schema Migration Runner (idempotent, checksum) — VERIFIED
internal/db/migrate.go:388 checksumDDL = SHA256. PlanMigrations:259-266 skips when recorded checksum matches (idempotent); EnsureSystemTables uses CREATE TABLE IF NOT EXISTS. Add-only in v1 (checksum mismatch → skip, documented in-code).
Tests: claimed 7; migrate_test.go actual 8.
1.1.7 CRUD Query Builder (tenant isolation, CAS, pagination) — PARTIAL
Tenant isolation: VERIFIED — every query includes tenant_id = ? (crud.go:219, 426, 539).
CAS / optimistic concurrency: VERIFIED — Update uses ... AND version = ? with version = version + 1 … RETURNING version (crud.go:294-317); conflict → ErrNotFound.
Pagination: PARTIAL / does NOT meet the spec's normative bounds. List at crud.go:414-419:

if params.PerPage < 1 || params.PerPage > 100 { params.PerPage = 20 }
Spec §558-559 requires: default 20, max 100, values above max clamped to 100, and non-numeric/negative → 400 VALIDATION_ERROR. The code instead resets over-100 and negative values to 20 (no clamp-to-100), and the handler (api/handler.go:42-43) parses page/per_page with strconv.Atoi ignoring errors (non-numeric → 0 → silently defaulted, never 400). So pagination exists but is non-conformant to the new normative bounds.
Tests: claimed 8; crud_test.go actual 22.
1.1.8 Child Storage (JSONB inline + table mode) — VERIFIED
internal/db/child.go:15-39 ChildStore with storage "jsonb"/"table"; jsonb paths are no-ops, table mode does extract/insert/update(replace-all)/delete/hydrate. Tests: claimed 6, child_test.go actual 6. ✓
1.1.9 Natural Key Counter (yearly/monthly/daily/never) — VERIFIED
internal/db/counter.go:148-162 computePeriod returns yearly 2026, monthly 2026-07, daily 2026-07-05, never/""→global. UPSERT with SQLite fallback. Tests: claimed 8, actual 8. ✓
1.1.10 Idempotency Store (TryClaim/Complete/Fail) — VERIFIED (naming) / retention FALSE
internal/db/idempotency.go: TryClaim:56, RecordCompleted:103, RecordFailed:111 (method names are RecordCompleted/RecordFailed, not literally Complete/Fail). Replay-on-completed, retry-on-pending/failed, expiry reset. Tests: claimed 8, actual 8. ✓
Retention: FALSE vs spec §11.3/§420. TTL is hard-coded ttl: 24 * time.Hour at idempotency.go:40. There is no reference to config key core.idempotency_retention anywhere (grep confirms zero hits). Spec explicitly states "implementations MUST NOT hard-code the window." A WithTTL setter exists but is never wired to config. This is a genuine spec violation.
1.1.11 Outbox (at-least-once, exponential backoff) — VERIFIED
internal/db/outbox.go: Enqueue (RETURNING + SQLite last_insert_rowid fallback), Dequeue claims via status='delivering', MarkFailed:138-167 exponential backoff 1<<retry seconds capped at 3600s, dead-letters on retry > maxRetries. At-least-once semantics documented and implemented. Tests: claimed 10, actual 10. ✓
Audit Logger — VERIFIED
internal/db/audit.go writeAuditLog:124 (dialect-aware INSERT), read helpers ListByEntity/ListByTenant. Called from CRUD insert/update/delete. Note: audit failures are swallowed (_ = err) in crud.go — "write-once" is not transactionally enforced. Tests: none claimed; audit_test.go actual 4.
Outbox Worker — VERIFIED
internal/db/outbox_worker.go background runLoop (poll ticker + cleanup ticker), processBatch delivers via EventHandler, marks completed/failed. Start/Stop lifecycle. Tests: none claimed; outbox_worker_test.go actual 4.
DB Dev Init — VERIFIED
cmd/forma-dev-init/main.go opens sqlite:.forma/data.db, EnsureSystemTables, ApplyMigrations, inserts sample data. Exists and functions as claimed.
Section 1.6 — Validation Engine
Existing + new field validators in internal/db/crud.go — mostly VERIFIED
validateSingleRule (crud.go:821-962) implements: email:823, pattern:832, min_length:849, max_length:859, min:869, max:876, positive:883, url:889, precision:898, future:914, past:927, min_items:940, max_items:950. required is enforced separately in validateRequired:103; default via applyDefaults:91. All present and functional. urlRegex:968, timeNow:971, parseDateTime:976 helpers exist as claimed.

Caveat: precision uses float64(int(num*multiplier)) truncation — works for small values but is fragile for large/float-imprecise numbers; not "accounting-grade" as labeled.
after:<field> / before:<field> cross-field — PARTIAL (shorthand syntax not parsed)
Wiring: crud.go:807-814 invokes validation.ValidateCrossField for rule names after/after_field/before/before_field/exists. validation/validator.go:31-101 compares datetimes correctly (after_field/before_field aliases supported).
Gap: the spec §10.6 documents the colon syntax after:<field>. ValidationRule.UnmarshalYAML (pkg/spec/entity.go:104-133) does NOT split on : — a bare string after:end_date becomes Name="after:end_date", which never matches the == "after" check, so cross-field validation silently does not run. It only works when authored as YAML map form {after: end_date} (→ Name="after", Value="end_date"). Tests (validation_test.go:100-146) exclusively use the map form, masking this gap.
internal/validation/validator.go — VERIFIED with subset caveat
ValidateCrossField:31 and ValidateActionParams:132 exist as claimed.
Caveat: validateActionParamRule:164-172 supports only a subset — min_length, max_length, min, max, positive, email, pattern, url (+ required handled inline). It does NOT support precision, future, past, min_items, max_items, or cross-field rules for action params, and returns an error for any other rule name.
Tests: claimed 9; validation_test.go actual 9. ✓ (matches)
exists:<resource> stub — CONFIRMED always-passes (FLAG)
validation/validator.go:55-64: parses target then return nil with // TODO(Fase 2): actual DB query. Confirmed it always passes. TestValidateCrossField_Exists (validation_test.go:156-163) explicitly asserts the stub passes. This matches todo's "Deferred to Fase 2" note, but it is a live always-true validator in the code path — flagged as required.
API integration — VERIFIED
internal/api/handler.go:373-394 HandleCustomAction decodes params, calls validation.ValidateActionParams when actionSpec.Params.Validate is non-empty, and on errors calls writeValidationErrors:358 (422 VALIDATION_ERROR). Execution returns 501 (Fase 2), as documented.
internal/api/router.go:93-121 custom-action wiring: resolves entity spec from registry (GetEntity), finds the action by name, and wires HandleCustomAction. Permission middleware wrapped via r.With(RequirePermission(...)) at :128-129.
§10.6 Field-Rule Vocabulary Coverage (Core-Basic)
Spec §10.6 (docs/spec/02-core-basic.md:354) vocabulary vs implementation:

Rule	Status
required, optional	required VERIFIED; optional is implicit no-op (acceptable)
min_length, max_length, pattern, email, url	VERIFIED (crud.go)
min, max, positive, precision	VERIFIED (crud.go)
future, past	VERIFIED
after:<field>, before:<field>	PARTIAL — works only via YAML map form, not the documented after:<field> colon shorthand
min_items, max_items	VERIFIED
unique	Handled at DDL level (unique index via f.Unique), not as a validateSingleRule case — acceptable but different mechanism
exists:<resource>	STUB, always-passes (Fase-2, flagged above)
script (Starlark escape hatch)	NOT implemented — deferred to Fase 2 (documented in todo)
All §10.6 Core-Basic rules are implemented except exists (stub) and script (deferred) — both documented as Fase-2 deferrals in todo.md.

Test Count: Claimed vs Actual (top-level func Test…)
File	Claimed	Actual	Note
db_test.go (1.1.1)	5	9 (+TestMain)	interface+factory+SQLite driver combined
config_test.go (1.1.2)	9	14	
sqlite_db (1.1.3)	7	0 dedicated file	~6 SQLite tests live in db_test.go
ddl_test.go (1.1.5)	7	10	
migrate_test.go (1.1.6)	7	8	
crud_test.go (1.1.7)	8	22	
child_test.go (1.1.8)	6	6	match
counter_test.go (1.1.9)	8	8	match
idempotency_test.go (1.1.10)	8	8	match
outbox_test.go (1.1.11)	10	10	match
audit_test.go	—	4	
outbox_worker_test.go	—	4	
validation_test.go (1.6)	9	9	match
The package-level total claim internal/db = 89 (todo Test Summary) does not match the ~103 top-level test functions found (the two counts use different methodologies; not a false claim, but the per-file "Tests" column understates actuals in several rows and overstates 1.1.3 which has no dedicated test file).

Summary of Material Findings (skeptical flags)
FALSE — idempotency retention (idempotency.go:40): 24h is hard-coded; no core.idempotency_retention config key exists anywhere. Directly violates spec §11.3/§420 ("MUST NOT hard-code the window").
PARTIAL — pagination bounds (crud.go:417-419 + handler.go:42-43): over-max and negative per_page reset to 20 instead of clamping to 100; non-numeric/negative never produce 400 VALIDATION_ERROR. Non-conformant with normative §559.
PARTIAL — after:/before: shorthand (pkg/spec/entity.go:104 + crud.go:808): the documented after:<field> colon syntax is not parsed; only the YAML map form works. Tests use only the map form, hiding the gap.
FLAG — exists:<resource> (validator.go:55-64): confirmed always-passes stub in a live code path (documented Fase-2 deferral).
Minor — action-param validation supports only a subset of rules; precision truncation is not robust for large/float values; audit-log write failures are silently swallowed (not truly write-once-guaranteed); 1.1.3 has no dedicated test file despite claiming 7 tests.
All 14 claimed §1.1 files and both §1.6 validator locations exist and are substantially implemented; the exceptions above are the concrete deviations from the ✅ claims and the spec.
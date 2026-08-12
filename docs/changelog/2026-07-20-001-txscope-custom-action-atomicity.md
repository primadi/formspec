# Request-Scoped Transaction Propagation for Custom Actions (TxScope)

**Date:** 2026-07-20
**Plan:** approved plan `generic-whistling-knuth` — follow-up to Fase 2.1 (todo 2.1.1), user-directed

## Context

Fase 2.1 made `HandleCreate`/`HandleUpdate` atomic (entity mutation + outbox
in one transaction), but explicitly left `HandleCustomAction` as a known
gap: a custom action's Starlark script can call `resource.save()`/
`.create()`/`.call()` any number of times, each previously committing on
its own. The user asked for this closed under two rules: (1) ctx must stay
consistent across every execution path — script, native, and sidecar alike
— so once a transaction is open, everything joins it; (2) a transaction
may span multiple *modules* as long as they share the same physical store,
but must error if it would span two genuinely different stores.

## Changes

### New: `TxScope` (`renderers/jsonbpersist/txscope.go`)
- Tracks at most one open transaction per action execution, keyed by
  **store identity** (`db.DB` instance), not module — corrected from an
  initial module-keyed design after user feedback: this codebase gives
  every `EntityStore` the same shared `db.DB` today (no per-module
  Datastore yet, Fase 2.9), so a custom action touching several modules
  must join fine; only a genuinely different connection pool must error.
- `join(ctx, base)` — write path: opens on first call, reuses on same
  base, returns `ErrCrossStoreTx` on a different base.
- `peek(base)` — read path: non-mutating, never opens/errors, returns the
  open tx only if it matches `base`. Deliberately **not** module-gated —
  SQLite's `SetMaxOpenConns(1)` means a read falling back to `s.db` while
  *any* transaction holds the sole connection deadlocks, not just delays.
- Process-local `scopeRegistry` (`RegisterScope`/`LookupScope`/
  `UnregisterScope`) correlates the sidecar protocol's two separate HTTP
  round-trips (outbound `/invoke/...`, inbound `/ctx/entity/{op}` — both
  served by the same `formspec dev` process, confirmed in `cmd/formspec/dev.go`).

### ctx-threading fix (prerequisite, previously silently dropped)
- `internal/starlark/executor.go`: `ScriptExecutor.Execute` and its five
  handler func-type fields (`SaveHandler`/`CallHandler`/`LoadHandler`/
  `CreateHandler`/`NextKeyHandler`) gained a `ctx context.Context` param.
- `internal/action/script.go`: passes `ctx` through instead of dropping it.
- `resource/formspec.go`: the five `Set*Handler` closures use the passed
  `ctx` instead of hardcoding `context.Background()`.

### `EntityStore` scope-awareness (`renderers/jsonbpersist/crud.go`, `tx.go`)
- New `runTx`/`writeDB`/`txReadDB` helpers: writes join an active scope
  (erroring via `join` on a cross-store mismatch) or fall back to
  self-contained `InTx`/direct `s.db`, unchanged when no scope is active.
- `Insert`/`Update`/`SoftDelete`/`UpdateFields`/`IncrementField`/
  `DecrementField` all route through these helpers.
- `getByIDRaw`/`GetByID`/`ValidateRelationTargets` read through
  `txReadDB` for read-your-own-writes within an active scope.

### Bug found and fixed during verification: self-deadlock in `Insert`
`ValidateRelationTargets` was called *inside* `Insert`'s own `InTx`/`runTx`
closure but resolved its DB independently via `ctx`/`s.db` rather than
using the closure's `txdb` — a second, independent query against `s.db`
while that same transaction already held SQLite's sole connection
deadlocked instead of erroring. Full-suite testing (`examples/Clinic-UI-Showcase`,
which exercises `Insert` on entities with real `relation` fields) hung
completely until this was found via `go test -timeout 15s`'s goroutine
dump. Fixed by giving `ValidateRelationTargets` an explicit `database DB`
parameter: `Insert` passes its own `txdb`, `Update` passes `txReadDB(ctx, s.db)`
(safe there — validation runs before `Update` opens any transaction).

### `HandleCustomAction` wiring (`internal/api/handler.go`)
- Opens a `TxScope` right before `RunBeforePhase`, wraps `ctx`.
- Rolls back on any `RunBeforePhase`/`Dispatch` error; `ErrCrossStoreTx`
  surfaces as its own `CROSS_STORE_TX` error code (plain string, matching
  this handler's existing convention — not the still-unwired `pkg/spec`
  FORMSPEC.* glossary).
- Resolves the emitted event and, if durable, enqueues it via
  `EnqueueOutboxTx` (new, `outbox.go`) onto the scope's own transaction —
  **before** `scope.Commit()`. `action.DeliverEvents` (best-effort
  websocket push / non-durable audit log) is called **after** commit —
  ordering matters: `DeliverEvents`'s non-durable audit-log write goes
  through `EventLogStore`'s plain `s.db`, not the scope; calling it before
  commit reproduced the exact same single-connection deadlock as the
  `ValidateRelationTargets` bug above (found via the same hanging
  `examples/Clinic-UI-Showcase` test after the first fix).

### Sidecar wiring (`internal/action/sidecar.go`, `internal/sidecar/ctx.go`)
- `SidecarExecutor.Execute` forwards the active scope's registry id as a
  new `X-FormSpec-Scope-Id` request header on the outbound `/invoke/...` call.
- `CtxHandler.ServeHTTP` reads that header on inbound `/ctx/{prim}/{op}`
  calls, looks up the scope, and wraps ctx before dispatch — no change
  needed to the `EntityLoader`/`EntityFullSaver`/`EntityFieldUpdater`/
  `EntityFieldCounter` interfaces themselves, since the resolved `conn` is
  already a live `*db.EntityStore` and scope-awareness lives inside it.
- **Not done in this pass**: none of the `sdk/php`/`sdk/python`/
  `sdk/typescript`/etc. reference SDKs send this header yet — a real,
  separate follow-up (these SDKs are real code in this repo, not merely an
  abstract "app process" as the initial plan draft assumed). Until
  updated, sidecar `ctx.entity.*` calls commit independently, same as
  before this change — not a regression.

## Explicitly out of scope
- `HandleCreate`/`HandleUpdate` do not adopt `TxScope` (their `PendingEvents`
  mechanism from Fase 2.1 is unaffected either way — composes safely later).
- `RunAfterPhase` still doesn't return an error (fire-and-forget, unchanged).
- Multi-Datastore-per-Module (Fase 2.9) itself is not built — `TxScope`'s
  store-identity check is structurally correct for when it exists, but is
  a no-op today since every module shares one store.

## Files affected
- `renderers/jsonbpersist/txscope.go` — new
- `renderers/jsonbpersist/tx.go` — `runTx`, `writeDB`, `txReadDB`
- `renderers/jsonbpersist/crud.go` — scope branching on all write methods;
  `ValidateRelationTargets` signature fix (explicit `database DB` param)
- `renderers/jsonbpersist/outbox.go` — `EnqueueOutboxTx`
- `renderers/jsonbpersist/txscope_test.go` — new
- `internal/starlark/executor.go` — ctx-threading
- `internal/action/script.go` — ctx passthrough
- `internal/action/sidecar.go` — `X-FormSpec-Scope-Id` header
- `internal/sidecar/ctx.go` — scope lookup + ctx wrap
- `internal/sidecar/txscope_test.go` — new
- `resource/formspec.go` — `Set*Handler` closures use real ctx
- `internal/api/handler.go` — `HandleCustomAction` scope wiring +
  commit-before-deliver ordering; `writeStoreError` `CROSS_STORE_TX` case
- `internal/api/handler_txscope_test.go` — new

## Verification
- `go build ./...`, `go vet ./...` — clean.
- Full `go test ./...` (60s timeout) — no hangs; only the two pre-existing,
  unrelated failures remain (`examples/Clinic-UI-Showcase` date-drift
  fixture `BACKDATE_EXCEEDED`, `internal/manifest`'s `formspec-app.yaml`
  parse error) — both confirmed via `git stash` during Fase 2.1 to predate
  this and today's work.
- New tests: `renderers/jsonbpersist/txscope_test.go` (join reuse,
  cross-store error, peek, commit/rollback no-ops, registry round-trip),
  `internal/api/handler_txscope_test.go` (two same-store writes roll back
  together on forced failure, including a read-your-own-writes assertion;
  a cross-store write surfaces `CROSS_STORE_TX` and still rolls back the
  local write), `internal/sidecar/txscope_test.go` (a `/ctx/entity/update`
  callback carrying a registered scope id joins the same transaction as a
  concurrent in-process write; no header falls back to independent commit).

## References
- docs/spec/backend/01-core-basic.md §3 (cross-Datastore async-only)
- docs/renderers/jsonb-persist/01-architecture.md §3, §4 (updated to match)
- docs/runtimes/04-formspec-sidecar.md §4.3a (new `X-FormSpec-Scope-Id` section)

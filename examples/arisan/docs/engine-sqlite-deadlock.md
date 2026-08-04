# Engine Bug: SQLite Deadlock on `resource.fetch()` in Script Actions

## Status
- **Discovered**: 2026-08-03 (smoke test Phase 7, local dev on SQLite)
- **Verified fix applied locally** — automation scripts now work end-to-end on SQLite
- **Upstream**: still unfixed in `github.com/primadi/forma@v0.0.0-20260803061320-e3d68a0bb6b7`

## Symptom
A custom action (`script` / `script_ref` impl) whose `.star` script calls
`resource.fetch(entity, id)` on an entity that has **relation fields**
(belongs_to / has_one) **deadlocks** on SQLite:

- `curl` hangs (HTTP 000 after 20–25 s timeout).
- A minimal script that only does `resource.set()` + `resource.save()` on the
  current record works fine (HTTP 200 in ~7 ms), proving the fetch is the trigger.
- On PostgreSQL (production) the same scripts run fine (no deadlock).

## Root Cause
`renderers/jsonbpersist/crud.go` → `(*EntityStore).resolveRelations()` executes
its batch relation query on the **raw base connection**:

```go
rows, err := s.db.QueryContext(ctx, q, idArgs...)   // BUG
```

Every other read path in the same file correctly uses the scope-aware helper:

```go
rows, err := txReadDB(ctx, s.db).QueryContext(ctx, q, idArgs...)   // FIX
```

Why it deadlocks: a custom action runs inside a request-scoped `TxScope`
(`renderers/jsonbpersist/txscope.go`). `TxScope.join()` lazily opens a
transaction on the *only* SQLite connection (single connection, write lock).
`resolveRelations` then tries to read through the base pool, which has **no
free connection** → deadlock. The engine even documents this exact hazard in
`tx.go`'s `txReadDB` doc comment:

> "…and — on a single-connection driver like SQLite — avoiding a deadlock
> against a pool with no free connections"

The bug is simply that `resolveRelations` missed adopting `txReadDB`.

## Local Patch Applied (module cache, NOT tracked in git)
`forma.exe` was rebuilt from the patched module cache:

- File: `%GOPATH%\pkg\mod\github.com\primadi\forma@v0.0.0-20260803061320-e3d68a0bb6b7\renderers\jsonbpersist\crud.go`
- Change: `s.db.QueryContext(...)` → `txReadDB(ctx, s.db).QueryContext(...)` (line ~1398, inside `resolveRelations`)
- Rebuild: `go build -o %GOPATH%\bin\forma.exe ./cmd/forma` (run from the module dir)

### ⚠️ Caveats
1. **Untracked** — the patch lives only in the Go module cache. It is wiped by
   `go clean -modcache` and lost if the engine version is upgraded.
2. If the patch is lost, scripts that `fetch()` relation-bearing records will
   deadlock again on SQLite **until upstream ships the fix**. Production on
   PostgreSQL is unaffected either way.
3. To re-apply: repeat the steps above, or vendor the engine and patch there.

## Affected Scripts in This App
- `spec/modules/arisan-field/transaction/contribution/scripts/validate.star`
  — `resource.fetch("bank-mutation", …)` (bank-mutation has `group_id` relation)
- `spec/modules/arisan-field/transaction/arisan-period/scripts/run-lottery.star`
  — `resource.fetch("contribution", …)` + `resource.create("draw", …)`

## Verification (after patch, SQLite dev)
| Test | Before | After |
|------|--------|-------|
| `POST …/contributions/{id}/validate` (full script w/ fetch) | HTTP 000 deadlock | HTTP 200, 7 ms |
| contribution: pending → validated, `matched_mutation_id` set | — | ✅ |
| bank-mutation: unmatched → matched, `matched_contribution_id` set | — | ✅ |
| `POST …/arisan-periods/{id}/run-lottery` | deadlock | HTTP 200, 7 ms |
| period: open → closed | — | ✅ |
| draw record created (amount, period_id, member_id, status=drawn) | — | ✅ |
| Re-running lottery on a closed period | — | HTTP 422 INVALID_TRANSITION (state machine guard) |

## Other Latent Spots (same class, not yet triggered)
Raw `s.db` still used in `crud.go`:
- `List` (lines ~1260, ~1292) — not script-reachable yet (query builder is a
  stub), but would deadlock if a future `entity.query()` runs inside an action.
- `Submit`/`Cancel`/`Amend` (lines ~922, ~963, ~1002, ~1010) — lifecycle-only,
  not script-reachable.

Recommended upstream fix: audit all `s.db.*` reads and route through
`txReadDB(ctx, s.db)` (writes through `runTx`/`writeDB`), matching the pattern
already used by `getByIDRaw`, `FindByField`, `ValidateRelationTargets`, and
`EnforceReferenceGuard`.

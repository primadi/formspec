# Session Note — 2026-08-17 (CLI batch: repl/seed/diff)

## Position

- Batch selesai: todo 3.4.1 (`diff`), 3.6.2 (`repl`), 3.6.3 (`seed`) — semua ✅.
- Plan: `docs/plan/formspec-repl-seed-diff.md`.
- Changelog: `2026-08-17-013/014/015`.

## What was done

- `formspec repl`: interactive Starlark console + one-shot `-e`. Added
  `App.Database()` getter + exported `NewCtxPrimitiveResolver`/`StateDirFromDSN`.
  New dep: `go.starlark.net/repl` (+ readline).
- `formspec seed`: `kind: Seed` YAML format (new — official `formspec/seed`
  module doesn't exist yet), idempotent via natural key.
- `formspec diff`: structural diff local manifests vs deployed DB schema via
  `MigrationRunner.PlanMigrations`; exit 1 on differences (CI gate).

## Verification

- `go test ./...`: 613 pass, 0 fail (was 571).
- `make build`: green. `go vet`: clean.

## Open questions / next

- Next unchecked items in phase order: Fase 1 `1.4.12` (MoneyType FX — spec
  "Open", needs design decision → stop & report), Fase 2 `2.6.4` (finish
  ctx.\*/secrets enforcement — blocker b resolved since 2.9.1), `2.9.4`
  (ctx.db module-scoped — design-heavy), Fase 3 `3.6.4` (summary rebuild),
  `3.7.1–3.7.4` (backup/restore/logs).
- `formspec seed` format is new/undocumented in `docs/cli-tools/` — consider
  documenting once `formspec/seed` official module lands.

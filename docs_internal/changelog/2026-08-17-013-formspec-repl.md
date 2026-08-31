# `formspec repl` — interactive Starlark console (todo 3.6.2)

**Date**: 2026-08-17
**Plan**: `docs/plan/formspec-repl-seed-diff.md`

Mengimplementasikan `formspec repl` (sebelumnya stub). Console Starlark
interaktif dengan akses `ctx.*` penuh — fitur first-class untuk debugging
script dan permukaan AI Agent Skill.

- `cmd/formspec/repl.go` (baru): bangun `formspec.New`, wire `ctx` (CtxAPI
  dengan datastore resolver live via `NewCtxPrimitiveResolver`), predeclare
  `resource` + `ok`/`fail`; mode interaktif via `go.starlark.net/repl`, mode
  one-shot `-e <expr>` untuk scripting/test.
- `resource/ctxresolver.go`: ekspor `NewCtxPrimitiveResolver` + `StateDirFromDSN`
  (refactor dari `ctxPrimitiveResolver`/`stateDirFromDSN`).
- `resource/formspec.go`: tambah getter `App.Database()`.
- `cmd/formspec/main.go`: dispatch + usage.
- `cmd/formspec/repl_test.go`: eval dasar, akses `ctx.config`, error handling.
- Dependency baru: `go.starlark.net/repl` (+ `github.com/chzyer/readline`).

`--environment` diterima untuk forward-compat (policy Control Plane di-defer).

# ctx.\* primitive uses enforcement (todo 2.6.4 blocker b)

**Date**: 2026-08-17
**Plan**: `docs/plan/uses-enforcement.md`

Menyelesaikan blocker (b) dari todo 2.6.4: enforcement akses `ctx.*`
primitives terhadap deklarasi `uses.primitives` action caller.

- `internal/starlark/context.go`: `CtxAPI.SetUses(*spec.UsesDecl)` +
  `SetStrictPrimitives(bool)` + `checkPrimitive(name)`; di strict mode, akses
  `ctx.db/cache/lock/queue/pubsub/storage/kvstore` yang tidak dideklarasikan
  di `uses.primitives` → `USES_VIOLATION`.
- `internal/starlark/executor.go`: `ScriptExecutor.Execute` kini menerima
  `uses *spec.UsesDecl` (bukan `callerResources []string`); `SetStrictPrimitives`;
  `declaredUsesResources` helper.
- `internal/action/script.go`: `SetStrictPrimitives`; `Execute` meneruskan
  `action.Uses` penuh.
- `resource/formspec.go`: `newDispatcher` set `SetStrictPrimitives(cfg.StrictMode
|| cfg.ProdMode)`.
- Test: `resource/ctx_uses_enforcement_test.go` (strict blocked tanpa
  deklarasi, strict allowed dengan deklarasi, dev mode relaxed).

Catatan: `ctx.secrets` (6.8) belum ada — enforcement secrets menunggu
implementasi `ctx.secrets` itu sendiri. Module auto-suspend + incident audit
tetap subsistem baru.

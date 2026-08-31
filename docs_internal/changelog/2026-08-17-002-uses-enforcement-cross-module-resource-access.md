# 2026-08-17-002 — Uses Enforcement cross-module resource access (todo 2.6.4)

**Apa:** Menghidupkan enforcement `uses.resources` untuk akses lintas-resource
dari Starlark script — menyelesaikan blocker (a) todo 2.6.4.

**Sebelumnya:** `uses` declaration hanya divalidasi statis; di runtime, script
bisa memanggil `resource.call()`/`fetch()`/`create()` ke modul lain tanpa
deklarasi apa pun di `uses.resources`. Blocker (a) dari todo: rantai
call-handler Starlark tidak membawa `uses` milik caller action.

**Sekarang:**

- Caller `uses.resources` di-thread melalui rantai eksekusi script:
  - `internal/action/script.go` `ScriptExecutor.Execute` → `declaredUsesResources(action)`
  - `internal/starlark/executor.go` `ScriptExecutor.Execute(..., callerResources)`
  - handler `CallHandler`/`LoadHandler`/`CreateHandler` kini membawa
    `callerResources []string` (+ `fromModule` ditambahkan ke Load/Create).
- `resource/formspec.go` `newDispatcher`: ketiga closure memanggil
  `checkCrossModuleUses(fromModule, targetModule, targetEntity, callerResources)`.
  Cross-module access diblokir (`USES_VIOLATION`) bila target tidak
  dideklarasikan; same-module selalu diizinkan. Matcher menerima
  `{module}.{entity}`, `{module}/{entity}`, wildcard `{module}.*`, `*`.

**Kenapa:** kontrak `01-core-basic.md` §5 / `06-script-runtime.md` §4 — akses
lintas-resource wajib dideklarasikan di `uses`; undeclared → ditolak saat
runtime, bukan silently diizinkan.

**File terdampak:**

- `internal/action/script.go` — `declaredUsesResources`, setter signatures,
  `Execute` meneruskan caller resources
- `internal/starlark/executor.go` — `Execute(..., callerResources)`, handler signatures
- `resource/formspec.go` — `checkCrossModuleUses` + wiring di 3 closure
- `internal/api/handler_txscope_starlark_test.go` — update `SetCreateHandler` signature
- `resource/uses_enforcement_test.go` — unit test matcher (7 kasus)
- `resource/uses_enforcement_e2e_test.go` — e2e: blocked tanpa uses, allowed dengan uses, same-module allowed
- `docs/plan/uses-enforcement.md` — plan baru
- `docs/runtimes/05-engine-api-layer.md` — §2.2/§5 update status enforcement
- `docs/plan/todo.md` — 2.6.4 sebagian selesai

**Gap tersisa (tercatat di todo):** `ctx.db`/`ctx.secrets`/`ctx.*` enforcement
menunggu 2.9.1; module auto-suspend + incident audit belum ada; middleware stub
`UsesEnforcement` tetap dead code.

**Verifikasi:** `go test ./...` → 557 pass, 9 fail pre-existing
(Clinic-UI-Showcase e2e — script-ref resolution, tidak terkait). Test baru:
13 pass di package `resource` (7 unit + 3 e2e + 3 fixture/helper).

**Referensi:** `docs/plan/uses-enforcement.md`, todo 2.6.4.

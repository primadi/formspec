# Plan — Uses Enforcement (todo 2.6.4)

**Status**: In Progress (2026-08-17) · **Referensi**: `docs/plan/todo.md` 2.6.4,
`docs/spec/backend/01-core-basic.md` §5, `docs/spec/backend/06-script-runtime.md` §4,
`docs/runtimes/05-engine-api-layer.md` §2.2

## Masalah

`uses` declaration (`uses.resources`, `uses.db`, `uses.secrets`, `uses.primitives`)
seharusnya menegakkan akses lintas-resource di runtime — undeclared cross-module
access harus diblokir (01-core-basic §5: "`uses` yang undeclared harus ditolak
saat resolusi, bukan silently diizinkan"). Dua blocker dari inspeksi sebelumnya:

- **(a)** `resource.call()`/`fetch()`/`create()` dispatch path tidak membawa
  `uses` milik caller action, jadi tidak ada yang bisa diperiksa. Perlu plumbing
  melalui rantai call-handler Starlark.
- **(b)** `ctx.db`/`ctx.secrets`/`ctx.*` enforcement diblokir 2.9.1
  (`SetDatastoreResolver` belum di-wire) — **di luar scope task ini**.
- Module auto-suspend + incident audit adalah subsistem baru — **di luar scope**.

## Scope task ini

Enforce **cross-module resource access** dari Starlark script terhadap deklarasi
`uses.resources` caller action. Same-module selalu diizinkan (module punya
resource sendiri). Cross-module (`targetModule != fromModule`) hanya diizinkan
jika target `{module}.{entity}` (atau wildcard `{module}.*` / `*`) dideklarasikan
di `uses.resources` action yang sedang dieksekusi. Pelanggaran → error
`USES_VIOLATION` (runtime, bukan warning).

Ini menyelesaikan blocker (a). Blocker (b) + auto-suspend/audit tetap tercatat
sebagai gap di todo.

## Desain

### 1. Thread caller `uses.resources` melalui rantai eksekusi script

Rantai saat ini:

```
internal/action/script.go  ScriptExecutor.Execute(ctx, action spec.Action, params)
  → e.engine.Execute(ctx, scriptPath, module, entity, id, resource, params, ws, user, version)
internal/starlark/executor.go  ScriptExecutor.Execute(...)
  → res.SetCallFunc / SetLoadFunc / SetCreateFunc  (ResourceAPI)
  → e.CallHandler / e.LoadHandler / e.CreateHandler  (closure di resource/formspec.go)
```

`action.Uses.Resources` hanya terlihat di `internal/action/script.go`. Perlu
diteruskan sampai ke closure `resource/formspec.go`.

- `internal/action/script.go` `Execute`: ekstrak `callerResources := []string`
  dari `action.Uses.Resources` (nil-safe), teruskan sebagai param tambahan ke
  `e.engine.Execute(...)`.
- `internal/starlark/executor.go` `Execute(...)`: tambah param
  `callerResources []string`; teruskan ke closure `SetCallFunc`/`SetLoadFunc`/
  `SetCreateFunc`.

### 2. Perluas signature handler

- `CallHandler`: `func(ctx, workspaceID, fromModule, targetModule, targetEntity, action, params, callerResources []string)`
- `LoadHandler`: `func(ctx, workspaceID, fromModule, module, entity, id, callerResources []string)`
- `CreateHandler`: `func(ctx, workspaceID, fromModule, module, entity, data, callerResources []string)`

Catatan: `fromModule` harus ditambahkan ke `LoadHandler`/`CreateHandler` (saat ini
tidak membawanya) supaya pengecekan cross-module bisa dilakukan di kedua handler.

### 3. Enforcement di `resource/formspec.go`

Helper `checkCrossModuleUses(fromModule, targetModule, targetEntity string, declared []string) error`:

- `targetModule == "" || targetModule == fromModule` → nil (same-module / unqualified).
- Deklarasi `{module}.{entity}` cocok target, atau wildcard `{module}.*`, `*`,
  atau qualifier slash `{module}/{entity}` → nil.
- Selain itu → error `USES_VIOLATION: undeclared cross-module access to {target}`.

Dipanggil di ketiga closure (`SetCallHandler`, `SetLoadHandler`,
`SetCreateHandler`) sebelum dispatch/store call. Error dibungkus → script `fail` →
action error (HTTP 500/ACTION_ERROR) — konsisten dengan perilaku runtime lain.

### 4. Format deklarasi

`uses.resources` memakai notasi yang sudah dipakai contoh
(`examples/Clinic-UI-Showcase/.../prescription/entity.yaml`: `resources:
[medicine.find, medicine.update]`; `examples/arisan/...`: `[contribution.find,
draw.create]`; `reff_docs/examples/Order-to-Cash`: `[payment-gateway.create-session,
customer.find]`). Cross-module pakai qualifier `{module}.{entity}` (atau
`{module}/{entity}` per spec `06-script-runtime.md` §3) — matcher menerima
keduanya + wildcard.

## File terdampak

| File                                                | Perubahan                                                        |
| --------------------------------------------------- | ---------------------------------------------------------------- |
| `internal/action/script.go`                         | `Execute` ekstrak + teruskan `callerResources`; setter signature |
| `internal/starlark/executor.go`                     | `Execute` param `callerResources`; handler signatures            |
| `resource/formspec.go`                              | closure enforcement (`checkCrossModuleUses`)                     |
| `internal/api/handler_txscope_starlark_test.go`     | update `SetCreateHandler` signature                              |
| `internal/api/uses_enforcement_test.go`             | test baru: cross-module block, same-module allow, wildcard       |
| `docs/plan/todo.md`                                 | tandai 2.6.4 (sebagian)                                          |
| `docs/renderers/jsonb-persist/04-query-and-keys.md` | catatan uses enforcement                                         |

## Level of effort

Medium. Self-contained, tidak menunggu 2.9.1.

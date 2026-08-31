# 2026-08-30-006 — Infra Registry fase D: config/log routable (9 primitive lengkap)

**Plan:** `docs/plan/infra-registry-3-level.md` (fase D)

## Apa yang diubah

`config` dan `log` — dua primitive terakhir yang sebelumnya hanya builtin —
kini **routable melalui datastore resolver**, melengkapi closed set 9
primitive (`db`, `cache`, `lock`, `queue`, `pubsub`, `storage`, `kvstore`,
`config`, `log`):

- **Capability `Logger`** (`internal/starlark/primitive.go`):
  `Log(ctx, level, event, meta)` — kontrak backend log terpusat.
- **Backend baru** (`renderers/jsonb-persist/datastore/configlog.go`):
  `KVConfig` (KV-backed config, key dinamespace `config:`), `KVLog` (log
  append via KV — Redis untuk multi-instance), `MemoryLog` (built-in
  default + memory driver), `FileLog` (fs driver, JSON lines), `DBConfigLog`
  (sqlite/postgres via `db.DB`).
- **Driver×serves diperluas** (`pkg/spec/datastore.go`): `config` kini
  kompatibel dengan memory/valkey/redis/sqlite/postgres; `log` dengan
  memory/fs/valkey/redis/sqlite/postgres.
- **Override di CtxAPI** (`internal/starlark/context.go`): `ctx.config` dan
  `ctx.log` mem-probe resolver dulu — bila ada service yang melayani
  (runner dengan conn non-nil), runner yang di-resolve dikembalikan langsung
  (punya `.get` dan `.info/.warn/.error`); bila tidak, fallback ke builtin
  (`Config` store / in-memory `logAPI` dengan `LogEntries`).
- `primitiveRunner` kini melayani `info/warn/error` (Logger) dan `get`
  menerima capability `Config` untuk primitive `config`.

## Kenapa

Deployment multi-instance membutuhkan config store dan log sink terpusat —
sebelumnya `ctx.log` selalu in-memory per-proses (log terfragmentasi) dan
config hanya dari Config manifest lokal. Kini keduanya bisa diarahkan ke
service terpusat (mis. Redis) via `kind: Datastore` `serves: [config, log]`

- selection App/Module (fase B).

## File terdampak

- `internal/starlark/primitive.go` — capability `Logger`, `builtinLog`,
  `builtinGet` config-aware, `AttrNames` +info/warn/error
- `internal/starlark/context.go` — `configPrim`/`logPrim` handles, probe +
  fallback di `Attr("config")`/`Attr("log")`
- `renderers/jsonb-persist/datastore/configlog.go` — baru (5 backend)
- `pkg/spec/datastore.go` — `Serves()` config/log
- `resource/datastoreregistry.go` — wiring config/log per driver + built-in
- Test: `resource/datastoreregistry_test.go` (+1: ConfigLogRoutable)

Verifikasi: 978 test Go lulus tanpa regresi.

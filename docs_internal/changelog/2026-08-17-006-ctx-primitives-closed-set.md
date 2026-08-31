# 2026-08-17-006 — Closed Set 9 Primitives + Dev Auto-Provision (2.9.2, 2.9.3)

**Referensi**: `docs/plan/todo.md` (2.9.2, 2.9.3), `docs/plan/ctx-primitives-closed-set.md`,
`docs/spec/platform/06-datastore.md` §2/§5.

## Apa yang diubah

Melengkapi closed set 9 primitive `ctx.*` dengan backend nyata dan
auto-provision `'default'` di single-server mode (lanjutan 2.9.1 yang hanya
me-wire `ctx.db().query()`):

- **`internal/starlark/primitive.go`** — tambah capability interfaces `Queue`
  (Enqueue/Dequeue), `PubSub` (Publish/Subscribe), `Storage`
  (Upload/Download); tambah operasi runner `enqueue`/`dequeue`,
  `publish`/`subscribe`, `upload`/`download`.
- **`renderers/jsonb-persist/datastore/memory/`** (baru) — backend in-memory
  `KV` (cache/kvstore, dengan TTL), `Lock`, `Queue`, `PubSub`, dan filesystem
  `Storage` (dengan sanitasi traversal).
- **`resource/ctxresolver.go`** (baru) — `ctxPrimitiveResolver` auto-provision
  `'default'`: db→SQLite/Postgres, cache/lock/queue/pubsub/kvstore→in-memory,
  storage→filesystem; named datastore selain `'default'` → error jelas
  (menunggu 2.9.4). Dipakai `newDispatcher` (dan `formspec.New` → dev.go).
- `config`/`log` **bukan** di-routing lewat resolver — keduanya builtin
  terpisah (`ctx.config`/`ctx.log`) yang sudah ada.

## Kenapa

Sebelumnya semua primitive selain `db` gagal dengan "no live datastore".
Sekarang script Starlark bisa memakai cache/kvstore/lock/queue/pubsub/storage
terhadap backend `'default'` yang auto-provision di dev — fondasi Fase 7
(Service, Hook, Validation L6, sidecar proxy).

## File terdampak

- `internal/starlark/primitive.go`, `internal/starlark/primitive_test.go`
- `renderers/jsonb-persist/datastore/memory/{memory,storage}.go` + `memory_test.go`
- `resource/ctxresolver.go` (baru), `resource/formspec.go`,
  `resource/ctx_primitives_e2e_test.go` (baru)
- `docs/plan/ctx-primitives-closed-set.md` (baru), `docs/plan/todo.md`

## Verifikasi

`go test ./...` hijau (19 paket ok, 0 fail); `make build` hijau.

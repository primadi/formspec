# Plan — Fase 2.9.2 + 2.9.3: Closed Set 9 Primitives + Dev Auto-Provision

**Tanggal**: 2026-08-17 · **Status**: In progress
**Referensi**: `docs/plan/todo.md` (2.9.2, 2.9.3), `docs/spec/platform/06-datastore.md` §2/§5,
`docs/runtimes/02-formspec-resource.md` §4/§7, `docs/runtimes/04-formspec-sidecar.md` §4.3

## Tujuan

2.9.1 me-wire resolver sehingga `ctx.db().query()` jalan terhadap database utama app.
Increment ini melengkapi **closed set 9 primitive** (`db`, `cache`, `lock`, `queue`,
`pubsub`, `storage`, `config`, `kvstore`, `log`) dengan backend nyata, dan
**auto-provision `'default'`** di dev mode sesuai `platform/06-datastore.md` §5:

| Primitive | Backend dev `'default'`                       |
| --------- | --------------------------------------------- |
| `db`      | SQLite (database utama app — sudah ada 2.9.1) |
| `kvstore` | in-memory KV                                  |
| `cache`   | in-memory KV + TTL                            |
| `lock`    | in-memory distributed lock                    |
| `queue`   | in-memory FIFO queue                          |
| `pubsub`  | in-memory pub/sub                             |
| `storage` | filesystem lokal                              |
| `config`  | Config registry (non-secret keys)             |
| `log`     | structured logger (sudah ada `ctx.log`)       |

Semua primitive mendukung `.named("name")` (binding multi-datastore). Di single-server
dev, named datastore selain `'default'` mengembalikan error jelas (belum ada definisi
Datastore dari Control Plane).

## Perubahan

| File                                                      | Perubahan                                                                                                                                                                                                                                  |
| --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `internal/starlark/primitive.go`                          | Tambah capability interfaces: `Queue` (Enqueue/Dequeue), `PubSub` (Publish/Subscribe), `Storage` (Upload/Download), `Config` (Get). Tambah operasi runner: `enqueue`/`dequeue`, `publish`/`subscribe`, `upload`/`download`, `get` (config) |
| `renderers/jsonb-persist/datastore/memory/memory.go`      | Backend in-memory: `Cache`, `Lock`, `Queue`, `PubSub`, `KVStore` (implementasi capability interfaces)                                                                                                                                      |
| `renderers/jsonb-persist/datastore/memory/storage.go`     | Backend filesystem `Storage` (upload/download ke direktori)                                                                                                                                                                                |
| `renderers/jsonb-persist/datastore/memory/config.go`      | Backend `Config` (get non-secret key)                                                                                                                                                                                                      |
| `resource/formspec.go`                                    | `newDispatcher` resolver: auto-provision `'default'` untuk semua primitive (in-memory + filesystem + db)                                                                                                                                   |
| `cmd/formspec/dev.go`                                     | Wire resolver yang sama di dev mode                                                                                                                                                                                                        |
| `internal/starlark/primitive_test.go`                     | Test operasi baru (queue/pubsub/storage/config)                                                                                                                                                                                            |
| `renderers/jsonb-persist/datastore/memory/memory_test.go` | Test unit backend in-memory                                                                                                                                                                                                                |

## Keputusan

- Backend in-memory hidup di `renderers/jsonb-persist/datastore/memory` (dekat
  `datastore` registry yang sudah ada), bukan `internal/` — konsisten dengan lokasi
  `DBQuerier` dan `datastore` registry.
- Capability interfaces tetap didefinisikan di `internal/starlark` (kontrak kanonik,
  sama seperti 2.9.1) — backend in-memory mengimplementasikannya secara struktural.
- `config` primitive resolve ke Config registry app (non-secret keys); secret key
  hanya lewat `ctx.secrets` (Fase 6.8, belum ada).
- `storage` dev backend = filesystem di bawah `--state-dir` (default `./.formspec`).
- Named datastore selain `'default'` → error jelas "no live datastore ... only
  default is auto-provisioned in dev (todo 2.9.4)".

## Verifikasi

- `go test ./...` hijau.
- Test baru: script `ctx.cache().set/get/delete`, `ctx.lock().acquire/release`,
  `ctx.queue().enqueue/dequeue`, `ctx.pubsub().publish/subscribe`,
  `ctx.storage().upload/download`, `ctx.kvstore().get/set/delete`.
- `make build` hijau.

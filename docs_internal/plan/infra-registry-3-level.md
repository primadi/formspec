# Infra Registry 3-Level — Real-Infra → Logical Primitive Mapping

**Status:** In Progress · **Mulai:** 2026-08-30 · **Referensi spec:** `docs/spec/platform/06-datastore.md`, `05-plane-protocol.md`

## Masalah

Saat ini belum ada infra registry — semua registrasi infra tercampur di app
registry (`resource/datastoreregistry.go`) dan sebagian lewat jalur implisit
(env-var `FORMSPEC_STORAGE`/`FORMSPEC_STREAM`/`FORMSPEC_MINIO_*`,
auto-provision `'default'`). 9 logical primitive (`db`, `cache`, `lock`,
`queue`, `pubsub`, `storage`, `kvstore`, `config`, `log`) belum semuanya bisa
diregistrasi eksplisit, dan tiap primitive belum menjamin dukungan >1 service
(mis. 2 db) dengan default yang overridable.

## Arsitektur Target: 3-Level Registry

1. **Infra Registry** (cloud control, per environment) — registrasi eksplisit
   service fisik (`pg-main`, `valkey-1`, `minio-assets`). Unit registrasi =
   service; struktur `map[PrimitiveType]{default, services}`. Default per
   primitive = pointer ke service teregistrasi (bukan backend implisit),
   overridable.
2. **App Registry** (app builder, per `kind: App`) — logical primitive:
   `default` + named alias (mis. `db`: default→`pg-main`,
   `analytics`→`pg-analytics`). Portable antar workspace.
3. **Workspace Binding** (workspace owner/cloud) — bukan registrasi logical
   baru; pemetaan logical→fisik via `access.filter`; boleh override default.

Chain resolusi: `ctx call → action uses.datastores → module datastores →
app default/named → workspace binding → infra service`. Mengarah ke bawah =
mempersempit, tidak melebar.

## Fase

| Fase | Isi                                                                                                    | Status                                                                               |
| ---- | ------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ |
| A1   | InfraRegistry: multi-service per primitive + default overridable (`SetDefault`)                        | ✅ 2026-08-30                                                                        |
| A2   | Factory per-primitive: Redis lock/queue/pubsub (`rediskv`), minio/s3 via named Datastore               | ✅ 2026-08-30                                                                        |
| A3   | `DatastoreRegistry` jadi adapter di atas InfraRegistry; hapus duplikasi `dsEntry.open` vs `NewFactory` | ✅ 2026-08-30 (public API dipertahankan; unifikasi penuh factory menyusul di fase B) |
| B    | App Registry: deklarasi `datastores` map di `kind: App`; enforce `UsesDecl.Datastores`                 | ✅ 2026-08-30                                                                        |
| B2   | Workspace Binding: snapshot per-workspace evaluasi `access.filter`                                     | ⏸️                                                                                   |
| C    | Buka `.named()` resmi (gate `uses.datastores`); error codes spec §6                                    | ✅ 2026-08-30                                                                        |
| D    | `config`/`log` routable (9 lengkap)                                                                    | ✅ 2026-08-30                                                                        |
| E    | Distribusi via Control Plane snapshot; hapus env-var implicit                                          | ⏸️                                                                                   |
| F    | Update spec normatif `06-datastore.md` §1.1/§4/§5, `05-plane-protocol.md` §4.1                         | ⏸️                                                                                   |

## Fase A — Detail Teknis

### File yang diubah/dibuat

| File                                                  | Perubahan                                                                                                                                             |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `renderers/jsonb-persist/datastore/rediskv/lock.go`   | BARU — `Lock` (SET NX PX + token, release compare-and-delete Lua)                                                                                     |
| `renderers/jsonb-persist/datastore/rediskv/queue.go`  | BARU — `Queue` (LPUSH/RPOP FIFO, JSON payload, non-blocking dequeue)                                                                                  |
| `renderers/jsonb-persist/datastore/rediskv/pubsub.go` | BARU — `PubSub` (Redis pub/sub, JSON payload)                                                                                                         |
| `renderers/jsonb-persist/datastore/rediskv/redis.go`  | Refactor: helper `dialRedis` dipakai bersama                                                                                                          |
| `pkg/spec/datastore.go`                               | Tambah `AllPrimitiveTypes()`                                                                                                                          |
| `resource/datastoreregistry.go`                       | Refactor: `serviceEntry` per service + `defaults map[PrimitiveType]string`; API baru `SetDefault`/`Default`; driver minio/s3; redis lock/queue/pubsub |
| `resource/datastoreregistry_test.go`                  | Test baru: SetDefault override, multi-service                                                                                                         |
| `renderers/jsonb-persist/datastore/rediskv/*_test.go` | Test lock/queue/pubsub (skip tanpa Valkey)                                                                                                            |

### Keputusan teknis

- Public API `DatastoreRegistry` dipertahankan (`NewDatastoreRegistry`,
  `LoadManifests`, `Resolve`, `Binding`) — pemanggil di `resource/formspec.go`
  dan `resource/ctxresolver.go` tidak berubah.
- Default per primitive di-resolve di `Resolve`: plain call (`name == ""`)
  → `defaults[pt]`, bukan hardcoded `'default'`.
- `.named("default")` untuk module ter-bind tetap ditolak (escape hatch,
  spec §1.1) — pembukaan `.named()` resmi adalah Fase C.
- NATS tetap fail-loud (deferred — keputusan audit sebelumnya).
- Kredensial minio/s3 via `spec.connection.extra` (`access_key`, `secret_key`,
  `use_ssl`) dengan fallback env `FORMSPEC_MINIO_*`.

## Verification

1. `rtk go test ./renderers/jsonb-persist/datastore/... ./resource/...`
2. Skenario: 2 db teregistrasi; `SetDefault("db", ...)` mengubah resolusi
   plain call; driver×serves tetap divalidasi.
3. Integrasi Redis lock/queue/pubsub vs Valkey dev container.
4. `rtk go test ./...` — tidak ada regresi.

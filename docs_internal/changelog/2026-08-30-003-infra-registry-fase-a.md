# 2026-08-30-003 — Infra Registry fase A: multi-service per primitive + default overridable

**Plan:** `docs/plan/infra-registry-3-level.md` (fase A1–A3)

## Apa yang diubah

Restrukturisasi `resource/datastoreregistry.go` dari registry per-datastore
menjadi **Infra Registry per-primitive**: unit registrasi = service (instance
infra fisik dengan logical name), tiap primitive type dapat menampung >1
service (mis. 2 db), dan tiap primitive punya **default yang overridable** via
API baru `SetDefault(primitive, service)` / `Default(primitive)` /
`Services(primitive)`. Default kini pointer ke service teregistrasi — bukan
backend implisit.

Backend baru:

- `rediskv.Lock` / `rediskv.Queue` / `rediskv.PubSub` — driver Redis/Valkey
  untuk primitive `lock`, `queue`, `pubsub` (sebelumnya hanya in-memory),
  melengkapi compatibility set `platform/06-datastore.md` §2.
- Driver `minio`/`s3` kini bisa lewat named `kind: Datastore` (endpoint/bucket
  dari `spec.connection`, kredensial dari `spec.connection.extra` dengan
  fallback env `FORMSPEC_MINIO_*`) — sebelumnya hanya via env-var boot.

## Kenapa

Registrasi real-infra ke logical infra harus eksplisit dan terpusat; model
lama mencampur infra registry ke app registry dan menyembunyikan sebagian
infra di balik env-var. Fase A adalah fondasi untuk App Registry (fase B),
Workspace Binding (fase B2), dan distribusi via Control Plane snapshot
(fase E).

## File terdampak

- `resource/datastoreregistry.go` — restrukturisasi (`serviceEntry`,
  `services`+`defaults` map, `SetDefault`/`Default`/`Services`, driver
  minio/s3, redis lock/queue/pubsub)
- `renderers/jsonb-persist/datastore/rediskv/{lock,queue,pubsub}.go` — baru
- `renderers/jsonb-persist/datastore/rediskv/redis.go` — helper `dialRedis`
  bersama
- `pkg/spec/datastore.go` — `AllPrimitiveTypes()`
- Test: `resource/datastoreregistry_test.go` (+2 test),
  `rediskv/{lock,queue,pubsub}_test.go` — baru

Public API `DatastoreRegistry` (`NewDatastoreRegistry`, `LoadManifests`,
`Resolve`, `Binding`) dipertahankan — pemanggil tidak berubah. Verifikasi:
970 test Go lulus tanpa regresi; test integrasi Redis lock/queue/pubsub hijau
vs Valkey dev container.

# 2026-08-31-001 — Infra Registry fase B2 + E: Workspace Binding & hapus env-var implicit

**Plan:** `docs_internal/plan/infra-registry-3-level.md` (fase B2 + E)

## Apa yang diubah

**Fase B2 — Workspace Binding (level 3):**

- `internal/artifact`: `DatastoreBinding` (entri snapshot: name + spec +
  permission ceiling) + `DatastoreRegistration` (registrasi service di
  Control Plane: name + spec + environment + labels) + store methods
  `UpsertDatastore`/`ListDatastores` (interface + MemStore).
- `internal/control/snapshot.go`: `buildSnapshot` kini mengevaluasi
  `access.filter` setiap service teregistrasi terhadap workspace — hanya
  service yang cocok masuk `snapshot.Datastores`; service tak cocok tidak
  muncul sama sekali (workspace tidak bisa melihatnya). Permission ceiling
  (`access.permission`) ikut binding.
- `resource/datastoreregistry.go`: `LoadSnapshotDatastores(bindings)` —
  populate Infra Registry dari snapshot (jalur Control-Plane-distributed;
  manifest lokal tetap jalur dev). Built-in `'default'` tidak pernah
  diganti dari snapshot. `Permission(service)` mengekspos ceiling dari
  binding (override `access.permission` spec).

**Fase E — hapus env-var implicit:**

- `buildStreamBackend(dsReg)` — backend stream Tier-2 kini di-resolve via
  registry: service Redis/Valkey yang `serves` queue/pubsub → Redis
  Streams; else in-memory (dev default). Env `FORMSPEC_STREAM`/
  `FORMSPEC_REDIS_ADDR` dihapus.
- Storage resolver file field — di-resolve via registry: service
  minio/s3 yang `serves` storage → object store; else filesystem. Env
  `FORMSPEC_STORAGE`/`FORMSPEC_MINIO_*` dihapus dari boot path.

## Kenapa

Menutup model 3-level: Infra Registry (level 1) kini terdistribusi via
snapshot Control Plane dengan otorisasi per-workspace; App Registry (level
2) memilih logical name; Workspace Binding (level 3) memetakan logical→fisik
dengan ceiling permission. Semua infra kini teregistrasi eksplisit lewat
`kind: Datastore` — tidak ada jalur implisit env-var tersisa.

## File terdampak

- `internal/artifact/{artifact,store}.go` — DatastoreBinding/
  DatastoreRegistration + store methods
- `internal/control/snapshot.go` — evaluasi access.filter di buildSnapshot
- `resource/datastoreregistry.go` — LoadSnapshotDatastores + Permission +
  permissions map
- `resource/formspec.go` — buildStreamBackend(dsReg), storage via registry
- Test: `internal/control/snapshot_test.go` (baru),
  `resource/datastoreregistry_test.go` (+1 SnapshotBinding)

Verifikasi: 980 test Go lulus tanpa regresi.

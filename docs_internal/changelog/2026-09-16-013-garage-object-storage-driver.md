# 2026-09-16-013 — Garage sebagai driver object storage default

**Plan:** `docs_internal/plan/garage-object-storage-driver.md` ·
**Kontrak:** `docs/spec/platform/06-datastore.md` §1–§2

## Apa yang diubah

**Driver baru `garage` (default), `minio`/`s3` tetap didukung.**
`pkg/spec/datastore.go` menambah `DatastoreDriverGarage` + konstanta
`DefaultStorageDriver`, dan `Serves()` untuk garage → `[storage]`.

**Satu client S3, tiga driver.** Client S3-compatible diekstrak ke paket baru
`renderers/jsonb-persist/datastore/s3store` (`Config{Endpoint, AccessKey,
SecretKey, Bucket, Region, UseSSL}`, `DefaultRegion = us-east-1`, path-style
addressing, plus `Stat`/`Delete`/`Link`/`ChunkUploader` yang dipindah dari
`minio/capabilities.go`). `datastore/garage` dan `datastore/minio` kini wrapper
tipis yang mem-pin default (endpoint/port/bucket/region) di atas client itu;
`resource/datastoreregistry.go` memilihnya per driver dan memakai prefix env
`FORMSPEC_GARAGE_*` / `FORMSPEC_MINIO_*`. `resource/formspec.go` (resolver
storage), `datastore/factory.go` (`garageFactory`), dan `cmd/formspec/dev.go`
(registrasi factory) ikut menyertakan `garage`.

**Dev container: MinIO → Garage.** Service `minio` di `.devcontainer/compose.yaml`
diganti `garage` (`dxflrs/garage:v2.4.1`, `--single-node --default-bucket`,
volume `garage-data`), dengan `garage.toml` baru (single node,
`db_engine = "sqlite"`, `replication_factor = 1`, `s3_region = "us-east-1"`,
`allow_world_readable_secrets = true` khusus dev), kredensial
`GARAGE_DEFAULT_*` di `.env`, dan `forwardPorts` 19000 (S3 API) + 19003
(Admin API/metrics). Env `FORMSPEC_GARAGE_*`/`FORMSPEC_MINIO_*` tetap jadi
fallback kredensial jalur `kind: Datastore` — bukan jalur boot implisit.

## Kenapa

Garage adalah object store S3-compatible self-hosted yang lebih ringan untuk
dev/prod kecil dan tidak memerlukan console terpisah. Karena Garage, MinIO, dan
S3 berbicara API yang sama, perbedaannya cukup di default endpoint — sehingga
satu client bersama (`s3store`) menghapus ~260 baris duplikasi dan membuat
pergantian driver jadi persoalan satu field `spec.driver`.

## File terdampak

- `pkg/spec/datastore.go` — driver `garage`, `DefaultStorageDriver`, enum schema
- `renderers/jsonb-persist/datastore/s3store/{storage,capabilities}.go` (baru)
- `renderers/jsonb-persist/datastore/garage/{storage,storage_test}.go` (baru)
- `renderers/jsonb-persist/datastore/minio/storage.go` (wrapper tipis;
  `capabilities.go` dihapus), `storage_test.go`
- `renderers/jsonb-persist/datastore/factory.go` — `garageFactory`
- `resource/datastoreregistry.go`, `resource/formspec.go`, `cmd/formspec/dev.go`
- `.devcontainer/{compose.yaml,garage.toml,.env,devcontainer.json}`
- `schemas/formspec.schema.json`, `docs/kind/infra/Datastore.md` (regenerasi),
  `deploy/operator/crds/formspec.dev_datastores.yaml`
- Docs: `AGENTS.md`, `docs/spec/platform/06-datastore.md`,
  `docs/reference/primitives.md`, `docs/spec/frontend/07-component-kinds.md`,
  `docs/registry/05-self-hosting.md`, `docs/guides/order-to-cash-tutorial.md`,
  `docs/comparison/*`, `ai_skills/formspec-kinds/SKILL.md` (+3 salinan examples),
  komentar `internal/api/file.go` + `internal/starlark/primitive.go`
- Test: `resource/datastoreregistry_test.go`
  (`TestDatastoreRegistry_ObjectStorageDrivers`)

Verifikasi: `go build ./...` bersih; `go test ./...` hijau (termasuk 3 subtest
driver object storage). `go.mod` tetap memakai `github.com/minio/minio-go/v7`
(SDK client S3, bukan server MinIO).

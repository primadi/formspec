# Plan — Garage sebagai Driver Object Storage Default

**Status:** ✅ Selesai (2026-09-16)
**Kontrak:** [`docs/spec/platform/06-datastore.md`](../../docs/spec/platform/06-datastore.md) §1, §2
**Referensi lain:** `.devcontainer/compose.yaml`, `.devcontainer/garage.toml`

## Tujuan

Mengganti MinIO dengan **Garage** sebagai object storage bawaan dev container
dan sebagai **driver default** untuk `kind: Datastore` ber-`serves: [storage]`.
Driver `minio` **tetap ada** — deployment yang sudah menjalankan MinIO tidak
perlu berubah.

## Keputusan desain

| #   | Keputusan                                                                                                       | Alasan                                                                                                                                           |
| --- | --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| D1  | Tambah `spec.DatastoreDriverGarage = "garage"` sebagai driver tersendiri (bukan alias `s3`)                     | `garage` terlihat eksplisit di YAML dan di pesan error; `s3` tetap berarti "S3 generik / cloud"                                                  |
| D2  | `spec.DefaultStorageDriver = garage` sebagai satu titik rujukan default                                         | Menghindari literal `"garage"` tersebar                                                                                                          |
| D3  | Ekstrak client S3 bersama ke paket baru `datastore/s3store`                                                     | Garage, MinIO, dan S3 berbicara API yang sama — menghindari duplikasi ~260 baris (storage + capabilities)                                        |
| D4  | `datastore/garage` & `datastore/minio` jadi wrapper tipis yang mem-pin default (endpoint, port, bucket, region) | Perbedaan antar-driver hanya default; implementasi tetap satu                                                                                    |
| D5  | `garage` sebagai default memakai region `us-east-1` di `garage.toml`                                            | Cocok dengan `s3store.DefaultRegion` (default `minio-go`) sehingga tidak perlu konfigurasi region di YAML                                        |
| D6  | Dev container menyalakan Garage dengan `--single-node --default-bucket`                                         | Layout cluster + access key + bucket dibuat otomatis dari `GARAGE_DEFAULT_*` di `.env` — tanpa langkah manual `garage layout assign`             |
| D7  | Path-style addressing dipaksa (`BucketLookupPath`)                                                              | Garage selalu menerima path-style; vhost-style butuh wildcard DNS yang tidak ada di dev container                                                |
| D8  | `driver: s3` tanpa `connection.host` tetap jatuh ke endpoint MinIO seperti sebelumnya                           | Tidak mengubah perilaku driver yang sudah ada (perubahan hanya aditif)                                                                           |
| D9  | MinIO dihapus dari `compose.yaml` (bukan dijalankan berdampingan)                                               | Menghindari dua object store hidup bersamaan; driver `minio` tetap teruji lewat `datastore/minio/storage_test.go` yang skip bila tidak reachable |

## Fase & File

### Fase 1 — Spec (small)

- `pkg/spec/datastore.go` — tambah `DatastoreDriverGarage`, `DefaultStorageDriver`,
  enum `@schema`, dan `Serves()` untuk garage → `[storage]`.

### Fase 2 — Ekstraksi client S3 (medium)

- `renderers/jsonb-persist/datastore/s3store/storage.go` (**baru**) — client S3
  generik: `Config{Endpoint, AccessKey, SecretKey, Bucket, Region, UseSSL}`,
  `New()`, `Upload`/`Download`, `DefaultRegion`.
- `renderers/jsonb-persist/datastore/s3store/capabilities.go` (**baru**) —
  `Stat`/`Delete`/`Link`/`ChunkUploader` (dipindah apa adanya dari
  `minio/capabilities.go`).
- `renderers/jsonb-persist/datastore/garage/storage.go` (**baru**) — wrapper
  `garage` (`DefaultEndpoint = garage:3900`, `DefaultS3Port = 3900`,
  `DefaultBucket`, `DefaultRegion`, `NewStorage`).
- `renderers/jsonb-persist/datastore/minio/storage.go` — jadi wrapper tipis
  dengan signature baru `NewStorage(Config)`; `minio/capabilities.go` dihapus
  (isi pindah ke `s3store`).

### Fase 3 — Wiring driver (small)

- `resource/datastoreregistry.go` — cabang object storage menerima
  `garage`/`minio`/`s3`; default endpoint/bucket/prefix env per-driver
  (`FORMSPEC_GARAGE_*` vs `FORMSPEC_MINIO_*`); pesan error `not supported`
  menyebut `garage`.
- `resource/formspec.go` — resolver storage menerima driver `garage`.
- `renderers/jsonb-persist/datastore/factory.go` — `garageFactory`.
- `cmd/formspec/dev.go` — registrasi factory `garage` untuk ctx listener.

### Fase 4 — Dev container (small)

- `.devcontainer/garage.toml` (**baru**) — single node, `db_engine = "sqlite"`,
  `replication_factor = 1`, `[s3_api] s3_region = "us-east-1"`,
  `[admin]` token/metrics, `allow_world_readable_secrets = true` (dev-only).
- `.devcontainer/compose.yaml` — service `minio` → `garage`
  (`dxflrs/garage:v2.4.1`, `--single-node --default-bucket`), volume
  `minio-data` → `garage-data`, `depends_on` app diperbarui.
- `.devcontainer/.env` — `GARAGE_DEFAULT_ACCESS_KEY/SECRET_KEY/BUCKET`.
- `.devcontainer/devcontainer.json` — `forwardPorts` 19000 (S3 API) + 19003
  (Admin API/metrics).

### Fase 5 — Schema, docs, test (medium)

- `make generate-schema` + `make generate-kind-docs` — enum `garage` masuk
  `schemas/formspec.schema.json` + `docs/kind/infra/Datastore.md`.
- `deploy/operator/crds/formspec.dev_datastores.yaml` — enum driver ditambah
  `garage`.
- `AGENTS.md`, `docs/spec/platform/06-datastore.md` (+catatan shared client),
  `docs/kind/infra/Datastore.md`, `docs/reference/primitives.md`,
  `docs/spec/frontend/07-component-kinds.md`, `docs/registry/05-self-hosting.md`,
  `docs/guides/order-to-cash-tutorial.md`, `docs/comparison/*`,
  `ai_skills/formspec-kinds/SKILL.md` (+3 salinan di `examples/*/.agents/`),
  komentar `internal/api/file.go` & `internal/starlark/primitive.go`.
- `resource/datastoreregistry_test.go` — `TestDatastoreRegistry_ObjectStorageDrivers`
  (garage/minio/s3: serves = `[storage]`, gagal koneksi bukan "unsupported",
  primitive lain ditolak).
- `renderers/jsonb-persist/datastore/garage/storage_test.go` (**baru**) — upload/
  download/delete terhadap Garage live; skip bila tidak reachable.

## Catatan implementasi

- `go.mod` tetap memakai `github.com/minio/minio-go/v7` — itu **SDK client S3**,
  bukan server MinIO; Garage tidak punya SDK Go sendiri.
- Garage tidak menyediakan console UI seperti MinIO; administrasi lewat Admin API
  (:3903) atau `docker exec <container> /garage ...`.
- Region presigned URL mengikuti `[s3_api] s3_region`; karena `s3store` mengisi
  default `us-east-1`, `garage.toml` disetel ke nilai yang sama.

## Traceability

Changelog: `docs_internal/changelog/2026-09-16-013-garage-object-storage-driver.md`.

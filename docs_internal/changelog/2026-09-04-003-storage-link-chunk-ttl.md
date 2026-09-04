# 2026-09-04-003 — ctx.storage(): link generation, chunked upload, 1x-download & TTL

Plan: `docs_internal/plan/storage-links-plan.md` (todos 7.17.4–7.17.7).

## Apa yang berubah

Perluasan primitive `storage` dengan empat kemampuan baru, tanpa mengubah
kontrak `Upload/Download []byte` yang lama:

1. **Capability interfaces baru** — `Linker` (link download ber-TTL),
   `Deleter`, `Stater` (ukuran objek), `ChunkUploader` (`init_upload` /
   `put_chunk` / `complete_upload`) di `internal/starlark/primitive.go`,
   mirror identik di `internal/sidecar/ctx.go` (pola structural interface).
2. **Link download (7.17.6)** — route `POST /{module}/{entity}/{id}/{field}/link`
   menerbitkan presigned URL MinIO untuk `visibility: signed` (menggantikan
   `501 SIGNED_URL_NOT_IMPLEMENTED`), atau app-token link (opaque 256-bit)
   yang dikonsumsi via `GET .../storage/link/{token}`. Tabel sistem baru
   `formspec_storage_link` (`renderers/jsonb-persist/link.go` + DDL di
   `migrate.go`) menyimpan budget download secara atomic.
3. **1x download + TTL (7.17.6)** — `StorageSpec.one_time` menghapus objek
   setelah link dikonsumsi; `StorageSpec.ttl` + sweeper
   (`internal/api/storage_sweeper.go`, worker baru di `resource/formspec.go`)
   menghapus objek yang tidak pernah didownload.
4. **Chunked upload (7.17.5)** — routes `upload/init`, `upload/{uid}/part/{n}`,
   `upload/{uid}/complete`; MinIO via S3 multipart (`minio.Core`), fs/memory
   via parts-dir + concat. `StorageSpec.chunk_size_mb` jadi hint ukuran part.
5. **Limit ukuran (7.17.7)** — global `FORMSPEC_UPLOAD_MAX_MB` (default 100)
   / `FORMSPEC_DOWNLOAD_MAX_MB` (default 200) + per-field `max_size_mb`
   (existing) / `max_download_mb` (baru). Download melebihi limit → `413
FILE_TOO_LARGE` via cek `Stat` sebelum objek dimuat.
6. **ctx.storage() Starlark + sidecar** — ops baru `link`, `stat`, `delete`,
   `init_upload`, `put_chunk`, `complete_upload` (sidecar: `upload`,
   `download`, `link`, `stat`, chunk ops; payload base64).

## File terdampak

- `internal/starlark/primitive.go` — capability interfaces + builtins
- `internal/sidecar/ctx.go` — mirror interfaces + dispatch storage ops
- `internal/api/file.go` — link/chunk/limit routes + `storageCaps`
- `internal/api/storage_sweeper.go` (baru), `handler.go`, `router.go`
- `pkg/spec/entity.go` — `StorageSpec` (+`OneTime`, `TTL`, `ChunkSizeMB`, `MaxDownloadMB`)
- `renderers/jsonb-persist/link.go` (baru), `migrate.go` (DDL)
- `renderers/jsonb-persist/datastore/minio/capabilities.go` (baru),
  `storage.go` (field `core`); `datastore/memory/capabilities.go` (baru)
- `resource/formspec.go` — wiring linkStore, env limits, sweeper worker
- `schemas/` — regenerate via `make generate-schema`
- Docs: `docs/spec/backend/05-field-types.md`, `docs/reference/primitives.md`
- Tests: `internal/api/link_test.go`, `renderers/jsonb-persist/link_test.go`,
  `renderers/jsonb-persist/datastore/memory/capabilities_test.go`

## Verifikasi

`go build ./...` ✅ · `go test ./...` ✅ (1037 test, 67 paket) ·
`make generate-schema` ✅.

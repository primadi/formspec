# Plan: ctx.storage() — Link Generation, Chunked Upload, 1x-Download & TTL

> Referensi: `docs/reference/primitives.md`, todo 7.17 (7.17.2 `visibility: signed`
> masih 501), changelog 2026-08-24-020 / 2026-08-25-007.

## Tujuan

Perluas primitive `storage` dengan:

1. **Generate link download** — app-issued token untuk data privat; presigned
   URL MinIO untuk `visibility: signed` (menggantikan `501 SIGNED_URL_NOT_IMPLEMENTED`).
2. **Chunked upload** file besar — ops multipart baru, kontrak `Upload/Download
[]byte` lama tetap (tidak breaking).
3. **1x download + TTL** — file terhapus setelah di-download (one_time) atau
   otomatis dihapus jika tidak didownload dalam batas waktu (ttl), via metadata
   app-level (tabel link) yang bekerja untuk semua driver.
4. **Limit ukuran upload/download** — global config + override per-field;
   download yang melebihi limit ditolak `413` (cek ukuran via Stat/HEAD dulu,
   tanpa refactor streaming).

## Keputusan

- Link design: **keduanya** — app-token (default) + presigned (`visibility: signed`).
- Chunking: **ops chunk baru**, kontrak `[]byte` tetap.
- 1x download + TTL: **app-level metadata** (driver-agnostic).
- Limit: \*\*global config (`FORMSPEC_UPLOAD_MAX_MB`, `FORMSPEC_DOWNLOAD_MAX_MB`)
  - per-field (`max_size_mb` existing, `max_download_mb` baru)\*\*; efektif = min.
- Surface: manifest `StorageSpec` + `ctx.storage()` Starlark + HTTP routes.

## Fase

### Fase 1 — Kontrak & Spec (fondasi)

1. Capability interfaces baru di `internal/starlark/primitive.go` (mirror identik
   di `internal/sidecar/ctx.go`, pola structural interface):
   - `Linker`: `Link(ctx, path, ttl) (url, err)`
   - `Deleter`: `Delete(ctx, path) error`
   - `Stater`: `Stat(ctx, path) (int64, error)`
   - `ChunkUploader`: `InitChunkUpload` / `PutChunk` / `CompleteChunkUpload`
2. `StorageSpec` (`pkg/spec/entity.go`): `OneTime bool`, `TTL string`,
   `ChunkSizeMB int`, `MaxDownloadMB int`. `make generate-schema`.
3. Config global limit via env (dibaca di `resource/formspec.go`, di-wire ke
   `HandlerFactory`): `FORMSPEC_UPLOAD_MAX_MB` (default 100),
   `FORMSPEC_DOWNLOAD_MAX_MB` (default 200).

### Fase 2 — Implementasi Backend (parallel per driver)

4. MinIO (`renderers/jsonb-persist/datastore/minio/storage.go`): `Linker`
   (PresignedGetObject/PutObject), `ChunkUploader` (S3 multipart), `Deleter`
   (RemoveObject), `Stater` (StatObject).
5. fs/memory (`renderers/jsonb-persist/datastore/memory/storage.go`): `Deleter`
   (os.Remove), `Stater` (os.Stat), `ChunkUploader` (dir parts + concat).
   `Link` → app-route URL (fs tidak bisa presigned).
6. Link metadata store: tabel `formspec_storage_link` (DDL di
   `renderers/jsonb-persist/migrate.go` `SystemTableDDLs`) + `StorageLinkStore`
   (pola `JobStore`). Metode: `Issue`, `Consume` (atomic), `Sweep`.

### Fase 3 — HTTP Routes (`internal/api/file.go` + `router.go`)

7. `POST /{module}/{entity}/{id}/{field}/link` — permission view → cek ukuran
   → presigned (jika `signed` + Linker) atau app-token URL.
8. `GET /storage/link/{token}` — validasi expiry/count → serve atau 302 →
   one_time tercapai → delete object + mark consumed.
9. Chunk routes: `POST .../upload/init`, `POST .../upload/{uid}/part/{n}`,
   `POST .../upload/{uid}/complete` (enforce limit & allowed_types di complete).
10. Download limit: `HandleFileDownload` cek `Stater.Stat` → `413 FILE_TOO_LARGE`.

### Fase 4 — ctx.storage() Starlark + Sidecar

11. `primitiveRunner.Attr` (`internal/starlark/primitive.go`): `link`, `delete`,
    `stat`, `init_upload`, `put_chunk`, `complete_upload` — capability check
    "not yet implemented for this backend".
12. Sidecar (`internal/sidecar/ctx.go`): dispatch cases `upload`, `download`,
    `link`, `stat`, `delete` (storage), chunk ops (payload base64).

### Fase 5 — TTL Sweeper

13. Goroutine ticker di `resource/formspec.go` (pola background loop outbox):
    hapus object dengan `ttl` lewat (belum didownload) + purge rows
    consumed/expired.

### Fase 6 — Docs, Schema, Tests

14. `make generate-schema`; docs (`docs/reference/primitives.md`, field-types
    §1.3); changelog; todo update.
15. Tests: minio storage_test, memory storage_test (chunk/delete/stat),
    link store, route tests (pola `internal/api/file_test.go`), starlark
    primitive_test.

## Verification

1. `go test ./...`
2. `make generate-schema` + `formspec validate` manifest contoh.
3. E2E manual: chunk upload → complete → link → download → one_time terhapus;
   download kedua 410; file `ttl` terhapus setelah sweep; download > limit → 413.

## Effort

Large. Fase 1–3 inti; Fase 4–6 pendukung.

# 2026-08-24-021 — MinIO Storage Backend (opsi file | minio)

## Apa yang diubah

Melengkapi wiring storage file fields (todo 7.17.1) dengan backend **MinIO/S3** sebagai opsi
kedua, selain filesystem. Storage kini punya dua backend yang dipilih via env:

- `FORMSPEC_STORAGE=file` (default) → filesystem (`memory.Storage`) di `.formspec/storage`.
- `FORMSPEC_STORAGE=minio` → MinIO/S3 via SDK `minio-go` (`datastore/minio`).

Konfigurasi MinIO via env: `FORMSPEC_MINIO_ENDPOINT` (default `minio:9000`),
`FORMSPEC_MINIO_ACCESS_KEY`/`FORMSPEC_MINIO_SECRET_KEY` (default `minioadmin`),
`FORMSPEC_MINIO_BUCKET` (default `formspec`), `FORMSPEC_MINIO_USE_SSL` (`true`/`false`).

## Kenapa

DevContainer sudah menyediakan MinIO (`minio:9000`, kredensial default `minioadmin`). Sebelumnya
storage hanya filesystem karena SDK belum ada di `go.mod`. Kini developer bisa memilih backend
storage sesuai deployment (dev = file, prod = MinIO/S3).

## File terdampak

- `go.mod` / `go.sum` — tambah `github.com/minio/minio-go/v7` (+ indirect deps).
- `renderers/jsonb-persist/datastore/minio/storage.go` (baru) — `Storage` MinIO/S3
  (Upload/Download, auto-create bucket), implementasi kontrak `api.Storage`/`ctx.storage()`.
- `renderers/jsonb-persist/datastore/minio/storage_test.go` (baru) — test integrasi terhadap
  MinIO live (skip jika tidak reachable).
- `resource/formspec.go` — resolver storage memilih backend via `FORMSPEC_STORAGE`.

## Verifikasi

- `go build ./...` — lulus.
- `go test ./...` — lulus (termasuk `TestStorageUploadDownload` terhadap MinIO live di
  devContainer: upload → download → verify).
- `npx vitest run` — 144 test lulus (tidak ada perubahan frontend).

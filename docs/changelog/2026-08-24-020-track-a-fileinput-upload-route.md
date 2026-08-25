# 2026-08-24-020 — Track A: FileInput + Upload Route (7.17.1 + 5.10.5)

## Apa yang diubah

Menuntaskan Track A widget strategy (docs/plan/widget-strategy.md) — file upload end-to-end:

**Backend (todo 7.17.1):**

- `internal/api/file.go` (baru) — `Storage` interface (Upload/Download, mirror `ctx.storage()`),
  `HandleFileUpload` (`POST /{module}/{entity}/{id}/{field}`) dan `HandleFileDownload`
  (`GET .../{field}`). Permission dinamis: upload = `{module}.{entity}.update`, download =
  `{module}.{entity}.view`. Enforcement `StorageSpec` (`allowed_types`, `max_size_mb`)
  server-side. Object key: `{ws}/{module}/{entity}/{id}/{field}/{uuid}-{name}`; key di-attach
  ke field record via `UpdateFields`.
- `internal/api/handler.go` — field `storage` + `SetStorageResolver` di `HandlerFactory`.
- `internal/api/router.go` — route file didaftarkan di kedua surface (`/_ui/entity` + `/api/v1`),
  plus passthrough `SetStorageResolver` di `RouterBuilder`.
- `resource/formspec.go` — storage resolver di-wire dengan **filesystem-backed** storage
  (`memory.Storage`) di `.formspec/storage` (default dev spec platform/06 §5).
- `internal/api/file_test.go` (baru) — upload → key tersimpan di record → download balik →
  permission denied 403.

**Frontend (todo 5.10.5):**

- `widgets/FileInput.tsx` (baru) — upload via `POST /_ui/entity/{module}/{entity}/{id}/{field}`,
  preview image/PDF, tombol Replace/Remove, enforcement size/type client-side dari `StorageSpec`.
  Mode create (belum ada record id) menampilkan hint "save first".
- `types/manifest.ts` — tipe `StorageSpec` + `StorageTransform` + field `storage` di `Field`.
- Router `FormFieldWidget` — case `fileinput`/`file` → `FileInput`.
- `derive.formWidget()` — field type `file` → `fileinput`.
- `DetailPage` — render field `file` sebagai link download.

## Kenapa

Menutup gap terakhir set wajib widget input (`07-component-kinds.md` §1). Developer kini bisa
menulis field `type: file` + `storage:` dan mendapat upload/preview end-to-end.

## Catatan MinIO

MinIO tersedia di devContainer, tapi **SDK MinIO (`minio-go`) belum ada di `go.mod`** dan tidak
ada client MinIO di kode Go. Implementasi ini memakai **filesystem-backed storage** (default dev
spec). Wiring MinIO/S3 = task lanjutan: tambah SDK + implementasi `Storage` di atas client MinIO,
lalu ganti resolver di `resource/formspec.go`.

## File terdampak

- `internal/api/file.go`, `file_test.go` — baru
- `internal/api/handler.go`, `router.go` — storage resolver + route
- `resource/formspec.go` — wire filesystem storage
- `renderers/react-shadcn/src/widgets/FileInput.tsx` — baru
- `renderers/react-shadcn/src/types/manifest.ts` — `StorageSpec` + `Field.storage`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`, `engine/derive.ts`,
  `kinds/page/DetailPage.tsx`, `widgets/index.ts` — wiring
- `docs/plan/todo.md` — tandai 7.17.1 + 5.10.5 ✅

## Verifikasi

- `go test ./...` — lulus (termasuk `TestFileUploadDownload`)
- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih

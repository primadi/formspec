# Fase 7 — Storage Spec Enforcement (7.17.2)

**Tanggal:** 2026-08-25 · **Todo:** §7.17.2

## Apa yang diubah

`StorageSpec` pada field `file` kini di-enforce penuh server-side (sebelumnya
hanya `max_size_mb` + `allowed_types` di upload).

- **`visibility`** — di `HandleFileDownload`:
  - `public` → akses anonim (tanpa cek permission `view`).
  - `private` (default) → wajib permission `view`.
  - `signed` → `501 SIGNED_URL_NOT_IMPLEMENTED` (butuh infra URL signing, deferred).
- **`max_count`** — di `HandleFileUpload`: field dengan `max_count > 1` kini
  menyimpan **array** key (multi-file); upload menolak bila sudah mencapai
  `max_count` (`FILE_COUNT_EXCEEDED`). Field dengan `max_count <= 1` tetap
  menyimpan string key tunggal.
- **Download multi-file** — `HandleFileDownload` mendukung field array key;
  client memilih file via `?index=N` (default 0).

## File terdampak

- `internal/api/file.go`

## Status

`go test ./...` hijau (0 fail). Todo 7.17.2 ditandai ✅ untuk `allowed_types`,
`max_size_mb`, `max_count`, `visibility` (public/private; signed deferred).
7.17.3 (`transform` — server-side resize/thumbnail) tetap belum dikerjakan.

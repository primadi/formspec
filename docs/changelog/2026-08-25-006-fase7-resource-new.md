# Fase 7 — Script Runtime Contract: `resource.new()` (7.14.4)

**Tanggal:** 2026-08-25 · **Plan:** `docs/plan/fase7-config-runtime.md` (WS-2) · **Todo:** §7.14.4

## Apa yang diubah

Kontrak API script runtime (`06-script-runtime.md` §2) — gap `resource.new()`
(handle baru untuk entity yang SAMA, belum tersimpan) kini diimplementasikan.

- **`internal/starlark/resource.go`** — `resource.new()` (builtin `new`):
  mengembalikan handle `ResourceAPI` baru untuk entity yang sama dengan
  `ID=""` dan data kosong; handler (`saveFn`/`callFn`/`loadFn`/`createFn`)
  di-propagate. Caller isi via `.set(...)` lalu `.save()`.
- **`resource/formspec.go`** — `SetSaveHandler` kini menerima `id == ""`:
  `save()` pada handle baru melakukan **INSERT** (bukan error), sedangkan
  `id != ""` tetap UPDATE. Ini membedakan `resource.new()` (insert via save)
  dari `resource.create()` (langsung persist entity lain).
- **Test**: `TestResourceAPI_New_SameEntityUnsaved` — verifikasi `save()` pada
  handle `new()` memanggil saveFn dengan `id == ""` dan data yang di-set.

`<Entity>.query()` (builder query §16) tetap di-defer — butuh scope builder
query yang besar.

## File terdampak

- `internal/starlark/resource.go`
- `internal/starlark/resource_test.go`
- `resource/formspec.go`

## Status

`go test ./...` hijau (0 fail). Todo 7.14.4 ditandai ✅ untuk bagian yang sudah
ada + `resource.new()`; `<Entity>.query()` di-defer.

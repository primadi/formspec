# Fix: Field Label Fallback di Frontend + Auto-detect SPA

## Problem

Field `patient_id` di entity `otc-sale` menampilkan `description` 
("Opsional — pembeli walk-in belum tentu pasien terdaftar") sebagai label
kolom tabel dan caption form, bukan humanized `name` ("Patient Id").

Sebelumnya sudah diperbaiki di `web/src/engine/derive.ts` (`fieldLabel()`
prioritas: `title` → humanized `name`), tapi perubahan tidak terefek karena:

1. `go run` meng-embed `cmd/forma/dist/` (stale), bukan `web/dist/`
2. User menjalankan `forma dev` dari subdirektori yang tidak memiliki
   `web/dist/` lokal, jadi fallback ke embedded SPA lama

## Perbaikan

### `cmd/forma/dev.go`

1. **`findWebDist()`** — fungsi baru yang mencari `web/dist/` dari CWD
   naik hingga root filesystem. Menemukan folder `web/dist/` bahkan jika
   `forma dev` dijalankan dari subdirektori project.

2. **SPA serving priority** di-update:
   ```
   --web-dir > findWebDist() (auto-detect) > embedded FS
   ```

### `cmd/forma/dist/`

Di-sync manual dari `web/dist/` hasil `npm run build` terbaru, sehingga
embedded mode juga memiliki fix untuk sementara.

## File Terkena Dampak

- `cmd/forma/dev.go` (+ fungsi `findWebDist()`, modifikasi SPA resolution)
- `cmd/forma/dist/` (sync build terbaru)

## Referensi

- `docs/plan/fix-field-label-fallback.md`
- `docs/plan/dev-positional-dir-arg.md`

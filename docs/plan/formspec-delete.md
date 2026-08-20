# Plan — Fase 3.5.1: `formspec delete`

**Tanggal**: 2026-08-17 · **Status**: In progress
**Referensi**: `docs/plan/todo.md` (3.5.1), `docs/cli-tools/02-formspec-cli.md` §3

## Tujuan

`formspec delete <kind> <name> --confirm` — hapus resource. Docs: "menandai
artifact versi berikutnya tanpa kind tersebut" (Control Plane). Karena Control
Plane di-defer, implementasi beroperasi terhadap **manifest lokal**: menghapus
manifest resource dari spec tree.

- `formspec delete <kind> <name> --confirm [--spec <path>]`
- File satu-dokumen → hapus file. File multi-dokumen → hapus dokumen yang cocok
  (via yaml.v3 node), sisakan dokumen lain.
- Wajib `--confirm`; tanpa → error, tidak menghapus apa pun.

## Perubahan

| File                                 | Perubahan                                                                      |
| ------------------------------------ | ------------------------------------------------------------------------------ |
| `cmd/formspec/delete.go` (baru)      | `runDelete` — cari manifest (kind+name), minta `--confirm`, hapus file/dokumen |
| `cmd/formspec/main.go`               | Route `delete` ke `runDelete`; hapus dari "not implemented"; tambah usage      |
| `cmd/formspec/delete_test.go` (baru) | Test hapus file satu-dokumen + dokumen multi-doc + tanpa `--confirm`           |
| `docs/plan/todo.md`                  | Tandai 3.5.1 ✅                                                                |

## Keputusan

- `document` = alias `entity` (konsisten dengan `get`/`describe`).
- Hapus file hanya bila file berisi satu dokumen; multi-doc → hapus dokumen
  yang cocok, tulis ulang sisanya.
- `--confirm` wajib — mencegah penghapusan tak sengaja.

## Verifikasi

- `go test ./cmd/formspec/...` hijau.
- `make build` hijau.
- Manual: `delete entity menu-item --confirm` pada spec contoh → file terhapus.

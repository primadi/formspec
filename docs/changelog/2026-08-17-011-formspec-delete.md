# 2026-08-17-011 — `formspec delete` (3.5.1)

**Referensi**: `docs/plan/todo.md` (3.5.1), `docs/cli-tools/02-formspec-cli.md` §3,
`docs/plan/formspec-delete.md`.

## Apa yang diubah

Menambah `formspec delete <kind> <name> --confirm` — hapus resource dari spec
tree (Control Plane di-defer, jadi beroperasi terhadap manifest lokal):

- File satu-dokumen → hapus file.
- File multi-dokumen → hapus dokumen yang cocok (via yaml.v3 node), sisakan
  dokumen lain.
- `--confirm` wajib — tanpa itu, error dan tidak menghapus apa pun.
- `document` = alias `entity` (konsisten dengan `get`/`describe`).

## Kenapa

`delete` sebelumnya jatuh ke "not implemented". Memberi jalur penghapusan
resource yang aman (dengan konfirmasi) dari spec tree — melengkapi siklus
mutation CLI read-only (`get`/`describe`).

## File terdampak

- `cmd/formspec/delete.go` (baru), `cmd/formspec/delete_test.go` (baru),
  `cmd/formspec/main.go`
- `docs/plan/formspec-delete.md` (baru), `docs/plan/todo.md`

## Verifikasi

`go test ./cmd/formspec/...` hijau; `go test ./...` hijau (19 paket ok, 0 fail);
`make build` hijau; manual: hapus file satu-dokumen + dokumen multi-doc + tanpa
`--confirm` → perilaku benar.

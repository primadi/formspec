# 2026-08-17-010 — `formspec get` + `formspec describe` (3.4.2, 3.4.3)

**Referensi**: `docs/plan/todo.md` (3.4.2, 3.4.3), `docs/cli-tools/02-formspec-cli.md` §3,
`docs/plan/formspec-get-describe.md`.

## Apa yang diubah

Menambah dua perintah inspeksi read-only (pola `kubectl get`/`describe`),
beroperasi terhadap manifest lokal karena Control Plane di-defer:

- **`formspec get <kind> [name] [--spec] [--output table|json]`** — ringkasan
  resource: kind, name, module, version, source. `document` diperlakukan
  sebagai alias `entity` (deprecated rename).
- **`formspec describe <kind> <name> [--spec]`** — detail per-kind. Untuk
  Entity: fields (name/type/required/description), actions (+ permission +
  impl), state machine (states + transitions), dan expose (permission
  surfaces). Non-Entity: spec sebagai JSON ter-indentasi.

Flag diparse manual agar bisa muncul setelah argumen posisional
(`get entity menu-item --spec ...`).

## Kenapa

`get`/`describe` sebelumnya jatuh ke "not implemented". Keduanya memberi cara
cepat memeriksa resource yang terdaftar di spec tree tanpa membuka file —
fondasi DX CLI read-only.

## File terdampak

- `cmd/formspec/get.go` (baru), `cmd/formspec/get_test.go` (baru),
  `cmd/formspec/main.go`
- `docs/plan/formspec-get-describe.md` (baru), `docs/plan/todo.md`

## Verifikasi

`go test ./cmd/formspec/...` hijau; `go test ./...` hijau (19 paket ok, 0 fail);
`make build` hijau; manual `get`/`describe` pada `examples/cafe/spec` → output
benar.

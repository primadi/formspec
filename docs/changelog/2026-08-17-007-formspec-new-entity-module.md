# 2026-08-17-007 — `formspec new entity` + `formspec new module` (3.1.3)

**Referensi**: `docs/plan/todo.md` (3.1.3), `docs/cli-tools/02-formspec-cli.md` §3,
`docs/spec/platform/08-project-layout.md`.

## Apa yang diubah

Melengkapi `formspec new <kind>` (todo 3.1.3) — anak tangga kedua scaffold CLI:

- **`cmd/formspec/new.go`** (baru) — `runNew` mendispatch `new app` (alias
  `generate node-app`, perilaku lama), `new module`, dan `new entity`.
  - `new module <name>` → `spec/modules/{module}/module.yaml`.
  - `new entity <name> [--module] [--characteristic] [--force]` →
    `spec/modules/{module}/{characteristic}/{entity}/entity.yaml` dengan
    fields dasar (`code`/`name`/`description`), `display_field`, `plural`, dan
    blok `expose` default. `characteristic` divalidasi closed set
    (master/transaction/reference/summary); module di-detect dari CWD
    (`detectModule` berjalan naik mencari `spec/modules/{module}/`) atau
    `--module`.
  - Flag diparse manual agar bisa muncul setelah argumen posisional
    (`new entity menu-item --module cafe-master`).
- **`cmd/formspec/main.go`** — route `new` ke `runNew`; tambah `new module`/
  `new entity` ke usage.
- **`cmd/formspec/new_test.go`** (baru) — test scaffold module, entity,
  validasi characteristic, dan deteksi module.

## Kenapa

`new` sebelumnya hanya mendukung `new app`. Menambah `new entity`/`new module`
memberi jalur scaffold cepat untuk manifest YAML yang valid (terverifikasi
`formspec validate` → 0 problem), mengurangi verbositas YAML.

## File terdampak

- `cmd/formspec/new.go` (baru), `cmd/formspec/new_test.go` (baru),
  `cmd/formspec/main.go`
- `docs/plan/todo.md`

## Verifikasi

`go test ./cmd/formspec/...` hijau; `make build` hijau; hasil scaffold
`formspec validate --spec spec` → 0 problem.

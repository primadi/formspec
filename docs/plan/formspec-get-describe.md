# Plan — Fase 3.4.2/3.4.3: `formspec get` + `formspec describe`

**Tanggal**: 2026-08-17 · **Status**: Complete
**Referensi**: `docs/plan/todo.md` (3.4.2, 3.4.3), `docs/cli-tools/02-formspec-cli.md` §3

## Tujuan

`formspec get` / `formspec describe` — inspeksi resource read-only, pola
`kubectl get`/`kubectl describe`. Control Plane di-defer, jadi keduanya
beroperasi terhadap **manifest lokal** (single-server dev mode), bukan registry
ter-deploy:

- `formspec get <kind> [name] [--spec] [--output table|json]` — ringkasan:
  kind, name, module, version, source.
- `formspec describe <kind> <name> [--spec]` — detail per-kind; untuk Entity:
  fields, actions (+ permission + impl), state machine, expose.

## Perubahan

| File                              | Perubahan                                                                                                                                                                    |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/formspec/get.go` (baru)      | `runGet` (table/JSON) + `runDescribe` (detail per-kind); `describeEntity` untuk Entity; helper `kindMatches` (alias document→entity), `specVersionOf`, `loadManifestsOrExit` |
| `cmd/formspec/main.go`            | Route `get`/`describe` ke `runGet`/`runDescribe`; hapus dari daftar "not implemented"; tambah usage                                                                          |
| `cmd/formspec/get_test.go` (baru) | Test `kindMatches`, `specVersionOf`, `describeEntity` output                                                                                                                 |
| `docs/plan/todo.md`               | Tandai 3.4.2, 3.4.3 ✅                                                                                                                                                       |

## Keputusan

- Flag diparse manual agar bisa muncul setelah argumen posisional
  (`get entity menu-item --spec ...`).
- `document` diperlakukan sebagai alias `entity` (deprecated rename).
- `describe` non-Entity mencetak spec sebagai JSON ter-indentasi.
- `get`/`describe` murni statis (baca manifest), tanpa DB — konsisten dengan
  `check`/`validate`.

## Verifikasi

- `go test ./cmd/formspec/...` hijau.
- `make build` hijau.
- Manual: `get entity`/`get entity menu-item`/`describe entity menu-item` pada
  `examples/cafe/spec` → output benar.

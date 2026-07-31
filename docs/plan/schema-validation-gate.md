# Plan: Validation Gate + Repair `crc-management` + Fix Skill

**Tanggal**: 2026-07-31
**Status**: Implemented (changelog 2026-07-31-005)
**Referensi**: todo 3.1.1, 3.1.1a · `docs/cli-tools/02-forma-cli.md` §2 · `ai_skills/forma-kinds/SKILL.md`

## Konteks

`examples/crc-management` di-generate memakai skill `ai_skills/forma-kinds`
yang contohnya usang, sehingga 21/21 manifest melanggar kontrak schema dan
16 entity ditolak engine loader (`expose: all`, `lifecycle: {doc_status: true}`,
`type: relation, target:`).

## Akar Masalah (3 lapis)

1. **Skill kadaluarsa** — contoh `expose: all # all|read|none`, relation
   `target:`, Module `depends_on:` tidak sesuai `pkg/spec`/`schemas/`.
2. **Tidak ada gate validasi** — `forma validate` masih stub; `Loader.Validate`
   hanya deep-validate Entity/Document (App/Module/Form/Workflow shallow).
3. **Kegagalan senyap** — `target:` diabaikan yaml.v3 → relation menggantung.

## File yang Diubah

| File | Perubahan |
|---|---|
| `examples/crc-management/spec/**` (21 manifest) | Repair ke sintaks canonical: hapus `expose` shorthand & `lifecycle` map, `version: v1`, relation `{type, resource}`, `state_machine`+`actions` di `checklist-submission`, Workflow canonical (`entity`+`on`+`steps`), App `version/vendor/root_url`, Module `depends` |
| `cmd/forma/validate.go` (baru) | `forma validate` — 2 lapis: engine loader + JSON Schema per kind |
| `cmd/forma/validate_test.go` (baru) | 8 test schema layer (expose/lifecycle/relation/workflow/app) |
| `cmd/forma/main.go` | Wire `case "validate"` |
| `go.mod` | + `github.com/santhosh-tekuri/jsonschema/v6` |
| `Makefile` | Target `validate-spec` |
| `ai_skills/forma-kinds/SKILL.md` | Contoh canonical: expose array, relation/child, state_machine, Workflow, App/Module |
| `docs/cli-tools/02-forma-cli.md` | Status `validate` |
| `docs/plan/todo.md` | 3.1.1 ✅, 3.1.1a ⏸️ |

## Keputusan Teknis

- **Schema = kontrak ke depan.** `make generate-schema` (dari `pkg/spec`)
  tidak menghasilkan diff → `schemas/` sinkron. Schema lebih ketat dari engine
  untuk konstruk shorthand yang `UnmarshalYAML` scalar+map (`guard`, `render`):
  generator schema hanya mengekspresikan bentuk objek. Ini gap ekspresivitas
  schema, bukan error — didokumentasikan, bentuk objek = cara lolos.
- **`forma validate` memakai embedded schema** (`SchemasFS`) agar jalan di
  project hasil `forma init`; `--schema <dir>` untuk override.
- **Validasi per-kind** (merge `kinds/<Kind>.schema.json` + `$defs` dari
  `forma.schema.json`) bukan oneOf top-level → pesan error presisi.

## Level of Effort

Medium (~1 sesi). Engine layer + schema layer selesai; honesty scan Starlark
ditunda (3.1.1a).

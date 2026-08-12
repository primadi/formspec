# 2026-07-31-004 — `formspec init` bundel JSON Schema + `yaml.schemas`

**Apa:** `formspec init` kini juga menscaffold `schemas/` (JSON Schema per kind,
Draft-07) dan `.vscode/settings.json` berisi `yaml.schemas` — sehingga project
baru langsung punya autocomplete + validasi manifest YAML di editor, tanpa
setup manual.

## Perubahan

- **`embed_schemas.go`** (baru, package root): embed `schemas/formspec.schema.json`
  + `schemas/kinds/*.schema.json` ke `SchemasFS embed.FS` — mirror pola
  `AISkillsFS`.
- **`cmd/formspec/init.go`**: `extractSchemas(targetDir)` menulis file schema ke
  `<project>/schemas/`; `.vscode/settings.json` ditulis (hanya jika belum ada,
  tidak menimpa settings user) dengan `yaml.schemas` memetakan
  `schemas/formspec.schema.json` → `spec/**/*.yaml` + `spec/**/*.yml`. Usage text
  dan output sukses ikut diperbarui.

## Alasan

`init` sebelumnya menghasilkan project tanpa schema, padahal `schemas/` +
`yaml.schemas` adalah bagian dari DX repo ini (lihat `CLAUDE.md` →
"JSON Schema untuk YAML Editor"). Editor YAML tidak punya grounding schema di
project hasil scaffold.

## File terdampak

- `embed_schemas.go` (baru)
- `cmd/formspec/init.go`
- `docs/plan/todo.md`
- `docs/plan/init-schema-scaffold.md` (baru)

## Referensi

- `docs/spec/platform/08-project-layout.md` (layout standar yang discaffold)
- `Makefile` target `generate-schema` (sumber schema: `pkg/spec` via `genjsonschema`)

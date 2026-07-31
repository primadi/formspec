# Plan — `forma init` membundel JSON Schema + `yaml.schemas`

**Tanggal:** 2026-07-31 · **Status:** Done (implementasi)

## Masalah

`forma init` menscaffold project baru (`spec/`, `forma-app.yaml`,
`.agents/skills/`, `.github/`) tetapi **tidak** menghasilkan:

1. Direktori `schemas/` berisi JSON Schema per kind (Draft-07).
2. `.vscode/settings.json` dengan registrasi `yaml.schemas`.

Akibatnya editor YAML (VS Code + YAML extension) tidak punya autocomplete /
validasi schema di project hasil `init` — padahal `schemas/` + `yaml.schemas`
sudah menjadi bagian dari developer experience repo ini (lihat `CLAUDE.md`
→ "JSON Schema untuk YAML Editor" dan `docs/spec/platform/08-project-layout.md`).

## Solusi

Mirror pola `AISkillsFS` (embed → extract saat init):

1. **`embed_schemas.go`** (baru, package `github.com/primadi/forma`):
   embed `schemas/forma.schema.json` + `schemas/kinds/*.schema.json` ke
   `SchemasFS embed.FS`.
2. **`cmd/forma/init.go`**:
   - `extractSchemas(targetDir)` — tulis `schemas/` ke project (dipakai dari
     `SchemasFS`, sama seperti `extractSkills`).
   - Tulis `.vscode/settings.json` berisi `yaml.schemas` yang memetakan
     `schemas/forma.schema.json` → `spec/**/*.yaml` + `spec/**/*.yml`.
     Hanya ditulis jika belum ada (tidak menimpa settings user).
   - Update usage text ("The project includes") + output sukses.

## File terdampak

| File | Aksi |
|---|---|
| `embed_schemas.go` | Baru — embed schemas |
| `cmd/forma/init.go` | Ubah — extract schemas + tulis `.vscode/settings.json` |
| `docs/plan/todo.md` | Ubah — tambah item Fase 3.1 |
| `docs/changelog/2026-07-31-004-init-bundle-schemas-yaml-schemas.md` | Baru |

## Level of effort

Small — ~1 jam, tidak ada dependensi antar task, referensi:
`docs/spec/platform/08-project-layout.md`, `Makefile` target `generate-schema`,
`docs/cli-tools/02-forma-cli.md`.

# 2026-07-31-007 — `formspec validate` auto-detect schema lokal, fallback embedded

**Apa:** `formspec validate` kini otomatis memakai schema di folder `schemas/`
lokal bila ada, fallback ke schema yang di-embed di binary.

## Perubahan

- **`cmd/formspec/validate.go`**: fungsi `resolveSchemaDir` — urutan sumber schema:
  1) `--schema` eksplisit (dipaksa), 2) `<spec>/../schemas` (folder `schemas/`
  di sebelah `spec/`, layout project `formspec init`), 3) `./schemas` (cwd),
  4) schema embed di binary. Baris pertama output mencetak sumber yang dipakai
  (`schema: <dir> (local)` / `schema: embedded`).
- **`docs/cli-tools/02-formspec-cli.md`** §2 dan **`ai_skills/schema-validation/SKILL.md`**:
  dokumentasi auto-deteksi + `--schema` untuk memaksa.

## Kenapa

Sebelumnya user harus lewat `--schema schemas` secara manual; sekarang project
hasil `formspec init` yang punya `schemas/` otomatis tervalidasi terhadap schema
lokalnya (persis yang dipakai editor lewat `yaml.schemas`), tanpa flag.

## File terdampak

- `cmd/formspec/validate.go`
- `docs/cli-tools/02-formspec-cli.md`
- `ai_skills/schema-validation/SKILL.md`
- `bin/formspec` (rebuild via `make build`)

## Referensi

- `docs/changelog/2026-07-31-005-*.md` (implementasi awal `formspec validate`)

# 2026-07-31-007 — `forma validate` auto-detect schema lokal, fallback embedded

**Apa:** `forma validate` kini otomatis memakai schema di folder `schemas/`
lokal bila ada, fallback ke schema yang di-embed di binary.

## Perubahan

- **`cmd/forma/validate.go`**: fungsi `resolveSchemaDir` — urutan sumber schema:
  1) `--schema` eksplisit (dipaksa), 2) `<spec>/../schemas` (folder `schemas/`
  di sebelah `spec/`, layout project `forma init`), 3) `./schemas` (cwd),
  4) schema embed di binary. Baris pertama output mencetak sumber yang dipakai
  (`schema: <dir> (local)` / `schema: embedded`).
- **`docs/cli-tools/02-forma-cli.md`** §2 dan **`ai_skills/schema-validation/SKILL.md`**:
  dokumentasi auto-deteksi + `--schema` untuk memaksa.

## Kenapa

Sebelumnya user harus lewat `--schema schemas` secara manual; sekarang project
hasil `forma init` yang punya `schemas/` otomatis tervalidasi terhadap schema
lokalnya (persis yang dipakai editor lewat `yaml.schemas`), tanpa flag.

## File terdampak

- `cmd/forma/validate.go`
- `docs/cli-tools/02-forma-cli.md`
- `ai_skills/schema-validation/SKILL.md`
- `bin/forma` (rebuild via `make build`)

## Referensi

- `docs/changelog/2026-07-31-005-*.md` (implementasi awal `forma validate`)

# 2026-07-31-006 — Skill `schema-validation` (validate + repair)

**Apa:** Menambahkan skill AI baru `ai_skills/schema-validation/SKILL.md` untuk
workflow generate → validate → fix → re-validate.

## Perubahan

- **`ai_skills/schema-validation/SKILL.md`** (baru): instruksi menjalankan
  `formspec validate`, membaca output dua lapis (`engine:` vs `schema:`), katalog
  perbaikan canonical (expose array, lifecycle string, relation `{type,resource}`,
  `spec.version`, App required fields, Module `depends`, Workflow canonical,
  dst.), dan gap schema-vs-engine yang diketahui (`guard`/`render` shorthand).
- **`ai_skills/README.md`**: inventori ditambah baris Schema Validation.

Tidak ada perubahan kode — `//go:embed ai_skills/*/SKILL.md` otomatis
menangkap folder baru (diverifikasi `go build ./cmd/formspec`).

## Kenapa

Usai `formspec validate` diimplementasikan (changelog 005), belum ada skill yang
memandu agen untuk menjalankannya dan memperbaiki error — padahal inilah alur
utama "generate → pastikan sesuai schema". Skill ini menutup gap tersebut dan
ikut di-distribusikan ke project hasil `formspec init`.

## File terdampak

- `ai_skills/schema-validation/SKILL.md` (baru)
- `ai_skills/README.md`

## Referensi

- `cmd/formspec/validate.go`, `docs/changelog/2026-07-31-005-*.md`
- `docs/plan/schema-validation-gate.md`

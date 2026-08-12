# 2026-08-10-002-create-formspec-app-workflow-skill

**Lapis**: AI Skill (baru)
**Referensi**: `docs/plan/ai-skills-and-spec-cleanup.md` §A.2

## Perubahan

Buat skill baru `formspec-app-workflow` — orkestrator full lifecycle pembuatan aplikasi FormSpec.

## Isi skill

4 fase sequential:

1. **Discovery** — output `docs/overview.md` (bahasa awam, untuk business owner)
2. **Proposal** — output `docs/architecture.md` + `docs/domain-model.md` (modul, karakteristik entity, state machine, decision checklist)
3. **Draft** — output `spec/modules/**/*.yaml` + `spec/apps/*.yaml` (write order: Module → Entity → UI overrides → App → validate)
4. **Iterate** — change management dengan model top-down: tentukan lapis terdampak → perbarui ke bawah → changelog

## Aturan kunci

- Fase tidak bisa dilompati — AI harus konfirmasi sebelum lanjut
- Docs ≠ Spec — docs = decisions + context, spec = definitions + precision
- Tidak redundancy — detail field hanya di spec
- 95% kasus cukup Entity — jangan over-engineer dengan UI overrides
- Validasi setiap saat: `formspec validate` setelah setiap perubahan signifikan
- Fitur belum implemented: `# TODO:` comment di YAML

## Delegasi ke skill lain

- `formspec-kinds` — syntax YAML per kind
- `formspec-spec-structure` — navigasi spec docs
- `schema-validation` — validate → repair → re-validate

## File terdampak

- `ai_skills/formspec-app-workflow/SKILL.md` — file baru
- `ai_skills/README.md` — tambah inventory entry

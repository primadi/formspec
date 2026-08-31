# 2026-08-10-005-cleanup-platform-03-kind-system-4-group

**Lapis**: Spec (`docs/spec/platform/03-kind-system.md`)
**Referensi**: `docs/plan/ai-skills-and-spec-cleanup.md` §B.3

## Perubahan

Restruktur §1 "Taksonomi Kind" dengan 4-group taxonomy:

- **Overview tabel**: Curation (2), Data (11), UI (15), Infra (5) — dengan mirror ke struktur `docs/spec/`
- **Rincian per Grup**: masing-masing grup punya tabel sendiri dengan kind → spec file reference
- **Referensi UI 3-layer**: update dari §9 ke §14 di `06-page-kinds.md`
- Format lama (flat concern-based table) diganti dengan 4 tabel terpisah yang lebih readable

## File terdampak

- `docs/spec/platform/03-kind-system.md` — §1 direwrite

# 2026-08-10-004-cleanup-frontend-06-page-kinds-ui-3-layer

**Lapis**: Spec (`docs/spec/frontend/06-page-kinds.md`)
**Referensi**: `docs/plan/ai-skills-and-spec-cleanup.md` §B.2

## Perubahan

Ekspansi §14 "Derivasi Otomatis (Layer 0)" menjadi "Derivasi Otomatis & UI 3-Layer Wrapping" yang mencakup:

- **Diagram ASCII** UI 3-layer model: Page → Form/Table → Entity
- **Layer 0 — Entity**: daftar lengkap apa yang auto-derived (Table, Form create, Form edit, Page detail, REST API, menu entries)
- **Aturan Wrapping**: tabel kapan override diperlukan (Entity saja vs Form vs Table vs Page)
- **Decision Flow**: diagram teks untuk menentukan apakah perlu UI kind override
- **`public` field**: kontrol route auto-generated per visual kind
- **Prinsip Kunci**: Entity dulu override belakangan, jangan over-engineer, override minimal, Page = komposisi

## File terdampak

- `docs/spec/frontend/06-page-kinds.md` — §14 direwrite penuh

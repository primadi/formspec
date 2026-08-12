# Master Plan: AI Skills + Spec Cleanup

**Last Updated**: 2026-08-10
**Status**: ✅ A.1 complete · ✅ A.2 complete · ✅ B.1 complete · ✅ B.2 complete · ✅ B.3 complete

> `⬜` not started · `🔄` in progress · `✅` complete · `⏸️` deferred

**Scope**: Two independent tracks — AI Skills (workflow for AI-assisted app creation) and Spec Cleanup (improve documentation quality of `docs/spec/`). Both tracks complete for P1-P3.
**Parallel**: Track A and Track B can be executed in parallel. Within Track A, A.2 depends on A.1. Within Track B, B.3 depends on B.1+B.2.
**Sumber**: Diskusi plan mode 2026-08-10.

---

## Decisions (from plan mode)

1. **4-group taxonomy**: Data (11) / UI (15) / Curation (2) / Infra (5) — mirrors `docs/spec/` structure (backend/frontend/platform)
2. **UI 3-layer model**: Entity → Form/Table → Page. Entity auto-derives default Form + Table + Page. Override only when needed.
3. **Table = bentuk dari Form** — both are visual overrides at the same layer above Entity
4. **Docs ≠ Spec**: Docs contain design decisions + big picture; Spec contains precise definitions. No redundancy.
5. **Skills built in parallel with spec implementation** — don't wait for spec to reach 100%
6. **Plan baru tidak replace `todo.md`** — berbeda scope (skills+docs vs engine implementation)

---

## Track A — AI Skills

### A.1 Rewrite `formspec-kinds` — 4-Group Taxonomy + UI 3-Layer Model ✅ COMPLETE

**Goal**: Restructure the kinds catalog from flat/concern-based to 4 clear groups, and document the UI 3-layer wrapping model as a first-class concept.

**Changes to `ai_skills/formspec-kinds/SKILL.md`**:

| Change | Detail |
|--------|--------|
| Restructure sections | Order: Curation → Data → UI → Infra (top-down: what you build first) |
| Add group headers | `## Data Kinds`, `## UI Kinds`, `## Curation Kinds`, `## Infra Kinds` with clear visual separators |
| Add UI 3-layer section | New `### UI 3-Layer Model` section BEFORE individual UI kind descriptions |
| Update Quick Reference | Replace flat "Concern → Kind" table with 4-group table |
| Update description frontmatter | Add group tags to description field |
| Update "Choosing the Right Kind" | Group by category |
| Preserve all existing content | Gotchas, syntax examples, YAML snippets — all preserved |

**UI 3-Layer Model (new section)**:

```
┌─────────────────────────────────────────────┐
│ PAGE  (route + composition)                 │
│  ┌───────────────────────────────────────┐  │
│  │ FORM / TABLE  (layout override)       │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │ ENTITY  (data model)            │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

| Declare | Auto-generated | When to override |
|---------|---------------|------------------|
| Entity only | Default Table + Form(create+edit) + Page(detail) + REST API | Custom layout, field order, hide fields |
| Form (public:true) | Auto-wrapped in Page, route `/<module>/form/<name>` | Form needs custom Page (multi-tab, side panel) |
| Table (public:true) | Auto-wrapped in Page, route `/<module>/table/<name>` | Table needs custom Page |
| Page | Route directly, no wrapping | — (Page always explicit) |
| Form/Table (public:false) | No route; embed-only inside authored Page | — |

**Dependensi**: Tidak ada.
**Effort**: Medium.

---

### A.2 Create `formspec-app-workflow` — Discovery → Proposal → Draft → Iterate ✅ COMPLETE

**Goal**: New skill that orchestrates the full lifecycle of creating a FormSpec application with AI assistance. Guides AI through 4 phases, delegates to other skills at appropriate points.

**New file**: `ai_skills/formspec-app-workflow/SKILL.md`

**Skill structure**:

```yaml
name: formspec-app-workflow
description: Orchestrates the full FormSpec app creation lifecycle — Discovery, Proposal, Draft (YAML spec), and Iterate (change management). Use when building a new FormSpec application, adding features to an existing app, or making changes that span multiple phases. Delegates to formspec-kinds for syntax, schema-validation for validation, and formspec-spec-structure for spec file navigation.
```

**Fase 1 — Discovery**:
- Output: `docs/overview.md` (bahasa awam, untuk business owner)
- Isi: tujuan bisnis, alur kerja, aturan bisnis, tech stack
- Aturan: AI tidak boleh lompat ke YAML sebelum kebutuhan bisnis dikonfirmasi
- Referensi: probing questions dari industry template (formspec-remote-mcp)

**Fase 2 — Proposal**:
- Output: `docs/architecture.md` + `docs/domain-model.md`
- Isi:
  - `architecture.md`: pembagian modul (bounded context), karakteristik entity, keputusan desain, dependensi antar modul
  - `domain-model.md`: daftar entity, field penting, relasi, state machine (Mermaid), karakteristik per entity
- Delegasi: `formspec-kinds` untuk pemilihan kind & karakteristik, `formspec-spec-structure` untuk navigasi spec docs
- Aturan: tentukan entity mana yang cukup derived vs butuh override Form/Page

**Fase 3 — Draft**:
- Output: `spec/modules/<module>/*.yaml` + `spec/apps/<app>.yaml`
- Urutan tulis: Module → Entity → (Form/Table override jika perlu) → (Page kustom jika perlu) → App
- Delegasi: `formspec-kinds` untuk syntax YAML, `schema-validation` untuk validate → repair → re-validate
- Aturan:
  - Tulis Entity dulu, baru Form/Page override hanya jika diperlukan
  - Setiap spec YAML harus lolos `formspec validate` sebelum dianggap selesai
  - Fitur yang belum diimplementasi engine: beri komentar `# TODO: menunggu implementasi <fitur>`

**Fase 4 — Iterate**:
- Trigger: perubahan kebutuhan bisnis, perubahan desain, atau perubahan detail field
- Aturan perubahan top-down:
  - Perubahan kebutuhan bisnis → update discovery → proposal → spec
  - Perubahan desain teknis → update proposal → spec
  - Perubahan field/validasi → update spec saja
- Changelog: `docs/changelog/YYYY-MM-DD-NNN.md` — catat lapis yang berubah + alasan + file terdampak
- Validasi: `formspec validate` harus clean setelah setiap iterasi

**Key rules across all phases**:
1. Fase tidak bisa dilompati — AI harus dapat konfirmasi sebelum lanjut
2. Docs ≠ Spec — docs = decisions + context, spec = definitions + precision
3. Tidak redundancy — detail field hanya di spec, tidak di-copy ke docs
4. 95% kasus cukup Entity — jangan over-engineer dengan Form/Page kustom
5. Setiap Entity wajib: `spec.version: v1`, `metadata.description`, karakteristik yang tepat

**Dependensi**: A.1 (`formspec-kinds` harus selesai dulu — skill ini merujuk ke formspec-kinds).
**Effort**: Medium-Large.

---

### A.3 Minor Update After Spec Cleanup

**Goal**: Update references in `formspec-kinds` and `formspec-app-workflow` after spec documents are cleaned up.

**Changes**:
- Update spec file references (e.g., section numbers may change)
- Update any outdated syntax or field references
- Verify all cross-references between skills and spec docs

**Dependensi**: B.1, B.2, B.3.
**Effort**: Small.

---

## Track B — Spec Cleanup

### B.1 `backend/01-core-basic.md` — Entity, Fields, Expose, Lifecycle (P1) ✅ COMPLETE

**Goal**: Complete and restructure the core backend spec — the most frequently referenced document during Draft phase.

**Issues to fix**:
- Atribut Entity yang masih kurang/belum lengkap
- Struktur dokumen — pastikan setiap section punya: definisi, contoh YAML, tabel atribut
- Field types catalog — pastikan semua field type (string, number, money, date, enum, relation, child, json, boolean, text, integer) terdokumentasi dengan atribut lengkap
- expose model — canonical list form documented clearly
- lifecycle + doc_status — interaksi dengan state_machine dijelaskan dengan jelas
- Permission model — resource+action, never hardcoded roles

**Dependensi**: Tidak ada.
**Effort**: Large.

---

### B.2 `frontend/06-page-kinds.md` + `07-component-kinds.md` — UI 3-Layer + Wrapping (P2) ✅ COMPLETE

**Goal**: Document the UI 3-layer model as a first-class concept and the auto-wrapping rules.

**Issues to fix**:
- Tambah section "UI 3-Layer Model" yang menjelaskan: Entity → Form/Table → Page hierarchy
- Auto-wrapping rules: kapan Entity menghasilkan default Form/Table/Page, kapan override diperlukan
- `public` field behavior di setiap visual kind
- Hubungan antara `kind: Page` (eksplisit) vs auto-derived Page wrapper

**Dependensi**: Tidak ada (bisa paralel dengan B.1).
**Effort**: Medium.

---

### B.3 `platform/03-kind-system.md` — 4-Group Taxonomy (P3) ✅ COMPLETE

**Goal**: Sinkronkan kind taxonomy dengan 4-group model (Data/UI/Curation/Infra) dan kind→plane mapping.

**Issues to fix**:
- Taxonomy 4 grup
- Kind→plane mapping (resource plane vs control plane)
- Meta-kinds (VisualSpecKind, Renderer, PersistBackend) relation to the 4 groups

**Dependensi**: B.1, B.2 (untuk konsistensi cross-reference).
**Effort**: Medium.

---

### B.4 Sisanya — Control Plane, Datastore, Marketplace (P4)

**Goal**: Cleanup remaining spec documents.

**Scope**:
- `platform/04-control-plane.md`
- `platform/05-plane-protocol.md`
- `platform/06-datastore.md`
- `platform/07-marketplace.md`
- `platform/08-project-layout.md`
- `backend/02-core-extended.md`

**Dependensi**: B.3.
**Effort**: Large.

---

## Execution Strategy

```
Session 1 (done):      A.1 (formspec-kinds rewrite)
Session 2 (done):      A.2 (formspec-app-workflow) + B.1 + B.2 + B.3
Session 3+:            B.4 (remaining spec docs)
```

Tracks A dan B bisa dikerjakan paralel dalam sesi yang sama karena tidak ada dependensi ketat kecuali A.2→A.1 dan B.3→B.1+B.2.

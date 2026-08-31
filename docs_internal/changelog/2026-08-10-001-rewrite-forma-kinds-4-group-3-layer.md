# 2026-08-10-001-rewrite-formspec-kinds-4-group-3-layer

**Lapis**: AI Skill (`formspec-kinds`)
**Referensi**: `docs/plan/ai-skills-and-spec-cleanup.md` §A.1

## Perubahan

Restruktur `ai_skills/formspec-kinds/SKILL.md` dari flat/concern-based ke 4 grup:

- **Curation** (App, Module) — dipindah ke paling atas karena ini yang pertama dideklarasi
- **Data** (Entity, Service, Config, Migration, Subscription, Workflow, Api, Webhook, Mockup, Integrator, KindDefinition)
- **UI** (Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Calendar, Listing, ApprovalInbox, NotificationCenter, Print, Theme)
- **Infra** (Renderer, PersistBackend, Environment, Policy, Datastore)

## Perubahan utama

- Tambah section "UI 3-Layer Model" — dokumentasi wrapping hierarchy Entity → Form/Table → Page, aturan auto-derive, decision flow kapan perlu UI override
- Quick Reference diganti dari flat table ke 4-group table
- "Choosing the Right Kind" dikelompokkan per kategori
- Deskripsi frontmatter diupdate dengan group tags
- Semua konten existing (gotchas, YAML examples, syntax rules) dipertahankan
- Version bump: 1.0 → 2.0

## File terdampak

- `ai_skills/formspec-kinds/SKILL.md` — rewrite penuh
- `ai_skills/README.md` — update deskripsi inventory

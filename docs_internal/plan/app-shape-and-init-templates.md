# Plan: App Shape Consultation + `init` Template Extraction

**Tanggal**: 2026-09-13
**Status**: ✅ Selesai
**Effort**: Medium

## Tujuan

1. Skill pembuatan App memperhatikan **bentuk App** — `access`
   (`private`/`public`) dan `app_renderer` (`sidebar-nav`/`topnav`/`no-nav`) —
   dan menyarankan ke user: satu App private, satu App public, atau dua App.
2. Referensi `docs/kind/` diarahkan ke URL publik (docs.formspec.dev), bukan
   path repo-relative yang rusak di project hasil scaffold.
3. Semua string template hardcoded di `cmd/formspec/init.go` dipindah ke
   direktori template yang di-embed (`cmd/formspec/template_init/`).

## Referensi Spec

- `docs/kind/curation/App.md` — atribut `access` + `app_renderer` (generated dari `pkg/spec`)
- `docs/spec/frontend/05-app-kinds.md` §1–§5 — App tier, chrome, access semantics
- `docs/spec/platform/02-workspace-app-module.md` §3–§4 — App + Menu
- `pkg/spec/resources.go` — `AppSpec`, `AppAccess`, `AppRendererNames`, `AppChrome`
- `docs/spec/platform/08-project-layout.md` — layout project scaffold

## Decided

- Konten App-shape otoritatif di `ai_skills/formspec-kinds/SKILL.md` (App
  section) + hook ringan di `ai_skills/formspec-app-workflow/SKILL.md`. Tidak
  membuat skill `app-authoring` baru (dipromosikan nanti bila perlu).
- `docs/kind` direferensikan via **URL absolut** dari skill + AGENTS.md hasil
  scaffold; docs tidak divendorkan ke project.
- `ai_skills/` **tetap di root** (dipakai bersama MCP `cmd/formspec/mcpserve.go`
  - ~84 cross-link docs); `template_init/` hanya untuk template file.

## Perubahan

### Fase 1 — Konsultasi App shape (skills)

- `ai_skills/formspec-kinds/SKILL.md` — subsection **App shape** (`access` ×
  `app_renderer`, heuristik satu/dua App, chrome opt-in, pasangan `Listing`),
  2 gotcha baru, count kind 33→34 + Curation 3 (tambah `Workspace`).
- `ai_skills/formspec-app-workflow/SKILL.md` — pertanyaan audiens di Discovery;
  item App shape di Proposal (content + decision checklist); catatan di
  write-order Draft langkah 4.
- `ai_skills/README.md` — count 34.
- `docs/kind/curation/App.md` — heuristik bentuk App di "Kapan Memakai".

### Fase 2 — Referensi docs/kind (URL)

- Link `docs/kind/` di skill → `https://docs.formspec.dev/kind/`.
- AGENTS.md hasil scaffold: section **Reference Docs** + section **App Shape**.
- `AGENTS.md` (repo) — baris `docs/kind/` di Key Reference Files.

### Fase 3 — Ekstraksi template `init`

- Baru: `cmd/formspec/template_init/` (committed) — `formspec-app.yaml`,
  `spec/apps/app.tmpl.yaml`, `spec/workspaces/ws.tmpl.yaml`, `.gitignore`,
  `AGENTS.md`, `.github/copilot-instructions.md`, `.vscode/settings.json`.
- Keputusan: `formspec init` **Starlark-only** — flag `--with-sidecar` + scaffold
  `app/` dihapus; sidecar polyglot tetap tersedia lewat `formspec generate <lang>-app`.
- Baru: `cmd/formspec/template_init.go` — `//go:embed all:template_init` +
  `extractTemplates` (substitusi `{{projectName}}`/`{{module}}`/`{{schemaURL}}`,
  rename `*.tmpl.yaml` → `<module>.yaml`, never-clobber untuk
  `copilot-instructions.md` + `settings.json`).
- `cmd/formspec/init.go` — hapus literal inline + mekanisme `{BT}` +
  `makeAgentsInstructions`; panggil `extractTemplates`.
- `cmd/formspec/init_test.go` — buang assertion `schemas/` + registry server
  yang stale; tambah assertion manifest module-named, hint App shape, docs URL.

### Fase 4 — Mirror + proses

- `examples/{arisan,cafe,crc-management}/.agents/skills/` disinkronkan penuh
  dari `ai_skills/` (mirror = copy skill yang dikirim bersama binary). Sync ini
  sekaligus membersihkan drift lama pada mirror `formspec-kinds`.

## Dependensi

Fase 1–2 independen. Fase 3 bergantung pada keputusan Fase 2 (URL docs ada di
template AGENTS.md). Fase 4 setelah Fase 1–2.

## Verifikasi

- `go build ./cmd/formspec/` ✅
- `go test ./cmd/formspec/ -run 'TestRunInit_ScaffoldsProject|TestRelevantSkillsFor'` ✅
- Smoke test scaffold di temp dir — struktur + AGENTS.md + App manifest tervalidasi.

# 2026-09-13-002 — App Shape consultation + `init` template extraction

## Apa yang diubah

**Konsultasi App shape.** Skill `formspec-kinds` (App section) kini
mendokumentasikan dua sumbu ortogonal `kind: App` — `access`
(`private` default / `public`) dan `app_renderer` (`sidebar-nav` default /
`topnav` / `no-nav`) — plus heuristik konsultasi (satu App private, satu App
public, atau dua App public+private), chrome opt-in pada `no-nav`, dan
pasangan `Listing` untuk App public. `formspec-app-workflow` menambahkan hook:
pertanyaan audiens di Discovery, decision item "App shape" di Proposal, dan
catatan App shape di write-order Draft. `docs/kind/curation/App.md` memberi
heuristik yang sama di "Kapan Memakai". Sekaligus diperbaiki drift count kind
(33 → 34, Curation 2 → 3 dengan `Workspace`) di `formspec-kinds` +
`ai_skills/README.md`.

**Referensi docs kind.** Link `docs/kind/` di skill diarahkan ke URL publik
`https://docs.formspec.dev/kind/` (sebelumnya path repo-relative
`../docs/kind/README.md` yang rusak di project hasil scaffold). AGENTS.md hasil
scaffold mendapat section **Reference Docs** (docs.formspec.dev) dan **App
Shape**; `AGENTS.md` repo mendapat baris `docs/kind/` di Key Reference Files.

**Ekstraksi template `init`.** Seluruh string template inline di
`cmd/formspec/init.go` dipindah ke `cmd/formspec/template_init/` (file nyata,
committed) yang di-embed via `//go:embed all:template_init`; renderer baru
`cmd/formspec/template_init.go` melakukan substitusi `{{projectName}}` /
`{{module}}` / `{{schemaURL}}`, rename manifest module-named
(`*.tmpl.yaml` → `<module>.yaml`), never-clobber untuk
`copilot-instructions.md` + `.vscode/settings.json`. `makeAgentsInstructions` +
mekanisme `{BT}` dihapus. **`formspec init` kini Starlark-only** — flag
`--with-sidecar` dan scaffold direktori `app/` dihapus (sidecar polyglot dibuat
terpisah via `formspec generate <lang>-app`, yang sudah ada). Scaffold App kini
memuat hint berkomentar `access:` / `app_renderer:`. `ai_skills/` tetap di root
(dipakai bersama MCP) — `template_init/` hanya untuk template file.

**Test.** `cmd/formspec/init_test.go` dibersihkan: assertion `schemas/` + HTTP
registry server yang stale dihapus, diganti assertion manifest module-named,
hint App shape, dan docs URL di AGENTS.md.

**Mirror contoh.** `examples/{arisan,cafe,crc-management}/.agents/skills/`
disinkronkan penuh dari `ai_skills/` (mirror = copy skill yang dikirim bersama
binary) — sekaligus membersihkan drift lama pada mirror `formspec-kinds` yang
tertinggal dari versi kanonik.

## Kenapa

Permintaan user: skill pembuatan App harus memperhatikan bentuk public/private
dan menyarankan tipe nav; konsultan perlu menyarankan sidebar-nav/topnav/no-nav;
referensi `docs/kind` perlu tempat yang benar; dan hardcode `init.go` perlu
dipisah ke folder template.

## File terdampak

- `ai_skills/formspec-kinds/SKILL.md`, `ai_skills/formspec-app-workflow/SKILL.md`, `ai_skills/README.md`
- `docs/kind/curation/App.md`
- `AGENTS.md` (repo), `cmd/formspec/template_init/AGENTS.md` (template)
- `cmd/formspec/init.go`, `cmd/formspec/template_init.go`, `cmd/formspec/template_init/**`
- `cmd/formspec/init_test.go`
- `examples/{arisan,cafe,crc-management}/.agents/skills/**`

## Referensi

- Plan: `docs_internal/plan/app-shape-and-init-templates.md`

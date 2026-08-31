# 2026-08-30-007 — Restrukturisasi docs (docs_internal) + konsolidasi kontrak datastore

## Apa yang diubah

**Restrukturisasi dokumentasi** — pisahkan dokumen final dari dokumen kerja:

- `docs/plan/` → `docs_internal/plan/`, `docs/changelog/` → `docs_internal/
  changelog/`, `docs/presentations/` → `docs_internal/presentations/`,
  `docs/technical-notes/` → `docs_internal/technical-notes/` (git mv).
- `docs/` kini hanya berisi dokumen final developer-facing: `spec/`, `kind/`,
  `renderers/`, `runtimes/`, `cli-tools/`, `reference/`, `guides/`,
  `architecture/`, `ai/`, `comparison/`, `registry/`.
- 17 file di `docs/`, `ai_skills/`, `.github/` di-update path-nya
  (`docs/plan/` → `docs_internal/plan/` dst). CLAUDE.md + `docs/README.md`
  mendefinisikan pembagian baru.
- `docs-site/.vitepress/config.mts`: `srcExclude` folder internal dihapus
  (folder itu tidak pernah masuk srcDir lagi). Build VitePress diverifikasi:
  `dist/` bersih tanpa folder internal.

**Konsolidasi kontrak datastore** (menyusul implementasi fase A–D infra
registry):

- `docs/spec/platform/06-datastore.md` v0.2.0 — kontrak tunggal model
  3-level: Infra Registry (registrasi service eksplisit, multi-service per
  primitive, default overridable), App Registry (seleksi `datastores` map,
  named logical primitive `primitive/alias`), Workspace Binding
  (`access.filter`/`access.permission`); chain resolusi; driver×serves
  terkini (config/log routable); error codes §6.
- `docs/kind/infra/Datastore.md` — contoh manifest benar + gotchas named
  primitive & multi-service.
- `docs/spec/platform/02-workspace-app-module.md` — `Module.spec.datastores`
  map + legacy `datastore:`; narasi lintas-service.
- `docs/spec/backend/06-script-runtime.md` §5 — katalog 9 primitive,
  named primitive, config/log backend.
- `docs/reference/primitives.md` — BARU: referensi ringkas 9 primitive,
  3-level registry, driver×serves, contoh manifest, error codes.
- `ai_skills/formspec-kinds/SKILL.md` — bagian Datastore + decision table.

## Kenapa

Dokumentasi final tercampur dokumen kerja/sejarah di satu tree, dan kontrak
datastore stale terhadap implementasi (masih melarang `.named()`, belum ada
App Registry). `docs/` kini bersih untuk developer FormSpec; docs-site
otomatis hanya memuat konten final.

## File terdampak

Lihat daftar di atas; migrasi via `git mv` (riwayat terjaga).

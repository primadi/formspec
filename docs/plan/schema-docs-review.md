# Plan: Resolusi Review Schema ↔ Docs

**Tanggal:** 2026-07-31 · **Status:** ✅ Selesai (lihat `docs/changelog/2026-07-31-001-resolusi-review-schema-docs.md`)
**Referensi spec:** `docs/spec/backend/01-core-basic.md`, `05-field-types.md`; `docs/spec/frontend/06-page-kinds.md`, `07-component-kinds.md`; `docs/spec/platform/02-workspace-app-module.md`, `04-control-plane.md`, `06-datastore.md`
**Todo:** `docs/plan/todo.md` Fase 11

## Tujuan
Menutup kontradiksi antara `pkg/spec`/JSON Schema/`renderers/web` dan `docs/spec/`.
Arah resolusi (disepakati): **case-by-case sesuai bukti** — kode yang jalan jadi
acuan; docs normatif diimplementasi di kode/schema; fitur aspirational
ditandai **Open** di docs dan ditracking ke plan/todo yang sudah ada.

## Arah per item

| Item | Arah | Alasan |
|---|---|---|
| `Config.keys` collapse | Fix generator → `$ref ConfigKey` | Bug generator murni; struct `ConfigKey` sudah ada & cocok docs |
| `Entity` vs `Document` | `Entity` canonical, `Document` alias deprecated | Selaras docs/examples/CLI/skills + keputusan todo 0.2 |
| `Module` `vendor/datastore/config/ai_index` | Docs→kode (tambah struct) | Normatif di `02-workspace-app-module.md` §2 & `06-datastore.md` §1.1 |
| `App` `publishes/consumes` + `app_renderer/theme_ref/auth_config_ref` | Docs→kode | Contoh normatif di `02-workspace-app-module.md` §3; `[]string` tidak pernah dikonsumsi kode |
| `Policy`/`Environment` | Docs→kode (struct baru) | Contoh normatif lengkap di `04-control-plane.md` §2/§5; schema bare `{}` |
| `attachment` alias `file` | Docs→kode (tambah enum + normalisasi) | `05-field-types.md` §1.3 menyatakan alias; biaya kecil |
| `Entity.auth` | Kode→docs (dokumentasikan) | `EntityAuth` ada di kode, belum didokumentasikan |
| `computed` `script` vs `formula` | Docs→kode (`formula` inline expr) | Runtime `renderers/jsonbpersist/crud.go` mengevaluasi `Computed.Formula` via `starlark.EvalExpr` |
| Form `layout.sections`/`readonly`/`render` | Docs→kode | Renderer pakai `sections` flat, `read_only`, `render:{mode}` |
| Table `filters`/`default_sort` | Docs→kode | Renderer/Go pakai objek `filters` + string `default_sort` (`-field` = desc) |
| Kanban zero-config/`group_by`/`drag_guard`/`wip_limit`/`card_fields` | Kode→docs (Open) | Sudah ditracking `docs/plan/kanban-full-implementation.md`; schema punya `status_field` wajib, `columns{status,label}`, `card_template` |
| Dashboard/Widget inline union | Docs→kode (ref-based) | `07-component-kinds.md` §2–3 + kode `DashboardWidget{ref,layout}` konsisten; §7 docs kontradiktif dgn dirinya sendiri |
| Report `params/source/group_by/exports` | Docs→kode + Open `source.filter` | Go `parameters`/`entity`/`groups`/`export`; renderer pakai `parameters`/`groups` |
| Print `formats` | Kode→docs (hapus) | Satu format per manifest = `output.format`; `Formats` tidak dikonsumsi siapa pun |
| Page `binds`/`mode:custom`, BlockRef `needs` | Defer + Open | **Koreksi review:** fitur ini ada di docs, BUKAN di `pkg/spec` — schema-ahead yang dimaksud review ternyata docs-ahead |

## File terkena
- `pkg/spec/{entity,frontend,resources,spec,control}.go`
- `internal/genjsonschema/{converter,generator,kinds}.go`
- `internal/manifest/loader.go`, `internal/entity/{registry,registry_test}.go`,
  `internal/ui/{validate,ui_test}.go`
- `renderers/web/src/types/manifest.ts`, `renderers/web/src/kinds/report/ReportRenderer.tsx`
- `schemas/formspec.schema.json` + `schemas/kinds/*` (regen)
- Docs: `06-page-kinds.md`, `07-component-kinds.md`, `05-field-types.md` (implied),
  `01-core-basic.md`, `02-workspace-app-module.md` (implied), `04-control-plane.md` (implied),
  `06-datastore.md` (implied), `07-vertical-modules.md`, `docs/runtimes/02-formspec-resource.md`
- Contoh manifest Report: `examples/*`, `verticals/*` (perbarui ke bentuk kode)

## Deferred (Open — bukan bagian effort ini)
- Kanban zero-config derivasi kolom / `group_by` / `drag_guard` / `wip_limit` /
  `card_fields` → `docs/plan/kanban-full-implementation.md`
- Report `source.filter` (filter deklaratif parameterized)
- Dashboard/Widget rendering (`stat`/`chart`) → todo Fase 5.7
- Page master-detail `binds` / `layout.mode: split`, custom Page `mode: custom` /
  `binds`, `BlockRef.needs` → todo Fase 5
- Binding `Module.datastore` runtime → todo 2.9.4
- Eksekusi control-plane (Policy/Environment) → Fase Cloud

## Catatan
- Test e2e `examples/Clinic-UI-Showcase` merah di base code (bukan akibat
  perubahan ini): fixture tanggal hardcoded `2026-07-12` melebihi backdate
  limit 3 hari; `script_ref` di-resolve relatif cwd. Di luar scope review.

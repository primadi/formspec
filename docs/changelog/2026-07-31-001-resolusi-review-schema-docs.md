# 2026-07-31-001 — Resolusi Review Schema↔Docs

**Apa:** Menutup kontradiksi antara `pkg/spec`/JSON Schema/`renderers/web` dan
`docs/spec/` hasil review 2026-07-31. Arah resolusi: case-by-case sesuai bukti —
kode yang jalan jadi acuan, docs normatif diimplementasi, fitur aspirational
ditandai **Open** di docs dan ditracking ke plan/todo yang sudah ada.

## Perubahan kode & schema

- **Generator (A1):** `internal/genjsonschema` — map-of-struct kini emit
  `additionalProperties: {$ref}` (`ConfigSpec.keys` → `ConfigKey`); `ConfigKey`
  ditambah ke shared types. `schemas/` di-regenerate.
- **Entity canonical (A2):** balik rename v0.3.0 — `kind: Entity` canonical,
  `kind: Document` deprecated alias. `EntitySpec` struct primary (`DocumentSpec`
  = alias), `ValidateEntitySpec` canonical (`ValidateDocumentSpec` = alias,
  konsolidasi dari dua validator yang tumpang tindih). `registry.go` kirim
  `kind: "Entity"`. Schema kind `Entity` primary.
- **Module/App (B1):** `ModuleSpec` +`vendor`, `datastore`, `config`, `ai_index`
  (`AiIndexDecl`); `Dependency` +`version`. `AppSpec` `publishes`/`consumes`
  `[]string` → `[]AppInterface`/`[]AppConsume` (objek `{service, actions}` /
  `{app, service, actions}`); +`app_renderer`, `theme_ref`, `auth_config_ref`.
- **Policy/Environment (B2):** `pkg/spec/control.go` (baru) — `EnvironmentSpec`
  + `EnvironmentPlane`, `PolicySpec` + `PolicyApproval` (dari
  `04-control-plane.md` §2/§5); schema tak lagi bare `{}`.
- **Entity (B3):** `FieldType` +`attachment` (alias `file`, dinormalisasi di
  `ValidateEntitySpec`); dokumentasikan `spec.auth` (`EntityAuth`) di
  `01-core-basic.md` §1.4.
- **Form render (C1):** bentuk kanonik `render: { mode }` (objek, sesuai
  renderer/TS); `FormRenderDecl` terima juga shorthand skalar; perbaiki
  `internal/ui` validate + fixture (`name:` → `field:`). **Memperbaiki test
  `internal/ui` yang sebelumnya merah** karena fixture `render: {mode}` gagal
  di-unmarshal.
- **Print (C5):** hapus `PrintSpec.Formats []string` (satu format per manifest =
  `output.format`).
- **Report (C4):** TS `ReportParam.name` → `field`, `ReportSpec.groups:
  string[]` → `ReportGroup[]` (+ interface), renderer pakai `g.field`/
  `param.field`; contoh manifest Report diperbarui ke bentuk kode (`field:`
  params, `groups` objek, `totals.fn`).

## Perubahan docs

- `06-page-kinds.md`: Form (`sections` flat, `read_only`, `render` objek),
  Table (`filters` objek, `default_sort` string `-field`), Kanban (`status_field`
  wajib, `columns{status,label}`, `card_template`; tandai zero-config/
  `group_by`/`drag_guard`/`wip_limit`/`card_fields` **Open**), Dashboard/Widget
  (ref-based, konsisten `07-component-kinds.md`), Report (`parameters`/`groups`/
  `export`; tandai `source.filter` **Open**), Page master-detail & custom page
  ditandai **Open**.
- `07-component-kinds.md`: `needs:` ditandai **Open** (belum di `BlockRef`).
- `01-core-basic.md`: §1.4 `spec.auth`.
- `05-field-types.md`: §5.1 `computed` → `formula` (ekspresi Starlark inline,
  bukan `script` ref — sesuai runtime `renderers/jsonbpersist/crud.go`).
- `07-vertical-modules.md`: catatan `publishes`/`consumes` → kini spec-blessed.
- `docs/runtimes/02-forma-resource.md`: `DocumentSpec` → `EntitySpec`.

## Catatan test (pre-existing, bukan akibat perubahan ini)

`examples/Clinic-UI-Showcase` e2e gagal di base code juga (diverifikasi via
`git stash`): (1) fixture `transaction_date: 2026-07-12` kini melebihi
backdate limit 3 hari (hari ini 2026-07-31); (2) `script_ref` di-resolve
relatif ke cwd (`spec/scripts/*.star`) sehingga gagal saat `go test ./...` dari
repo root. Dua-duanya di luar scope review ini.

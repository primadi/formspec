# 2026-08-22-004 — Edit Role: GrantsEditor Semua Page + Form Role per-App

## Apa yang diubah

Layar Edit Role diperbaiki agar admin mudah memahami **page apa yang sedang
disetting** dan **permission apa saja di dalamnya yang perlu di-setting**.

### 1. GrantsEditor menampilkan SEMUA page app (bukan hanya authored pages)

Akar masalah: `GrantsEditor` sebelumnya membangun tree hanya dari
`bundle.pages` (authored pages). Cafe app hanya punya 1 authored page
(`access-management`); halaman CRUD entity (orders, payments, menu-items, dll)
di-derive frontend dari `bundle.entities` dan tidak ada di `bundle.pages` —
jadi GrantsEditor hanya menampilkan 1 checkbox.

Sekarang GrantsEditor membangun tree dari 3 sumber:

- **Authored pages** (`bundle.pages`) — block/tab actions (existing).
- **Derived entity CRUD pages** (`{entity}-page`, dari `bundle.entities`) —
  standard CRUD actions (list/view/create/update/delete) + custom actions
  (dari `entity.actions`).
- **Navigation-only kinds** (`{kind}:{name}`, dari `bundle.dashboards/
reports/wizards/kanbans/timelines/prints`) — action "view".

UX diperjelas: tiap page menampilkan icon per kind, judul human-readable,
badge module, dan route; tiap action menampilkan label human-readable
("Lihat daftar", "Buat", "Ubah", dst) + permission string termaterialisasi
inline; ada search/filter dan preview permission termaterialisasi di bawah.

Perbaikan tambahan:

- Entity pages menampilkan **standard CRUD actions** (list/view/create/update/
  delete) + custom actions — sebelumnya hanya custom actions (entity.actions
  di bundle hanya berisi actions yang dideklarasikan).
- **Table block** (di authored page/tab) kini menurunkan standard CRUD lengkap
  (list/view/create/update/delete) + row/bulk actions — sebelumnya hanya
  list/view + row_actions yang dideklarasikan. Ini membuat tab Users di Access
  Management menampilkan New User (create) dan Edit User (update), bukan hanya
  Lihat daftar/detail. Berlaku di `blockActions` (frontend) dan
  `blockFootprint` (backend materializer).
- `user-edit` form diberi `mode: edit` agar derived page-nya memetakan ke
  permission `update` (bukan `create`).
- `dashboardPermissions` di-dedupe (widget yang mereferensikan entity sama
  tidak menghasilkan permission duplikat).
- **Urutan grants mengikuti urutan menu app** — `buildPageModels` mengurutkan
  page berdasarkan `bundle.menu` (depth-first leaf routes); page yang tidak ada
  di menu tetap di akhir sesuai urutan build.

### 2. Materializer mendukung derived entity pages + navigation kinds

`internal/auth/materialize.go`:

- `resolveFootprint()` — resolve grant page ke 3 jenis: authored page
  (`uiReg.Pages`), navigation kind (`{kind}:{name}`), derived entity page
  (`{entity}-page`).
- `entityFootprint()` — derive standard CRUD + lifecycle + custom actions dari
  entity registry (mirror `registerStandardPermissions`).
- `navigationFootprint()` — Report → `required_permission`/`{module}.{entity}.
list`; Wizard/Kanban/Timeline/Print → `{module}.{entity}.view`; Dashboard →
  aggregate view permissions widget entities.
- `findEntityModule()` — resolve nama entity ke module (error ambiguous jika
  nama entity ada di >1 module).

### 3. Form Role per-App

- `internal/auth/module/master/role/forms/form.yaml` — hapus field `app` &
  `module` (security per-App, tidak perlu ditanyakan).
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — saat submit entity
  `formspec.core.role`, auto-fill `app` dari `bundle.app.name` (konteks app
  saat ini) jika kosong.

## Kenapa diubah

Admin perlu melihat seluruh permukaan app (semua page + action) saat menyusun
role, bukan hanya authored pages. Security per-App membuat field App/Module
redundan — scope diambil otomatis dari konteks app.

## File yang terkena dampak

- `internal/auth/materialize.go` — resolveFootprint/entityFootprint/
  navigationFootprint/findEntityModule
- `internal/auth/materialize_test.go` — test derived entity page, custom
  action, dashboard/report/kanban navigation, unknown kind; setup test pakai
  `RegisterArtifactManifest` (non-internal)
- `renderers/react-shadcn/src/widgets/GrantsEditor.tsx` — rewrite: semua page,
  header jelas, label + permission inline, search, preview; entity pages
  menampilkan standard CRUD + custom actions; dashboard permission dedup
- `internal/auth/module/master/user/forms/edit.yaml` — tambah `mode: edit`
- `internal/auth/module/master/role/forms/form.yaml` — hapus app/module
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — auto-set app

## Referensi

- Plan: `docs/plan/access-management-ui.md`, session plan (edit role)
- Spec: `docs/spec/frontend/04-spec-resolution-api.md` §4 (grant-per-halaman →
  materialized permission), `docs/spec/platform/02-workspace-app-module.md` §3
  (auth per-App)
- Todo: 6.3.1, 6.3.2, 5.12.5 (role + materialisasi)

## Verifikasi

- `go build ./...` hijau
- `go test ./...` hijau (733 pass, termasuk test materializer baru)
- `cd renderers/react-shadcn && npx tsc --noEmit` hijau
- `cd renderers/react-shadcn && npx vitest run` hijau (100 pass)

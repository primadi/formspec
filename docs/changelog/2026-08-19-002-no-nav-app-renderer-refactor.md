# 2026-08-19-002 — App Renderer Archetypes: `no-nav` + `access` + `section:` + TopNavShell + persist_backend

## Apa yang diubah

Refactor pasca-001: memisahkan dua sumbu yang tadinya tergabung di
`landing-page` — **chrome** (`app_renderer`) dan **auth** (`access`), plus
implementasi `topnav` dan seam backend persist.

**Spec & schema:**

- `app_renderer` = archetype chrome (`sidebar-nav`/`topnav`/`no-nav`); hapus
  `landing-page` (bukan nama renderer — "landing/marketing" hanyalah kombinasi
  `no-nav` + `public`).
- `AppSpec.Access` (enum `private`/`public`, default private) — sumbu auth
  terpisah, pemicu bundle anonim + data seam publik + boleh root `/`.
- `AppSpec.StackFamily` (default `react-shadcn`) + `AppSpec.PersistBackend`
  (default `jsonb-persist`; nama tak ter-install / tak implement kontrak
  `formspec/storage.entity-persist` → **ERROR** di apply/check).
- `PageBlock.Landing` → `PageBlock.Section` (`SectionBlock`/`SectionCTA`/
  `SectionItem`) — blok presentasi generik, bukan milik satu archetype.
- Regenerasi schema (`make generate-schema`): App schema punya `access`/
  `stack_family`/`persist_backend`/`no-nav`; `SectionBlock` di $defs.

**Server:**

- `internal/app/resolve.go`: default `access`/`stack_family`/`persist_backend`;
  root `/` hanya untuk App `access: public`.
- Pemicu "publik" pindah dari `AppRenderer == "landing-page"` → `Access ==
"public"` di `router.go` (`publicEntities`) dan `meta.go` (bundle
  `alwaysVisible`).
- Meta bundle + `/_meta/apps` ekspos `access`/`stack_family`/`persist_backend`.

**Frontend (renderers/react-shadcn):**

- `LandingShell` → `NoNavShell`; `LandingBlocks` → `components/sections/
SectionBlocks.tsx` (`SectionBlockRenderer`).
- **`TopNavShell` (baru)** — chrome penuh nav atas (dropdown group +
  breadcrumb + mobile drawer); menu di-resolve via hook bersama
  `useResolvedMenu` (dipakai juga oleh `Sidebar`).
- `App.tsx`: registry `APP_SHELLS`; collapse surface `"landing"` → `"app"` +
  flag `public`; boot anonim saat `access: public`.

**Contoh & docs:**

- `examples/storefront/`: `storefront.yaml` → `app_renderer: no-nav` +
  `access: public`; `backoffice.yaml` → `access: private`; `home.yaml` →
  `landing:`→`section:`.
- `examples/arisan/` → `app_renderer: topnav` (demo chrome ketiga).
- Docs spec (`01-visual-hierarchy`, `05-app-kinds` §1/§4, `06-page-kinds`
  §1/§10, `README`, `platform/02`), renderer docs, glossary, ai_skills
  diperbarui ke model archetype + access.
- **Hapus `examples/{arisan,cafe,crc-management}/schemas/`** (snapshot stale)
  - repoint `.vscode/settings.json` contoh ke `../../schemas/` (satu source of
    truth — schema lokal `schemas/`).

## Kenapa diubah

`landing-page` menggabungkan chrome (no-nav) & auth (public) yang sebenarnya
ortogonal — sidenav/topnav/no-nav app bisa public ATAU private. Nama "landing"
berkonotasi marketing padahal maksudnya "tanpa navigasi standar". Produk
belum released, jadi rename masih murah. Sekaligus menyiapkan seam backend
persist (`persist_backend`) dan shell (`stack_family`) untuk masa depan.

## File terdampak

- `pkg/spec/{frontend.go,resources.go}` · `internal/genjsonschema/generator.go`
- `internal/{app/resolve.go, api/router.go, api/meta.go, api/meta_test.go,
api/descriptor.go, ui/meta.go, manifest/loader.go}`
- `renderers/react-shadcn/src/{App.tsx, types/manifest.ts, stores/meta.ts,
hooks/useResolvedMenu.ts, shell/{Sidebar.tsx, NoNavShell.tsx, TopNavShell.tsx,
index.ts}, components/sections/SectionBlocks.tsx, kinds/{page/PageRenderer.tsx,
listing/ListingRenderer.tsx}}`
- `examples/{storefront, arisan, cafe, crc-management}/**`
- `docs/spec/frontend/{01,05,06,README}.md` · `docs/spec/platform/02-*.md` ·
  `docs/guides/authoring-a-shell.md` · `docs/reference/glossary.md` ·
  `docs/renderers/shadcn-shell/03-*.md` · `docs/kind/ui/Listing.md` ·
  `ai_skills/{formspec-kinds,formspec-spec-structure}/SKILL.md`
- `schemas/**` (regen) · `docs/plan/landing-page.md` · `docs/plan/todo.md`

## Referensi

- Plan: `docs/plan/landing-page.md` · Todo: 5.1.1–5.1.3, 5.1a.1–5.1a.3
- Spec: `docs/spec/frontend/01-visual-hierarchy.md` §1, `05-app-kinds.md` §1–§4

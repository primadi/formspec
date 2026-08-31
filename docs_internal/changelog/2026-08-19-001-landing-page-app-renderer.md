# 2026-08-19-001 — Landing Page App Renderer (Model A)

## Apa yang diubah

Mengimplementasikan todo 5.1.3 — App renderer `landing-page` (Model A: App
publik terpisah, chrome + auth per-App). Landing page kini didukung end-to-end
di spec, server, dan renderer shadcn-shell.

**Spec & schema:**

- `PageBlock.Landing` (blok declarative `hero`/`feature_grid`/`card`/
  `carousel`/`cta`) + `ValidatePageSpec` (blocks/tabs mutual exclusion, closed
  set) di `pkg/spec/frontend.go`.
- `ValidateAppSpec` + `AppRendererNames` (closed set sidebar-nav/topnav/
  landing-page) + longgarkan pattern `root_url` ke `^(/|/app(/.*)?)$` di
  `pkg/spec/resources.go` (landing App boleh memegang root `/`).
- Kind `Listing` terdaftar (`pkg/spec/spec.go` + KnownKinds + UI registry +
  Bundle) — menutup 5.13.5 di sisi kontrak.

**Server:**

- `internal/app/resolve.go`: default `app_renderer` + validasi + izinkan root
  `/` untuk App landing.
- Meta API: `AppSummary.app_renderer` diekspos; bundle App landing dibangun
  `alwaysVisible` (surface publik anonim) di `internal/api/meta.go`.
- Data seam publik: entity di module App landing mendapat anonim `list`/`find`/
  `create` di `/_ui/entity/` (`RouteDescriptor.Public`, `router.go`) —
  update/delete tetap permission-gated.

**Frontend (renderers/react-shadcn):**

- `shell/LandingShell.tsx` (baru): chrome minimal (brand bar + nav publik +
  footer), tanpa sidebar/auth.
- `App.tsx`: surface `landing` di root `/{ws}/`, pemilihan shell dari
  `app_renderer`, boot anonim, login `returnTo` + same-origin guard.
- `components/landing/LandingBlocks.tsx` (baru): Hero/FeatureGrid/Card/
  Carousel/CTA declarative.
- `kinds/listing/ListingRenderer.tsx` (baru): katalog read-only (search +
  filter, tanpa row/bulk/create) — menutup 5.13.5.
- `kinds/page/PageRenderer.tsx` merender blok `landing`.

**Contoh:** `examples/storefront/` — workspace dua App (`storefront` landing
root `/` + `backoffice` `/app/backoffice`) satu module `catalog`; divalidasi
`0 problem` (schema lokal).

## Kenapa diubah

Halaman publik (landing/marketing/pendaftaran) butuh surface tanpa chrome dan
tanpa auth yang sebelumnya tidak didukung — satu-satunya App renderer adalah
`sidebar-nav`. Model A dipilih (diskusi user) karena paling aman dan konsisten
dengan spec `01-visual-hierarchy.md` §1 (chrome ditentukan di level App, bukan
per-Page): publik & admin = dua App, renderer & komponen sama.

## File terdampak

- `pkg/spec/{frontend.go,resources.go,spec.go}`
- `internal/manifest/loader.go` · `internal/ui/{registry.go,meta.go}` ·
  `internal/api/{meta.go,router.go,descriptor.go}` · `internal/app/resolve.go`
  · `internal/genjsonschema/generator.go`
- `renderers/react-shadcn/src/{App.tsx,shell/LandingShell.tsx,shell/router.tsx,
shell/index.ts,types/manifest.ts,stores/meta.ts,
components/landing/LandingBlocks.tsx,kinds/page/PageRenderer.tsx,
kinds/listing/ListingRenderer.tsx}`
- `examples/storefront/**` · `docs/plan/landing-page.md` ·
  `docs/renderers/shadcn-shell/03-kind-renderers.md` · `docs/kind/ui/Listing.md`

## Referensi

- Plan: `docs/plan/landing-page.md` · Todo: 5.1.3, 5.13.5
- Spec: `docs/spec/frontend/01-visual-hierarchy.md` §1, `05-app-kinds.md` §4,
  `06-page-kinds.md` §1/§10

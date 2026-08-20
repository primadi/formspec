# Plan: App Renderer Archetypes — `sidebar-nav` / `topnav` / `no-nav` (+ `access`)

**Status:** Implemented · **Todo:** 5.1.1–5.1.3, 5.1a.1–5.1a.3 · **Changelog:** `2026-08-19-001` (awal) · `2026-08-19-002` (refactor archetype)

> **Superseded naming:** dokumen ini awalnya berjudul "Landing Page — App
> Renderer `landing-page`". Refactor 002 mengganti `landing-page` → `no-nav`
> dan memisahkan sumbu **chrome** (`app_renderer`) dari sumbu **auth**
> (`access`). "Landing/marketing" hanyalah satu kombinasi (`no-nav` + `public`),
> bukan nama renderer.

## TL;DR

App renderer `app_renderer` menentukan **bentuk chrome** (`sidebar-nav` /
`topnav` / `no-nav`) — bukan soal public/private. **Auth adalah sumbu
terpisah** (`access`: `private` default / `public`). Renderer & komponen
**sama** untuk semua chrome — bedanya hanya wrapper shell + pola auth.
**Tidak ada per-Page access/presentation** — spec `01-visual-hierarchy.md` §1
tetap berlaku ("keputusan chrome selesai di App renderer; tidak ada
`route_mode` di level Page").

## Keputusan desain

1. **Model A (App-level)** — bukan Model C (per-Page access). Chrome per-App
   lewat `app_renderer`; auth per-App lewat `access`. Dua App dalam satu
   workspace = pola alami publik/marketing + produk (satu domain).
2. **Renderer & komponen SAMA** untuk semua shell; `NoNavShell`/`TopNavShell`
   hanya chrome.
3. **Konten presentasi** = perluas closed block set `PageBlock` dengan
   primitif declarative: `section: { type: hero|feature_grid|card|carousel|cta }`
   (rename dari `landing:`). Generik — reusable di App mana pun, bukan milik
   satu archetype. Bukan template baku, bukan asset JS custom.
4. **Anonim create in scope** (pendaftaran publik) dengan mitigasi: read
   (list/find) + create publik untuk entity di module App `access: public`;
   update/delete tetap permission-gated (ops admin di App private).
5. **Login = Shell-level** (bukan kind: Page), public by construction, per
   workspace `/{ws}/login`, redirect `returnTo` + same-origin guard.

## Perubahan

### Contract & spec

- `pkg/spec/resources.go`: `ValidateAppSpec` + `AppRendererNames` (closed set:
  sidebar-nav, topnav, no-nav) + `DefaultAppRenderer`; `Access` enum
  (private|public, default private); `StackFamily` (default react-shadcn);
  `PersistBackend` (default jsonb-persist, nama tak ter-install → ERROR);
  pattern `root_url` dilonggarkan jadi `^(/|/app(/.*)?)$`.
- `pkg/spec/frontend.go`: `PageBlock.Section *SectionBlock` +
  `SectionBlock`/`SectionCTA`/`SectionItem` + `SectionBlockTypes` +
  `ValidatePageSpec` (blocks/tabs mutually exclusive, section type closed set).
- `pkg/spec/spec.go`: `KindListing`.
- `internal/manifest/loader.go`: validasi App + Page; `Listing` masuk KnownKinds.
- `internal/genjsonschema`: shared defs `SectionBlock`/`SectionCTA`/`SectionItem`.

### Server

- `internal/app/resolve.go`: default `app_renderer`/`access`/`stack_family`/
  `persist_backend`, validasi, root `/` untuk App `access: public`.
- `internal/ui/meta.go` + `internal/api/meta.go`: `AppSummary.app_renderer` +
  `access` + `stack_family` + `persist_backend`; bundle App `access: public`
  dibangun `alwaysVisible` (surface anonim); `Listings` di Bundle.
- `internal/api/router.go` + `descriptor.go`: `RouteDescriptor.Public` —
  entity di module App `access: public` dapat anonim list/find/create di
  `/_ui/entity/`.
- Data seam publik + mitigasi (rate limit/honeypot disiapkan; enforcement
  field `exclude: [public_api]` = concern security audit Fase 6).

### Frontend

- `src/shell/NoNavShell.tsx` (rename `LandingShell`): brand bar + nav opsional
  - footer + Outlet. `src/shell/TopNavShell.tsx` (baru): nav atas + dropdown +
    breadcrumb + mobile drawer. Hook bersama `src/hooks/useResolvedMenu.ts`.
- `src/App.tsx`: registry `APP_SHELLS` (`sidebar-nav`→AppShell,
  `topnav`→TopNavShell, `no-nav`→NoNavShell); surface collapse `landing`→
  `app` + flag `public`; boot anonim saat `access: public`; login `returnTo`
  same-origin.
- `src/components/sections/SectionBlocks.tsx` (rename `LandingBlocks`):
  Hero/FeatureGrid/Card/Carousel (no-dependency)/CTA.
- `src/kinds/listing/ListingRenderer.tsx` (baru): katalog read-only.
- `src/kinds/page/PageRenderer.tsx`: render blok `section`.
- `src/types/manifest.ts` + `src/stores/meta.ts` + `src/shell/router.tsx`:
  tipe/route Listing + section.

### Contoh & docs

- `examples/storefront/`: dua App (`storefront` no-nav+public `/` +
  `backoffice` sidebar-nav+private `/app/backoffice`) satu module `catalog`.
- `examples/arisan/`: `app_renderer: topnav` (demo chrome ketiga).
- `docs/renderers/shadcn-shell/03-kind-renderers.md` §2, `docs/kind/ui/Listing.md`.

## Deferred / catatan

- Field-level `exclude: [public_api]` server-side enforcement & rate-limit
  tulis anonim: concern Fase 6 (auth) — siap contract, belum enforcement penuh.
- `topnav` sudah diimplementasikan (`TopNavShell`); `stack_family` lain
  (flutter/react-mui) + validasi renderer penuh = 5.16.
- Nav `NoNavShell`: derive dari App menu (leaf yang punya route).

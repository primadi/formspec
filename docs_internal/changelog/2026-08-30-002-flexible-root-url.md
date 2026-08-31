# 2026-08-30 — 002 — `root_url` fleksibel di dalam workspace

## Apa

`App.spec.root_url` tidak lagi dipaksa prefix `/app/`. Sekarang bebas di
dalam workspace: `/` (workspace root), `/barbershop`, `/apps/barbershop`,
`/app/kafe`, dll. Server me-mount SPA shell **dinamis** di setiap `root_url`
App (sebelumnya hard-coded hanya `/{ws}/_admin` dan `/{ws}/app`).

Motivasi: single-app workspace tidak harus memakai URL verbose
`/{ws}/app/barbershop` — cukup `/{ws}` atau `/{ws}/barbershop`.

## Aturan validasi baru (`internal/app/resolve.go`)

- Tetap required + unik per workspace; trailing `/` dinormalisasi.
- Reserved first segment ditolak (bentrok surface tetap engine): `_ui`,
  `api`, `_admin`, `assets`, `health`, `login`, `register`, `_ws`, `print`.
  `app` tetap valid (konvensi lama).
- Overlap prefix diizinkan — resolusi longest-prefix menang (pola landing
  page `/` + backoffice `/app/backoffice` tetap berfungsi).

## File terkena dampak

- `internal/app/resolve.go` — validasi baru + `reservedAppSegments` (+ `resolve_test.go` baru)
- `internal/api/router.go` — dynamic SPA mount dari `b.apps` + dedupe (`router_spa_test.go` baru)
- `renderers/react-shadcn/src/App.tsx` — `RootSurface` me-render App apapun (bukan hanya `access: public`) yang menang longest-prefix di catch-all `/:workspace/*`
- `pkg/spec/resources.go` — `@schema` pattern `^(/|/[^/]+(/[^/]+)*)$` → regenerate `schemas/` + `docs/kind/`
- Docs: `docs/spec/platform/02-workspace-app-module.md` §4, `docs/spec/frontend/05-app-kinds.md` §1, `docs/kind/curation/App.md`, `ai_skills/*` + mirror `examples/*/.agents/skills/`

## Referensi

- Plan: `docs/plan/flexible-root-url.md`
- Changelog terkait: `2026-08-19-001-landing-page-app-renderer.md` (longgarkan pertama), `2026-08-30-001-app-version-vendor-optional.md`

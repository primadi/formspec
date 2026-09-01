# 2026-09-01-001 — Render context standard Phase 2: spec.context declaration

## Apa

Phase 2 dari plan `docs_internal/plan/render-context-standard.md`:
deklarasi `spec.context` dengan **closed source set**
`session | entity | api | const | expr`.

- `pkg/spec/frontend.go`: tipe `ContextDecl` (name/source/entity/id/call/
  params/value/expr/fallback), `ContextSourceSet`, `ValidateContextDecls`
  (unique name + closed source + required field per source). Field
  `Context []ContextDecl` di `PageSpec` & `FormSpec`; validasi di
  `ValidatePageSpec`.
- `internal/genjsonschema/generator.go`: `ContextDecl` masuk sharedTypes
  allowlist → schema valid.
- `renderers/react-shadcn/src/hooks/useRenderContext.ts` (baru): resolver
  sequential (agar `expr` bisa refer entry sebelumnya), state loading/error/
  fallback. Source: `session`→user, `const`→value, `expr`→evalFormSpecExpr,
  `entity`→apiGet + permission ceiling `{entity}.view` + id token `{user.x}`.
- `PageRenderer.tsx`: `useRenderContext` di-wire ke PageBlocks; loading gate
  (skeleton) saat context async masih resolve.
- `registry/spec/modules/portal/pages/profile.yaml`: demo `context:` const
  (`greeting`) + expr (`is_admin`).

## Kenapa

Melengkapi kontrak context render: standard slots (`user`, `route`, `fields`)
selalu ada; data tambahan wajib dideklarasikan eksplisit dengan sumber
tertutup — selaras prinsip Closed Primitives & Security by Default
(permission ceiling untuk entity).

## Deferred — `source: api`

Service actions hanya di-route di surface `/api/v1` (deny-by-default,
API-key auth); tidak ada route UI-surface. `api` source butuh task terpisah
(UI-surface service action route) sebelum bisa dipakai context. Saat ini
deklarasi `api` resolve ke fallback.

## Verifikasi

- `go test ./pkg/spec ./internal/genjsonschema` — 43 pass.
- `vitest run` — 166 pass, 0 fail.
- `formspec validate -spec registry/spec -schema schemas` — 13 manifest,
  0 problem.
- Browser: profile page menampilkan "Selamat datang, TestUser" (const +
  user slot tergabung).

## File terdampak

- `pkg/spec/frontend.go`, `pkg/spec/frontend_test.go`
- `internal/genjsonschema/generator.go`
- `renderers/react-shadcn/src/hooks/useRenderContext.ts` (baru)
- `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx`
- `renderers/react-shadcn/src/types/manifest.ts`
- `registry/spec/modules/portal/pages/profile.yaml`
- `schemas/`, `docs/kind/` (regenerated)
- `docs_internal/plan/render-context-standard.md` (status Phase 2 ✅)

## Lanjutan

Phase 3 (plan): reaktivitas (`source: entity` + realtime). Deferred:
`source: api` (butuh UI-surface service route).

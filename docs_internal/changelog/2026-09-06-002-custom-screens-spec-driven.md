# 2026-09-06-002 — Custom screens spec-driven: query param slot + auth action contract + config source

**Plan**: `docs_internal/plan/custom-screens-spec-driven.md` (3 fase, semuanya selesai)

## Apa yang diubah

Tiga perluasan closed-set agar custom screen sederhana (login, landing) bisa
pure-YAML tanpa `asset` JS — prinsip tetap: spec mengontrol WHAT, asset
mengontrol HOW, tidak ada JS di spec.

1. **Query param di standard slot `route`** — `PageRenderer.tsx` (PageBlocks)
   membangun slot `route = { params, query, path }` dari `useParams` +
   `useSearchParams` + `useLocation`, dimasukkan ke base `useRenderContext`.
   Token `{route.query.returnTo}` dll kini tersedia di title, section text,
   `expr`, dan `fallback`.
2. **Auth action contract (Phase C plan auth-screens-spec-driven)** —
   `FormSpec.auth_action` (closed set: `login | register | change_password |
forgot_password | reset_password`), mutually exclusive dengan `entity`;
   validasi `ValidateFormSpec` + wiring loader Form kind. Renderer baru
   `AuthFormRenderer.tsx` men-render form auth pure-YAML dan dispatch submit
   ke `FormspecAuth` (`forgotPassword` ditambahkan); sukses login/register →
   session boot + redirect `returnTo` (same-origin guard). Bundle filter
   Forms (`internal/ui/meta.go`) kini menyertakan form tanpa entity yang
   mendeklarasikan `auth_action`.
3. **Source `config` di `spec.context`** — `ConfigKey.Public` (opt-in);
   endpoint `GET /{ws}/_ui/config/{name}` hanya menyajikan key non-secret +
   `public: true`; `ContextDecl.Config` + source `"config"` di
   `ContextSourceSet`; resolver `useRenderContext.ts` dengan cache
   in-memory per config name; fallback on error.

## Kenapa

Diskusi desain 2026-09-06: apakah spec perlu bisa membaca query string /
session var / app var / JS? Keputusan: TIDAK ada JS di spec; sebaliknya
perluasan kecil di closed set (query param, auth action, config opt-in)
menghabiskan sebagian besar use case custom screen. Mekanisme auth yang
ditolak: `binds`/`needs` (enforcement hanya parse URL entity), route via
`source: api`/Service (auth butuh rate-limit/audit handler existing),
`PageBlock.auth` baru (duplikasi layout engine Form).

## File terdampak

- `pkg/spec/frontend.go` (FormSpec.AuthAction, FormAuthActions,
  ValidateFormSpec, ContextDecl.Config, ContextSourceSet)
- `pkg/spec/resources.go` (ConfigKey.Public)
- `internal/manifest/loader.go` (validasi Form kind)
- `internal/ui/meta.go` (bundle Forms: include auth forms)
- `internal/api/router.go`, `internal/api/config_handler.go` (baru),
  `internal/config/registry.go` (PublicFor), `resource/formspec.go` (wiring)
- `renderers/react-shadcn/src/`: `kinds/page/PageRenderer.tsx` (route slot +
  auth form branch), `kinds/form/AuthFormRenderer.tsx` (baru),
  `hooks/useRenderContext.ts` (resolver config), `lib/formspec-client.ts`
  (forgotPassword + export createAuth), `types/manifest.ts`
- `schemas/` (regenerated), `docs/kind/ui/{Page,Form}.md` (narrative),
  plan `auth-screens-spec-driven.md` Phase C ✅

## Verifikasi

`go build ./...` · `go test` (spec/manifest/ui/api/config) · `tsc --noEmit`
bersih · `vitest` 166 pass · `make generate-schema` (141 shared types).

## Deferred

- Endpoint `POST /{ws}/_ui/auth/logout` (revocation refresh token
  server-side) — logout client-side sudah berfungsi.
- Contoh custom login page pure-YAML di examples/ (dokumentasi lanjutan).

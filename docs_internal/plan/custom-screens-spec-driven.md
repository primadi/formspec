# Plan — Custom Screens Spec-Driven (query param + auth action contract + config source)

**Status**: ✅ Complete (2026-09-06) — changelog `2026-09-06-002`.
**Implementasi**: Phase 1 (`route` slot di `PageRenderer.tsx`), Phase 2
(`FormSpec.AuthAction` + `AuthFormRenderer.tsx` + bundle filter auth forms),
Phase 3 (`ConfigKey.Public` + `GET /{ws}/_ui/config/{name}` + resolver
`config` di `useRenderContext.ts`). Deferred: endpoint logout server-side.

Date: 2026-09-06
Referensi spec: `docs/spec/frontend/06-page-kinds.md` (context, sections), `docs/spec/frontend/07-component-kinds.md`,
`docs/spec/backend/01-core-basic.md` §10 (Config), `docs/guides/authentication.md`
Plan terkait: `auth-screens-spec-driven.md` (Phase C), `render-context-standard.md`

## Latar belakang

Custom screen seperti login hari ini memerlukan `asset` (custom component JS) untuk kebutuhan
non-trivial. Diskusi 2026-09-06 memutuskan: **spec mengontrol WHAT, asset mengontrol HOW** —
tidak ada JS di spec. Sebaliknya, tiga perluasan kecil di closed set membuat custom screen
sederhana bisa pure-YAML:

1. **Query param** — standard slot `route` diperluas (`params`, `query`, `path`) sehingga
   spec bisa membaca `{route.query.returnTo}` dll. Read-only, tanpa permission implication.
2. **Auth action contract** (Phase C dari `auth-screens-spec-driven.md`) — `FormSpec.auth_action`
   (closed set) yang membuat custom login/register/reset page pure-YAML; endpoint `/_ui/auth/*`
   sudah ada, frontend sudah punya `FormspecAuth` wrapper.
3. **Source `config`** — `spec.context` source baru yang membaca key `kind: Config` module,
   opt-in per key (`ConfigKey.public`), non-secret only.

Mekanisme auth yang DITOLAK: extends `binds`/`needs` (enforcement hanya parse URL entity),
route via `source: api`/Service (auth = platform concern, rate-limit/audit sudah di handler),
`PageBlock.auth` baru (duplikasi layout engine Form).

## Steps

### Phase 1 — Query param di standard slot `route` (small)

1. `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx`: standard slot
   `route = { params, query, path }` (dari `useParams` + `useSearchParams`), merge ke base
   `useRenderContext` dan context yang diteruskan ke SectionBlocks (`t()` interpolation).
   → `{route.params.id}`, `{route.query.returnTo}`, `{route.path}` dipakai di title, section
   text, `expr`, `fallback`.
2. Update `docs_internal/plan/render-context-standard.md` + `docs/spec/frontend/06-page-kinds.md`.

### Phase 2 — Auth action contract (large, depends on Phase 1 untuk returnTo)

3. `pkg/spec/frontend.go`: `FormSpec.AuthAction` (closed set:
   `login | register | change_password | forgot_password | reset_password`), mutually
   exclusive dengan `Entity`; validasi di `ValidateFormSpec`. Field mapping konvensional:
   `username, password, current_password, new_password, email, token`.
4. `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`: `onSubmit` dispatch ke
   `FormspecAuth` method yang sesuai (bukan entity CRUD) saat `auth_action` terisi;
   error codes (`USERNAME_TAKEN`, `WEAK_PASSWORD`, `REGISTRATION_CLOSED`, …) → pesan form;
   login sukses → session boot (sudah di `FormspecAuth.login`) + redirect
   `route.query.returnTo` dengan same-origin guard (pola `LoginPage.tsx`).
5. Route guard: Page ber-form `auth_action` boleh public (cek `AuthPage.tsx` / `router.tsx`).
6. Docs: `docs/guides/authentication.md` (custom auth page), plan Phase C selesai.

### Phase 3 — Source `config` di spec.context (medium, paralel dengan Phase 1)

7. `pkg/spec/resources.go`: `ConfigKey.Public bool` (opt-in ekspos ke UI; secret tidak pernah).
8. Backend: `GET /{ws}/_ui/config/{module}` — hanya key non-secret & public
   (filter dari `internal/config/registry.go`).
9. `pkg/spec/frontend.go`: `ContextDecl.Config` (`"module.key"`) + source `"config"` di
   `ContextSourceSet` + validasi.
10. `renderers/react-shadcn/src/hooks/useRenderContext.ts`: resolver `config` (GET
    `../config/{module}`, cache per module, fallback on error) + TS types via `make generate`.

## Verification

1. `go build ./... && go test ./...`
2. `cd renderers/react-shadcn && npx vitest`
3. `make generate && make generate-schema` — tanpa drift
4. Manual `make dev`: token `{route.query.x}`, custom login pure-YAML, config public/secret.

## Effort

Phase 1 small · Phase 2 large · Phase 3 medium. Phase 1 & 3 independen; Phase 2 butuh Phase 1.

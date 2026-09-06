# Plan — Auth Screens Spec-Driven (login, setup, change-password, reset-password, oauth-callback, chrome auth)

Status: ✅ Complete (2026-09-06)
Date: 2026-09-06
Referensi spec: `docs/spec/frontend/05-app-kinds.md`, `docs/spec/frontend/06-page-kinds.md` §13 (asset), `docs/spec/frontend/07-component-kinds.md` §4 (component contract), `docs/guides/authentication.md`

## Latar belakang

Auth screens (login, setup, change-password, reset-password, oauth-callback, chrome auth area) saat ini
hardcoded di `renderers/react-shadcn/src/shell/`. Spec mengontrol auth **behavior** (`App.spec.access`,
`chrome.auth`, `auth_config_ref`, OAuth providers, registration policy, `Page.public`) tapi **tidak**
auth **presentation** — tidak ada cara deklaratif untuk override login/setup/change-password dengan UI
custom.

Keputusan (diskusi 2026-09-06): auth screens spec-driven (Opsi A) — `App.spec.auth` mereferensikan
`kind: Page`; default = **spec default** (embedded YAML di module `formspec.core`), bukan hardcoded.
Konsisten dengan prinsip "Everything is a Resource" + "Convention over Configuration".

## Arsitektur

- `App.spec.auth.{login_page, setup_page, change_password_page, reset_password_page, oauth_callback_page, chrome_auth}`
  — refs ke Page kind (`module/name`) / component ref.
- Default auth pages = Page specs embedded di module `formspec.core`, `mode: custom` + `asset` →
  built-in shell component (LoginScreen, SetupScreen, dll di-refactor jadi asset components).
- Bundle resolve refs (default/override) → `bundle.app.auth`; frontend guard render page yang di-resolve.
- Custom auth page butuh auth actions (`login`/`register`/`logout`/`change-password`/`reset-password`).

## Steps

### Phase A — Spec & Bundle (backend, medium)

1. `pkg/spec/resources.go`: struct `AppAuth` + field `Auth *AppAuth` di `AppSpec` (dekat `AuthConfigRef`).
   Field: `LoginPage, SetupPage, ChangePasswordPage, ResetPasswordPage, OAuthCallbackPage string` +
   `ChromeAuth string` (component ref). Tambah `// @schema` comments → `make generate-schema`.
2. Validasi: `ValidateAppSpec` — refs format `module/name`, `chrome_auth` component ref valid.
   Resolve-time: ref harus ada di ui registry Pages.
3. `internal/ui/meta.go`: `AppContext` tambah `Auth *spec.AppAuth`; `AppSummary` tambah `Auth *AuthConfig`
   (resolved: tiap slot = override App ATAU default `formspec.core/<slot>`); `BuildBundle` resolve + validasi.
4. `internal/api/meta.go` `resolveAppContext`: isi `Auth: resolved.Spec.Auth`.
5. `renderers/react-shadcn/src/types/manifest.ts`: `AppSummary.auth` + `AuthConfig` interface (via `make generate`).

### Phase B — Default auth page specs (embedded, medium)

6. Direktori baru `internal/auth/ui-module/` (pola `internal/auth/core.go` `//go:embed module` +
   `ui.Registry.LoadEmbedded`): `login.yaml`, `setup.yaml`, `change-password.yaml`, `reset-password.yaml`,
   `oauth-callback.yaml` — `kind: Page`, module `formspec.core`, `mode: custom`, `asset: formspec-core/auth/<slot>`.
7. Wire: load embedded UI module saat boot (dekat `RegisterCoreEntities` / `resource/formspec.go`).

### Phase C — Auth actions untuk Page (large, open design) ✅ 2026-09-06

8. Mekanisme aksi auth yang bisa di-bind ke form Page custom (`login`/`register`/`logout`/
   `change-password`/`reset-password`). OPEN: (a) action type baru di PageSpec, (b) via `binds`/`needs`
   component contract, (c) service callable. Backend: expose endpoint yang sudah ada (`/_ui/auth/*`)
   sebagai action contract; frontend: form action handler.
   **SELESAI** via plan `custom-screens-spec-driven.md` Phase 2 — keputusan: (a) varian Form —
   `FormSpec.auth_action` (closed set: `login | register | change_password | forgot_password |
reset_password`), mutually exclusive dengan `entity`; renderer dispatch ke `FormspecAuth`
   (`AuthFormRenderer.tsx`). Ditolak: (b) `binds`/`needs` (enforcement hanya parse URL entity;
   auth bukan footprint) dan (c) service callable (auth = platform concern, rate-limit/audit
   sudah di handler).
9. Default pages TIDAK butuh ini (built-in component handle auth internal) — hanya custom pages.

### Phase D — Frontend guard & renderer (large)

10. `App.tsx`: auth guard baca `bundle.app.auth.login_page` → redirect ke route page tsb (bukan hardcoded
    login); setup guard baca `setup_page`; route login/setup/change-password/reset-password/oauth-callback
    render Page yang di-resolve via PageRenderer.
11. Refactor `shell/*.tsx` jadi built-in asset components (renderer untuk default specs): `LoginScreen.tsx`,
    `SetupScreen.tsx`, `ChangePasswordDialog.tsx` (jadi page), `ResetPasswordScreen.tsx`, `OAuthCallback.tsx`.
    Kontrak mount asset (07-component-kinds §4).
12. `AuthArea.tsx`: overridable via `chrome_auth` component ref; default tetap dari `chrome.auth`
    (links/button/none).

### Phase E — Docs

13. SEKARANG (independen): fix nav docs-site (`docs-site/.vitepress/config.mts` guides array —
    authentication.md missing) + backend layering di `docs/architecture/08-repo-structure.md`.
14. SETELAH refactor: restructure `docs/guides/authentication.md` (setup + app access + auth narrative,
    forward param), cross-link `05-app-kinds.md` + `03-kind-renderers.md`, update `docs/guides/README.md`.

## Effort

- A (spec+bundle): medium · B (default specs): medium · C (auth actions): large (open design) ·
  D (frontend): large · E (docs): medium

## Dependensi

```
A(1-5) → B(6-7) → C(8-9) & D(10-12) [parallel, butuh A+B] → E(13 sekarang, 14 setelah)
```

## Verification

1. `go build ./...` + `go test ./...` (backend, termasuk ui/meta_test, api/meta_test)
2. `cd renderers/react-shadcn && vitest` (frontend)
3. Manual: App tanpa `auth` → default specs render; App dengan override → custom page render;
   `chrome_auth` override → AuthArea custom
4. `make generate` (TS types + schema) — tidak ada drift
5. `cd docs-site && npm run build` (nav)

## Open decisions

- C8: mekanisme auth actions (action type vs binds/needs vs service)
- D11: kontrak asset untuk built-in auth components (props: workspace, app, onLogin, returnTo)
- Default page route: `/_auth/<slot>` vs reuse existing routes (`/_admin/login`, `/_admin/setup`)

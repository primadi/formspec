# Changelog — Auth Screens Spec-Driven

Tanggal: 2026-09-06
Referensi plan: `docs_internal/plan/auth-screens-spec-driven.md`

## Apa yang diubah

Auth screens kini spec-driven: `App.spec.auth` — refs ke `kind: Page` untuk login, setup,
change-password, reset-password, oauth-callback, dan chrome auth area. Default = spec default
(embedded YAML di module `formspec.core`), bukan hardcoded React screens.

## Kenapa

Auth screens (login/setup/change-password/dll) sebelumnya hardcoded di
`renderers/react-shadcn/src/shell/`. Spec mengontrol auth behavior tapi tidak presentation —
developer tidak bisa override dengan UI custom. Keputusan diskusi 2026-09-06: spec-driven (Opsi A),
konsisten dengan "Everything is a Resource".

## File yang terkena dampak

**Backend:**

- `pkg/spec/resources.go` — struct `AppAuth` + field `Auth` di `AppSpec` + validasi ref `module/name` (`validatePageRef`)
- `internal/ui/meta.go` — `AppContext.Auth`, `AppSummary.Auth`, `AuthConfig`, `resolveAuth` (default `formspec.core/<slot>`), `allows` kini juga pass `formspec.core`
- `internal/api/meta.go` — `resolveAppContext` isi `Auth`
- `internal/genjsonschema/generator.go` — `AppAuth` masuk allowlist `sharedTypes`
- `internal/auth/module/pages/{login,setup,change-password,reset-password,oauth-callback}.yaml` — default auth page specs (baru, `mode: custom` + `asset: formspec-core/auth/<slot>`)

**Frontend:**

- `src/types/manifest.ts` — `AppSummary.auth` + `AuthConfig`
- `src/shell/authAssets.ts` — registry built-in auth assets (baru)
- `src/shell/AuthPage.tsx` — resolver slot → built-in component / PageRenderer override (baru)
- `src/shell/LoginPage.tsx` — LoginPage dipindah dari App.tsx (baru)
- `src/shell/ChangePasswordPage.tsx` — versi page dari ChangePasswordDialog (baru; dialog tetap ada)
- `src/App.tsx` — routes auth pakai `AuthPage`; hapus LoginPage lokal
- `src/shell/UserMenu.tsx` — Change Password → navigasi ke page (bukan dialog)
- `src/shell/AuthArea.tsx` — overridable via `chrome_auth` (AssetRenderer)
- `src/kinds/page/PageRenderer.tsx` — CustomPage resolve built-in auth assets
- `src/lib/formspec-client.ts` — namespace `formspec.auth` (login/register/logout/changePassword/resetPassword) untuk custom auth pages
- `src/shell/AssetRenderer.tsx` — pass `workspace` ke formspec client
- `src/shell/LoginScreen.tsx` — fix duplikat deklarasi `email` (pre-existing)

**Docs:**

- `docs-site/.vitepress/config.mts` — authentication.md masuk nav guides (sebelumnya missing)
- `docs/architecture/08-repo-structure.md` — §3.1 layering auth (domain vs transport + system screens)

## Status

✅ Complete (Phase A–E). Test: `go test ./...` hijau, vitest 166 pass, `tsc --noEmit` bersih,
docs-site build OK, schema `AppAuth` ter-generate.

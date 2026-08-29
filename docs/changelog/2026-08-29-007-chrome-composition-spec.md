# 2026-08-29-007 — Chrome Composition Spec (`App.spec.chrome`)

## Apa yang diubah

`no-nav` kini benar-benar tanpa navigasi: `NoNavShell` tidak lagi hardcode
Sign in/Sign up, nav link, maupun footer — semuanya dikontrol sub-spec baru
`App.spec.chrome` yang ortogonal terhadap `app_renderer` dan `access`
(`docs/spec/frontend/05-app-kinds.md` §5). Elemen: `brand`, `nav`, `auth`,
`footer`, `breadcrumbs`, `theme_switcher` — semua default `auto` (default
archetype). Default di-resolve di backend meta (`internal/ui.resolveChrome`)
dan dikirim sebagai `bundle.app.chrome` — renderer membaca nilai final.

- `pkg/spec` — struct `AppChrome` + field `Chrome` + validasi enum di
  `ValidateAppSpec`; schemas regenerated.
- `internal/ui` — `ChromeConfig` resolved + `resolveChrome()` (matriks
  default: no-nav → nav=none, auth=none; sidebar/topnav → nav=menu,
  auth=links).
- `renderers/react-shadcn` — komponen bersama `AuthArea` (links/button/none ×
  anonim/signed-in); `NoNavShell`, `AppShell`, `TopNavShell` membaca chrome
  (auth controls, breadcrumbs, theme_switcher kini override-able).
- `registry/spec/apps/registry.yaml` — `chrome: {nav: menu, auth: links}`
  (perilaku portal tidak berubah, kini eksplisit di manifest).
- Docs: `05-app-kinds.md` §4 rewrite + §5 baru (renumber §5→§6, §6→§7 beserta
  cross-ref), `03-kind-renderers.md`, glossary.

## Kenapa

`no-nav` sebelumnya menyiratkan auth controls otomatis — App landing publik
ikut menampilkan Sign in/Sign up yang tidak diminta manifest. Kini `no-nav` =
konten murni; skenario katalog-publik-dengan-login dinyatakan eksplisit.

## File terdampak

`pkg/spec/resources.go`, `internal/ui/meta.go`, `internal/api/meta.go`,
`renderers/react-shadcn/src/shell/{AuthArea,NoNavShell,AppShell,TopNavShell}.tsx`,
`renderers/react-shadcn/src/types/manifest.ts`, `schemas/dist/latest/`,
`registry/spec/apps/registry.yaml`, `docs/spec/frontend/05-app-kinds.md`,
`docs/renderers/shadcn-shell/03-kind-renderers.md`,
`docs/reference/glossary.md`.

## Referensi

- Plan: `docs/plan/chrome-composition-spec.md` · todo 14.b

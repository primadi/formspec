# 2026-08-31-005 — Theme switcher + theme binding (theme_ref) untuk registry portal

## Apa

- `NoNavShell` kini merender `ThemeSwitcher` (light/dark/system + manifest
  themes) saat `chrome.theme_switcher: show` — sebelumnya hanya SideNav/TopNav.
- **Theme binding (§6) di-wire end-to-end**: `App.spec.theme_ref` → bundle
  (`app.theme`) → auto-apply di renderer selama user belum memilih tema sendiri
  (flag `themeTouched` di prefs store; toggle mode tidak menghitung).
- Registry portal opt-in: `theme_switcher: show` + `theme_ref: registry-theme`
  (Theme kind baru, token indigo + radius lembut) — tema default portal lebih
  menarik, mode tetap default `system`.

## Kenapa

Portal registry tampil tanpa cara ganti tema dan tidak bisa menetapkan tema
default per-App meski `theme_ref` sudah ada di `pkg/spec`. Lihat
`docs_internal/plan/registry-theme-switcher.md`.

## File terdampak

`registry/spec/apps/registry.yaml`, `registry/spec/modules/portal/themes/registry-theme.yaml` (baru),
`renderers/react-shadcn/src/shell/NoNavShell.tsx`, `renderers/react-shadcn/src/shell/NoNavShell.test.tsx`,
`renderers/react-shadcn/src/App.tsx`, `renderers/react-shadcn/src/stores/prefs.ts`,
`renderers/react-shadcn/src/types/manifest.ts`, `internal/ui/meta.go`, `internal/api/meta.go`.

## Verifikasi

`go test ./internal/ui ./internal/api` (143 ok) · `vitest NoNavShell` (6 ok) ·
`tsc --noEmit` clean · `formspec validate -spec registry/spec` (theme OK; fail
lain pre-existing schema drift).

# 2026-09-06-005 — Fix blank login screen (AuthPage builtin lookup + unstable zustand selectors)

## Apa

Dua perbaikan frontend pada `renderers/react-shadcn/src/shell/`:

1. **`AuthPage.tsx` — builtin auth lookup key mismatch (root cause blank screen)**
   `DEFAULT_AUTH_REFS` memakai format module/name (`formspec.core/login`) sedangkan
   `BUILTIN_AUTH_ASSETS` di-key oleh asset ref (`formspec-core/auth/login`).
   Lookup `BUILTIN_AUTH_ASSETS[DEFAULT_AUTH_REFS[slot]]` selalu `undefined` →
   `AuthPage` return `null` → layar blank tanpa error untuk SEMUA default auth
   screen (login/register/setup/reset/change-password/oauth-callback).
   Diperbaiki dengan map jembatan `SLOT_BUILTIN_REFS`.

2. **Unstable zustand selectors — React error #185 (max update depth)**
   `?? []` di dalam selector mengembalikan array baru setiap `getSnapshot` →
   `useSyncExternalStore` infinite loop → crash blank saat `bundle` masih `null`
   (kondisi normal di halaman login). Diperbaiki dengan memindahkan fallback ke
   luar selector (konvensi yang sudah ada di `LinkedAccountsDialog.tsx`):
   - `shell/LoginScreen.tsx` (oauth_providers — memicu crash saat reload)
   - `kinds/dashboard/DashboardRenderer.tsx` (widgets)
   - `kinds/form/FormRenderer.tsx` (forms)
   - `kinds/page/DetailPage.tsx` (entities)

## Kenapa

Ditemukan saat menjalankan `examples/Clinic-UI-Showcase`: akses
`/default/klinik` → redirect ke `/default/klinik/login?returnTo=...` → blank
screen. Regression dari commit `27266f9` (auth screens spec-driven) — bug
duplicate key format antara dua map.

## File yang terkena dampak

- `renderers/react-shadcn/src/shell/AuthPage.tsx`
- `renderers/react-shadcn/src/shell/LoginScreen.tsx`
- `renderers/react-shadcn/src/kinds/dashboard/DashboardRenderer.tsx`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`
- `renderers/react-shadcn/src/kinds/page/DetailPage.tsx`

## Verifikasi

- Rebuild SPA (`npx vite build`), reload login page → form login tampil.
- Login admin → redirect via `returnTo` → `/default/klinik/dashboard/clinic-dashboard`
  render penuh (sidebar, dashboard widget, breadcrumbs).

## Referensi

- Changelog `2026-09-06-001` / `2026-09-06-002` (fitur yang memperkenalkan regresi)

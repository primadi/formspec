# 2026-10-02-005 — Change Password di permukaan App (404 → route terdaftar)

## Apa yang diubah

- `renderers/react-shadcn/src/App.tsx`: `SurfaceShell` kini mendaftarkan
  `<Route path={surfacePath − mountPrefix + "/change-password"}
element={<AuthPage slot="change_password_page" />} />` di `<Routes>` bersarang,
  sebelum catch-all. Logika `surfacePath − mountPrefix` yang tadinya inline di
  IIFE route root di-hoist jadi `surfaceRelative` dan dipakai kedua route.
- `renderers/react-shadcn/src/shell/ChangePasswordPage.tsx`: navigasi setelah
  sukses memakai `useSurface().surfacePath()` (root permukaan aktif) menggantikan
  `/${workspace}/_admin` yang hardcoded.
- `docs/renderers/shadcn-shell/05-routing.md`: subsection baru **§2 E. Auth
  screen — root permukaan** (path per-permukaan, beda `_admin` vs App).

## Kenapa

`UserMenu` menavigasi ke `surfacePath("change-password")`, tetapi route itu hanya
terdaftar top-level untuk `_admin` (`/:workspace/_admin/change-password`).
Permukaan App di-mount lewat splat `/:workspace/app/*` dengan `root_url` bebas,
jadi path-nya tak pernah bisa dinyatakan statis → jatuh ke catch-all.

**Terukur:** `http://localhost:5174/kafe/app/pos/change-password` →
"Page not found" (sebelum) → form Change Password dengan chrome App (sesudah);
`/kafe/change-password` (App `kafe-qr`, `root_url: /`) juga jalan; `_admin`
tidak berubah (route statis top-level tetap menang atas `/:workspace/_admin/*`).

Regresi berasal dari commit `27266f9` (auth screens spec-driven), yang
mengganti `ChangePasswordDialog` di user menu dengan navigasi route tanpa
mendaftarkan route-nya di permukaan App. Kembali ke dialog ditolak karena
mematikan `App.spec.auth.change_password_page` di permukaan App.

## Dampak & verifikasi

- Frontend saja. `npx vitest run src/shell` → 68 test lulus; `tsc --noEmit` bersih.
- Plan: `docs_internal/plan/auth-screens-app-surface.md` · Todo: **14.c.5**.

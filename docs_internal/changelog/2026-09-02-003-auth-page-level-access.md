# 2026-09-02-003 — Fase 3 Auth Redesign: Page-Level Access Gating

## Apa

Gate akses per-page (auth redesign Fase 3) — `App.spec.access` jadi default,
page bisa override per-page:

- **Public surface** (App access: public): hanya page eksplisit
  `public: false` yang butuh session; sisanya anonim.
- **Private surface** (App access: private): hanya page eksplisit
  `public: true` yang anonim; sisanya butuh session.
- `permissions` pada page tetap gate permission (backend filter via
  `allowedPage` — sudah ada).

### Backend

- Tidak ada perubahan spec — `PageSpec.Public` (route generation) dan
  `PageSpec.Permissions` sudah ada. Yang baru: page tanpa permissions di App
  private sudah ship ke caller anonim (dipin dengan test).
- Test baru `TestHandleMetaUI_PrivateApp_PublicPageShipsToAnonymous` — page
  `public: true` di App private ship ke anonim; page ber-permission difilter.

### Frontend

- `router.tsx` — `guard()` jadi surface-aware: menerima `surfacePublic`;
  public surface → `public: false` = RequireSession; private surface →
  `public: true` = anonim, sisanya RequireSession.
- `buildRoutes` menerima `surfacePublic` (dari `SurfaceShell`).
- `App.tsx` — `SurfaceShell` tidak redirect ke login saat route saat ini
  adalah page `public: true` di App private (`currentPageIsPublic` match
  path, param segment `:id` diabaikan).

## Kenapa

User poin 3: konsep App public/private dipertahankan sebagai default, tapi
gate lebih mudah per-page — public = siapa pun, private = butuh permission.
Sebelumnya hanya `public: false` di App public yang didukung; page public di
App private tidak bisa.

## Verifikasi

- Test backend: page public di App private ship ke anonim; page
  ber-permission difilter. ✅
- Browser: home (public App) load anonim; profile (`public: false`) redirect
  login. ✅
- `go test ./internal/api ./internal/ui`: 144 passed; `tsc --noEmit` bersih.

## File terdampak

- `renderers/react-shadcn/src/shell/router.tsx`
- `renderers/react-shadcn/src/App.tsx`
- `internal/api/meta_test.go`
- `registry/web/dist/` (sync build)

## Referensi

- Plan: `/memories/session/plan.md` (Fase 3)
- Todo: 5.2.14

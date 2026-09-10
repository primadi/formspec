# 2026-09-10-010 — fix AuthPage static import PageRenderer (INEFFECTIVE_DYNAMIC_IMPORT)

## Apa

`src/shell/AuthPage.tsx` mengganti static import `PageRenderer` menjadi lazy
dynamic import (`lazy(() => import(...))` + `Suspense fallback={null}`), selaras
dengan 3 pemakaian dinamis lainnya (`App.tsx`, `shell/router.tsx`,
`lib/preload.ts`).

## Kenapa

Saat release `v0.0.3`, Vite/Rollup memunculkan warning
`[INEFFECTIVE_DYNAMIC_IMPORT]`: `PageRenderer.tsx` di-import dinamis di 3
tempat (untuk memisahkannya dari entry chunk) tapi juga di-import statis oleh
`AuthPage.tsx` — static import mengalahkan semua dynamic import, sehingga
`PageRenderer` tidak pernah masuk chunk terpisah. Pola yang sama pernah terjadi
di `OverlayHost`/`FormRenderer` (changelog `2026-07-27-008`).

## File terkena dampak

- `renderers/react-shadcn/src/shell/AuthPage.tsx`

## Verifikasi

- `tsc --noEmit` bersih
- `vite build` tanpa warning `INEFFECTIVE_DYNAMIC_IMPORT`
- `vitest run src/shell` — 11 test passed

## Referensi

- `docs_internal/changelog/2026-07-27-008-fix-chunk-size-warning-web-build.md`

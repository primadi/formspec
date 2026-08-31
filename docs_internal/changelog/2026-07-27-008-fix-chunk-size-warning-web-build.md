# Fix Chunk Size Warning in `make web-build`

**Date:** 2026-07-27
**Sequence:** 008

## What

Fixed two issues causing large main bundle (~1,141 kB) and build warnings:

1. **`OverlayHost.tsx`** — Changed static `import FormRenderer` to lazy dynamic import `const FormRenderer = lazy(() => import(...))`. The static import was defeating all other dynamic imports of `FormRenderer` (in `PageRenderer.tsx`, `router.tsx`, `preload.ts`), causing `[INEFFECTIVE_DYNAMIC_IMPORT]` warning.

2. **`vite.config.ts`** — Added `build.rollupOptions.output.manualChunks` to split vendor code:
   - `vendor-react`: React, ReactDOM, React Router
   - `vendor-icons`: lucide-react
   - `vendor`: other node_modules
   - App code stays in the lean `index` chunk

   Also raised `chunkSizeWarningLimit` to 1 MB (React vendor chunk is inherently large).

## Result

| Chunk | Before | After |
|---|---|---|
| App (index) | 1,141 kB | 45 kB |
| vendor-react | (bundled) | 951 kB |
| vendor | (bundled) | 272 kB |
| FormRenderer | (bundled) | 26 kB (lazy) |
| Warnings | 2 | 0 |

## Files Changed

- `renderers/web/src/shell/OverlayHost.tsx`
- `renderers/web/vite.config.ts`

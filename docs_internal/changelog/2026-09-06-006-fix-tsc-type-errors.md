# 2026-09-06-006 — Fix 5 pre-existing TypeScript errors (tsc -b kini clean)

## Apa

Memperbaiki 5 type-error yang membuat `npm run build` (`tsc -b`) gagal:

1. `hooks/useRenderContext.ts:132` — `fetchPublicConfig` menerima client **getter**
   `() => KyInstance` (lazy, hanya di-invoke saat cache miss), tapi call site
   mempassing hasil invoke `getClient()`. Fix: pass `getClient` (getter-nya).
2. `kinds/table/TableRenderer.tsx:120` — `f.spec.entity` / `f.module` optional di
   tipe manifest; `resolveEntityRef` butuh `string`. Fix: `?? ""`.
3. `shell/OverlayHost.tsx:79` — `form.spec.entity` optional. Fix: `?? ""`.
4. `widgets/GrantsEditor.tsx:180,182` — `form.spec.entity` optional dipakai 2x
   sebagai argumen `push(entityRef: string)`. Fix: hoist
   `const entityRef = form.spec.entity ?? ""` di blok `if (form)`.

## Kenapa

Error-nya pre-existing dari fitur render-context-standard & custom-screens
(changelog 002/003 & 2026-09-06-002) — build frontend terakhir bypass type-check
(`npx vite build`), jadi tidak tertangkap.

## File yang terkena dampak

- `renderers/react-shadcn/src/hooks/useRenderContext.ts`
- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx`
- `renderers/react-shadcn/src/shell/OverlayHost.tsx`
- `renderers/react-shadcn/src/widgets/GrantsEditor.tsx`

## Verifikasi

- `npx tsc -b` → exit 0 (zero error)
- `npm run build` (tsc + vite) → ✓ built
- `npx vitest run` → 166 tests passed (8 files)

## Referensi

- Changelog `2026-09-06-005` (rebuild bypass yang menyembunyikan error ini)

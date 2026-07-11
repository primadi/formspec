# Frontend Renderer — Fase 4.F2 App Shell & Navigation

**Date:** 2026-07-11  
**Plan:** `docs/plan/todo.md` Fase 4.F2  

## What

Membangun App Shell, sidebar navigasi, router dinamis, overlay host (modal/drawer), dan login screen untuk manifest-driven renderer.

## Files Created

### Shell Components
- `web/src/shell/AppShell.tsx` — Layout utama: sidebar + topbar/breadcrumb + content area + OverlayHost. Sidebar collapsible, breadcrumb otomatis dari path.
- `web/src/shell/Sidebar.tsx` — Navigasi tree: merge `kind: Menu` manifests + derived module menus. Permission-filtered, icons dari lucide-react, nested groups, tooltips saat collapsed.
- `web/src/shell/router.tsx` — Dynamic route builder dari MetaBundle: Page routes, derived CRUD routes (list/detail/create/edit), Wizard/Kanban/Timeline routes.
- `web/src/shell/OverlayHost.tsx` — Modal/drawer host via query string (`?action=edit&form=...&mode=modal|drawer`). Menggunakan shadcn Dialog (modal) dan Sheet (drawer). Back button close.
- `web/src/shell/LoginScreen.tsx` — Token input screen untuk prod mode.
- `web/src/shell/index.ts` — Barrel export.

### Kind Renderer Stubs
- `web/src/kinds/table/TableRenderer.tsx` — Stub untuk Fase 4.F3
- `web/src/kinds/form/FormRenderer.tsx` — Stub untuk Fase 4.F3
- `web/src/kinds/page/DetailPage.tsx` — Stub untuk Fase 4.F3
- `web/src/kinds/page/PageRenderer.tsx` — Stub untuk Fase 4.F3

### App Root
- `web/src/App.tsx` — Complete rewrite: boot sequence (parse URL → fetch meta → build routes → render), routing structure (`/:workspace/_admin/*`, `/:workspace/app/*`), loading/error states, 404.

### shadcn UI Components Added
- `web/src/components/ui/{dialog,sheet,input,separator,tooltip,scroll-area,skeleton,breadcrumb,avatar}.tsx` — via shadcn CLI, base-nova preset

## Verification
- `npx tsc -b --noEmit` — ✅ Zero TypeScript errors
- `npx vitest run` — ✅ 84/84 tests passing (existing FormaExpr tests)

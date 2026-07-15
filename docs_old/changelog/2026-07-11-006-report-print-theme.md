# Frontend Renderer — Fase 4.F5 Pelengkap (Report, Print, Theme)

**Date:** 2026-07-11  
**Plan:** `docs/plan/todo.md` Fase 4.F5  

## What

Implementasi Report (parameterized tabular + CSV export), Print (html printable document), dan Theme (tokens → CSS custom properties).

## Files Created

### Report Renderer
- `web/src/kinds/report/ReportRenderer.tsx` — Parameterized tabular report:
  - Parameter form untuk filter input
  - Server-side data fetching via list API
  - Client-side grouping (spec.groups)
  - Client-side totals (sum/avg/count/min/max)
  - **CSV export** client-side via Blob + download
  - Format helpers: currency, date, datetime, percentage

### Print Renderer
- `web/src/kinds/print/PrintRenderer.tsx` — Printable document:
  - `format: html` via `window.print()`
  - Declarative header/body/footer dari PrintSpec
  - Field list, child table, separator, totals body items
  - `@page` CSS untuk paper size dari spec
  - Toolbar hidden saat print (`no-print` class)

### Theme Renderer
- `web/src/kinds/theme/ThemeRenderer.tsx` — Theme tokens → CSS custom properties:
  - Sets `--token` variables on `:root`
  - Applies `<style>` if stylesheet provided
  - Cleanup on unmount
  - Non-visual component, rendered di App.tsx

### Router Update
- `web/src/shell/router.tsx` — Routes untuk Report dan Print

### App Update
- `web/src/App.tsx` — ThemeRenderer diterapkan di AdminShell

## Verification
- `npx tsc -b --noEmit` — ✅ Zero TypeScript errors
- `npx vitest run` — ✅ 84/84 tests passing

## Coverage Final

| Kind | Renderer | Route | Status |
|---|---|---|---|
| Page | Blocks + Tabs | `spec.route` | ✅ F4 |
| Form | react-hook-form + zod | create/edit/view | ✅ F3 |
| Table | TanStack sort/paginate/search | list/browse | ✅ F3 |
| Dashboard | Widget canvas | `/dashboard/:name` | ✅ F4 |
| Widget | Standalone | `/widget/:name` | ✅ F4 |
| Report | Tabular + CSV export | `/report/:name` | ✅ F5 |
| Wizard | Multi-step stepper | `/wizard/:name` | ✅ F4 |
| Kanban | Status board | `/kanban/:name` | ✅ F4 |
| Timeline | Chronological journal | `/timeline/:name` | ✅ F4 |
| Menu | Sidebar tree | inline | ✅ F2 |
| Print | HTML + `window.print()` | `/print/:name` | ✅ F5 |
| Theme | CSS custom properties | global | ✅ F5 |

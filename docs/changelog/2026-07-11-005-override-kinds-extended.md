# Frontend Renderer — Fase 4.F4 Override Kinds & Extended Kinds

**Date:** 2026-07-11  
**Plan:** `docs/plan/todo.md` Fase 4.F4  

## What

Implementasi kind renderer untuk Page (blocks/tabs), Dashboard/Widget, Wizard, Kanban, dan Timeline — melengkapi 12 UI kinds di spec Frontend §2–§13.

## Files Created/Updated

### Page Renderer (replaced stub)
- `web/src/kinds/page/PageRenderer.tsx` — Full page renderer:
  - **Blocks variant**: grid layout dengan `layout.columns`, blok form/table/widget/component/html
  - **Tabs variant**: tabbed interface via `?tab=` query string, lazy-render per tab
  - **Permission gate**: 403 jika caller tidak memiliki permission yang dibutuhkan
  - **Component block**: placeholder "Fase 4.F6" untuk custom components

### Dashboard & Widget (baru)
- `web/src/kinds/dashboard/DashboardRenderer.tsx` — Widget canvas dengan grid layout:
  - Mendukung widget metric, chart, list, table
  - Permission-filtered per widget entity
  - Placeholder untuk chart/list rendering (Fase 4.F6)
- `web/src/kinds/widget/WidgetRenderer.tsx` — Standalone widget page

### Wizard (baru)
- `web/src/kinds/wizard/WizardRenderer.tsx` — Multi-step stepper:
  - Step state via `?step=N` query parameter
  - Progress bar, step indicators with checkmarks
  - Step-level field input, summary display
  - Final submit via custom action

### Kanban (baru)
- `web/src/kinds/kanban/KanbanRenderer.tsx` — Drag-and-drop status board:
  - Columns dari KanbanSpec.columns
  - Cards dari entity list API, difilter by status_field
  - Search support, max cards per column
  - Semantic colors tiap kolom

### Timeline (baru)
- `web/src/kinds/timeline/TimelineRenderer.tsx` — Chronological event journal:
  - Infinite scroll via IntersectionObserver
  - Date grouping (date/month/year/none)
  - Custom display fields (title/subtitle/content)
  - Sticky date headers

### Router Update
- `web/src/shell/router.tsx` — Routes untuk Dashboard, Widget, Wizard, Kanban, Timeline menggunakan lazy-loaded renderers (sekarang real, bukan placeholder)

## Verification
- `npx tsc -b --noEmit` — ✅ Zero TypeScript errors
- `npx vitest run` — ✅ 84/84 tests passing

## Coverage

| Kind | Status | Route |
|---|---|---|
| Page | ✅ Blocks + Tabs | Via `spec.route` |
| Form | ✅ (F3) | Create/Edit/View |
| Table | ✅ (F3) | List/Browse |
| Dashboard | ✅ Grid layout | `/dashboard/:name` |
| Widget | ✅ Standalone | `/widget/:name` |
| Report | ⏳ Fase 4.F5 | `/report/:name` |
| Wizard | ✅ Stepper | `/wizard/:name` |
| Kanban | ✅ Board | `/kanban/:name` |
| Timeline | ✅ Journal | `/timeline/:name` |
| Menu | ✅ (F2) | Sidebar |
| Print | ⏳ Fase 4.F5 | `/print/:name` |
| Theme | ⏳ Fase 4.F5 | CSS variables |

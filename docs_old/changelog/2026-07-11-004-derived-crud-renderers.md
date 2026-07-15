# Frontend Renderer — Fase 4.F3 Derived CRUD Renderers

**Date:** 2026-07-11  
**Plan:** `docs/plan/todo.md` Fase 4.F3  
**Design:** `docs/implementation/frontend-renderer.md` §5.3–§5.6  

## What

Implementasi derived CRUD renderer — jantung "otomatis dari spec" (D17). Entity schema dari Meta API di-derive menjadi Table, Form, dan Detail Page tanpa menulis YAML frontend.

## Files Created/Updated

### Derivation Engine (baru)
- `web/src/engine/derive.ts` — Entity schema → default TableSpec/FormSpec dengan override resolution (authored > derived). Aturan derivasi sesuai design doc §5.3: kolom table (max 8, non-child), form render mode heuristic (modal/drawer/separate_page berdasarkan field count + child table), menu per module.
- `web/src/engine/lifecycle.ts` — Lifecycle patterns §1.7: `plain_crud`, `two_step_autosave`, `two_step_manual`, `one_step`. Juga `getAvailableTransitions()` untuk state machine.
- `web/src/engine/registry.tsx` — Kind → React renderer lookup registry.

### Field Widget Library (baru)
- `web/src/widgets/TextInput.tsx` — Input/Textarea (auto-switch berdasarkan `max_length > 120`)
- `web/src/widgets/NumberInput.tsx` — Number input (integer/decimal, precision-aware)
- `web/src/widgets/Select.tsx` — Enum dropdown
- `web/src/widgets/Switch.tsx` — Boolean toggle
- `web/src/widgets/Badge.tsx` — Status badge dengan semantic colors (draft/submitted/paid/cancelled/dll)
- `web/src/widgets/index.ts` — Barrel export

### Kind Renderers (update dari stub)
- `web/src/kinds/table/TableRenderer.tsx` — Full TanStack Table: server-side pagination, sort, search, row actions (view/edit/delete + custom) dengan permission gate dan confirm dialog.
- `web/src/kinds/form/FormRenderer.tsx` — react-hook-form + zod: create/edit/view modes, zod schema dari field rules (min/max/email/pattern), auto-save debounced (2s) untuk `two_step_autosave` lifecycle, CAS version header, 409 handling via toast.
- `web/src/kinds/page/DetailPage.tsx` — Readonly field grid (2 columns), child tables display, state machine transition buttons, audit info (created_at/modified/version).

### UI Components (tambahan)
- `web/src/components/ui/textarea.tsx` — Styled textarea
- `web/src/components/ui/select-native.tsx` — Styled native select

## Verification
- `npx tsc -b --noEmit` — ✅ Zero TypeScript errors
- `npx vitest run` — ✅ 84/84 tests passing

## Milestone D10

**App tanpa manifest frontend → `/_admin` CRUD lengkap tercapai:**
1. Boot → fetch `/_meta/ui` → build routes
2. Setiap entity mendapat route list/detail/create/edit
3. Table derived dari entity schema (columns, search, sort, pagination)
4. Form derived dengan zod validation dari field rules
5. Detail page dengan state machine transitions
6. Sidebar menu derived per module
7. Permission gate pada setiap tombol aksi

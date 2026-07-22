# Surface-Aware Navigation, OverlayHost Wiring, FormaExpr Completeness, Lifecycle Patterns

**Date**: 2026-07-20  
**Component**: `renderers/web/src/` (frontend shell + kinds)  
**Related todo/plan**: `docs/plan/todo.md` tasks 5.1–5.4, `docs/plan/session/plan.md`

## Changes

### Phase A: Surface-Aware Navigation (5.4.1)
- Created `renderers/web/src/hooks/useSurface.ts` — shared hook that detects admin vs app surface from `location.pathname` and provides `surfacePath()`, `adminPath()` helpers
- Refactored hardcoded `` `/${workspace}/_admin/...` `` in **11 locations** across `TableRenderer`, `FormRenderer`, `DetailPage`, `WizardRenderer`, `AppShell`, `Sidebar` to use `useSurface()`
- `LoginScreen` kept original navigasi with typed workspace (not affected by hook since `/login` route has no `:workspace` param)

### Phase B: OverlayHost + Form Modal/Drawer (5.3.1, 5.3.2)
- Extended `FormRenderer` with `inOverlay` and `onClose` props — suppresses its own header/back-button and calls `onClose` after save instead of navigating
- Wired `OverlayHost` to render `<FormRenderer>` inside Dialog/Sheet instead of placeholder "Fase 4.F3"
- Updated `TableRenderer` to trigger overlays via `?action=create&form=...` search params when authored form has `render.mode` = modal/drawer

### Phase C: FormaExpr Completeness (5.3.5)
- Added `evalVisibleWhen`, `evalRequiredWhen`, `evalCompute` imports and wiring in `FormRenderer`
- Added `me` (user) from session store to `fieldContext`
- `visible_when` — skips invisible fields and sections
- `required_when` — runtime required check in addition to static `entityField.required`
- `compute` — `useEffect` auto-evaluates compute expressions and sets values via `form.setValue()`

### Phase D: Lifecycle Patterns (5.3.4)
- Added `getLifecycle` import in `FormRenderer`
- `one_step` (quickSubmit) — single "Create & Submit" button that POSTs to `create-submit` endpoint
- `two_step_manual` — "Save Draft" + "Submit" buttons, no auto-save
- Existing `plain_crud` and `two_step_autosave` unaffected

## Files Affected

| File | Change |
|---|---|
| `renderers/web/src/hooks/useSurface.ts` | **NEW** — surface detection hook |
| `renderers/web/src/kinds/table/TableRenderer.tsx` | 3 hardcoded nav → useSurface; overlay trigger; useSearchParams + metaBundle |
| `renderers/web/src/kinds/form/FormRenderer.tsx` | 2 hardcoded nav → useSurface; inOverlay/onClose props; FormaExpr wiring; lifecycle patterns |
| `renderers/web/src/kinds/page/DetailPage.tsx` | 2 hardcoded nav → useSurface |
| `renderers/web/src/kinds/wizard/WizardRenderer.tsx` | 1 hardcoded nav → useSurface |
| `renderers/web/src/shell/OverlayHost.tsx` | Wire FormRenderer, entity resolution, modal/drawer toggle |
| `renderers/web/src/shell/AppShell.tsx` | Breadcrumb Home link → surfacePrefix; useSurface import |
| `renderers/web/src/App.tsx` | No functional change (LoginPage redirect stays hardcoded) |
| `renderers/web/src/shell/LoginScreen.tsx` | Reverted to original (uses typed workspace from form, not useSurface) |

## Verification
- TypeScript compilation: `npx tsc --noEmit` → clean
- Vite production build: `npx vite build` → success (1 warning: static+dynamic import FormRenderer)
- Go backend: `go build ./cmd/forma/` → success
- E2E: `TestVisitLifecycle_EndToEnd` passes; other tests fail due to fixture date backdate limit (pre-existing, unrelated)

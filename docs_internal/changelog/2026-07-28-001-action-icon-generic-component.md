# Extract ActionIcon as Generic Reusable Component

## Changes

- **New** `renderers/web/src/components/ui/action-icon.tsx` — generic icon component
  that resolves lucide-react icons by name at runtime. Uses `import * as LucideIcons`
  (same pattern as Sidebar) so any icon from YAML manifest is supported without
  increasing bundle size.
- **Updated** `renderers/web/src/kinds/table/TableRenderer.tsx` — removed local
  switch-case `ActionIcon` function and its direct imports (`Eye`, `Pencil`,
  `Trash2`, `Play`, `Check`, `Download`). Both callsites (row actions and bulk
  actions) now use the shared `<ActionIcon iconName={...} />` component.

## Rationale

- Manifest-driven framework needs runtime icon resolution — can't hardcode a
  limited icon set
- Sidebar already bundles all lucide-react via `import *`, so this adds zero
  bundle overhead
- Props `iconName` + `className` make the component generic and reusable across
  the codebase

## Files Affected

| File | Action |
|---|---|
| `renderers/web/src/components/ui/action-icon.tsx` | Created |
| `renderers/web/src/kinds/table/TableRenderer.tsx` | Edited |

## References

- Plan: `docs/plan/todo.md` → Task: Extract ActionIcon as reusable component

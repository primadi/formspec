# Refactor Sidebar icon resolution to use shared resolveIcon

## Changes

- **New** `renderers/web/src/lib/icon-resolver.ts` — shared utility that exports
  a single `resolveIcon(name: string)` function. Converts kebab-case/PascalCase
  icon names to lucide-react components using namespace import.
- **Updated** `renderers/web/src/components/ui/action-icon.tsx` — now imports
  `resolveIcon` from `@/lib/icon-resolver` instead of having its own copy.
- **Updated** `renderers/web/src/shell/Sidebar.tsx` — removed local `resolveIcon`
  function and `import * as LucideIcons`. Now imports `resolveIcon` from
  `@/lib/icon-resolver`, the same source as ActionIcon.

## Rationale

- Eliminates duplicate icon resolution logic between ActionIcon and Sidebar
- Sidebar's old `resolveIcon` only handled single-hyphen kebab-case; shared
  version handles multiple hyphens and underscores correctly
- Single source of truth for any future consumer that needs runtime icon lookup

## Files Affected

| File | Action |
|---|---|
| `renderers/web/src/lib/icon-resolver.ts` | Created |
| `renderers/web/src/components/ui/action-icon.tsx` | Edited |
| `renderers/web/src/shell/Sidebar.tsx` | Edited |

## References

- Plan: `docs/plan/todo.md` → Task: Refactor Sidebar to use shared resolveIcon

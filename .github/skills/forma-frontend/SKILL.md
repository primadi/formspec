---
name: forma-frontend
description: "Use when: working on Forma frontend code — React, TypeScript, shadcn/ui, manifest-driven renderers, FormaExpr, kinds, widgets, shell, or any file under renderers/web/. Provides UI kind catalog, key paths, FormaExpr grammar, and design rules."
---

# Forma Frontend Skill

Context for AI coding agents working on the Forma frontend (React, TypeScript, shadcn/ui).

## Key paths
- `renderers/web/src/` — shadcn-shell renderer (React + TypeScript + Vite)
- `renderers/web/src/kinds/` — Manifest-driven renderers per UI kind
- `renderers/web/src/engine/` — Derivation, permissions, lifecycle, entityRef
- `renderers/web/src/lib/formaexpr/` — FormaExpr AST interpreter (lexer, parser, eval)
- `renderers/web/src/lib/api/` — ky-based HTTP client (client, meta)
- `renderers/web/src/shell/` — AppShell, Sidebar, router, OverlayHost
- `renderers/web/src/stores/` — Zustand stores (meta, session, prefs)
- `renderers/web/src/types/manifest.ts` — TypeScript types mirroring pkg/spec/
- `renderers/web/src/widgets/` — Field-level input widgets
- `renderers/web/src/components/ui/` — shadcn/ui base components

## UI Kinds (tier: page)
- Page, Form (data-entry), Table (table-list), Kanban, Calendar, Wizard
- Dashboard, Widget, Report, Print, Timeline, ApprovalInbox, NotificationCenter, Listing

## FormaExpr
- Client-side expression grammar (subset of Starlark)
- Uses: `visible_when`, `readonly_when`, `required_when`, `compute`
- Context: `fields.*`, record for title interpolation
- Error behavior: nonexistent field = ERROR (not silent fail-safe)

## Key design rules
- Manifest-driven: SPA reads manifests via `/_meta/ui` at runtime
- Two surfaces: `/_admin` (auto-derived) and `/app` (authored)
- Derived by default: Entity → Table + Forms + Page + Menu
- Design-time layout: modal/drawer/separate_page decided in manifest
- Asset contract: `mount(el, props, forma)` / `unmount(el)`
- `forma` client: `forma.api`, `forma.subscribe`, `forma.navigate`, `forma.theme`, `forma.ui`, `forma.files`
- CSP sandbox for asset components
- CSS scoped to container
- Permission-driven UI (never page-based auth)

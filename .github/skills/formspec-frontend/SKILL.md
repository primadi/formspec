---
name: formspec-frontend
description: "Use when: working on FormSpec frontend code — React, TypeScript, shadcn/ui, manifest-driven renderers, FormSpecExpr, kinds, widgets, shell, or any file under renderers/react-shadcn/. Provides UI kind catalog, key paths, FormSpecExpr grammar, and design rules."
---

# FormSpec Frontend Skill

Context for AI coding agents working on the FormSpec frontend (React, TypeScript, shadcn/ui).

## Key paths

- `renderers/react-shadcn/src/` — shadcn-shell renderer (React + TypeScript + Vite)
- `renderers/react-shadcn/src/kinds/` — Manifest-driven renderers per UI kind
- `renderers/react-shadcn/src/engine/` — Derivation, permissions, lifecycle, entityRef
- `renderers/react-shadcn/src/lib/formspec-expr/` — FormSpecExpr AST interpreter (lexer, parser, eval)
- `renderers/react-shadcn/src/lib/api/` — ky-based HTTP client (client, meta)
- `renderers/react-shadcn/src/shell/` — SideNavShell, Sidebar, router, OverlayHost
- `renderers/react-shadcn/src/stores/` — Zustand stores (meta, session, prefs)
- `renderers/react-shadcn/src/types/manifest.ts` — TypeScript types mirroring pkg/spec/
- `renderers/react-shadcn/src/widgets/` — Field-level input widgets
- `renderers/react-shadcn/src/components/ui/` — shadcn/ui base components

## UI Kinds (tier: page)

- Page, Form (data-entry), Table (table-list), Kanban, Calendar, Wizard
- Dashboard, Widget, Report, Print, Timeline, ApprovalInbox, NotificationCenter, Listing

## FormSpecExpr

- Client-side expression grammar (subset of Starlark)
- Uses: `visible_when`, `readonly_when`, `required_when`, `compute`
- Context: `fields.*`, record for title interpolation
- Error behavior: nonexistent field = ERROR (not silent fail-safe)

## Key design rules

- Manifest-driven: SPA reads manifests via `/_meta/ui` at runtime
- Two surfaces: `/_admin` (auto-derived) and `/app` (authored)
- **Routing: see `docs/renderers/shadcn-shell/05-routing.md`** — four route kinds:
  A authored Page (`spec.route`) · B derived Form/Table Page (`/<module>/form|table/<n>`,
  mode always `view`) · C derived entity CRUD (`/<module>/<plural>[/new|/:id[/edit]]`) ·
  D overlay (`?action=&form=&mode=`, not a route). Which Form is used: A/B/D by
  explicit `form.ref`; C by the `{entity}-create/-edit/-form` naming convention.
  Form's own `spec.mode` does NOT pick the runtime mode.
- Menu is a CONSUMER of routes, not their source. `MenuItem.permissions` = RBAC,
  enforced server-side (`filterMenu`); `MenuItem.when` = business condition,
  evaluated client-side (fail-open). `when` is never an authorization gate.
- FormSpecExpr callables are a closed set (`len`/`sum`/`amount`/`currency`/`today`);
  `formspec check` rejects anything else at deploy time.
- Derived by default: Entity → Table + Forms + Page + Menu
- Design-time layout: modal/drawer/separate_page decided in manifest
- Asset contract: `mount(el, props, formspec)` / `unmount(el)`
- `formspec` client: `formspec.api`, `formspec.subscribe`, `formspec.navigate`, `formspec.theme`, `formspec.ui`, `formspec.files`
- CSP sandbox for asset components
- CSS scoped to container
- Permission-driven UI (never page-based auth)
- Auth forms use standard `autocomplete` tokens (`username`, `current-password`, `new-password`) driven by `auth_action`; never `"nope"`/`"off"` on credential fields (Chromium password-form guidance), and keep a hidden `workspace`/`app` input for context

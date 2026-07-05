# Forma Frontend Spec v0.4.0 (draft)

**Status:** Draft — awaiting review
**License:** Creative Commons CC0
**Governed by:** Forma Foundation Document v2.0 (esp. D10, D14, D17, D20, D33, D35, D36)
**Resolves:** Q12 (frontend kind catalog + asset escape hatch), Q9 (realtime subscription convention)
**Requires:** Forma Core Basic v0.2.0

> Frontend in Forma is described, not programmed — until it can't be, and
> then the escape hatch is explicit. This spec defines the six UI kinds,
> the renderer contract, the custom-component contract, and the realtime
> convention. It deliberately covers only **patterned UI** (~80% of
> business-app screens); arbitrary UI belongs in `asset` components (D14).

---

## 1. Architecture

### 1.1 Rendering model: interpreted, not generated

The official frontend is a **manifest-driven renderer**: a SPA shell served
by `forma-resource` that reads UI manifests through the meta API
(`/api/v1/_meta/ui`) and renders them at runtime.

Why interpreted (normative rationale):
- Manifests are **live** — `script_ref` and admin-panel edits (D34 visual
  editor) take effect without a build step.
- One renderer serves both surfaces (§1.2) and every app identically.
- The AI/GitOps story stays intact: UI state is always the YAML, never a
  compiled artifact that can drift.

Codegen still exists for what needs it: TypeScript types and API clients
(Core §29) consumed by **custom components**, which are the only built
artifacts in a Forma frontend.

The renderer MUST also be consumable **as a library**: an existing
React/vue/plain app embeds Forma screens (`<FormaPage name="order-detail"/>`
via the official adapter) — enabling incremental adoption in both
directions (§7).

### 1.2 Two surfaces, one renderer

| Surface | Source | Setup required |
|---|---|---|
| **Admin panel** (`/_admin`) | derived 100% from Entity manifests | zero — the PocketBase benchmark (D10) |
| **App UI** (`/app`) | composed via the kinds in this spec | only what deviates from defaults |

### 1.3 Derived by default (D17)

Every Entity automatically yields, with no UI manifest at all:
- a list **Table** (columns from indexed + first N fields),
- **create/edit Forms** (all writable fields, rules → client validation),
- a detail **Page** (fields + state-machine actions as buttons),
- **Menu** entries per module.

UI kinds exist to *override* these defaults, never as a requirement.
A team can ship a complete internal tool with zero frontend manifests.

### 1.4 Permission-driven UI (dividend of D20)

The renderer receives the caller's effective permissions and MUST hide or
disable any element whose backing action the caller cannot invoke: action
buttons, menu items, table bulk actions, form submit for `update` without
permission. No `visible_to_role:` fields exist in this spec — visibility
derives from the permission catalog. (Expression-based visibility for
*business* conditions does exist: §6.)

### 1.5 Three file types, upheld

`yaml` (the kinds below) · `script` (server-side logic — unchanged) ·
`asset` (static files + custom component bundles). The renderer itself is
part of the implementation, not the app.

---

## 2. Kind Catalog

Nine kinds, one concern each (Foundation Appendix B row 6):

| Kind | Concern | Overrides |
|---|---|---|
| `Page` | route + composition (blocks, tabs, or one full component) | derived detail pages |
| `Form` | input/edit layout for one entity | derived forms |
| `Table` | list/browse for one entity | derived list |
| `Dashboard` | widget canvas — defaults + user customization (§5.2) | — |
| `Widget` | one dashboard widget, publishable by any module (§5.1) | — |
| `Report` | parameterized tabular report + export (§5.3) | — |
| `Menu` | navigation tree | derived module menus |
| `Print` | paper/PDF document for one entity | — (D10, Frappe print format) |
| `Theme` | look & feel — distributable, marketplace artifact (§10) | default theme |

All share the manifest format (Core §3). `metadata.module` scopes them like
any resource. The vocabulary below is **closed** (D33): new UI needs never
add YAML syntax — they become components (§7).

---

## 3. `kind: Page`

A routed screen composing blocks.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Page
metadata:
  name: order-detail
  module: billing
spec:
  route: /orders/:id            # app-relative; params bind to blocks
  title: "Order {order.number}" # interpolation from bound data
  blocks:
    - form:  { ref: order-view, entity: order, id: ":id", mode: view }
    - table: { ref: order-payments, param: { order_id: ":id" } }
    - component:                # escape hatch — §7
        asset: billing/assets/payment-timeline.js
        props: { order_id: ":id" }
  layout: { columns: 2 }        # blocks flow into a simple grid
```

Rules: routes are unique per app; `:params` are the only dynamic route
syntax; blocks reference Form/Table/Dashboard by name or inline a
component. No conditional block trees — a page needing branching layout is
a component.

**Tabs variant** — several sub-screens on one route (master data, settings —
keeps the Menu uncluttered):

```yaml
spec:
  route: /settings
  tabs:
    - { label: Umum,     form:  { ref: settings-general } }
    - { label: Pajak,    form:  { ref: settings-tax } }
    - { label: Produk,   table: { ref: product-list } }
    - { label: Advanced, component: { asset: core/assets/advanced.js } }
```

`blocks` and `tabs` are mutually exclusive. A third form — **full-custom
page** — is a single `component:` entry with no blocks/tabs: route,
title, and permission gating come from the manifest; everything inside is
the component's (JS/HTML/CSS) — the page-level escape hatch.

## 4. `kind: Form`

Layout + behavior for one entity's input, replacing the derived form.
This is the Frappe form-layout lesson (D10) made explicit.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Form
metadata:
  name: order-edit
  module: billing
spec:
  entity: order
  mode: edit                    # create | edit | view
  layout:
    sections:
      - title: Customer
        columns: 2
        fields:
          - { field: customer_id, widget: relation-picker }
          - { field: member_tier, readonly: true }
      - title: Items
        fields:
          - field: items          # child → editable line grid widget
            widget: child-grid
            columns: [product_id, quantity, price]
      - title: Totals
        visible_when: "fields.items != null and len(fields.items) > 0"   # §6
        fields:
          - { field: total, readonly: true,
              compute: "sum([i.quantity * i.price for i in fields.items])" }
  actions:                      # buttons — backed by real actions only
    - { action: checkout, label: "Checkout", style: primary }
    - { action: void, confirm: "Batalkan order ini?" }
```

Rules:
- Every `field` MUST exist on the entity; every `action` MUST exist and is
  permission-gated automatically (§1.4).
- **Closed client-behavior vocabulary** (all FormaExpr, all reactive —
  re-evaluated when referenced fields change): `visible_when`,
  `readonly_when`, `required_when`, `compute`. Nothing else — the moment
  imperative side effects are needed (`on_change` logic), the field becomes
  a custom widget (§7): the D33 boundary, frontend edition.
- `widget` comes from a **closed widget registry** (text, number, decimal,
  date, datetime, select, relation-picker, child-grid, file, toggle,
  textarea, json). Custom widgets are components registered per §7.
- Field `rules` from the entity manifest are enforced client-side for UX;
  **server-side validation (Core §13) remains the authority — client
  checks are never security.**

## 5. `kind: Table` and `kind: Dashboard`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: order-list, module: billing }
spec:
  entity: order
  columns:
    - { field: number, link: order-detail }     # link → Page name
    - { field: customer.name }                  # relation traversal (include)
    - { field: total, format: currency }
    - { field: status, widget: badge }
  filters: [status, created_at]                 # rendered from field types
  default_sort: { field: created_at, direction: desc }
  search: true
  realtime: true                                # §8
  row_actions: [mark-paid, void]
  bulk_actions: [export]
```

```yaml
apiVersion: forma.dev/v1alpha1
kind: Dashboard
metadata: { name: sales-today, module: billing }
spec:
  refresh: 60                    # seconds; or realtime: true (§8)
  widgets:
    - stat:  { title: "Omzet hari ini", entity: sales-daily-summary,
               field: total, filter: { date: today } }
    - chart: { type: line, entity: sales-daily-summary,
               x: date, y: total, range: 30d }
    - table: { ref: order-list, limit: 5 }
    - component: { asset: billing/assets/heatmap.js }
```

Dashboard widgets read **summary entities or `list` actions only** — a
dashboard is a projection viewer, never a query builder; anything needing
custom aggregation becomes a summary entity fed by durable events (Core
§9.1), keeping the read path cheap and permission-checked.

### 5.1 `kind: Widget` — module-contributed widgets

Any module publishes named dashboard widgets into the app's **widget
catalog** — how a GL module ships its own visualizations without touching
anyone's dashboard:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Widget
metadata: { name: gl-cashflow-chart, module: gl,
            description: "Arus kas 30 hari" }
spec:
  size: { w: 2, h: 1 }                  # grid units; user may resize
  chart: { type: line, entity: gl-cashflow-summary, x: date, y: net, range: 30d }
  # body is exactly one of: stat | chart | table | component (§5 shapes)
```

Visibility in the catalog is **derived**: a user sees a widget only if they
hold the underlying entity/action's read permission (D20 dividend — no
`visible_to` field exists).

### 5.2 Customizable dashboards — preference is data, not manifest

```yaml
kind: Dashboard
spec:
  customizable: true
  defaults: [sales-today-stat, gl-cashflow-chart, order-list-widget]
```

With `customizable: true`, users add/remove/resize/reorder widgets from the
catalog; the resulting layout is stored as a **runtime preference** in
`forma.core` (per user, per dashboard) — never written back to YAML.
Normative principle: **manifests define what is possible; preferences
record what is chosen.** (The UI extension of Appendix C's "data is not
manifest".) `forma describe dashboard <name>` shows defaults + catalog;
user layouts are data, inspectable via the API like any record.

### 5.3 `kind: Report` — parameterized tabular output

The third read pattern (Dashboard = glance, Table = browse, Report =
parameterized, grouped, exportable):

```yaml
apiVersion: forma.dev/v1alpha1
kind: Report
metadata: { name: sales-by-category, module: billing }
spec:
  required_permission: reports.sales-by-category
  params:
    - { field: date_from, type: date, required: true }
    - { field: date_to,   type: date, required: true }
    - { field: branch,    type: relation, resource: branch }
  source: { entity: order, filter: { status: paid,
            paid_at: { between: [":date_from", ":date_to"] } } }
  columns: [number, customer.name, category, total]
  group_by: [category]
  totals: [total]
  exports: [xlsx, csv, print: receipt-style]      # print → kind: Print
```

- `source` is an entity query or a `list`-shaped action — always
  permission-checked; a Report never embeds SQL.
- Exports run as **async jobs** (Core §17.1) whose files land in the
  download tray (§7.1).
- Interactive pivots and exotic visualizations are components — Report
  covers the patterned majority.

## 6. FormaExpr — client-side expressions (normative)

`visible_when`, `readonly_when`, `required_when`, `compute`, and `title`
interpolation use **FormaExpr**: the *expression subset* of Starlark —
literals, field refs (`fields.x`, `user.permissions`), comparisons,
`and/or/not`, arithmetic, `len`, `sum`, list comprehensions. **No**
function definitions, loops, imports, or `ctx`.

- Implemented as a **small AST interpreter in JS** shipped with the
  renderer — no transpilation step, no build. The grammar is deliberately
  tiny so this stays a few hundred lines.
- One grammar shared with server-side guards keeps the mental model single;
  the sandboxes differ.
- Evaluated by the renderer in the browser — **UX only, never
  authorization and never validation of record** (both stay server-side).
- Implementations MUST reject non-expression constructs at `forma validate`.

**The language line (normative rationale):** declarative expressions =
FormaExpr; imperative frontend code = **JS/TS via the component contract**
(§7). Full Starlark in the browser is rejected: its value is server-side
sandboxing, which the browser does not need (all calls go through
`forma.api` as the logged-in user), while its cost there is real — no
idiomatic DOM access, no npm ecosystem, two-layer debugging.

## 7. Component contract — the `asset` escape hatch (D14)

For the ~20% of UI that is not patterned. A component is an **ES module**
in `assets/` with a framework-agnostic mount contract:

```js
// modules/billing/assets/payment-timeline.js
export default {
  mount(el, props, forma) { /* render into el */ },
  unmount(el) { }
}
```

- `forma` is the injected client: `forma.api` (generated, typed — calls run
  **as the logged-in user**, so all security remains server-side),
  `forma.subscribe(entity, cb)` (§8), `forma.navigate(page, params)`,
  `forma.theme` (tokens), **`forma.ui`** — the standard UI services:
  `toast(msg, level)`, `dialog(opts)`, `confirm(msg)`, `drawer(component,
  props)` — and **`forma.files`** (§7.1). Declarative actions use the same
  services (an action's `confirm:` renders through `forma.ui.confirm`), so
  app and framework dialogs look identical.

### 7.1 Transfer manager — uploads & downloads (renderer infrastructure)

Not a kind — standard facilities like `forma.ui`:

- **Upload tray:** `forma.files.upload(field|opts)` — background queue with
  progress, retry, cancel, shown in a global tray. The `file` widget uses
  it automatically; size/type constraints come from the field's rules and
  are re-enforced server-side. Targets `ctx.storage` via staged endpoints,
  always tenant-scoped.
- **Download tray:** no new mechanism — Table exports, Report exports, and
  Print renders are all **async jobs** (Core §17.1) producing files in
  storage. The tray subscribes to `job.completed` on the `jobs` channel and
  lists the resulting downloads. Large files stream; links are
  permission-checked per request, never public.

- The renderer ships a **base component library** (closed, themeable):
  the form widgets (§4), tabs, badge, card, empty-state, breadcrumb,
  skeleton/loading, pagination. Custom components may compose these via
  `forma.components`.
- Components receive props from YAML; they MUST NOT read globals or fetch
  outside `forma.api` (renderer serves them under a strict CSP —
  `connect-src` limited to the app origin). A bundle MAY include
  **scoped CSS** (injected under the component's container only).
- Custom **widgets** (usable in Form/Table) register via
  `export const widget = { name, mount, ... }` and are referenced by name.
- **Hybrid React, both directions:** (a) React components inside the Forma
  renderer — `@forma/react` wraps the mount contract; (b) Forma screens
  inside an existing React app — the renderer as a library:
  `<FormaPage name="order-detail" params={{id}}/>` (§1.1). The mount
  contract itself has no framework dependency.
- Bundles are ordinary assets: versioned with the module, signed with the
  artifact (D24 — the IP-can-be-binary rule applies to UI too).

### 7.2 Headless form engine — full-control JS without rewriting the plumbing

`forma.form(entity, { mode, id? })` returns a **headless** form instance:
field state, dirty tracking, client validation compiled from the entity's
field rules, `visible_when/readonly_when/required_when/compute` evaluation,
and `submit()` wired to the right action (create/update, with `version`
CAS and error mapping). No layout, no widgets — the developer owns 100% of
the markup. The full-control ladder is thus: managed Form → custom widget →
component → full-custom page → **headless** → raw `forma.api`.

### 7.3 Component `needs:` — the frontend `uses`

Components are opaque to footprint derivation (§11), so a component that
calls `forma.api` MUST declare what it touches where it is placed:

```yaml
- component:
    asset: billing/assets/checkout-wizard.js
    needs:
      actions: [order.create, order.checkout, customer.find,
                payment-gateway.create-session]
      subscribe: [billing.order]
```

`forma.api` calls outside `needs` fail client-side (and were never
authorized server-side anyway); `forma validate` warns on unused
declarations — the D20 honesty pattern, frontend edition.

### 7.4 Unmanaged clients — mobile and beyond

An unmanaged client (Flutter, native, any SPA) is a **first-class API
consumer today**: HTTP (Core §16), realtime WebSocket (§8), server-enforced
permissions, generated typed clients — official codegen targets:
TypeScript and **Dart**; push notifications via `forma/notify`. Nothing in
this spec is required for such clients; a *managed* mobile renderer remains
open (F3).

## 8. Realtime convention (resolves Q9 / D10-PocketBase)

Declarative subscription to entity changes:

- **Channel convention:** `entity:{module}.{name}` with events
  `created | updated | deleted`, payload = event payload fields (Core §12),
  always tenant-scoped.
- **Server-side filter:** a client receives an event only if it holds the
  entity's `view` permission — the same check as `find`, evaluated per
  message. Subscription is never broader than read access.
- **Declarative use:** `realtime: true` on Table/Dashboard = auto
  subscribe + patch rows in place. Programmatic use:
  `forma.subscribe("billing.order", cb)` in components.
- Transport rides the existing WebSocket delivery (Core §16); wire format
  follows Core §19.4 broadcasts.
- Realtime is **non-durable by definition** (UI class, Core §12.1) — a
  reconnecting client refetches; it never replays.

## 9. `kind: Menu` and `kind: Print`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Menu
metadata: { name: main, module: core }
spec:
  items:
    - { label: Orders, page: order-detail-list, icon: receipt }
    - label: Finance
      items:
        - { label: Journal, page: journal-list }
    - { label: Settings, page: settings, when: "user.has('core.settings.view')" }
```

Items pointing to pages whose backing permissions the user lacks are hidden
automatically (§1.4); `when` adds business conditions on top.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: order
  paper: { size: A5, margin: 12mm }
  header: { logo: true, title: "Nota {order.number}" }
  body:
    - fields: [number, paid_at, customer.name]
    - child_table: { field: items, columns: [product_id, quantity, price] }
    - totals: { field: total, format: currency }
  footer: { text: "Terima kasih — {tenant.name}" }
```

Print renders server-side to PDF via `ctx.print(entity_id, "receipt")` —
the Companion's `renderReceiptPDF()` becomes declarative. Fully custom
documents remain scripts/components; Print covers the patterned majority
(invoice, receipt, delivery note, label).

## 10. `kind: Theme` — look & feel as a marketplace artifact

Uniform-looking apps are the known cost of a shared renderer; the answer is
a theme system deep enough to sell (Shopify/WordPress precedent — aligns
with D22):

```yaml
apiVersion: forma.dev/v1alpha1
kind: Theme
metadata: { name: batik-dark, module: acme-themes }
spec:
  tokens:                       # design tokens — colors, radius, spacing,
    color.primary: "#B8860B"    # typography scale, logo slots
    radius.md: 10px
  stylesheet: assets/batik-dark.css     # CSS layer over the base library
  widgets:                      # optional skin overrides per base widget
    badge: assets/widgets/badge.js
```

- A Theme ships inside a module → versioned, signed (D24), sellable on the
  marketplace like any module.
- The **Data Owner selects the theme per workspace** (no manifest change in
  the app); tokens can be further overridden by `Config` keys under
  `theme.*` for per-workspace branding.
- Themes restyle the **base component library** (§7) — they never alter
  layout semantics or bypass permission-driven visibility.
- Components read the active theme via `forma.theme`. No per-kind styling
  vocabulary exists in Page/Form/Table (that road leads to CSS-in-YAML);
  styling lives in Theme and in component scoped CSS only.

## 11. Page capability footprint & task-based administration (D38)

Admins think in tasks ("kasir boleh halaman POS"), not in permission
strings. This section gives them that — **without ever moving enforcement
off the resource layer**.

- Every Page has a **derived capability footprint**: the union of what its
  composition requires — Forms → their entities' create/update actions,
  Tables → list, Reports/Widgets → their sources, components → their
  explicit `needs:` (§7.3). Shown by `forma describe page <name>` and in
  the role editor.
- Granting a role "access to page X" **materializes** the footprint into
  ordinary resource permissions on that role — expanded, visible,
  auditable, revocable. When a later app version changes a page's
  footprint, the delta surfaces for re-consent (the D20/D21 pattern).
- **Implicit page-based authority is rejected** (normative): the server
  cannot verify that a request originated from a page — any client can
  claim UI provenance — and unmanaged clients (§7.4) never touch pages at
  all. Client-side `needs` gating is DX, not security; the resource layer
  (Core §15) remains the only enforcement.
- Multi-entity submits (wizards) SHOULD be one server-side **composite
  action** for atomicity, not sequential client calls.
- Identity & membership (D37): user identity is workspace-level; app
  membership and role assignments are per-app — the renderer of app X
  simply has no session for users without membership in X.

## 12. Conformance

A frontend implementation conforms when it provides:

1. Admin panel derived 100% from Entity manifests with zero configuration.
2. Derived defaults (§1.3) and override resolution for all nine kinds,
   including Page `tabs` and full-custom-page variants.
3. Permission-driven visibility from the caller's effective permission set.
4. FormaExpr AST interpreter with the exact subset of §6 (incl. the four
   reactive behaviors), rejected at validate time when exceeded; client
   checks never substitute server enforcement.
5. Component mount contract + injected `forma` client incl. `forma.ui`
   services + base component library + scoped CSS + CSP constraints.
6. Renderer consumable as a library (embed direction, §1.1).
7. Realtime convention of §8 including per-message permission filtering.
8. Print rendering server-side for the §9 vocabulary.
9. Theme resolution per workspace: tokens + stylesheet + widget skins,
   without layout or permission side effects.
10. Meta API (`/_meta/ui`) exposing merged, permission-filtered UI manifests.
11. Widget catalog with permission-derived visibility; customizable
    dashboards persisting user layouts as runtime preferences (never YAML).
12. Report execution with permission-checked sources; exports as async
    jobs; upload/download trays per §7.1.
13. Headless form engine (§7.2); component `needs` declaration with
    validate-time honesty check (§7.3); Dart + TypeScript client codegen.
14. Page footprint derivation, materialized task-based grants with
    re-consent on delta, and rejection of implicit UI-provenance authority
    (§11).

---

## Open questions (this spec)

| # | Question |
|---|---|
| F1 | Per-widget option schemas for the base registry (list itself now fixed in §4/§7) |
| F2 | Offline/optimistic UI: out of scope v1, revisit for POS use cases? |
| F3 | Mobile: responsive renderer only, or a native shell later? |
| F4 | Page transitions/wizard flows (multi-step forms) — kind or component? |
| F5 | Theme review policy for marketplace (CSS can visually spoof — Verified Badge criteria for themes) |
| F6 | Report: grouping depth, subtotal formatting options, scheduled reports (via forma/scheduler → mail) |

## Changelog

### v0.4.0
- Headless form engine `forma.form()` — full-control JS ladder completed
- Component `needs:` declaration (frontend `uses`) feeding footprints
- §11: page capability footprint + materialized task-based grants;
  implicit UI-provenance authority rejected (D38)
- Unmanaged clients first-class (Dart + TS codegen targets); managed
  mobile stays F3 (D37/D38 context)

### v0.3.0
- `kind: Widget` — module-contributed dashboard widgets, catalog with
  permission-derived visibility
- Customizable dashboards; normative principle "manifests define what is
  possible; preferences record what is chosen" (user layout = runtime data)
- `kind: Report` — params + grouped tabular output + async exports
- `forma.files` + upload/download trays as renderer infrastructure

### v0.2.0
- Page: `tabs` variant + full-custom-page (single component) variant
- `forma.ui` standard services (toast/dialog/confirm/drawer) + base
  component library (closed, themeable) + component scoped CSS
- Closed reactive behavior vocabulary: `visible_when`, `readonly_when`,
  `required_when`, `compute`
- FormaExpr: implemented as JS AST interpreter (no transpile); language
  line normatized — expressions FormaExpr, imperative code JS/TS
- Hybrid both directions: React-in-Forma and Forma-in-React (renderer as
  library)
- `kind: Theme`: distributable, signed, marketplace-ready; Data Owner
  selects per workspace

### v0.1.0
- Initial draft: interpreted renderer, two surfaces, derived-by-default,
  six-kind catalog, FormaExpr, component mount contract, realtime
  convention (Q9 resolved), Print (D10 resolved), theming via Config.

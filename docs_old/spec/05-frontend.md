# Forma Frontend Spec v0.5.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (D10, D14, D17, D20, D24, D33, D35, D36, D38)
**Requires:** Core Basic v0.2.0
**Source inspiration:** frontend-concept.md (8+1 page type taxonomy — incorporated as Kinds and pattern guidance)

> Frontend in Forma is described, not programmed — until it can't be, and then the escape hatch is explicit. This spec defines the UI kinds, the renderer contract, the custom-component contract, and the realtime convention. It deliberately covers only **patterned UI** (~80% of business-app screens); arbitrary UI belongs in `asset` components (D14). Forma adopts a **Hybrid Low-Code** approach: the 80% patterned majority is declared in YAML; the 20% imperative minority uses the explicit `asset` escape hatch (§7).

---

## 1. Architecture

### 1.1 Rendering Model: Interpreted, Not Generated

The official frontend is a **manifest-driven renderer**: a SPA shell served by `forma-resource` that reads UI manifests through the meta API (`/api/v1/_meta/ui`) and renders them at runtime.

Why interpreted:
- Manifests are **live** — `script_ref` and admin-panel edits take effect without a build step.
- One renderer serves both surfaces (§1.2) and every app identically.
- The AI/GitOps story stays intact: UI state is always the YAML, never a compiled artifact that can drift.

Codegen still exists for TypeScript types and API clients (Core §27), consumed by **custom components**, which are the only built artifacts in a Forma frontend.

The renderer MUST also be consumable **as a library**: an existing React/Vue app embeds Forma screens (`<FormaPage name="order-detail"/>` via the official adapter) — enabling incremental adoption in both directions.

### 1.2 Two Surfaces, One Renderer

| Surface | Source | Setup required |
|---|---|---|
| **Admin panel** (`/_admin`) | Derived 100% from Document manifests | Zero — the PocketBase benchmark (D10) |
| **App UI** (`/app`) | Composed via the kinds in this spec | Only what deviates from defaults |

> **Important scope note:** This spec defines UI kinds for **business applications** built with Forma (Page, Form, Table, Dashboard, etc.). It does **NOT** define admin UIs for the Control Plane (`forma/ops`) or Resource Plane admin console (`forma/console`). Those are first-party Forma applications with separate architecture documentation in `docs/architecture/02-admin-surfaces.md`. The business app admin panel (`/_admin`) IS part of this spec — it is auto-generated from Document manifests via the renderer defined here.

### 1.3 Derived by Default (D17)

Every Document automatically yields, with no UI manifest at all: a list **Table**, **create/edit Forms**, a detail **Page**, and a derived navigation entry in the App's menu (grouped by the Document's module, for any module not already covered by an authored `App.spec.menu`/`Module.spec.menu` entry — Core Basic §4.4). UI kinds exist to *override* these defaults. A team can ship a complete internal tool with zero frontend manifests.

### 1.4 Permission-Driven UI

The renderer receives the caller's effective permissions and MUST hide or disable any element whose backing action the caller cannot invoke: action buttons, menu items, table bulk actions, form submissions. Visibility derives from the permission catalog — no `visible_to_role:` fields exist. Expression-based visibility for *business* conditions does exist (§6).

### 1.5 Hybrid Low-Code — The 80/20 Split

Forma targets automating **80–90%** of standard enterprise UI patterns through declarative YAML kinds. The remaining 10–20% — arbitrary layouts, bespoke interactions, complex visualizations — use the explicit `asset` component escape hatch (§7). This split is not a limitation; it is a deliberate boundary that keeps the YAML vocabulary closed (D33) while ensuring no use case is impossible.

A programmer never writes boilerplate CRUD, filtering, pagination, or realtime subscription wiring. They write YAML for the patterned majority, and JS/TS components only for truly unique needs. The result: faster delivery, consistent UX, and AI-assisted development guardrails (one format, predictable structure).

### 1.6 Design-Time Layout Locking

Container decisions — whether a form opens in a **modal**, a **drawer**, or a **separate page** — are made at *design time* through the manifest, never at runtime by user preference or ad-hoc code. This preserves UX consistency across the application and protects state-management stability (route-based vs overlay-based state have different lifecycle implications).

- **Modal / Drawer:** Lightweight entities (≤5 fields). User stays in list context. Controlled via query string (`?action=edit&id=1`).
- **Separate Page:** Dense documents (many fields, complex validation, child tables). Has its own route (`/document/:id/edit`).
- **Decision is per-Form, not per-document:** The same document may have a modal quick-create form and a separate-page full-edit form — each is a distinct `kind: Form` with its own `render` declaration.

This principle is enforced by the renderer: no runtime switching between modes for the same Form manifest. If a different context is needed, declare a second Form.

### 1.7 UI Patterns — Lifecycle vs Plain CRUD

The renderer determines which UI pattern to use based on whether the reserved action `submit` is enabled or disabled on the Document — **not** based on `characteristic: transaction` alone. These two flags are independent: `characteristic: transaction` is purely about date/accounting period semantics (§Core 14a), while the UI pattern is purely about whether the draft→submit lifecycle is meaningful in business terms.

```
Action "submit" explicitly disabled (see §Core 4.1d)
  → Plain CRUD: one "Save" button, no Submit button,
    no concept of draft displayed to the user
    (doc_status is null — lifecycle-free, no lifecycle concept)

Action "submit" ACTIVE (default if not written)
  → Choose one of three patterns below, via manifest `ui:` hint.
    Default if not declared: 2-step + auto-save.
```

**Three patterns for resources with active lifecycle:**

| Pattern | When to use | UI displayed |
|---|---|---|
| **2-step + auto-save** (default) | Complex documents, needs review (Invoice, Order, Contract) | Silent auto-save while draft (debounced `update`), one explicit "Submit" button |
| **2-step manual** | Draft intentionally split to another person for review first | Separate "Save Draft" + "Submit" buttons |
| **1-step (create-submit)** | High-volume quick entry (POS, clinic queue) | One button, uses built-in `create-submit` action (§Core 4.1b) — no concept of draft visible in UI, atomic (all-or-nothing) |

```yaml
resource:
  name: invoice
  type: document
  characteristic: transaction

actions:
  - name: create-submit          # built-in reserved action, auto-derived
    ui:
      button_label: "Save & Submit"
      style: primary
      show_when: "quick_entry_mode"
```

Two standard buttons (**Save Draft**/auto-save, **Submit**) are always automatically available from the model without needing to be declared. The built-in `create-submit` action adds a third button as an optional fast path — it does not replace the two basic buttons.

---

## 2. Kind Catalog

Twelve kinds, one concern each:

| Kind | Concern | Overrides |
|---|---|---|
| `Page` | Route + composition (blocks, tabs, or one full component) | Derived detail pages |
| `Form` | Input/edit layout for one document | Derived forms |
| `Table` | List/browse for one document | Derived list |
| `Dashboard` | Widget canvas — defaults + user customization | — |
| `Widget` | One dashboard widget, publishable by any module | — |
| `Report` | Parameterized tabular report + export | — |
| `Wizard` | Multi-step business process with stepper navigation | — |
| `Kanban` | Drag-and-drop status board per document | — |
| `Timeline` | Chronological, append-only event journal | — |
| `Print` | Printable document for one document — multi-target output | — |
| `Theme` | Look & feel — distributable, marketplace artifact | Default theme |

All share the manifest format (Core §3). `metadata.module` scopes them like any resource. The vocabulary is **closed** (D33): new UI needs never add YAML syntax — they become components (§7). The kinds added in v0.5.0 (Wizard, Kanban, Timeline) are generic UI patterns, not business-case-specific syntax; they pass the D33 litmus test.

> Navigation is **not** in this table. There is no standalone `kind: Menu` — the navigation tree is `App.spec.menu` (authoritative) and, optionally, `Module.spec.menu` (a default suggestion an App can adopt). See Core Basic §4.4/§4.5.

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
  route: /orders/:id
  title: "Order {order.number}"
  blocks:
    - form:  { ref: order-edit, entity: order, id: ":id", mode: view }
    - table: { ref: order-payments, param: { order_id: ":id" } }
    - component:
        asset: billing/assets/payment-timeline.js
        props: { order_id: ":id" }
  layout: { columns: 2 }
```

**Rules:** routes are unique per app; `:params` are the only dynamic route syntax; blocks reference Form/Table/Dashboard by name or inline a component.

**Tabs variant** — several sub-screens on one route:
```yaml
spec:
  route: /settings
  tabs:
    - { label: General,  form:  { ref: settings-general } }
    - { label: Tax,      form:  { ref: settings-tax } }
    - { label: Products, table: { ref: product-list } }
```

`blocks` and `tabs` are mutually exclusive. **Full-custom page** = single `component:` entry with no blocks/tabs.

**Tabbed Resources rationale:** When an app has many small master-data entities (genders, marital statuses, specialties, categories), giving each its own sidebar menu entry creates navigation clutter. Group related small resources under one `kind: Page` with `tabs` — each tab hosts one Table or Form. The user sees one menu entry, one route, organized sub-screens. This is a *design-time grouping decision*; the renderer treats each tab as an independently permission-checked resource.

**Configuration Page pattern:** For system settings — key-value parameters whose *structure* is locked by the developer and whose *values* the administrator may update — use a `kind: Page` with `tabs` variant. Each tab references a `kind: Form` in `mode: edit` backed by a document with `characteristic: reference`. The renderer MUST NOT render a "New Item" or "Delete" button for reference documents; only the Update action is surfaced. Example:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: system-settings, module: core }
spec:
  route: /settings
  title: "System Configuration"
  tabs:
    - { label: Clinic,   form: { ref: settings-clinic,   entity: config, id: "clinic" } }
    - { label: Integration, form: { ref: settings-api,   entity: config, id: "api" } }
    - { label: Billing,  form: { ref: settings-billing, entity: config, id: "billing" } }
```

The backing document (`config`) has `characteristic: reference` and one row per settings group. The Form `mode: edit` over `id` surfaces the key-value fields; no `mode: create` form is needed because the structure is seeded by the module, not created at runtime.

---

## 4. `kind: Form`

Layout + behavior for one document's input, replacing the derived form.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Form
metadata:
  name: order-edit
  module: billing
spec:
  entity: order
  mode: edit                    # create | edit | view
  render: separate_page         # modal | drawer | separate_page (default: modal)
  layout:
    sections:
      - title: Customer
        columns: 2
        fields:
          - { field: customer_id, widget: relation-picker }
          - { field: member_tier, readonly: true }
      - title: Items
        fields:
          - field: items
            widget: child-grid
      - title: Totals
        visible_when: "fields.items != null and len(fields.items) > 0"
        fields:
          - { field: total, readonly: true,
              compute: "sum([i.quantity * i.price for i in fields.items])" }
  actions:
    - { action: checkout, label: "Checkout", style: primary }
    - { action: void, confirm: "Cancel this order?" }
```

**`render` — design-time container decision** (§1.6). Determines how the form is presented to the user:

| `render` | Behavior | When to use |
|---|---|---|
| `modal` (default) | Overlay dialog above current page. Dismissed on save/cancel. Route unchanged; state preserved underneath. | Lightweight entities (≤5 fields). Quick create/edit without losing list context. |
| `drawer` | Slide-in panel from the right. Same overlay behavior as modal but wider, suited for forms with side-by-side sections. | Medium forms (5–12 fields), especially with `columns: 2` layout. |
| `separate_page` | Dedicated route. Full-page form with its own breadcrumb and URL. Back navigation returns to the calling context. | Dense entities (12+ fields, child tables, complex validation). Needs deep-linking. |

The same document may have multiple Forms with different `render` values — e.g., a `modal` quick-create form and a `separate_page` full-edit form. The decision is per-Form, declared in the manifest, and enforced by the renderer (no runtime switching).

**Rules:** every `field` MUST exist on the document; every `action` MUST exist and is permission-gated automatically (§1.4). **Closed client-behavior vocabulary** (FormaExpr, §6): `visible_when`, `readonly_when`, `required_when`, `compute`. The moment imperative side effects are needed, the field becomes a custom widget (§7). Field `rules` from the document manifest are enforced client-side for UX; **server-side validation remains the authority — client checks are never security.**

---

## 5. `kind: Table`, `kind: Dashboard`, `kind: Widget`, `kind: Report`

### Table
```yaml
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: order-list, module: billing }
spec:
  entity: order
  columns:
    - { field: number, link: order-detail }
    - { field: customer.name }
    - { field: total, format: currency }
    - { field: status, widget: badge }
  filters: [status, created_at]
  default_sort: { field: created_at, direction: desc }
  search: true
  realtime: true
  row_actions: [mark-paid, void]
  bulk_actions: [export]
```

### Dashboard
```yaml
apiVersion: forma.dev/v1alpha1
kind: Dashboard
metadata: { name: sales-today, module: billing }
spec:
  customizable: true                   # users add/remove/reorder from widget catalog
  defaults: [sales-today-stat, gl-cashflow-chart]
  refresh: 60                          # or realtime: true
  widgets:
    - stat:  { title: "Today's Revenue", entity: sales-daily-summary, field: total }
    - chart: { type: line, entity: sales-daily-summary, x: date, y: total, range: 30d }
    - component: { asset: billing/assets/heatmap.js }
```

Dashboard widgets read **summary entities or `list` actions only**. Custom aggregations become summary entities fed by durable events (Core §12.1). **Customizable dashboards:** user layouts stored as runtime preferences in `forma.core` — **manifests define what is possible; preferences record what is chosen.** Never written back to YAML.

### Widget (module-contributed)
```yaml
apiVersion: forma.dev/v1alpha1
kind: Widget
metadata: { name: gl-cashflow-chart, module: gl }
spec:
  size: { w: 2, h: 1 }
  chart: { type: line, entity: gl-cashflow-summary, x: date, y: net, range: 30d }
```

Visibility in the catalog is **derived**: a user sees a widget only if they hold the underlying entity/action's read permission (D20 dividend).

### Report (parameterized tabular output)
```yaml
apiVersion: forma.dev/v1alpha1
kind: Report
metadata: { name: sales-by-category, module: billing }
spec:
  required_permission: reports.sales-by-category
  params:
    - { field: date_from, type: date, required: true }
    - { field: date_to,   type: date, required: true }
  source: { entity: order, filter: { status: paid,
            paid_at: { between: [":date_from", ":date_to"] } } }
  columns: [number, customer.name, category, total]
  group_by: [category]
  totals: [total]
  exports:                                        # print → kind: Print
    - xlsx
    - csv
    - print: receipt-style
```

- `source` is an entity query — always permission-checked. Report never embeds SQL.
- Exports run as **async jobs** (Core §17); files land in the download tray.

---

## 6. FormaExpr — Client-Side Expressions

`visible_when`, `readonly_when`, `required_when`, `compute`, and `title` interpolation use **FormaExpr**: the *expression subset* of Starlark — literals, field refs (`fields.x`), comparisons, `and/or/not`, arithmetic, `len`, `sum`, list comprehensions. **No** function definitions, loops, imports, or `ctx`.

- Implemented as a **small AST interpreter in JS** in the renderer — no transpilation, no build.
- One grammar shared with server-side guards keeps the mental model single; sandboxes differ.
- Evaluated in the browser — **UX only, never authorization and never validation** (both stay server-side).
- Implementations MUST reject non-expression constructs at `forma validate`.

**The language line:** declarative expressions = FormaExpr; imperative frontend code = **JS/TS via the component contract** (§7). Full Starlark in the browser is rejected.

---

## 7. Component Contract — The `asset` Escape Hatch

For the ~20% of UI that is not patterned. A component is an **ES module** in `assets/` with a framework-agnostic mount contract:

```js
// modules/billing/assets/payment-timeline.js
export default {
  mount(el, props, forma) { /* render into el */ },
  unmount(el) { }
}
```

- `forma` is the injected client: **`forma.api`** (generated, typed — runs as the logged-in user, all security server-side), **`forma.subscribe(entity, cb)`** (§8), **`forma.navigate(page, params)`**, **`forma.theme`** (tokens), **`forma.ui`** — standard services: `toast(msg, level)`, `dialog(opts)`, `confirm(msg)`, `drawer(component, props)`, and **`forma.files`** (§7.1).

### 7.1 Transfer Manager (renderer infrastructure, not a kind)

- **Upload tray:** `forma.files.upload(field|opts)` — background queue with progress, retry, cancel.
- **Download tray:** Table/Report/Print exports are async jobs producing files in storage. The tray subscribes to `job.completed` on the `jobs` channel and lists resulting downloads. Links are permission-checked per request.

### 7.2 Base Component Library

The renderer ships a **closed, themeable** base component library: form widgets (§4), tabs, badge, card, empty-state, breadcrumb, skeleton/loading, pagination. Custom components may compose these via `forma.components`.

### 7.3 Headless Form Engine

`forma.form(entity, { mode, id? })` returns a **headless** form instance: field state, dirty tracking, client validation from entity rules, FormaExpr evaluation, and `submit()` wired to the right action (create/update, with `version` CAS). No layout, no widgets — developer owns 100% of markup. The full-control ladder: managed Form → custom widget → component → full-custom page → headless → raw `forma.api`.

### 7.4 Component `needs:` — the Frontend `uses`

Components are opaque to footprint derivation, so a component that calls `forma.api` MUST declare what it touches where it is placed:

```yaml
- component:
    asset: billing/assets/checkout-wizard.js
    needs:
      actions: [order.create, order.checkout, customer.find]
      subscribe: [billing.order]
```

`forma.api` calls outside `needs` fail client-side (and were never authorized server-side anyway). `forma validate` warns on unused declarations.

### 7.5 Unmanaged Clients — Mobile and Beyond

An unmanaged client (Flutter, native, any SPA) is a **first-class API consumer today**: HTTP (Core §16), realtime WebSocket (§8), server-enforced permissions, generated typed clients — official codegen targets: TypeScript and **Dart**. Nothing in this spec is required for such clients.

---

## 8. Realtime Convention

Declarative subscription to entity changes:

- **Channel convention:** `entity:{module}.{name}` with events `created | updated | deleted`, payload = event payload fields, always tenant-scoped.
- **Server-side filter:** a client receives an event only if it holds the entity's `view` permission — evaluated per message.
- **Declarative use:** `realtime: true` on Table/Dashboard = auto-subscribe + patch rows in place.
- **Programmatic use:** `forma.subscribe("billing.order", cb)` in components.
- Realtime is **non-durable by definition** (UI class, Core §12.1) — a reconnecting client refetches; it never replays.

---

## 9. `kind: Print`

### Menu — see Core Basic §4.4/§4.5

Navigation is not a standalone kind here — it's `App.spec.menu` (authoritative) and `Module.spec.menu` (default suggestion), documented in full (the `MenuItem` shape, the 3-level nesting cap, the `view`/`route` leaf resolution rules) in Core Basic §4.4. Items pointing to a `view` whose backing permission the caller lacks are hidden automatically (§1.4 below); `when` adds business conditions on top.

### Print
```yaml
apiVersion: forma.dev/v1alpha1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: order
  output:
    format: pdf                   # pdf | thermal | dotmatrix | html
    paper: { size: A5, margin: 12mm }
  header: { logo: true, title: "Receipt {order.number}" }
  body:
    - fields: [number, paid_at, customer.name]
    - child_table: { field: items, columns: [product_id, quantity, price] }
    - totals: { field: total, format: currency }
  footer: { text: "Thank you — {tenant.name}" }
```

Print renders documents for one entity. The `output.format` determines the rendering pipeline:

| Format | Pipeline | Paper sizes | Use case |
|---|---|---|---|
| `pdf` (default) | Server-side PDF generation | `A4`, `A5`, `Letter`, `Legal`, `custom: { width, height, unit }` | Invoice, delivery note, report |
| `thermal` | Server-side ESC/POS byte stream → raw printer | `thermal_58mm`, `thermal_80mm` | Point-of-sale receipt, pharmacy slip, queue ticket |
| `dotmatrix` | Server-side plain text + escape codes for continuous-feed printers | `dotmatrix_80col`, `dotmatrix_136col` | Warehouse pick list, legacy accounting print |
| `html` | Client-side `window.print()` with `@media print` CSS — no server rendering | Any (CSS `@page` size) | Browser-native printing, preview-before-print workflows |

**Rules:**
- `output.paper.size` is validated against the selected format — `thermal_58mm` is only valid with `format: thermal`, etc.
- All formats except `html` render server-side and produce a downloadable file (lands in the download tray, §7.1).
- `html` format renders in the browser; the `@media print` stylesheet is injected by the renderer, hiding global navigation (sidebar, navbar) and applying the declared paper size via CSS `@page { size: ... }`.
- Custom paper: `custom: { width: 80, height: 200, unit: mm }` — validated at `forma validate`.
- Programmatic print: `ctx.print(entity_id, "receipt")` returns the rendered document; format selection is per-Print manifest, not per-call.
- Fully custom documents remain scripts/components; Print covers the patterned majority (invoice, receipt, delivery note, label, queue ticket).

**Thermal receipt example (58mm):**
```yaml
apiVersion: forma.dev/v1alpha1
kind: Print
metadata: { name: pos-receipt, module: pos }
spec:
  entity: transaction
  output:
    format: thermal
    paper: { size: thermal_58mm }
  header:
    logo: true
    title: "TOKO SEJAHTERA"
    subtitle: "Jl. Merdeka No. 123"
  body:
    - fields: [number, created_at, cashier.name]
    - separator: "-----------------------------"
    - child_table: { field: items, columns: [product.name, quantity, price, subtotal] }
    - separator: "-----------------------------"
    - totals: { field: total, format: currency }
  footer:
    text: "Terima kasih — Barang yang sudah dibeli tidak dapat dikembalikan"
```

---

## 10. `kind: Theme` — Look & Feel as a Marketplace Artifact

```yaml
apiVersion: forma.dev/v1alpha1
kind: Theme
metadata: { name: batik-dark, module: acme-themes }
spec:
  tokens:
    color.primary: "#B8860B"
    radius.md: 10px
  stylesheet: assets/batik-dark.css
  widgets:                      # optional skin overrides per base widget
    badge: assets/widgets/badge.js
```

- A Theme ships inside a module → versioned, signed (D24), sellable on the marketplace.
- The **Data Owner selects the theme per workspace** (no manifest change in the app); tokens can be overridden by `Config` keys under `theme.*`.
- Themes restyle the **base component library** (§7.2) — they never alter layout semantics or bypass permission-driven visibility.

---

## 11. `kind: Wizard` — Multi-Step Process

A Wizard guides the user through a sequential business process that spans multiple steps, potentially touching multiple entities. The framework manages stepper navigation, step validation, inter-step field dependencies, per-instance autosave, and completion behavior.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Wizard
metadata:
  name: patient-registration
  module: clinic
spec:
  title: "Patient Registration — {step.title}"
  entity: visit             # no `action`: final step does a plain create on this entity
                             # using the accumulated step data
  on_complete:
    restart: true            # reset stepData/currentStep to 0 instead of navigating away
    redirect: null           # path to navigate to instead; ignored when restart: true
    banner:
      - { label: "Queue Number", field: response.queue_number }
  steps:
    - title: "Find Patient"
      layout: search_select
      entity: patient
      search_fields: [nik, name, phone]
      allow_create: true                 # "New Patient" button if not found
    - title: "Select Poly & Doctor"
      required: [polyclinic_id, doctor_id]
      fields:
        - { field: polyclinic_id, entity: polyclinic, type: dropdown, required: true }
        - { field: doctor_id, entity: doctor, type: dropdown, required: true,
            depends_on: polyclinic_id }  # filter doctors by selected polyclinic
      on_prev: discard-poly-selection    # optional action fired when leaving via Previous
    - title: "Confirm & Submit"
      on_enter: prefill-visit-defaults   # optional action fired when the step becomes active
      summary:
        - { label: "Patient", field: patient.name }
        - { label: "Polyclinic", field: polyclinic.name }
        - { label: "Doctor", field: doctor.name }
```

**Rules:**
- `action` (wizard-level) is optional. If set, it's a server-side action that atomically writes all step data on final submit — it MUST exist on at least one document involved in the wizard, and receives the accumulated wizard state as input. If omitted (as above), the final step does a plain `create` on `entity` using the accumulated step data — every field the entity needs must already be resolved by prior steps (e.g. a `patient_id` captured via an eager `search_select` create in step 1).
- `on_complete` controls what happens after a successful final submit:
  - `restart: true` clears `stepData` and returns to step 0 instead of navigating away — for front-desk-style flows where one completion should immediately make way for the next (e.g. register one patient, then the next).
  - `redirect` navigates to the given path instead. Ignored when `restart: true`.
  - `banner` renders info from the just-completed submission using the same dotted-path resolution as step `summary`, but resolved against `response.*` (the API response of the final submit) rather than `stepData` — required because `stepData` itself is cleared on restart.
- Step-level `required: [field, ...]` gates the Next button — Next is disabled until every listed field has a value in `stepData`.
- Step-level hooks: `on_enter` fires an action when the step becomes active (including on Back); `on_next` (previously the bare `action` property) fires on Next before advancing; `on_prev` fires when leaving the step via Previous. All three are optional and best-effort — a failing hook does not block navigation.
- `depends_on` establishes a client-side filter chain: when `polyclinic_id` changes, the `doctor_id` dropdown re-fetches with the new filter. This is UX-only; server-side validation is the authority.
- Steps are sequential — the renderer enforces completion of step N before step N+1 is accessible. Back navigation is always allowed (step N-1 data is preserved).
- A Wizard page has its own route (`/wizard/:name`); step state is tracked in the URL (`?step=2`) for deep-linking and browser back-button support. Each open wizard is additionally identified by a `?instance=<id>` param (auto-generated if absent) — `stepData` autosaves to `localStorage` under `wizard:{name}:{instance}`, so ordinary multi-tab use (Ctrl+click) and page refresh don't clobber or lose in-progress data. There is no server-side draft row.
- When a wizard step needs custom UI beyond fields/dropdowns, use `component:` within the step — the component receives `{ wizard, step, data, forma }` props.

**Relationship to other kinds:** A Wizard is essentially a stateful composition of Form-like steps with a stepper shell. If a process needs only linear form sections without sequential enforcement, use `kind: Form` with `layout.sections`. Wizard exists for the pattern where each step depends on the previous and the final commit is either a single server-side action or a plain create on the target entity (D38).

---

## 12. `kind: Kanban` — Status Board

A Kanban renders an entity's records as draggable cards across status columns. Dragging a card from one column to another triggers an optimistic `update` of the entity's status field, with CAS `version` for conflict detection.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Kanban
metadata:
  name: pharmacy-queue
  module: pharmacy
spec:
  entity: prescription
  status_field: status
  realtime: true                         # default on — live card movement across users
  columns:
    - { value: queued,       label: "Waiting",       color: gray }
    - { value: compounding,  label: "Compounding",   color: orange }
    - { value: ready,        label: "Ready for Pickup", color: green }
  card:
    title_field: number
    subtitle_field: patient.name
    badge_field: priority                 # optional — color-coded badge
    footer_fields: [created_at, doctor.name]
    assignee_field: pharmacist_id         # optional — avatar in card corner
  filters: [priority, created_at]
  search: true
  row_actions: [view-detail, print-label]
  max_cards_per_column: 50               # renderer paginates within column beyond this
```

**Rules:**
- **Drag-drop = `update` action on `status_field`:** The renderer calls the entity's `update` action with the new status value. Optimistic UI — card moves immediately; on 409 (CAS conflict) the card snaps back and a toast shows the conflict.
- **`status_field` values MUST match `columns[].value` exactly.** A drop to an unlisted value is rejected client-side. The entity's state machine (Core §14) MAY define valid transitions; the renderer consults it to prevent invalid column moves (snap-back + toast).
- **`realtime: true` is default and strongly recommended.** Other users' card movements are reflected live. Without realtime, stale column counts are possible.
- Each column shows a **count badge**. The renderer fetches counts via `list` with `group_by: status_field`.
- **Permission-driven:** drag is disabled if the user lacks the entity's `update` permission. Row actions are permission-gated per §1.4.
- **Custom card rendering:** set `card.component` with an asset path — the component receives `{ card, column, forma }` and renders into the card slot. The default card template uses the fields declared above.
- Kanban has its own route (`/kanban/:name`); column state is not in the URL (intentional — drag-drop is ephemeral UI state).

**When to use Kanban vs Table:** Use Kanban when the primary mental model is *status progression* (pharmacy queue, order fulfillment, issue tracker). Use Table when the primary mental model is *data inspection* (master data, transaction log). Both can coexist for the same entity — e.g., a Table for the order backlog and a Kanban for the fulfillment board.

---

## 13. `kind: Timeline` — Event Journal

A Timeline renders entity records as a vertical chronological feed, grouped by date. Designed for append-only audit trails, activity logs, and medical records — data that is written once and never mutated.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Timeline
metadata:
  name: patient-medical-history
  module: clinic
spec:
  entity: medical_record
  bind_param: patient_id                  # filter context — from route, parent page, or fixed value
  bind_value: ":patient_id"              # route param placeholder
  display:
    title_field: visit_date              # primary line — typically a date/timestamp
    subtitle_field: doctor.name          # secondary line — actor or category
    content_field: diagnosis_and_notes   # body — rich text or plain string
    icon_field: visit_type               # optional — maps to icon (consultation, emergency, etc.)
  group_by: date                         # date | month | year | none
  sort: desc                             # newest first (default)
  page_size: 20                          # infinite-scroll threshold
  empty_state: "No medical records found for this patient."
```

**Rules:**
- **Append-only enforcement:** The renderer MUST NOT render create, edit, or delete buttons for a Timeline entity. The entity SHOULD disable its `update` and `delete` actions via per-action `disabled: true` (Core §11.1), leaving only `create` on the server side. If the entity still has `update`/`delete` actions, the renderer ignores them for Timeline — the kind is the guard.
- **`bind_param` + `bind_value`:** The Timeline is always filtered to a parent context — a patient, an order, a machine. `bind_param` names the entity field used as filter; `bind_value` is the value, typically a route `:param` or a fixed ID.
- **Grouping:** Cards are visually grouped under date headers ("Today", "Yesterday", "12 June 2026"). `group_by: none` renders a flat continuous feed.
- **Infinite scroll:** The renderer fetches pages as the user scrolls down (cursor-based, using `created_at`). `page_size` controls batch size.
- **Realtime:** A Timeline subscribes to `created` events on its entity and prepends new cards at the top without disturbing scroll position.
- **Custom card rendering:** set `display.component` with an asset path — the component receives `{ record, forma }` and renders the card body. The date grouping and chrome (line, dot, date header) remain framework-rendered.
- Timeline has its own route (`/timeline/:name`) or can be embedded as a block inside a `kind: Page`.

**When to use Timeline vs Table:** Use Timeline when the temporal sequence is the primary narrative (medical history, audit log, activity feed, chat). Use Table when the user needs to sort, filter, and operate on rows (transaction list, master data). A Timeline is fundamentally a *read-only story*; a Table is an *operational surface*.

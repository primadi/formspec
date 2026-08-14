---
name: formspec-kinds
description: Catalog of all FormSpec resource kinds — Entity, Service, App, Module, Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Calendar, Listing, ApprovalInbox, NotificationCenter, Print, Theme, Config, Migration, Subscription, Workflow, Api, Webhook, Mockup, KindDefinition, Integrator, Renderer, PersistBackend, Environment, Policy, Datastore. Use when the user asks about FormSpec kinds, needs to choose the right kind for a task, asks how to declare a YAML manifest, or mentions specific kinds by name. Also use when creating a new FormSpec app to understand which kinds to declare.
metadata:
  version: "1.0"
  source: docs/spec/platform/03-kind-system.md + schemas/kinds/
---

# FormSpec Kinds — Complete Catalog

Every FormSpec resource is declared as a YAML manifest with a `kind` field.
This catalog lists every built-in kind, grouped by concern.

## Universal Manifest Format

All kinds share the same top-level structure:

```yaml
apiVersion: formspec.dev/v1
kind: Entity               # PascalCase
metadata:
  name: invoice            # kebab-case, unique per (kind, module)
  module: billing          # owning module
  description: "..."       # recommended for AI readability
  labels: {}
  annotations: {}
spec:
  # kind-specific body
```

Key rules:
- `metadata.name` — kebab-case, unique per (kind, module)
- `metadata.module` — owning module name
- `metadata.description` — **always include** for AI readability
- `spec` body is kind-specific — see individual kind sections below

## Quick Reference: Concern → Kind

| Concern | Kind(s) | Spec File |
|---------|---------|-----------|
| Domain model | `Entity`, `Service` | `backend/01-core-basic.md` |
| Curation | `App`, `Module` | `platform/02-workspace-app-module.md` |
| Configuration | `Config` | `backend/01-core-basic.md` §10 |
| Custom DDL | `Migration` | `backend/01-core-basic.md` §4 |
| Cross-module events | `Subscription` | `backend/01-core-basic.md` §7 |
| Business process | `Workflow` | `backend/02-core-extended.md` §2 |
| API surface | `Api`, `Webhook`, `Mockup` | `backend/02-core-extended.md` |
| Extension | `KindDefinition` | `platform/03-kind-system.md` §2 |
| Reactive bridge | `Integrator` | `backend/02-core-extended.md` §5 |
| Visual renderer | `Renderer` | `frontend/03-renderer-kind.md` |
| Storage renderer | `PersistBackend` | `backend/04-persist-backend.md` |
| Visual — page tier | `Page`, `Form`, `Table`, `Dashboard`, `Widget`, `Report`, `Wizard`, `Kanban`, `Timeline`, `Calendar`, `Listing`, `ApprovalInbox`, `NotificationCenter`, `Print`, `Theme` | `frontend/05,06,07` |
| Governance | `Environment`, `Policy` | `platform/04-control-plane.md` |
| Infrastructure | `Datastore` | `platform/06-datastore.md` |

---

## Domain Model Kinds

### Entity — Stateful Business Data

The most important kind. Represents persistent business data with:
- **Characteristic** (mutually exclusive):
  - `master` — stable reference data (Customer, Product). May have lifecycle.
  - `transaction` — append-heavy, time-partitioned (Invoice, Journal Entry). **Requires** explicit `transaction_date` field.
  - `reference` — read-only seed data (Provinces, Tax Rates). Supports **find-or-create**: GET auto-creates record if not found.
  - `summary` — system-managed projection (GL Balance). Create/update/delete **permanently disabled** via API.
- **Lifecycle** via `doc_status`: `draft → submitted → cancelled` (closed set). Reserved actions: create, update, submit, cancel, delete, amend (and derived create-submit, amend-submit).
- **Field reserved**: `owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version` — auto-managed, never declare as custom field.
- **Relationships**: `child` (lifecycle tied to parent) vs `relation` (independent lifecycle). Test: "does this make sense without the parent?"
- **Permissions**: `permission = resource + action`, never hardcoded role names.
- **Update after submit is always denied** — Entity is immutable after submit. Use named custom actions for post-submit changes.

```yaml
apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: invoice
  module: billing
  description: "Sales invoice with line items"
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: invoice_number
      type: string
      required: true
    - name: transaction_date
      type: date
      required: true
    - name: total_amount
      type: money
      required: true
  # expose is an ARRAY of {type, actions} — the `all`/`read`/`none` shorthand
  # does NOT exist. Omit expose entirely for UI-only (external API → 404).
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
```

**`spec.version` is required** on every Entity (`version: v1`).

**Relations** are declared with `type: relation` **plus** a sibling `relation:`
object — never a `target:` key (`target` is silently ignored by the YAML
loader, leaving a dangling relation):

```yaml
fields:
  - name: customer_id
    type: relation
    relation: { type: belongs_to, resource: billing.customer }
    required: true
```

**`child`** declares an *embedded* collection (inline fields, storage
jsonb|table) — it is NOT a reference to another entity:

```yaml
fields:
  - name: items
    type: child
    child:
      storage: jsonb
      sequence_field: line_no
      fields:
        - { name: description, type: string }
        - { name: quantity, type: integer }
```

If the child must be a separately CRUD-able entity, use `relation`
(`belongs_to`) instead.

**`expose`** canonical list form (`docs/spec/backend/01-core-basic.md` §8.4):

| Value | Meaning |
|---|---|
| `expose: []` / omitted | UI + internal callers only; external API → 404 |
| `expose: [{ type: rest, actions: [list, find] }]` | read-only external API |
| `expose: [{ type: rest, actions: [list, find, create, update, delete] }]` | full CRUD external API |

`expose` only controls the external surface; UI is always available and gated
by permissions. `kind: Api` overrides how the external surface is published.

**`lifecycle`** (if used) is a STRING enum
(`two_step_autosave | two_step_manual | plain_crud`), NOT a map. The built-in
`doc_status` lifecycle is default-on — do NOT write `lifecycle: {doc_status: true}`.

**State machine** for business states beyond doc_status (`docs/spec/backend/02-core-extended.md` §1):

```yaml
spec:
  version: v1
  characteristic: transaction
  fields:
    - name: status
      type: enum
      enum_values: [draft, in_progress, completed, cancelled]
      default: draft
      index: true
  state_machine:
    field: status
    initial: draft
    states:
      - { name: draft, label: "Draft" }
      - { name: in_progress, label: "In Progress" }
      - { name: completed, label: "Completed" }
      - { name: cancelled, label: "Cancelled" }
    transitions:
      - { from: draft, to: in_progress, via: start-work }
      - { from: in_progress, to: completed, via: complete }
      - { from: "*", to: cancelled, via: cancel }
  actions:
    - name: start-work
      description: "Mulai pengerjaan"
      required_permission: billing.invoice.start-work
      audit: true
```

Transitions use `via` (the triggering action name) — `action` is only a
legacy alias. `guard` is `{ expression, message }`, not a list of roles.

### Service — Stateless Computation

Stateless, pure computation. No `characteristic`, `doc_status`, or lifecycle guards.

```yaml
apiVersion: formspec.dev/v1
kind: Service
metadata:
  name: tax-calculator
  module: billing
spec:
  inputs:
    - name: amount
      type: number
  outputs:
    - name: tax
      type: number
  handler:
    type: native
    ref: "TaxService.Calculate"
```

---

## Curation Kinds

### App — Curated Collection of Modules

An App is a **curation** — a basket of modules declared via `spec.modules`.
An App does NOT own objects; Modules do. The same Module can be mounted by
multiple Apps in the same workspace.

`spec.version`, `spec.vendor`, and `spec.root_url` are **required**.
`root_url` must start with `/app/` and be unique within the workspace.

**Menu is owned by App** (§4 of the platform spec). Menu = "what can be
reached via navigation" — it must be decided at the same level as
view/action visibility (different Apps can expose different subsets of the
same Module). Analogy: **Module = catalog, App.menu = shopping list from
that catalog.**

The menu is defined in `spec.menu` as a list of `MenuItem` nodes. Three
node types (validated at load):

| Node | `type` | level | Required | Forbidden |
|------|--------|-------|----------|-----------|
| **Adopt** | `module` | 1 only | `module` | `label`, `icon`, `view`, `route`, `children` |
| **Group** | (empty) | 1–2 | `label`, `children` | `module`, `view`, `route` |
| **Leaf** | (empty) | 2–3 | `label`, `module`, exactly one of `view`/`route` | `children` |

- **Adopt node** splices the entire `Module.spec.menu` default suggestion at
  this position. Module must be in `spec.modules`.
- **Group node** creates a submenu. Children can come from different modules.
- **Leaf node** links to a view or a raw route. `view` resolves a registered
  manifest (Page, Dashboard, Widget, Report, Wizard, Kanban, Timeline, or
  Print — NOT Form/Table). `route` is an escape hatch for derived entity-list
  routes (`/<module>/<plural>`) or external URLs.
- Nesting capped at **3 levels**. Order of items = display order.

```yaml
kind: App
spec:
  modules: [clinic, pharmacy]
  root_url: /app/klinik
  menu:
    # Adopt: splice module's default menu suggestion
    - type: module
      module: clinic
    # Group with mixed children from different modules
    - label: "Farmasi"
      icon: "pill"
      children:
        - { label: "Antrian Resep", view: pharmacy-queue, module: pharmacy }
        - { label: "Semua Resep", route: /pharmacy/prescriptions, module: pharmacy }
    # Leaf: direct view (resolved server-side to /wizard/checklist-fill)
    - { label: "Isi Checklist", icon: "edit", view: checklist-fill-wizard, module: crc-field }
```

### Module — Bounded Context

A Module **owns** objects (Entity, Service, VisualSpecKind instances).
One Module = one complete business bounded context. Module structure is a
closed set: Entity, Service, and VisualSpecKind instances.

`spec.version` is **required**. Dependencies use `depends` (array of
`{module, version?}`), NOT `depends_on`.

**Module may provide a default menu suggestion** (`spec.menu`) — same
`MenuItem[]` type as App's menu, but **module-relative**: leaf nodes never
set `module` (it's implied = this module when adopted). `view` is the
manifest name within this module; `route` is a raw URL (typically
`/<module>/<plural>` for derived entity-list routes).

```yaml
kind: Module
metadata:
  name: clinic
spec:
  version: 1.0.0
  vendor: acme-corp
  depends:
    - module: formspec/core
  menu:
    - label: "Klinik"
      icon: "stethoscope"
      children:
        - { label: "Dashboard", icon: "layout-dashboard", view: clinic-dashboard }
        - { label: "Daftar Kunjungan", icon: "list", view: visits-page }
        - label: "Kasir"
          icon: "wallet"
          route: /clinic/payments
    - { label: "Isi Checklist", icon: "edit", view: checklist-fill-wizard }
```

**Menu structure rule (1–2 levels, always categorized).** Every menu
(`App.spec.menu` and `Module.spec.menu`) must be authored as a tree of
**Group nodes** (a `label` + `children`) with **Leaf nodes** underneath —
never as bare top-level leaves. The renderer nests any orphan leaf (a leaf
with no enclosing group) under the **last** group it saw, which silently
corrupts the navigation. Concretely:

- **Level 1 = category** (Group node: `label` + `children`).
- **Level 2 = item** (Leaf node: `label` + exactly one of `view`/`route`).
- Use a **third level only** when a category genuinely needs sub-groups
  (rare); nesting is capped at 3 levels.
- Every module's default menu suggestion should open with its own category
  group so adopted modules render as distinct top-level categories instead
  of collapsing into one.

Canonical 2-level module menu (each module = one or more categories):

```yaml
kind: Module
metadata:
  name: crc-field
spec:
  version: 1.0.0
  vendor: trakindo
  menu:
    - label: "Eksekusi"          # level 1 — category (Group node)
      icon: "clipboard-check"
      children:
        - { label: "Dokumen Checklist", icon: "clipboard-check", route: /crc-field/checklist-documents }  # level 2 — leaf
        - { label: "Isi Checklist", icon: "edit", view: checklist-fill-wizard }
```

A module with several concerns uses several categories (each a Group node):

```yaml
  menu:
    - label: "Laporan"
      icon: "bar-chart-3"
      children:
        - { label: "CRC Summary", icon: "layout-dashboard", view: crc-summary-dashboard }
        - { label: "Laporan Ringkasan", icon: "bar-chart-3", view: checklist-summary-report }
    - label: "Portal"
      icon: "globe"
      children:
        - { label: "Portal Customer", icon: "globe", view: customer-portal }
```

**view resolves ALL visual kinds** (via server-side registration):
Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Print.

Every visual kind has a `public` field (default `true`). When `public: true`,
the framework auto-generates a Page wrapper with route
`/<module>/<kind-lowercase>/<name>` — the kind can be navigated directly.
When `public: false`, the kind is embed-only (no standalone route; can only
appear inside an authored Page's blocks/tabs). Set `public: false` on
Forms/Tables that are meant to be used exclusively inside a Page.

**`public` field per visual kind:**

```yaml
kind: Form
metadata:
  name: quick-create-invoice
  module: billing
spec:
  public: true   # default — auto-Page route /billing/form/quick-create-invoice
  entity: billing.invoice
  mode: create
```

**Menu resolution flow:**
1. `App.spec.menu` defines the tree (authoritative).
2. `type: module` adopt nodes expand to `Module.spec.menu` (default
   suggestion) — App can freely override/restrict/rearrange.
3. Server resolves `view` → concrete `route` from the registered manifest.
4. `route` leaves are sent as-is (no server resolution).
5. If a module has no `spec.menu`, its adopt node expands to empty.
   The App then has no navigation entries for that module unless other
   leaves/groups reference it.

---

## Configuration, DDL, and Events

### Config — Module-Level Configuration

Module configuration, read via `ctx.config` in scripts.
**Not to be confused** with `formspec-app.yaml` (which is CLI dev/serve config).

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: billing-config
  module: billing
spec:
  data:
    tax_rate: 0.11
    currency: IDR
```

### Migration — DDL-Only Structural Changes

For custom indexes, triggers, or other DDL that Entity field definitions
don't cover. The framework computes structural diffs automatically — use
Migration only when you need something beyond what fields express.

```yaml
apiVersion: formspec.dev/v1
kind: Migration
metadata:
  name: add-invoice-index
  module: billing
spec:
  up: "CREATE INDEX idx_invoice_date ON invoice(transaction_date)"
  down: "DROP INDEX idx_invoice_date"
```

### Subscription — Cross-Module Event Reaction

React to events from another module's resources.

```yaml
apiVersion: formspec.dev/v1
kind: Subscription
metadata:
  name: on-invoice-submitted
  module: general-ledger
spec:
  events:
    - resource: billing.invoice
      action: submit
  handler:
    type: native
    ref: "GLHandler.OnInvoiceSubmitted"
```

---

## Business Process & API Surface

### Workflow — Multi-Approver Approval

Approval-based role gating attached to ONE Entity state-machine transition.
The states/transitions live on the Entity (`state_machine` above); the
Workflow only intercepts a single `from → to` transition and adds approval
steps. Never declare `states:`/`transitions:` inside a Workflow manifest.

```yaml
apiVersion: formspec.dev/v1
kind: Workflow
metadata:
  name: journal-posting-approval
  module: gl
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:
    - { roles: [gl.supervisor], approvers: 1 }
    - { roles: [gl.controller], approvers: 1,
        when: "resource.amount > 100000000" }
  on_reject: { to: rejected }
  escalation: { after: 48h, notify_roles: [gl.manager] }
```

Step fields: `roles` (eligibility), `approvers` (quorum, default 1),
`mode` (`all` | `any` | `sequential`), `when` (FormSpecExpr to skip a step),
`escalation` (`after`, `notify_roles`, `reassign_roles`).

### Api — External API Surface Override

Overrides the auto-generated REST API for an Entity.

### Webhook — Verified Inbound Endpoint

Declares inbound webhook endpoints with verification.

### Mockup — Integration Simulation

Simulates third-party integrations for testing.

### Integrator — Cross-Module Reactive Bridge

Reactive bridge between modules (e.g., sales→inventory, sales→GL).

---

## Renderer Kinds

### Renderer — Visual Renderer Implementation

Implements a VisualSpecKind for a specific shell/stack (e.g., React/shadcn).

### PersistBackend — Storage Renderer Implementation

Implements the storage seam (e.g., JSONB on Postgres/SQLite).

---

## Visual Kinds (Page Tier)

ALL of these are instances of `VisualSpecKind` with tier `page`.
They exist only to **override** the auto-derived defaults from Entity.

### Derived by Default

Every Entity automatically generates:
- CRUD REST API endpoints
- A `Table` (list/browse view)
- Two `Form`s (create, edit)
- A detail `Page`
- Menu entries in the admin panel

**You only need to declare visual kinds when you want to override these defaults.**

### Page — Route + UI Composition

Defines a route and composes UI from blocks, tabs, or full custom components.

### Form — Data Entry / Edit Layout

Overrides the input/edit layout for a specific Entity.
Uses `visible_when`, `readonly_when`, `required_when`, `compute` expressions (FormSpecExpr).

### Table — List / Browse View

Overrides the list view for a specific Entity.
Supports column selection, sorting, filtering, inline/batch editing.

### Dashboard — Widget Canvas

A grid of Widgets. Declared at module level, not per-entity.

### Widget — Single Dashboard Widget

A single tile on a Dashboard. Can be attached to an Entity for data binding.

### Report — Parameterized Tabular Report

Declared at module level. Parameterized query with output formatting.

### Wizard — Multi-Step Business Process

Guided multi-step process. Typically used for complex data entry workflows.

### Kanban — Drag-and-Drop Status Board

Visualizes Entity records as cards across status columns.
Supports drag-and-drop to change status.

### Timeline — Chronological Event Journal

Append-only chronological feed of events for an Entity.

### Calendar — Date-Based Entity View

Calendar view for Entity data with date fields (e.g., appointments, deadlines).

### Listing — Public Catalog

Public-facing listing, paired with `landing-page` App kind.

### ApprovalInbox — Pending Approval Task Queue

Shows pending approval tasks for the current user.

### NotificationCenter — In-App Notification Feed

In-app notification feed with read/unread state.

### Print — Printable Document

Defines a printable document template for Entity data.

### Theme — Look & Feel

CSS variables and styling configuration for the UI.

---

## Governance & Infrastructure

### Environment — Deployment Target

Declares a deployment target (dev, staging, production).
**Control Plane kind** — managed by Platform Operator.

### Policy — Governance Rules

Declares governance rules (security, compliance, resource limits).
**Control Plane kind** — managed by Platform Operator.

### Datastore — Named Infrastructure Connection

Declares a named database/object-storage connection.
**Control Plane kind** — managed by Platform Operator.

---

## Extension Kinds

### KindDefinition — Declare a New Kind

Extends the kind system. Used by official modules (e.g., `Seed`, `Schedule`,
`MailTemplate`) or third-party modules with namespaced kinds.

```yaml
apiVersion: formspec.dev/v1
kind: KindDefinition
metadata:
  name: Seed
  module: formspec/seed
spec:
  group: seed.formspec.dev
  version: v1
  scope: module
  handler:
    type: native
    ref: "FormaSeed.Apply"
```

---

## Choosing the Right Kind

| What you need | Kind to use |
|---------------|-------------|
| Store & manage transactional data | `Entity` (`characteristic: transaction`) |
| Stable reference data | `Entity` (`characteristic: master`) |
| Read-only seed data | `Entity` (`characteristic: reference`) |
| System-managed aggregates | `Entity` (`characteristic: summary`) |
| Computation without state | `Service` |
| Approval-based state transitions | `Workflow` |
| Inbound webhook endpoint | `Webhook` |
| Mock third-party integration | `Mockup` |
| Cross-module reactive bridge | `Integrator` |
| React to another resource's events | `Subscription` |
| Override external API surface | `Api` |
| Custom DDL (index, trigger) | `Migration` |
| Screen / route | `Page` |
| Data entry form | `Form` |
| List / browse table | `Table` |
| Multi-step process | `Wizard` |
| Drag-drop status board | `Kanban` |
| Chronological event feed | `Timeline` |
| Dashboard with widgets | `Dashboard` + `Widget` |
| Parameterized report | `Report` |
| Printable document | `Print` |
| Look & feel | `Theme` |
| Calendar view | `Calendar` |
| Public catalog | `Listing` |
| Approval task queue | `ApprovalInbox` |
| Notification center | `NotificationCenter` |
| Named DB/storage connection | `Datastore` |
| Deployment target | `Environment` |
| Governance rule | `Policy` |

---

## Gotchas

- **Entity characteristics are mutually exclusive.** `formspec apply` rejects more than one.
- **`summary` is a characteristic, NOT a fourth resource type.** It's still `kind: Entity`.
- **`doc_status` is a closed set**: `draft`, `submitted`, `cancelled`. Don't add custom statuses — use a separate field.
- **`delete` guard is absolute** (equivalent to `ON DELETE RESTRICT`). No `override_permission` can bypass it.
- **Update after submit is always denied, no exceptions.** Use named custom actions for post-submit field changes.
- **`transaction` characteristic REQUIRES** an explicit `transaction_date` field — `formspec apply` rejects if missing.
- **Visual kinds are overrides only.** You almost never need them — Entity auto-derives everything.
- **95% of cases: the answer is `Entity`.** Needing a new kind means extending the framework, not building an app.
- **Permission = resource + action.** Never hardcode role names in YAML.
- **`formspec-app.yaml` is CLI config, NOT a `kind: Config` manifest.**
- **`expose` is an ARRAY of `{type, actions}`** — the `all`/`read`/`none`
  shorthand does not exist and fails to unmarshal.
- **`target:` on a field is silently ignored** by the YAML loader, producing a
  dangling relation. Use `relation: { type: belongs_to, resource: <mod.entity> }`.
- **Module dependency key is `depends`** (array of `{module, version?}`), not `depends_on`.
- **Validate before you trust:** run `formspec validate --spec <dir>` (engine loader +
  JSON Schema) and rely on the editor `yaml.schemas` → `schemas/formspec.schema.json`
  for autocomplete/validation. `spec.version: v1` is required on every Entity.
- **`on:` is a normal YAML key** for Workflow (`on: { transition: ... }`) — do
  not quote it; only YAML 1.1 parsers (e.g. PyYAML) misread it as boolean `true`.
- **Menu: always provide `spec.menu` in Module if you use `type: module` adopt
  nodes in App.** An adopt node with an empty/null module menu produces no
  navigation entries — the module has zero sidebar visibility. If ALL modules
  in an App lack menus, the UI sidebar and default redirect will be empty.
- **`view` resolves ALL visual kinds (Page, Form, Table, Dashboard, Widget,
  Report, Wizard, Kanban, Timeline, Print).** Form and Table are now valid
  `view` targets — each gets an auto-derived Page wrapper with route
  `/<module>/form/<name>` or `/<module>/table/<name>` (unless `public: false`).
  No need to use `route` escape hatch for Forms/Tables anymore.
- **`public` (default `true`) on every visual kind** controls whether the kind
  gets a standalone route via auto-derived Page wrapper. Set `public: false`
  for embed-only Forms/Tables.
- **Menu nesting is capped at 3 levels.** Adopt nodes only at level 1; groups
  at levels 1–2; leaves at levels 2–3.
- **Table `default_sort` must reference an existing field** on the target entity.
  Check the entity's field list before setting `default_sort`. Framework-managed
  fields (`id`, `version`, `created_at`, `updated_at`, `created_by`, `updated_by`,
  `doc_status`) are always valid; custom fields like `modified` do NOT exist
  unless explicitly declared.
- **Dashboard widget `ref` uses just the widget name** — NOT `module.name`
  format. Example: `ref: doc-in-progress`, not `ref: crc-report.doc-in-progress`.
  The registry indexes widgets by `metadata.name` only. Module-qualified refs
  will fail validation with "widget ref not found" even when the widget file exists.

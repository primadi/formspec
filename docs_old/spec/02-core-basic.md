# Forma Core Basic Spec v0.3.0

**Status:** Draft — realigned under Forma Overview; Document Model integration
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (Decisions D1–D50)

> This document defines the **minimum specification** required to build a conforming Forma implementation. Features not listed here are defined in Core Extended, Control Spec, Frontend Spec, Plane Protocol Spec, and Marketplace Spec.

> **v0.3.0:** `kind: Entity` renamed to `kind: Document` (see §4.1). Introduces built-in document lifecycle (`doc_status` — draft/submitted/cancelled/NULL), eight reserved actions with framework-enforced guards (`create`, `update`, `submit`, `cancel`, `delete`, `amend`, `create-submit`, `amend-submit`), transaction date semantics, period closing, data archiving, and a canonical error glossary. All previous `kind: Entity` manifests remain loadable (backward-compatible deprecated path) but new manifests SHOULD use `kind: Document`.

---

## Part I — Foundation

### 1. Scope

Core Basic covers: multi-tenant CRUD applications with auto-generated API, admin panel, and docs; document lifecycle (draft / submitted / cancelled) with framework-enforced guards; background job processing; transactional event delivery between resources; transaction date semantics with period closing; data archiving and retention; type-safe code generation from manifests; business rules via sandboxed scripting (Starlark).

**Not in Core Basic:** Workflow, `kind: Integrator`, Webhook, Mockup, Hooks, Query Builder, streaming, file fields → Core Extended. Control Plane governance → Control Spec. Frontend kinds → Frontend Spec. Scheduler, mail, notifications, seeding → official modules (not spec). Boundary detection, saga/compensation → Core Basic §14d (spec) but deferred as implementation concern.

### 2. Core Philosophy

1. **Everything is a Resource.** Lifecycle: `Define → Persist → Act → Emit → Deliver`.
2. **One Definition, Many Protocols.** A manifest is the single source of truth for HTTP, WebSocket, admin panel, docs, and generated types.
3. **One Format for Everything.** Every concept uses `apiVersion/kind/metadata/spec` (Section 3). Tooling is generic.
4. **Three File Types Only.** `yaml` (description), `script` (logic), `asset` (static/custom UI). Nothing else.
5. **Convention over Configuration.** Sensible defaults; override only what you need.
6. **Security by Default.** Auth required; anonymous access must be explicit; tenant isolation automatic and non-bypassable; cross-tenant → 404.
7. **Location Transparency.** Callers never know where a resource runs — the registry resolves it.
8. **Contract before Implementation.** Manifest first, `impl` second.
9. **Lifecycle by Convention.** Every Document has a built-in lifecycle (`doc_status`: draft → submitted → cancelled) enforced by the framework. Developers define business-specific state machines on separate fields — the two layers are independent.

---

## Part II — Manifest Format & Resource Kinds

### 3. The Forma Manifest Format

All Forma YAML files contain one or more **manifests**. A manifest MUST have exactly four top-level keys:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Document
metadata:
  name: invoice
  module: billing
  description: Customer invoice    # recommended
spec:
  # kind-specific body
```

#### 3.1 `apiVersion`

`forma.dev/v1alpha1` for this spec version. Implementations MUST reject unknown apiVersions. Versions graduate `v1alpha1 → v1beta1 → v1`; within `v1`, changes are additive only.

#### 3.2 `kind`

PascalCase. Core Basic built-in kinds: `App`, `Module`, `Document`, `Service`, `Config`, `Migration`, `Subscription`. Additional kinds are defined in other specs and registered by modules via `KindDefinition` (Extended). Unknown kinds MUST fail validation.

> **Guardrail:** application developers should almost never define new kinds. In 95% of cases, the right answer is a `Document`.
> **Backward compatibility:** `kind: Entity` is accepted as a deprecated alias for `kind: Document`. Loaders MUST treat them identically but MAY emit a deprecation warning.

#### 3.3 `metadata`

| Key | Required | Rules |
|---|---|---|
| `name` | yes | kebab-case, unique per (kind, module) |
| `module` | yes* | owning module (*omit only inside a `kind: Module` manifest) |
| `description` | no | human/AI-readable summary |
| `labels` | no | string map, for selection/tooling |
| `annotations` | no | string map, tool-specific, never affects behavior |

#### 3.4 Multi-document files

A `.yaml` file MAY contain multiple manifests separated by `---`. Loaders MUST treat N files with one manifest each and one file with N manifests as identical.

### 4. Resource Kinds

#### 4.1 `kind: Document`

Stateful, persisted, source of truth for business data. Supports CRUD, built-in lifecycle, state machine, and events.

**Resource taxonomy — two types only:**

```
Resource (umbrella term — unchanged: "Resource Plane", resource.call())
├── type: document   → business term: "Dokumen"
│     characteristic:           # single value, mutually exclusive:
│       - master        → stable reference data (Customer, Product)
│                          MAY have lifecycle (if submit enabled) or not
│       - transaction    → append-heavy, time-partitioned (Invoice, JE)
│                          REQUIRES transaction_date field (§14a)
│       - reference      → read-only seed data, owned by App Owner (Provinces, Tax Rates)
│       - summary        → system-managed projection (GL Balance)
│                          create/update/delete via API permanently disabled
└── type: service    (unchanged)  → business term: "Layanan"
```

**Characteristic rules:**
- Exactly **one** value per Document (mutually exclusive). `forma apply` MUST reject documents with multiple characteristic values.
- Lifecycle behavior (`doc_status` / reserved actions) is **independent** of characteristic — it is controlled by `spec.actions.submit.disabled`. A `master` Document with `submit` enabled has a full lifecycle; a `master` with `submit` disabled does not.
- `summary` documents always have lifecycle bypassed (`submit` implicitly disabled).

`summary` is **not** a separate resource type — it is a characteristic value on `type: document`, exactly like `master`/`transaction`/`reference`. "Laporan" (report) is a Document with `characteristic: summary`, not a third resource type.

**API Exposure (D49):** Private by default. No external endpoint is created unless the document opts in via `spec.expose` — a per-protocol declaration (`rest`, `grpc`, `ws`). Without `expose`, the document is only accessible to internal callers (same-process services, Starlark scripts, events). See §11.1.

##### 4.1a Reserved Fields

The following fields are **reserved** — they MUST NOT be reused as custom field names. They apply automatically to all `type: document`, are set by the framework, and are read-only from developer code.

| Field | Function | Written manually by developer? |
|---|---|---|
| `owner` | Who created it | No |
| `created_at` | System timestamp — when the record was created, actual event order | No |
| `modified` | System timestamp — last modified | No |
| `doc_status` | Standard lifecycle status (see §4.1b) | No — only changes via reserved actions |
| `amends` | UUID of the cancelled original (set by `amend`, §4.1b) | No |
| `amended_by` | UUID of the new version (set on the cancelled original by `amend`, §4.1b) | No |
| `version` | Optimistic concurrency counter — auto-incremented on every update | No |
| `transaction_date` | Business date / accounting period (see §14a) — **MUST be explicitly declared** when `characteristic: transaction`. `forma apply` REJECTS if missing. NOT auto-injected like `doc_status`. | Yes, but subject to `backdate_policy` / `forward_date_policy` |

**Principle:** business fields are freely named by the developer (including names like `status`, `fulfillment_stage`, etc. — these are valid and encouraged so they don't conflict in meaning with the already-reserved `doc_status`).

##### 4.1b Reserved Actions — Standard Lifecycle

Six actions are **reserved words**. If an action name matches exactly, the standard guard activates automatically. Developers MAY add extra `conditions` on top, but CANNOT remove the base guard.

```yaml
doc_status:
  values: [draft, submitted, cancelled]   # CLOSED — no new states can be added.
                                            # Granular business process needs → separate field
                                            # (see "Order" example below)

actions:
  guards:
    create: "doc_status auto-set = draft"
    update: "doc_status == draft"
    submit: "doc_status == draft  →  doc_status = submitted"
    cancel: "doc_status == submitted  AND  no_pending_references  →  doc_status = cancelled"
    delete: "doc_status == draft  AND  no_referencing_documents"   # CANNOT be overridden
    amend:  "doc_status == submitted OR doc_status == cancelled  →
             if submitted: doc_status = cancelled, set amended_by
             →  create new linked Document (with amends link), start as draft"
```

**`delete` vs `cancel` — two different reference-guard strictness levels:**

- `delete` **removes the row entirely**. If another Document with a `relation` field points here, the guard is **absolute — no `override_permission`** — exactly `ON DELETE RESTRICT` in relational databases. This is data integrity violation (dangling reference), not a business process concern.
- `cancel` **does not remove the row**, only changes `doc_status`. The guard can be opened via a `before_cancel` handler that unwinds dependencies first (§4.1c), or via `override_permission` in certain cases.

The `delete` guard is purely based on the field type (`relation`) in other resources pointing here — applies automatically regardless of this resource's `doc_status`, regardless of whether it was ever `submit`-ted. Documents that are lifecycle-free (see §4.1d) are still protected from `delete` if referenced by other transactions.

**`update` after `submit` is always rejected — no exceptions.** This is what makes a Document "immutable" after submit. However, custom actions MAY still change specific fields after submit (e.g., `mark-paid` setting `paid_at`), as long as they go through a named path with explicit guards and are recorded in the audit log *by name* (not "document updated", but "action mark-paid executed").

**Lifecycle transitive gating:** Lifecycle reserved actions have a dependency chain: `submit ← cancel ← amend`. If `submit` is disabled, `cancel` and `amend` are **implicitly disabled** regardless of their explicit `disabled` value. If `cancel` is disabled, `amend` is implicitly disabled. The loader MUST enforce this at `forma apply` time — overriding `disabled` values where necessary — and MUST emit a warning when doing so. When all three (`submit`, `cancel`, `amend`) are disabled (explicitly or implicitly), the Document is **lifecycle-free**: `doc_status` is `null`, all lifecycle guards on `update`/`delete` are bypassed, and the Document behaves as plain CRUD (see §4.1d).

**`create-submit` — standard derived composite action (7th reserved action):** When both `create` AND `submit` are enabled (neither is `disabled: true`), a standard composite action `create-submit` is **automatically available** without needing to be declared in the manifest. It executes `create` + `submit` atomically in a single database transaction. Developers MAY override it with additional `conditions` but CANNOT weaken the base guards. `forma apply` MUST reject a declared `create-submit` action when `submit` is disabled.

**`amend-submit` — standard derived composite action (8th reserved action):** When both `amend` AND `submit` are enabled, a standard composite action `amend-submit` is automatically available. It executes `amend` (cancel original + create new draft + link) then `submit` (submit the new document) atomically — a single-click correction with immediate re-approval.

For custom multi-step actions beyond these built-ins, use an inline Starlark script that calls `ctx.call_action()` sequentially.

**Referenceability rule:** Only `doc_status = null` (lifecycle-free, §4.1d) and `doc_status = 'submitted'` documents can be targeted by `relation` fields created via `create`/`update`. Documents with `doc_status = 'draft'` or `'cancelled'` MUST be rejected as relation targets at runtime. This is enforced by the framework during `create`/`update` of the referencing Document.

**Amend version chain:** The `amend` action sets two reserved metadata fields:
- `amends` (on the new document) — UUID of the cancelled original
- `amended_by` (on the cancelled original) — UUID of the new version
Both are read-only, set by the framework, and form an audit trail of corrections. The `version` column (normative table §19) increments per amend cycle: 1 → 2 → 3.

**The reference guard (`cancel`) is generic based on field type, not manual self-awareness:** fields of type `relation`/reference — both standard fields and extension fields (`ext_*`) — are automatically included in the check "is this document still referenced by another submitted Document?" Documents don't need to write custom code for this.

**Never write to a controlled field directly** (`resource.doc_status = "x"`). If a custom action needs to trigger a transition, it **MUST** call the reserved action as a method (`ctx.call_action(resource, "submit")`), not write the raw field — so that guards, hooks, and audit are fully executed.

**Example: Base `doc_status` vs Granular Business Field**

```yaml
resource:
  name: order
  type: document

fields:
  - name: fulfillment_stage        # free name, does NOT conflict with doc_status
    type: enum
    enum_values: [awaiting_payment, paid, fulfilled]
  # doc_status is NOT written here — reserved, automatically present

state_machine:
  field: fulfillment_stage         # purely business, independent from doc_status
  initial: awaiting_payment
  transitions:
    - from: awaiting_payment
      to: paid
      via: mark-paid
    - from: paid
      to: fulfilled
      via: fulfill

actions:
  - name: mark-paid                 # free name -> custom, guard written manually
    conditions:
      - script: "doc_status == 'submitted'"
      - script: "fulfillment_stage == 'awaiting_payment'"
```

There is no `maps_to` between `fulfillment_stage` and `doc_status` — the two are independent. Any relationship between them (if needed) is expressed through ordinary `conditions` on actions, not a new mechanism.

##### 4.1c Multi-Step Actions via Inline Script

For custom actions that need to call multiple reserved or custom actions sequentially, write an inline Starlark script handler that calls `ctx.call_action()`:

```python
def handle(params, ctx):
    order = ctx.call_action("order", "create", params)
    ctx.call_action("order", "submit", {"id": order.id})
    return order
```

The framework executes each `ctx.call_action()` within the same request context. When the calls target resources in the same dataspace, the entire sequence runs inside a single database transaction — if any step fails, all prior changes roll back automatically via standard ACID properties. No separate composite action mechanism is needed.

For the specific case of `create` + `submit`, use the built-in `create-submit` action (§4.1b) which is auto-derived and requires no handler at all.

Cross-boundary calls (different dataspace/process) enter the Saga flow (§14d) for compensation.

##### 4.1d Master Data: Lifecycle-Free, No Fourth Resource Category

A `type: document` resource where all lifecycle actions (`submit`, `cancel`, `amend`) are disabled (explicitly or via transitive gating) is **lifecycle-free**. The Document has no `doc_status` — the framework does not enforce any lifecycle guards. `update` and `delete` are always allowed as long as other guards (notably referential integrity for `delete`) are satisfied.

This covers Master Data (Customer, Product) — documents that exist as reference data and never go through a draft→submitted lifecycle. No fourth resource category is needed: a Document with lifecycle disabled is zero-cost, no special type.

**Implementation detail:** `doc_status` is `null` in the database. The lifecycle column exists structurally but carries no semantics. All lifecycle reserved actions (`submit`/`cancel`/`amend`) are implicitly disabled.

Protection from dangerous `delete` (§4.1b, guard revision) still applies automatically based on the `relation` field type in other resources — it does not depend on lifecycle status. So no lifecycle-based guidance is needed — the reference guard on `delete` is strict enough on its own.

**Explicit signal for the UI generator (see Frontend Spec §1.7):** a resource that genuinely never intends to have a submit lifecycle should explicitly disable the lifecycle actions:

```yaml
resource:
  name: customer
  type: document
  characteristic: master

actions:
  - name: submit
    disabled: true
  # cancel and amend are implicitly disabled via transitive gating
```

This is not just documentation of intent — it's the signal the UI generator uses to decide between displaying plain CRUD (§Frontend 1.7) versus the draft→submit lifecycle pattern.

##### 4.1e Summary Documents: Bypass Lifecycle, Forever Active

`characteristic: summary` is not just a documentation hint — it **totally changes** how this Document is written. Standard actions `create`/`update`/`delete` are automatically **disabled via API** — only `list`/`find` are available to regular callers. Values are written **only** through the internal compute engine (trigger-driven), not through regular action calls from outside.

**Important consequence: `doc_status` is not meaningful for Summary.** The entire draft→submit→cancel lifecycle mechanism (§4.1b) operates through action calls, and Summary never receives external action calls — so the `doc_status` field that is normally auto-injected for all `type: document` has **no operational meaning** here. Implementations MAY keep it for structural consistency, but its value MUST remain static/ignored, not something that changes via submit/cancel like regular Documents.

```yaml
resource:
  name: gl-balance
  type: document
  characteristic: summary
  # Standard actions create/update/delete automatically disabled — only list/find
  # doc_status: NOT relevant, never changes via submit/cancel
```

Summary is **forever active, never archived** — if source transactions remain queryable (live or via archive access), Summary can be rebuilt. Archiving a Summary would turn it into a "static snapshot" that no longer reflects data reality — contradicting the purpose of Summary as a "live projection." See also §14a (period finalization) and Core Extended §26 (compute engine syntax).

#### 4.2 `kind: Service`

Stateless, pure computation. MUST NOT hold internal state. External integrations (SFTP, third-party APIs) MUST be wrapped as Services — auth, permission, audit, and tenant isolation apply uniformly. Services do NOT have `characteristics`, `doc_status`, or lifecycle guards — those are exclusive to `kind: Document`.

#### 4.3 Infrastructure primitives (closed set)

NOT kinds. Accessed only via `ctx`: `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage` (support: `ctx.config`, `ctx.kvstore`, `ctx.log`). Users cannot define new ones. Mail, notify, scheduler, seed are official modules built on top of primitives.

#### 4.4 `kind: App`

Root project manifest. Unit of deployment, trust boundary, and interface publication. A **Workspace MAY contain more than one App** — all Apps in a workspace run simultaneously in the same process, distinguished by `root_url`. The same Module MAY be mounted by more than one App in the same workspace (e.g. an internal App and a public-facing App both mounting the same business module, exposing a different subset of its views).

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: klinik-sehat-internal
spec:
  version: 2.1.0
  vendor: acme-corp
  root_url: /app/klinik-internal   # routing prefix, MUST be unique across all Apps in the workspace and start with "/app/" (the renderer SPA is only mounted there)
  modules: [billing, acme-corp/general-ledger]
  menu:                            # navigation tree for this App — see "Menu" below
    - type: module
      module: billing
  publishes:                      # cross-app interfaces offered
    - service: icd-lookup
      actions: [search, find]
  consumes:                       # cross-app interfaces needed → triggers grant requests
    - app: bpjs-gateway
      service: claims
      actions: [submit-claim]
```

Default private. Cross-app access only via publish → request → **grant approved by Data Owner**, recorded, revocable, metered.

**Menu ownership.** Menu belongs to the App, not the Module — "Module is the catalog, `App.spec.menu` is the shopping list from that catalog." Because different Apps mounting the same Module may expose different subsets of its views (internal vs. public), the menu enumeration has to live at the same level as that visibility decision, i.e. the App. To avoid burdening every App with wiring every item from scratch, a Module MAY ship a `spec.menu` default suggestion (§4.5) that an App can adopt wholesale via a `type: module` entry, and still override/restrict/rearrange freely.

**`MenuItem`** (used identically in `App.spec.menu` and `Module.spec.menu`):

```go
type MenuItem struct {
    Type     string      // "module" = adopt-shorthand node; empty = plain group or leaf
    Label    string
    Icon     string
    Module   string      // required on `type: module` nodes and on leaf nodes; forbidden on group nodes
    View     string      // name of a registered View (Page/Table/Wizard/Kanban/Dashboard/Report/Timeline)
    Route    string      // escape hatch: raw URL for a leaf with no registered View (external link, custom path)
    When     string       // FormaExpr business condition
    Order    int
    Children []MenuItem
}
```

Menu nesting is capped at **3 levels**, and every node MUST be exactly one of three shapes (validated at load time, not silently coerced):

1. **Adopt node** (`type: module`) — level 1 only. Requires `module`; optionally `order` (to reposition the adopted block within the App's list). Forbids `label`, `icon`, `view`, `route`, `children` — those all come from the target Module's own `spec.menu`. Effect: splices that Module's entire suggested menu tree in at this position.
2. **Group node** (has `children`) — level 1 (category) or level 2 (parent-menu). Requires `label` and a non-empty `children`. Forbids `module`, `view`, `route` **on the group itself** — a category may contain children belonging to different modules, so `module` is only meaningful on the leaves underneath, never on the group.
3. **Leaf/action node** (no `children`, not `type: module`) — level 2 or level 3. Requires `label`, `module`, and **exactly one** of `view`/`route`. A level-3 leaf MUST NOT have `children` — this is what enforces the 3-level cap.

Resolution rules:
- A leaf with `view` resolves its route from that View's own registration (a `Page` uses its own `route:`; `Dashboard`/`Widget`/`Wizard`/`Kanban`/`Timeline`/`Report`/`Print` use the `/<kind-lowercase>/<name>` convention, e.g. `/wizard/patient-registration`, `/kanban/consultation-board`) — the route is never duplicated into the menu item, so it can't drift out of sync. `Form` and `Table` are **not** valid `view` targets — the renderer never mounts a standalone route for them (they only appear embedded in a Page's blocks, or as the framework's derived per-entity CRUD routes); reference the `Page` that embeds them instead, or fall back to `route` for a derived entity-list link.
- A leaf with `route` uses it verbatim (unresolved) — for links with no backing View (external URLs, custom paths).
- Every `module` referenced anywhere in `App.spec.menu` (leaf or adopt node) MUST be a member of that App's own `spec.modules` — an App can never silently navigate into a Module it doesn't mount.

There is no standalone `kind: Menu` — it has been folded entirely into `App.spec.menu` (authoritative) and `Module.spec.menu` (default suggestion).

#### 4.5 `kind: Module`

Package of manifests — identity, version, dependencies only. Contents discovered by scanning, not listed. `metadata.name` is the permission namespace (no alias). Vendor path for registry/dependency resolution.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata:
  name: general-ledger
spec:
  version: 1.2.0
  vendor: acme-corp
  depends:
    - module: forma/core
    - module: billing
      version: ">=1.0 <2.0"
  config:
    default_currency: IDR
  menu:                            # default menu suggestion, module-relative — see §4.4 "MenuItem"
    - label: "Akuntansi"
      icon: calculator
      children:
        - { label: "Jurnal Umum", view: journal-list }
        - { label: "Buku Besar", view: ledger }
```

`Module.spec.menu` uses the same `MenuItem` shape as `App.spec.menu`, except `module` is implicit (the module's own name) and therefore omitted on every item — an App adopts it wholesale via a `type: module` entry (§4.4).

Module permission footprint is **derived** — the aggregate of `required_permission` + `uses` across all its manifests. `forma module install` MUST present this footprint for consent.

#### 4.6 `kind: Migration`

Developer-written **custom DDL only** (indexes, functions, triggers, extensions, materialized views). No DML — enforced at runtime. Structural migrations are derived automatically from Document diffs.

#### 4.7 Permission model

Two explicit axes on every action, for **all five impl types**:

```yaml
actions:
  close_period:
    required_permission: general-ledger.close-period   # caller guard
    uses:                                              # code's own access
      db: { write: [financial] }
      resources: [billing.invoice.read]
    impl: { type: native, ref: gl/close_period }
```

- Grants are never derived from usage. Runtime MUST reject `ctx.*` access outside declared `uses`.
- For `script`/`script_ref`, `forma validate` MUST scan: undeclared usage → error; declared-but-unused → warning. The scan verifies honesty, never grants.
- `ctx.auth.has(p)` MUST reference a declared permission; otherwise validation fails.
- Every permission string is fully qualified as `{module}.{key}`. Inside a manifest, own-module prefix MAY be omitted and MUST be auto-prefixed.

### 5. Project Layout & File Types

```
myapp/
  apps/
    internal.yaml                 # kind: App
    public.yaml                   # kind: App (second App, same workspace)
  modules/
    billing/
      module.yaml                 # kind: Module
      documents/invoice.yaml      # kind: Document
      services/tax-calculator.yaml
      scripts/invoice_send.star   # Starlark script
      assets/                     # static files, custom UI
  config/
    app.yaml                      # kind: Config
  impl/                           # Go source — build-time only, not deployed
    billing/
      invoice_handler.go
```

Folder names are convention. Loaders MUST discover by scanning `*.yaml`, not by path — nothing stops a workspace from keeping a single `forma.yaml` at the root instead of `apps/<name>.yaml`; `apps/` is the recommended layout once a workspace has more than one App. The only hard rule: three file types (`.yaml`, `.star`, `assets/*`). `impl/` is build-time only, committed, excluded from deployment artifacts.

### 6. Compilation & Process Model

Two binaries, always — including development: `forma-resource` (Resource Plane runtime) and `forma-control` (Control Plane: governance, policy, signing — see Control Spec). Planes communicate via Plane Protocol (ETag-based conditional pull, no persistent stream, no write-back).

#### 6.0 Two-Stage YAML Pipeline

YAML manifests MUST go through a **two-stage pipeline** in all environments (including dev). Direct filesystem loading by the Resource Plane is non-conformant.

**Stage 1 — Registration:** Developer runs `forma apply -f <path>` (or `forma apply --watch` for hot-reload). The Control Plane validates, computes sha256, signs, and stores the artifact in its database.

**Stage 2 — Deployment:** The Resource Plane pulls the desired-state snapshot from Control Plane via `GET /v1/snapshot` (ETag-conditional). It compares sha256 hashes against its local deployment manifest. Only changed artifacts are fetched, verified, and loaded.

**Dev mode** (`make dev` or `forma-resource --dev` + `forma-control --dev`):
- Both planes start on localhost
- `forma apply --watch` auto-detects file changes and re-registers
- Resource Plane polls every 10 seconds (prod: 5 minutes)
- `POST /v1/poll` trigger reduces latency to ~100ms
- Self-signed signatures, no approval required

**Architectural rule:** The `loader` and `entity registry` MUST NOT read YAML from the filesystem at runtime. All manifest data enters through the Control Plane artifact API.

See [Plane Protocol Spec §0](../06-plane-protocol.md) for full pipeline specification.

#### 6.1 Five implementation types

| Type | Form | Sandbox | Hot Update | Use When |
|---|---|---|---|---|
| `native` | Fused Go binary | No (full trust) | No | Performance-critical, stable |
| `compiled` | Go plugin `.so` / WASM | Partial (WASM) | Yes | Hot-reload without restart |
| `script` | Inline Starlark | Yes | Yes | Prototypes, small rules |
| `script_ref` | Starlark stored in DB, versioned | Yes | Yes — editable from admin panel, rollback | Rules that change often |
| `sidecar` | Container via Unix socket | Container; trust = native | Yes | Need another language ecosystem |

Resolution priority: `native > compiled > sidecar > script_ref > script`. Sidecar trust = native.

#### 6.2 `ref` resolution for `impl: native`

Format: `{TypeName}.{MethodName}` (e.g., `"InvoiceResource.Send"`). The framework scans `impl/**/*.go` for exported types and methods. Duplicate type names = compile error.

#### 6.3 Choosing `script_ref` vs `native` (non-normative guidance)

- Only reads/writes own resource fields or calls other Forma resources → `script_ref`
- Needs network/filesystem/external library → `native`

### 7. Config

Config is a manifest, not a dotenv file:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Config
metadata:
  name: app
  module: core
spec:
  keys:
    invoice_due_days: { type: int, default: 30 }
    smtp_host:        { type: string, secret: true }
```

Values resolved per environment. Secrets and environment definitions governed by Control Plane. Scripts read via `ctx.config.get("key")` — never raw env vars.

### 8. Tenancy Model — Workspace

Forma has exactly **one** multi-tenancy model:

```
Workspace → App → Module → Resource
```

- **Applications are 100% tenancy-blind.** No tenancy switch, no single/multi mode, no tenant code. `tenant_id` exists only as internal isolation mechanism.
- Every Document is workspace-isolated at the query level — no exceptions, no global storage. Cross-workspace → **404**.
- `characteristic: reference` is a domain marker: seeded per-tenant by App Owner, read-only for Data Owner. Live/large shared datasets → provider apps publishing services.
- Installing multiple apps into one Workspace unifies tenant identity — basis for cross-app grants.
- Data belongs to the owner of the Workspace where the resource runs. Expired module licenses degrade to read-only; `list/find/export/backup` MUST NOT be license-gateable.

---

## Part III — Resource Definition

### 9. Document & Service Anatomy

```yaml
apiVersion: forma.dev/v1alpha1
kind: Document                    # or Service
metadata:
  name: invoice                    # singular, kebab-case
  module: billing
spec:
  version: v1
  plural: invoices                 # optional, auto-derived
  characteristic: transaction   # Document only — §4.1
  auth: {}                         # §15
  audit: {}                        # §15
  persist: {}                      # §20
  expose: []                       # §11.1 — absent = no external access
  fields: []                       # Document only — §10
  actions: []                      # §11
  events: []                       # §12
  state_machine: {}                # Document only — §14
```

Note: `doc_status` is NOT listed under `fields:` — it is a reserved field (§4.1a), always present, framework-managed.

#### 9.1 Naming conventions

| Element | Convention | Example |
|---|---|---|
| `metadata.name` | singular, kebab-case | `invoice`, `purchase-order` |
| field names | snake_case | `due_date` |
| action names | kebab-case | `mark-paid` |
| event names | kebab-case, past tense | `payment-received` |
| permission keys | dot notation, module-qualified | `billing.invoices.send` |

### 10. Field Spec

Fields are valid only on `kind: Document`.

**Reserved field names** (§4.1a) — `owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version` — MUST NOT be reused as custom field names. `forma apply` MUST reject any field declaration using a reserved name. `transaction_date` MUST be explicitly declared when `characteristic: transaction` (§14a).

```yaml
fields:
  - name: string          # required, snake_case
    type: FieldType       # required
    description: string
    required: bool        # default false
    immutable: bool       # default false
    unique: bool          # default false, unique per tenant
    natural_key: bool     # default false
    natural_key_rule: {}  # §10.4
    default: any
    audited: bool         # default false
    index: bool           # default false
    rules: []             # §10.6
    relation: {}          # only type: relation
    child: {}             # only type: child
    enum_values: []       # only type: enum
```

#### 10.1 Field types

`uuid` (v7 default — time-ordered), `string`, `integer` (64-bit), `decimal` (**MUST be used for money, never float**), `boolean`, `date` (`YYYY-MM-DD`), `datetime` (ISO 8601 with timezone), `enum`, `json`, `child`, `relation`.

#### 10.2 Child vs Relation

The line between `child` and `relation` is determined by **lifecycle ownership**:

| Aspect | `child` | `relation` |
|---|---|---|
| Lifecycle | **Follows parent** — when parent is submitted/cancelled, all children follow automatically | **Independent** — own `doc_status`, own lifecycle |
| Identity | `storage: jsonb` → no UUID, embedded in parent. `storage: table` → own UUID v7. | **Own UUID v7** — independent identity |
| Existence | Cannot exist without parent — atomically created/deleted with parent | Can exist independently |
| Query ability | `jsonb`: via parent only. `table`: direct queries + joins. | Direct query via own table |
| Example | Invoice → line_items (jsonb or table), Customer → addresses (table) | Order → customer (both independent) |

**Decision test — does it have meaning outside the parent?**

- Invoice line items have no meaning without the Invoice → **`child`**
- Customer Addresses have no meaning without the Customer → **`child`** (stored as `table` for queryability, but still follows Customer's lifecycle)
- An Order refers to a Customer, but both exist independently → **`relation`**

**Lifecycle is the only real distinction.** A child follows the parent's lifecycle always. Even a child stored as `table` with its own UUID gets submitted when the parent submits, and cancelled when the parent cancels. The child's `doc_status` column mirrors the parent's — it is never directly managed by the developer.

#### 10.3 Child Spec

```yaml
- name: items
  type: child
  child:
    storage: jsonb            # jsonb (default) | table
    sequence_field: line_number
    fields:
      - { name: line_number, type: integer, immutable: true }
      - { name: product_id,  type: uuid, rules: [required] }
      - { name: quantity,    type: integer, rules: [required, positive] }
```

| | `jsonb` (default) | `table` |
|---|---|---|
| Storage | Embedded in parent's `data` JSONB | Separate PostgreSQL table |
| UUID PK | ❌ No | ✅ Yes (auto-generated UUID v7) |
| Atomic with parent | Always (same JSONB document) | Same transaction |
| Direct query/index on child | ❌ No | ✅ Yes |
| Referencable from other documents | ❌ No | ✅ Yes (via UUID) |
| Best for | <100 items, simple structure | Many items, needs direct query or reference |

When `storage: table`, the child table includes a UUID primary key:

```sql
CREATE TABLE {schema}.{parent_module}_{parent_plural}__{child_name} (
  id          uuid        PRIMARY KEY DEFAULT gen_uuid_v7(),
  parent_id   uuid        NOT NULL REFERENCES {parent_table}(id) ON DELETE CASCADE,
  seq         integer     NOT NULL,                        -- ordering counter
  doc_status  text        GENERATED ALWAYS AS ... STORED,   -- mirrors parent's doc_status
  ...child fields...
);
```

The `doc_status` column mirrors the parent's — child lifecycle is always derived from parent, never independent.

#### 10.4 Natural key & generation

```yaml
- name: number
  type: string
  natural_key: true
  immutable: true
  unique: true
  natural_key_rule:
    strategy: sequence            # sequence | custom
    format: "{prefix}-{year}-{seq:06d}"
    prefix: { config: billing.invoice_prefix, default: "INV" }
    reset: yearly                 # never | yearly | monthly | daily
    scope_field: branch_id        # optional — see below
```

Counters live in `forma_natural_key_counters` (PK = tenant/resource/field/scope/period). Increments MUST be atomic, gap-free, duplicate-free. MUST NOT derive via `MAX()` scan. `ctx.next_key` is a helper over `ctx.lock`. Sequence numbers are allocated under `ctx.lock` inside the same transaction as the insert/update; if the transaction later fails its optimistic-concurrency (`version`) check and is retried, a gap MAY result — unless the document declares gap-free mode, in which case the lock MUST be held until commit.

`scope_field` (optional) names a field on the same entity whose value becomes the counter's `scope` component — e.g. a document with `branch_id` and `scope_field: branch_id` gets one independent sequence per branch instead of one shared across the whole tenant. Omitted (the default) reproduces prior behavior — one counter per tenant/resource/field/period, with `scope` always empty. `scope_field` currently applies only to automatic natural-key generation on `create` (`db.EntityStore.Insert`) — an explicit `ctx.next_key(field)` call from a script always uses the tenant-wide scope regardless of `scope_field`, since that path has no resource data available to resolve the scope value from. Wiring `ctx.next_key` the same way is a known follow-up, not yet implemented.

#### 10.5 Relation Spec

```yaml
relation:
  type: belongs_to        # belongs_to | has_many | has_one
  resource: customer
  foreign_key: customer_id   # optional, auto-derived
  on_delete: restrict        # restrict (default) | cascade | set_null
```

**`on_delete` behavior when the target document is deleted:**

| Value | Behavior | Use case |
|---|---|---|
| `restrict` (default) | **Absolute.** Cannot delete target if any reference exists. Same guard as `delete` action (§4.1b). Cannot be overridden. | Default safest behavior — prevents data integrity violations. |
| `cascade` | Deletes the referencing document too. Only applies if the referencing document is `draft` or lifecycle-free. `submitted` documents require cancel first before cascade can proceed. | Dependent documents that should not outlive their parent (e.g., Order → Order Allocation). |
| `set_null` | Sets the foreign key to `null` on referencing documents. Only valid if the field is NOT `required`. | Optional references that should be cleared on deletion (e.g., Task → Assignee). |

**For `child` fields** (type: child), `on_delete` is not applicable — children are always cascade-deleted with the parent atomically.

#### 10.6 Field rules

Presence: `required`, `optional`. String: `min_length`, `max_length`, `pattern`, `email`, `url`. Numeric: `min`, `max`, `positive`, `precision`. Date: `future`, `past`, `after:<field>`, `before:<field>`. Collection: `min_items`, `max_items`. Cross-field: `unique`, `exists:<resource>`. Escape hatch: inline Starlark `script` with `message`.

### 11. Action Spec

```yaml
actions:
  - name: send
    description: Send invoice to customer      # required
    required_permission: billing.invoices.send # or "public"
    disabled: false                            # §11.1 — disable a standard action
    uses:                                      # §11.2
      resources: [customer.find]
      primitives: [queue]
    idempotent: true                           # §11.3
    idempotency_key: { from: param, field: event_id }
    audit: true
    emits: invoice-sent
    call: sync                                 # sync | async
    expose: [rest, ws]
    params: {}                                 # §13
    conditions: []                             # §13
    impl: { type: script_ref, ref: billing/invoice_send }
```

#### 11.1 Reserved actions (Document only)

Six actions are **reserved** — they have framework-enforced base guards (§4.1b). Developers MAY add extra `conditions` but CANNOT weaken or remove the base guard:

| Action | Base Guard | Post-condition |
|---|---|---|
| `create` | none | `doc_status = draft` |
| `update` | `doc_status == draft` | — |
| `submit` | `doc_status == draft` | `doc_status = submitted` |
| `cancel` | `doc_status == submitted AND no_pending_references` | `doc_status = cancelled` |
| `delete` | `doc_status == draft AND no_referencing_documents` | row removed (see §4.1b — guard is absolute, no override) |
| `amend` | `doc_status == submitted OR doc_status == cancelled` | atomic: cancels original + creates linked new Document as `draft` |
| `create-submit` | same as `create` + `submit` composited (auto-derived) | atomic: `create` + `submit` in one transaction |
| `amend-submit` | same as `amend` + `submit` composited (auto-derived) | atomic: `amend` (cancel+new) + `submit` new Document in one transaction |

`create-submit` is available only when both `create` AND `submit` are enabled (neither is `disabled: true`). It is automatically derived by the framework — no manifest declaration needed. Developers MAY override it with additional `conditions` but CANNOT weaken the base guards of either `create` or `submit`. `forma apply` MUST reject a declared `create-submit` when `submit` is disabled.

`amend-submit` is available only when both `amend` AND `submit` are enabled. Same auto-derivation rules.

For lifecycle-free documents (§4.1d) where `submit`, `cancel`, and `amend` are all disabled, the lifecycle guard on `update` and `delete` is also bypassed (since `doc_status` is `null`). The `delete` referential integrity guard still applies regardless.

**Standard CRUD endpoints** (when exposed via `spec.expose`) are the same reserved actions — `list` / `find` / `create` / `update` / `delete`. The exposure model (§§4.1, D49) remains deny-by-default; no external endpoint is created unless the Document opts in via `spec.expose`. When enabled, the following endpoints are generated per protocol:

| Protocol | Opt-in | Standard endpoints |
|---|---|---|
| REST | `expose: [{type: rest}]` | `/{plural}`, `/{plural}/:id`, etc. |
| gRPC | `expose: [{type: grpc, enabled: true}]` | Full gRPC service with matching RPCs |
| WebSocket | `expose: [{type: ws, enabled: true}]` | Subscription channel per document + action |

**REST endpoint details (when enabled):**

| Action | Method | Path | Default permission |
|---|---|---|---|
| `list` | GET | `/` | `{module}.{plural}.list` |
| `find` | GET | `/:id` | `{module}.{plural}.view` |
| `create` | POST | `/` | `{module}.{plural}.create` |
| `update` | PATCH | `/:id` | `{module}.{plural}.update` |
| `delete` | DELETE | `/:id` | `{module}.{plural}.delete` |

The `actions` field inside `expose` filters which endpoints are generated; omit to enable all applicable (except `delete`). `expose: []` (empty) is equivalent to `expose` absent — no external surface is generated; both are valid ways to keep a document internal-only. `characteristic: summary` Documents: create/update/delete permanently disabled even if listed (§4.1e).

Setting `disabled: true` on a reserved action removes it from every surface — equivalent to the action never existing. Custom actions are simply omitted instead.

**Override:** use `kind: Api` (Core Extended §2) for custom paths, versioning, or disabling specific endpoints on a per-surface basis. `kind: Api` can only modify already-exposed surfaces — it cannot create access where `expose` has not been set.

#### 11.2 `uses` vocabulary & enforcement

```yaml
uses:
  resources: [customer.find]             # cross-module: fully qualified
  db: { read: [billing], write: [billing] }
  config: { read: [billing.invoice_prefix] }
  kvstore:
    - { scope: tenant, access: read_write, module: billing }
  primitives: [cache, queue, lock, storage, pubsub]
```

**One rule: if not declared, it is blocked. Always.** Enforcement error codes: `CONFIG_ACCESS_DENIED`, `KVSTORE_ACCESS_DENIED`, `USES_VIOLATION`. `uses.db` defaults to own module; cross-module MUST be declared → consent footprint; cross-module `write` → high-risk consent. A `USES_VIOLATION` triggers: request blocked, alert, module auto-suspend, incident audit. Auto-suspend is scoped to the workspace where the violation occurred — the module keeps running in other workspaces; platform-wide suspension is an explicit Cloud Owner emergency action, never automatic.

#### 11.3 Idempotency & optimistic concurrency (normative, D32)

`idempotent: true` REQUIRES an `idempotency_key` source (`header` / `param` / `server`). Framework maintains idempotency store `(tenant, action, key) → pending|completed + stored response`. Duplicate after completed → replay original response. Duplicate while pending → wait/409. `from: server` → two-step prepare (browser double-submit protection). Entries expire by retention, never deleted on commit. Retention defaults to **24 hours** and is read from the global config key `core.idempotency_retention` (a `forma.core` `kind: Config` key — environment-resolvable, changeable at runtime without redeploy); implementations MUST NOT hard-code the window.

**Optimistic concurrency via `version`**, default-on for all Documents: update accepts version client read; mismatch → `409 CONFLICT` with current version. `updated_at` is audit metadata only.

#### 11.4 Async actions

`call: async` with `result_delivery` (websocket event, poll URL), `progress` (websocket), `timeout`, `retry`. Request returns `202` with job tracking info.

#### 11.5 `runtime_script` — observability overlay

Optional, runs alongside any impl. Core Basic scope: `after` timing only; read-only access to `resource` + `result`. Cannot modify results or control flow. Subject to same `uses`. Full Hook Spec is Extended.

### 12. Event Spec

Every event MUST be declared and linked from an action via `emits`.

**Event naming convention:** reserved prefixes lock the event type.

- `before_*` events (e.g., `before_cancel`, `before_submit`, `before_delete`) → **always `sync`**. They are gates that must complete before state changes — logically impossible to be async.
- `on_*` events (e.g., `on_cancel`, `on_submit`, `on_delete`) → **always `async`**. They occur after commit, pure notifications — logically impossible to be sync and cancel something already committed.
- Custom events NOT following the `before_*`/`on_*` pattern (e.g., `reconcile_needed`, `stock_low_alert`) → **`type` MUST be written explicitly**, because there is no signal from the name.

`forma apply` MUST **reject** any event whose `type` contradicts the prefix (e.g., `type: async` under `before_cancel`).

These rules automatically apply to all reserved actions (§11.1) — every reserved action has a paired `before_{action}` (sync) and `on_{action}` (async) without needing to be manually specified per action.

**Event handler priority (sync events only):** handlers on a sync event run in order of `priority` (smaller runs first). Use multiples of 10 so new handlers can be inserted between existing ones without renumbering.

| Tier | Range | When to use | Default (if not written) |
|---|---|---|---|
| **Critical** | 1–9 | Gate that must be checked first, before any business logic — e.g., fraud check, compliance block. Use sparingly. | — |
| **Normal** | 10–89 | Majority of business logic handlers — Integrator, custom hooks. | **10** |
| **Low** | 90–99 | Non-critical side-effects that must still be sync but order-unimportant vs other handlers — e.g., local cache update. | — |

```yaml
events:
  - name: invoice-sent
    publish:
      durable: true          # default false
    payload:
      fields: [id, number, total, customer_id]
    deliver:
      - channel: audit_log
      - channel: websocket
        target: { scope: tenant }
      - channel: queue
        job: send-receipt-email
      - channel: reliable_event
        target: { resource: gl.journal-entry, action: create }
        retry: { max: 10, backoff: exponential, initial_delay_ms: 1000 }
        dead_letter: { resource: failed-event, action: create }
        idempotency_key: "invoice.sent.{id}"
```

#### 12.1 Durability contract

`publish.durable: true` → event written to outbox before action returns. Document: same DB transaction as data change (atomic). Service: independent outbox. Reliability requires **both** sides: publisher durable + subscriber durable = reliable. Non-durable publisher + durable subscriber = validation error.

#### 12.2 Outbox (normative)

Implementations MUST provide `forma_outbox` table and a worker: poll pending → idempotency check → **sync call** to target action → delivered, or backoff retry → dead-letter (see §22 `failed-event`).

#### 12.3 `kind: Subscription` — subscribing from outside (D35)

Consumer module reacts to another resource's event without modifying the publisher:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata:
  name: wa-on-order-paid
  module: notifications
spec:
  on: { resource: billing.order, event: paid }
  deliver:
    - { channel: queue, job: send-wa-notification }
```

- Publisher's contract consequences stay in publisher's `deliver`; optional/third-party reactions → Subscription.
- `forma describe document <name>` MUST display merged fan-out (publisher deliver + all Subscriptions).
- Subscriptions enter consumer module's consent footprint.
- Two-sided durability contract applies unchanged.

### 13. Validation Spec

Three levels in Core Basic, evaluated in order:

1. **Field** — per-field `rules` (§10.6), automatic, before handler.
2. **Input** — per-action `params.validate`.
3. **State** — per-action `conditions`, Document only.

Error response: `{ error: { code, message, details: [{level, field?, message}] }, meta: { request_id } }`.

### 14. State Machine Spec

Document only. Forma has a **two-layer** state model:

1. **`doc_status`** — built-in lifecycle, framework-enforced (§4.1b). Three values: `draft`, `submitted`, `cancelled`. This is a CLOSED set — no new values can be added. Granular business process needs use a separate field (layer 2).
2. **User-defined state machine** — for business-specific processes on a separate field, independent of `doc_status`.

```yaml
state_machine:
  field: fulfillment_stage        # a business field, independent from doc_status
  initial: awaiting_payment
  transitions:
    - from: awaiting_payment
      to: paid
      via: mark-paid              # action name
      guard: "doc_status == 'submitted'"
    - from: paid
      to: fulfilled
      via: fulfill
```

The two layers are **independent** — there is no `maps_to` between a business field and `doc_status`. Any relationship between them (if needed) is expressed through ordinary `conditions` on actions, not a new mechanism.

Only declared transitions allowed; anything else → `STATE_TRANSITION_ERROR`. Guards are inline Starlark. Role-based approval on transitions is `kind: Workflow` (Extended).

#### 14a Transaction Date & Period Closing

For Documents with `characteristic: transaction`, a field named `transaction_date` (type: `date` or `datetime`) MUST be explicitly declared. `forma apply` REJECTS a `characteristic: transaction` Document without this field. Unlike `doc_status` (always auto-injected), `transaction_date` is explicitly declared so the developer is fully aware it exists and where its value comes from.

**`transaction_date` vs `created_at`:**

| | `created_at` (system date) | `transaction_date` (business date) |
|---|---|---|
| Function | Actual event order, audit | Which accounting/reporting period recognizes it |
| Can be manipulated? | No | Yes, subject to `backdate_policy` / `forward_date_policy` |
| Used for sequencing/audit? | **Yes, always** | **Never** |

Sequencing/audit is always based on `created_at`. `transaction_date` purely determines which reporting period recognizes it. Using `transaction_date` for sequencing causes cascading recompute on backdate — an expensive real-world problem.

**Backdate & Forward-date Policy:**

```yaml
# Global default (forma.yaml)
transaction_defaults:
  backdate_policy:
    max_days_back: 3
    override_permission: null       # null = nobody can override
  forward_date_policy:
    max_days_forward: 0             # most conservative default
    override_permission: accounting.post_forward_dated
  period_guard:
    enabled: true

# Per-resource override
resource:
  name: journal-entry
backdate_policy:
  max_days_back: 7
  override_permission: accounting.post_backdated
```

**Period Closing** is modeled as a **Document itself** (`period-closing`), not just a CLI command — so it gets all the built-in infrastructure for free: `doc_status`, reference guards, audit trail, permission model.

```yaml
resource:
  name: period-closing
  type: document
  characteristic: transaction

fields:
  - name: transaction_date
    type: date
  - name: period_ref
    type: relation
    relation: gl/gl-monthly-balance

actions:
  - name: submit                 # submit = trigger forma summary finalize
    conditions:
      - script: "all_reconciliations_done(period_ref)"
  - name: cancel                 # cancel = trigger forma summary unfinalize
    required_permission: accounting.reopen_period
    conditions:
      - script: "reason != ''"
```

Documents targeting a closed period need an explicit `period_guard.override_permission`.

#### 14b Data Archiving & Retention

**Only transactions are archived; masters are snapshot to preserve temporal consistency.** The principle: a view-only app archive must be self-contained and consistent — it must not query the live DB. When old transactions are archived, referenced masters are snapshot "as-of" the archive date and stored together in the archive in Parquet format.

**What gets archived:**
- **Transactions (`characteristic: transaction`)** — always archived when age ≥ `max_age`: Invoices, Payments, Journal Entries, Purchase Orders, Stock Movements, etc.
- **Masters (`characteristic: master`)** — ONLY snapshot, if referenced by an archived transaction. The snapshot is stored alongside the transaction archive; the master row in production remains intact (not deleted). Master is flagged `locked_for_deletion = true` while any archived transaction references it.

**`forma archive run`:**

```bash
forma archive run --max-age 3y --dry-run
# Scans for transactions older than cutoff date
# Identifies master references
# Shows plan: what gets archived, what masters get snapshot
# Requires operator confirmation

forma archive run --max-age 3y
# Executes archive: writes transactions + master snapshots to Parquet
# Sets locked_for_deletion flags on referenced masters
# Removes archived transaction rows from production (optional: --delete-masters for orphaned masters)
```

**Archive storage format:**

```
archive-2021-2023.parquet/
  manifest.yaml
    archive_date: 2023-07-08
    max_age: 3y
    record_count: ...
  transactions/
    journal_entries.parquet
    invoices.parquet
  masters/
    customers.parquet      # snapshot as-of archive_date
    products.parquet
```

**View-only access:** `forma archive view --batch-id archive-2021-2023` — queries Parquet directly, no live DB. **Restore:** only to staging environment (`forma archive restore-batch`), with dependency-ordered restore. Selective per-document restore is NOT supported (risk of corrupt state).

**Retention config:**

```yaml
# forma.yaml
retention:
  archive_after: "3y"           # calculated from transaction_date
  strategy: cold_storage        # cold_storage | delete
  destination: s3://archive-bucket
  master_delete_allowed: true   # allow orphan master deletion from live (requires approval)
```

Documents may opt out: `resource.retention.disabled: true`.

**Master locking fields** (auto-injected when archiving is active):

```yaml
fields:
  - name: locked_for_deletion    # set true when archived transactions reference this master
    type: boolean
    default: false
    immutable: true
  - name: archived_reference_count
    type: integer
    default: 0
    audited: true
```

Summary Documents (`characteristic: summary`) are **never archived** — if source transactions remain queryable, the summary can be rebuilt. Archiving a summary would turn it into a static snapshot that no longer reflects data reality.

#### 14c Error Glossary

Framework-enforced errors (reserved action guards, period locks, saga, etc.) use standardized error codes — not hardcoded inline messages — for consistency when multi-language support is needed. A single file (`forma-error-glossary.yaml`) distributed as part of Core Basic, versioned with Core Basic.

Format: `code` (serves both as programmatic matcher AND i18n lookup key — no separate `key` field needed), `params`, `default_message`.

```yaml
- code: FORMA.DOC.UPDATE_NOT_DRAFT
  params: [resource_name, doc_status]
  default_message: "{resource_name} cannot be modified because it is {doc_status}, not draft"

- code: FORMA.DOC.DELETE_REFERENCED
  params: [resource_name, blocking_resource, blocking_id]
  default_message: "{resource_name} cannot be deleted because it is still referenced by {blocking_resource} #{blocking_id}"

- code: FORMA.DOC.CANCEL_REFERENCED
  params: [resource_name, blocking_resource, blocking_id]
  default_message: "{resource_name} cannot be cancelled because it is still referenced by {blocking_resource} #{blocking_id}"

- code: FORMA.DOC.SUBMIT_NOT_DRAFT
  params: [resource_name, doc_status]
  default_message: "{resource_name} cannot be submitted because it is {doc_status}, not draft"

- code: FORMA.PERIOD.CLOSED
  params: [transaction_date, period_ref]
  default_message: "Cannot post to {transaction_date}, period {period_ref} is closed"

- code: FORMA.TXN.BACKDATE_EXCEEDED
  params: [transaction_date, max_days_back]
  default_message: "Transaction date exceeds backdate limit of {max_days_back} days"

- code: FORMA.TXN.TRANSACTION_DATE_MISSING
  params: [resource_name]
  default_message: "{resource_name} has characteristic: transaction but no transaction_date field declared"

- code: FORMA.DOC.CREATE_SUBMIT_NOT_AVAILABLE
  params: [resource_name]
  default_message: "create-submit is not available on {resource_name}: submit action is disabled"

- code: FORMA.SAGA.OUTCOME_UNKNOWN
  params: [event_name, target_resource]
  default_message: "Call to {target_resource} result cannot be determined after retries exhausted — manual verification required"
```

**Rules:**
- `code` is never changed or reused after release — third-party integrations that `switch(error.code)` must not silently break. Add new entries for new situations.
- Only covers errors from framework-enforced mechanisms. Custom `conditions:` errors from developers are free to use their own messages but SHOULD follow the same `code` + `params` format with an App-specific namespace (not `FORMA.*`).

#### 14d Saga & Manual Intervention

When a custom action invokes `ctx.call_action()` on a resource in a **different dataspace or process** (cross-boundary), the framework detects this at runtime and manages compensation through a Saga log. Same-dataspace calls benefit from standard ACID rollback — no Saga needed.

**Error classification:**

| Class | Nature | Handling |
|---|---|---|
| Business error | Definitive response (validation rejected) | Compensate immediately |
| Server error | Definitive response (500-class, but reply did arrive) | Compensate immediately |
| Network error | **Unknown** whether execution happened or not | Idempotent retry first, NOT immediate compensate |

Network errors must not trigger immediate compensation: if the remote side already succeeded (only the response was lost), compensating would create new inconsistency, not fix it.

**Requirements for cross-boundary calls:**
- Target action MUST be `idempotent: true` — `forma apply` rejects Integrators targeting non-idempotent actions.
- Idempotency key is generated once at the start (stored in `ctx.kvstore`), reused across all retries — never regenerated per attempt.
- After retries exhausted without definitive response → enters `outcome_unknown`, NOT assumed success or failure.

**Config (default + per-Integrator override):**

```yaml
# Global
integrator_defaults:
  retry:
    max_attempts: 5
    backoff: exponential
    base_delay_ms: 500
    max_delay_ms: 30000
  outcome_unknown_after: 5
```

**Manual intervention queue** — resource: `compensation-failure-log` (`forma.core`, `persist.category: compliance`).

| Sub-status | Meaning | Correct action |
|---|---|---|
| `compensation_failed` | Step failed, undo attempted, undo also failed | Human fixes manually, state is known |
| `outcome_unknown` | Unknown whether step succeeded, retries exhausted | Human **verifies actual state first** before any action — must NOT present an automatic retry/compensate button |

CLI: `forma saga list` / `forma saga resolve <id>`. No unbounded automatic retry — when a human is needed, the system does not pretend it can resolve itself.

---

## Part IV — Runtime

### 15. Security

#### 15.1 Auth

```yaml
auth:
  required: true              # default
  strategies: [token, api_key, session]
```

Anonymous access requires `required_permission: public` on the specific action. Roles defined in Module, assigned via `forma.core`.

```yaml
roles:
  - name: billing-admin
    permissions: [billing.invoices.*]
  - name: billing-viewer
    permissions: [billing.invoices.list, billing.invoices.view]
```

#### 15.2 Workspace isolation

All operations scoped to current workspace's tenant. Enforcement at query level. Cross-workspace access → **404**. Scripts see identity via `ctx.tenant.id`; can never set it.

#### 15.3 Cross-app grant enforcement

Cross-app call valid only if a **grant** exists: signed by provider's Data Owner, covering exact interface+actions, not revoked. Runtime MUST verify before routing; ungranted → **404**. Every cross-app call is metered against its grant and auditable.

#### 15.4 Audit

Every action with `audit: true` records: who, what, resource, workspace, before/after state, timestamp, IP, request ID.

### 16. API Delivery

- **Exposure model (D49):** deny-by-default. No external endpoint is created unless the document opts in via `spec.expose`. Internal callers (same-process services, Starlark scripts, events) are unaffected and always have access.
- **Multi-protocol:** when a document opts in via `spec.expose: [{type: rest} | {type: grpc} | {type: ws}]`, the router MUST generate protocol-specific routes for each declared type. REST and WebSocket are required transports; gRPC is recommended.
- **Workspace prefix:** every external route is prefixed with the workspace identifier. `workspace_slug` is a human-readable alias (configurable per Workspace); the router MUST fall back to the workspace UUID when no slug is set.
- **Internal dispatch:** same-process callers MUST bypass the network — the router detects registry locality and dispatches via direct function call. Cross-process callers use the configured protocol adapter.
- **Route scalability:** the router MUST use a radix-tree (or equivalent O(path segments)) lookup structure so that route count does not linearly degrade performance.
- **REST (required when exposed):** `/{workspace_slug}/api/{version}/{module}/{plural}/:id/:action`. Child: `/{workspace_slug}/api/.../items`.
- **WebSocket (required when exposed):** all actions; channel convention in §19.4.
- **Admin panel (recommended):** auto-generated from manifests.
- **Query conventions:** `?page&per_page&sort&direction&fields&filter[field][op]=value&search&include`. Filter operators: `eq neq gt gte lt lte between in nin like ilike null notnull`.
- **Pagination bounds:** `per_page` defaults to **20**, maximum **100**; values above the maximum are clamped to it; non-numeric or negative values → `VALIDATION_ERROR` (422). Implementations MAY lower the maximum per document but MUST NOT raise it above 100.

Response envelopes: list `{ data, meta: {page, per_page, total, total_pages}, links }`. Single `{ data, meta: {request_id, timestamp} }`. Error `{ error: {code, message, details}, meta }`.

Standard error codes: `VALIDATION_ERROR` (422), `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404, incl. cross-workspace & ungranted cross-app), `CONFLICT` (409), `STATE_TRANSITION_ERROR` (422), `USES_VIOLATION`/`CONFIG_ACCESS_DENIED`/`KVSTORE_ACCESS_DENIED`/`CROSS_CATEGORY_ACCESS_DENIED` (500-class, logged), `INTERNAL_ERROR` (500, reference ID only).

### 17. Communication Patterns

| Need | Pattern |
|---|---|
| Result needed now | `call: sync` (blocking) |
| Long-running (report, import, email) | `call: async` (job queue) |
| Financial/critical event | reliable event (`publish.durable: true`) |
| UI update, loss acceptable | WebSocket broadcast (non-durable) |

Async job flow: request → `202` with job tracking; progress/completion on WebSocket channel `jobs`; handlers report via `ctx.job.progress(pct, message)`.

### 18. Registry

Valkey/Redis, keys prefixed `forma.`: service info + TTL, resource definitions, instance sets, metrics. Registration at boot; heartbeat every 10s (30s TTL); graceful shutdown = deregistration + `service.down`. **Locality-aware routing** (mandatory order): same process → same host → same zone → any.

---

## Part V — Data & Operations

### 19. Persist Spec

Hybrid storage: business data in JSONB; indexed fields get generated columns.

```yaml
persist:
  table: string          # default "{module}_{plural}"
  soft_delete: true
  category: operational  # operational | financial | compliance | analytics | master | archive
  indexes:
    - { field: status, type: btree }
```

Categories map to PostgreSQL schemas. Cross-category SQL joins forbidden (`CROSS_CATEGORY_ACCESS_DENIED`). Primary key: **UUID v7** (time-ordered) for all documents. Natural keys = unique constraint per tenant, never the PK.

Normative table structure:
```sql
CREATE TABLE {schema}.{module}_{plural} (
  id          uuid        PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid        NOT NULL,
  version     integer     NOT NULL DEFAULT 1,      -- optimistic concurrency
  doc_status  text,                                 -- reserved lifecycle (§4.1b)
                                                    -- NULL = lifecycle-free (§4.1d)
                                                    -- 'draft' | 'submitted' | 'cancelled'
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  created_by  uuid, updated_by uuid,
  data        jsonb       NOT NULL DEFAULT '{}'
);
```

The `doc_status` column is auto-injected by the framework for all `kind: Document`. It is NOT declared in `fields:` — it is a reserved field (§4.1a). Valid values: `NULL` (lifecycle-free), `'draft'`, `'submitted'`, `'cancelled'`.

Indexed fields → generated columns: `_field VARCHAR GENERATED ALWAYS AS (data->>'field') STORED`. Natural key → `UNIQUE (tenant_id, _field) WHERE deleted_at IS NULL`.

### 20. Migration

Three types: (1) **Structural** — fully automatic from Document diffs, never hand-written. (2) **Custom DDL** — `kind: Migration` for indexes, functions, triggers. DML rejected at runtime. (3) **Data migration** — scaffolded scripts, run/rollback by version.

Field rename MUST be declared via `renamed_from` on the field — otherwise the diff interprets it as drop+add; field removal requires two steps (deprecate, then remove) across two applied versions. Backfills belong to type-3 data-migration scripts.

### 21. Workspace Provisioning

Lifecycle: `create → provisioning → seed default roles + reference seeds → active` (emits `workspace.activated`); `suspend ⇄ reactivate`; `terminate`. Per-workspace config via `ctx.tenant.config("key", default)`.

### 22. forma.core Resources

All implementations MUST provide: `workspace`, `user`, `app-membership` (per-app membership + role assignments), `role`, `role-assignment`, `api-key`, `session`, `job`, `audit-log` (append-only, framework-written, read-only API), `failed-event` (append-only dead-letter store, framework-written — records event payload, target, error, attempt count; replayable via an admin action), `setting` (documents); `health`, `metrics` (services).

### 23. CLI

Key verbs: `forma apply|diff|delete|get|describe|validate`, `forma new <app>`, `forma dev`, `forma generate`, `forma repl`, `forma migrate`, `forma seed`, `forma backup create|inspect`, `forma restore`, `forma module list|install|uninstall` (install presents consent footprint), `forma script validate|test`.

### 24. Dev Environment

`forma dev` = one command, Docker Compose: Postgres 16, Valkey/Redis, Mailpit, MinIO, `forma-control:dev` (relaxed policy). Startup: health checks → migrate → seed → hot reload → regenerate types on YAML change → dashboard.

### 25. Backup & Restore

Normative (Credible Exit Guarantee D31): format is part of this open spec. Backup: full or incremental, filterable, storage files included, summaries excluded. Restore: partial, `--map-resource`, per-record Starlark transform, conflict modes (skip/overwrite/remap — UUIDs remapped with all FKs), `--dry-run` compatibility report. **Must never be license-gated (D27).**

---

## Part VI — Scripting & Codegen

### 26. Scripting (Starlark)

Single scripting language. Entry points: `def validate(resource, params, ctx)` → `ok()` / `fail(msg|{field, message})`; Document action scripts `def execute(resource, params, ctx)` → result object (`resource` bound to the target record; absent for `create`-style actions); Service action scripts `def execute(params, ctx)`. Sandbox limits: 5000ms, 64MB, 100k iterations, no network/filesystem/subprocess, ≤50 db queries, ≤1000 records per execution.

Resource API: `invoice.load(id)`, `invoice.find_by_number(nk)`, chainable `invoice.query().where(...).include(...).get()/first()/count()`, `invoice.new().set(...).save()`, `inv.call("mark-paid", {...})`, field access `inv.field.total`, child helpers `inv.add_child/update_child/remove_child("items", ...)`.

Context: `ctx.user.{id,role,permissions}`, `ctx.tenant.{id,name,config()}`, `ctx.now()`, primitives (all gated by `uses`): `ctx.db`, `ctx.cache`, `ctx.lock` (incl. `ctx.next_key`), `ctx.queue`, `ctx.pubsub`, `ctx.storage`, `ctx.kvstore`, `ctx.config`, `ctx.log`.

### 27. Codegen

`forma generate` derives typed client/server types (Go; TypeScript for frontend), permission/enum constants, and OpenAPI documents. Generated code is never hand-edited; manifests are the source of truth.

---

## Part VII — Conformance

An implementation is Core Basic-conforming when it provides:

1. **Manifest loader:** `apiVersion/kind/metadata/spec`, unknown apiVersion/kind rejected, multi-doc ≡ multi-file, discovery by scan. `kind: Entity` accepted as deprecated alias for `kind: Document`.
2. **Kinds:** App, Module, Document, Service, Config, Migration with specs as defined; derived surfaces (HTTP, WebSocket, docs) from Document.
3. **Fields:** incl. child (jsonb+table), relation, natural key with atomic gap-free generation. Reserved field names (`owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version`) enforced — custom fields MUST NOT reuse them; `transaction_date` validated for `characteristic: transaction`.
3. **Fields:** incl. child (jsonb+table), relation, natural key with atomic gap-free generation. Reserved field names (`owner`, `created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version`) enforced — custom fields MUST NOT reuse them; `transaction_date` validated for `characteristic: transaction`.
4. **Actions:** eight reserved actions (`create`, `update`, `submit`, `cancel`, `delete`, `amend`, `create-submit`, `amend-submit`) with framework-enforced base guards (§4.1b, §11.1) — developers MAY add conditions but CANNOT weaken guards; auto-derived `create-submit` and `amend-submit` when applicable; five impl types; `required_permission` + `uses` with runtime enforcement and validate-time honesty scan; framework-enforced idempotency store with response replay and prepare step; optimistic concurrency via `version` CAS.
5. **Events:** event naming convention enforced (`before_*` = sync, `on_*` = async); durability contract; transactional outbox + worker with idempotent sync delivery; `kind: Subscription` with compiled fan-out and consent-time footprint.
6. **Validation:** levels 1–3 with normative error envelope; two-layer state machine (`doc_status` built-in lifecycle + user-defined business state machine); `STATE_TRANSITION_ERROR` for invalid transitions.
7. **Security:** auth strategies, RBAC catalog-on-declaration, workspace isolation at query level (404), cross-app grant verification, audit.
8. **Persist:** category schemas, UUID v7, normative table structure (including `doc_status` column), cross-category SQL block.
9. **Migration:** structural derivation from Document diffs; `kind: Migration` DDL-only execution.
10. **Operations:** forma.core resources (incl. `compensation-failure-log`); CLI verbs (incl. `forma archive run|view|restore-batch`, `forma saga list|resolve`); dev environment contract; backup format + restore (transform, remap, dry-run) — never license-gated.
11. **Scripting:** Starlark sandbox with stated limits; Document action scripts (with `resource` bound) and Service action scripts; script context.
12. **Lifecycle:** `doc_status` (draft/submitted/cancelled/NULL) enforced at runtime; lifecycle-free detection (submit/cancel/amend all disabled → `doc_status = NULL`, guards bypassed); referenceability rule (only `submitted`/`null` documents can be targeted by `relation` fields); amendment version chain (`amends`/`amended_by`); reference guard on `delete` (ON DELETE RESTRICT) and `cancel` (with `before_cancel` unwind path).
13. **Transaction date:** `transaction_date` field validation for `characteristic: transaction`; backdate/forward-date policy enforcement; period closing guard.
14. **Error codes:** standard `FORMA.*` error codes per the error glossary (§14c), returned in normative error envelope.

# Forma Core Basic Spec v0.2.0 (complete draft)

**Status:** Complete draft — all Parts I–VII rewritten; ready for review
**License:** Creative Commons CC0
**Repository:** github.com/forma-dev/spec
**Supersedes:** v0.1.9 (archived)
**Governed by:** Forma Foundation Document v1.9 (Decisions D1–D35)

> This document defines the minimum specification required to build a working
> Forma-compatible implementation. Features not listed here are defined in
> **Core Extended**, **Control Spec**, and **Plane Protocol Spec**.

---

## Table of Contents

### Part I — Foundation ✅ (this draft)
1. Introduction
2. Core Philosophy
3. The Forma Manifest Format (`kind`)
4. Resource Kinds
5. Project Layout & File Types
6. Compilation & Process Model
7. Config
8. Tenancy Model

### Part II — Resource Definition ✅ (this draft)
9. Entity & Service Anatomy · 10. Field Spec · 11. Action Spec ·
12. Event Spec · 13. Validation Spec · 14. State Machine Spec

### Part III — Runtime ✅ (this draft)
15. Security · 16. Delivery · 17. Communication Patterns · 18. Registry ·
19. Conventions

### Part IV–VII ✅ (this draft)
20. Persist · 21. Migration · 22. Workspace Provisioning · 23. forma.core ·
24. CLI · 25. Dev Environment · 26. Backup & Restore · 27. Scripting ·
28. Script Context · 29. Codegen · 30. Full Example (Order-to-Cash) ·
31. Conformance

---

# Part I — Foundation

## 1. Introduction

Forma is an open standard for building business applications. **Core Basic**
is the minimum subset required for a working implementation supporting:

- Multi-tenant CRUD applications with auto-generated API, admin panel, and docs
- Background job processing
- Transactional event delivery between resources (e.g. invoice → journal)
- Type-safe code generation from manifests
- Business rules via sandboxed scripting

### 1.1 Not in Core Basic

Deferred to **Core Extended**: hooks, validation levels 4–6, streaming event
delivery, storage/file field types (note: the `ctx.storage` primitive for
object storage read/write IS in Core Basic — see §4.3; only declaring a
`type: storage` field on an Entity is Extended), notification & webhook,
module registry, load balancing & circuit breaker, i18n, query builder,
frontend kinds (Page, Form, Table, Dashboard, Menu).

Defined in **separate specs**: Control Plane governance (`forma-control.md`),
inter-plane protocol (`forma-plane-protocol.md`).

Provided as **official modules**, not spec (Foundation D12): scheduler
(`forma/scheduler`), mail (`forma/mail`), notifications (`forma/notify`),
seeding & factories (`forma/seed`).

## 2. Core Philosophy

1. **Everything is a Resource.** Lifecycle: `Define → Persist → Act → Emit → Deliver`.
2. **One Definition, Many Protocols.** A manifest is the single source of truth
   for HTTP endpoints, WebSocket handlers, admin panel UI, API docs, and
   generated types.
3. **One Format for Everything.** Every concept is declared in the same
   manifest format (Section 3). Tooling is generic: validate, diff, apply.
4. **Three File Types Only.** `yaml` (description), `script` (logic),
   `asset` (static/custom UI). Nothing else. (Foundation D14)
5. **Convention over Configuration.**
6. **Security by Default.** Auth required; anonymous access must be declared;
   tenant isolation automatic and non-bypassable; cross-tenant access → 404.
7. **Location Transparency.** `resource.call("invoice", "send", input)` —
   the registry resolves where it runs; the caller never knows.
8. **Contract before implementation.** Manifest first, `impl` second.

## 3. The Forma Manifest Format (`kind`)

All Forma YAML files contain one or more **manifests**. A manifest MUST have
exactly these four top-level keys:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: invoice
  module: billing
spec:
  # kind-specific body
```

### 3.1 `apiVersion`

`forma.dev/v1alpha1` for this spec version. Implementations MUST reject
unknown apiVersions with a clear error. Version graduates
`v1alpha1 → v1beta1 → v1` as the spec stabilizes; within `v1`, changes are
additive only.

### 3.2 `kind`

PascalCase. Core Basic built-in kinds: `App`, `Module`, `Entity`, `Service`,
`Config`, `Migration`, `Subscription`. The catalog is governed by the concern map
(Foundation Appendix B) and extensible in three layers (Foundation D18):

1. **Spec built-ins** — this document and sibling specs (Extended:
   `Workflow`, `Api`, `Webhook`, `Page`, `Form`, `Table`, `Dashboard`,
   `Menu`, `Print`, `Mockup`; Control: `Environment`, `Policy`).
2. **Official modules** — register kinds via `KindDefinition` (CRD-like;
   mechanism defined in Extended): `Seed`, `Factory`, `Schedule`,
   `MailTemplate`, `NotificationChannel`.
3. **Third-party modules** — namespaced kinds, subject to Verified Badge.

Unknown kinds MUST fail validation — never silently ignored. Guardrail
(non-normative): application developers should almost never define kinds;
needing a new kind means extending the framework — in most cases the right
answer is an `Entity`.

### 3.3 `metadata`

| Key | Required | Rules |
|---|---|---|
| `name` | yes | kebab-case, unique per (kind, module) |
| `module` | yes* | module the manifest belongs to (*omit only inside a Module manifest itself) |
| `description` | no | human/AI-readable summary |
| `labels` | no | string map, for selection/tooling |
| `annotations` | no | string map, tool-specific, never affects behavior |

### 3.4 `spec`

Kind-specific body. Defined per kind in this document and sibling specs.

### 3.5 Multi-document files

A `.yaml` file MAY contain multiple manifests separated by `---`. Loaders
MUST treat N files with one manifest each and one file with N manifests as
identical. Splitting across folders/files is purely a concern-separation
choice for humans.

## 4. Resource Kinds

### 4.1 `kind: Entity`

Stateful, persisted, source of truth for business data. Supports CRUD,
state machine, events. Examples: `invoice`, `customer`, `order`,
`currency-rate`.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: invoice
  module: billing
spec:
  characteristics: [transaction]   # master | transaction | reference | summary
  # tenant-isolated by default (§8) — no tenant block needed
  fields: {}        # Part II
  actions: {}       # Part II
  events: {}        # Part II
  state_machine: {} # Part II
  permissions: {}   # Part II
```

`summary` entities: no create/update/delete via API — system-managed only.

### 4.2 `kind: Service`

Stateless, pure computation. MUST NOT hold internal state or storage; data a
service needs lives in an Entity it reads at execution time. External system
integration (SFTP, legacy ERP, third-party API) MUST be wrapped as a Service —
never as custom infrastructure — so auth, permission, audit, and tenant
isolation apply uniformly.

### 4.3 Infrastructure primitives (closed set)

NOT kinds. Users cannot define them. Accessed only via `ctx`:
`ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage`
(support: `ctx.config`, `ctx.kvstore`, `ctx.log`). Common needs on top of
primitives (mail, notify, scheduler, seed) are official modules — the closed
set never grows for convenience. (Foundation D5, D12)

`ctx.storage` (the primitive for reading/writing objects to blob storage)
is Core Basic. Declaring a `type: storage` field on an Entity — which would
expose a file upload/download surface on the auto-generated API — is
Extended.

### 4.4 `kind: App`

The root manifest of a project. An App is the unit of **deployment, trust
boundary, and interface publication** — it is installed into a Workspace,
composes Modules, and everything in it is private by default.

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: klinik-sehat
  description: Clinic management
spec:
  version: 2.1.0
  vendor: acme-corp
  modules:                        # composed modules (local and marketplace)
    - billing
    - acme-corp/general-ledger
  publishes:                      # cross-app interface offered — default: nothing
    - service: icd-lookup
      actions: [search, find]
  consumes:                       # cross-app interfaces required — triggers
    - app: bpjs-gateway           # grant requests at install time, shown in
      service: claims             # the same consent screen as module footprint
      actions: [submit-claim]
```

Cross-app access model (Foundation D25, D27): consumer apps declare needed
interfaces in `consumes`; at install time the provider's **Data Owner
grants** them — recorded, revocable, metered. Ad-hoc requests after install
follow the same grant flow. Data always belongs to the owner of the
Workspace where the resource runs; expired module licenses degrade to
read-only, and `list/find/export/backup` MUST NOT be license-gateable, ever.

### 4.5 `kind: Module`

A Module is a package of manifests — identity, version, and dependencies
only. It does NOT list its contents (loaders discover by scanning, §5) and
does NOT alias its namespace.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata:
  name: general-ledger          # identity AND permission namespace
  description: Double-entry accounting core
spec:
  version: 1.2.0
  vendor: acme-corp             # registry/dependency path: acme-corp/general-ledger
  depends:
    - module: forma/core
    - module: billing
      version: ">=1.0 <2.0"
  config:                       # module-level defaults, app-overridable
    default_currency: IDR
```

Rules (Foundation D19):
- Permission namespace **is** `metadata.name`: `general-ledger.journal.post`.
  No alias field exists.
- `metadata.name` MUST be unique within an installed application; the vendor
  path is used for fetch/dependency resolution (go.mod analogy).
- A module's permission footprint is **derived**, never declared: the
  aggregate of `required_permission` + `uses` across all its manifests
  (§4.6). `forma module install` MUST present this footprint for consent.

### 4.6 `kind: Migration`

Developer-written **custom DDL only** (indexes, functions, triggers,
extensions, materialized views). Structural migrations are derived
automatically from Entity diffs and never hand-written. No DML — enforced
at runtime. Full spec in Part IV.

### 4.7 Permission model (normative summary — full spec in Part II)

Two explicit axes on every action, for **all five impl types** without
exception (Foundation D20):

```yaml
actions:
  close_period:
    required_permission: general-ledger.close-period   # caller guard
    uses:                                              # code's own access
      db: { write: [financial] }
      resources: [billing.invoice.read]
    impl: { type: native, ref: gl/close_period }
```

- Grants are never derived from usage — declaration is a human-approved
  contract; usage is behavior constrained by it.
- Runtime MUST reject `ctx.*` access outside declared `uses` (enforced via
  the identity proxy for native/sidecar, sandbox for scripts).
- For `script`/`script_ref`, `forma validate` MUST scan and report:
  undeclared usage → error; declared-but-unused → warning. The scan
  verifies honesty; it never grants.
- `ctx.auth.has()` MUST reference a declared permission; otherwise
  validation fails. No phantom permissions.

## 5. Project Layout & File Types

```
myapp/
  forma.yaml                      # kind: App (root — deployment & trust boundary)
  modules/
    billing/
      module.yaml                 # kind: Module
      entities/invoice.yaml       # kind: Entity
      services/tax-calculator.yaml
      scripts/invoice_send.star   # script (Starlark)
      assets/                     # static files, custom UI bundles
  impl/                           # Go source for impl: native/compiled (build-time only)
    billing/
      tax_calculator.go
      invoice_handler.go
  config/
    app.yaml                      # kind: Config
```

Folder names above are convention, not contract: loaders MUST discover
manifests by scanning `*.yaml`, not by path. The only hard rule is the three
file types (`.yaml`, `.star`, `assets/*`).

### 5.1 `impl/` directory (build-time only)

`impl/` contains Go source code for `impl: native` and `impl: compiled`
handlers. It is **build-time only** — committed to the repository but
**excluded from deployment artifacts**.

| Directory | Git | Deploy artifact | Contents |
|---|---|---|---|
| `spec/` (manifests + `.star`) | ✅ committed | ✅ included | YAML manifests, Starlark scripts, assets |
| `impl/` | ✅ committed | ❌ excluded | Go source for native/compiled handlers |
| `.forma/` | ❌ git-ignored | ❌ excluded | Compiled output (`.forma/build/`), cache |

During `forma build`, `impl/` is compiled and the resulting binary is fused
into the `forma-resource` runtime. During `forma deploy`, only `spec/` and
the compiled binary are shipped — `impl/` source is never sent to production.

During `forma dev`, `impl/` is compiled on-the-fly and hot-reloaded when
source files change (alongside `.star` scripts which are natively
hot-updatable).

## 6. Compilation & Process Model

Two binaries, always — including development (Foundation D3):

| Binary | Role |
|---|---|
| `forma-resource` | Resource Plane runtime: loads manifests, serves protocols, executes impl |
| `forma-control` | Control Plane: environments, policy (OPA), signing, approval, audit — see `forma-control.md` |

The planes communicate only via the Plane Protocol (mTLS, policy pull on
boot + 5-minute refresh, no write-back). A Resource Plane keeps serving with
last-known policy if Control is unreachable.

### 6.1 `impl` — five implementation types

Every action/business-rule declares how it is implemented:

```yaml
impl:
  type: script_ref        # native | compiled | script | script_ref | sidecar
  ref: billing/invoice_send
```

| Type | Form | Sandbox | Hot update | Notes |
|---|---|---|---|---|
| `native` | Fused Go binary | no (full trust) | no | Buildable locally by anyone (D8 — no Compile Service) |
| `compiled` | Go plugin / WASM | partial (WASM) | yes | |
| `script` | Inline Starlark | yes | yes | prototypes, small rules |
| `script_ref` | Starlark stored, versioned | yes | yes — editable from admin panel, rollback | rules that change often |
| `sidecar` | Other-language container via Unix socket | container; trust = native | yes | PHP/Python/Node/Java ecosystems |

Resolution priority: `native > compiled > sidecar > script_ref > script`.
Sidecar trust model equals native — identity proxy and Signed Query Registry
apply regardless of language (Foundation D6).

### 6.2 `ref` resolution for `native` and `compiled`

For `native` and `compiled` types, `ref` uses the format
`{TypeName}.{MethodName}`:

```yaml
impl: { type: native, ref: "PaymentGateway.CreateSession" }
```

The framework resolves this by scanning all `*.go` files under `impl/` for
an exported type `PaymentGateway` with an exported method `CreateSession`
matching the action's signature. Resolution rules:

1. **Type name** (`PaymentGateway`) MUST be unique across the entire `impl/`
   tree — duplicate type names across packages are a compile error.
2. **Method name** (`CreateSession`) MUST be an exported method on that type.
3. **Method signature** MUST match the action's declared params and return
   types (validated at `forma build` time).
4. The file path is irrelevant — only the Go type + method name matter.
   Developers MAY organize `impl/` files however they choose.

For `compiled` (Go plugin / WASM), the same resolution applies, with the
additional requirement that the plugin binary exports the type.

### 6.3 Choosing `script_ref` vs `native` (non-normative)

| Kebutuhan | Gunakan | Alasan |
|---|---|---|
| Update field + save | `script_ref` | Ringan, hot-updatable dari admin panel |
| Orchestration ringan (panggil 1-2 resource lain) | `script_ref` | Cukup ekspresif, tidak perlu compile |
| Validasi sederhana | `script_ref` | `conditions` + script handler |
| HTTP call ke API eksternal | `native` | Sandbox Starlark tidak punya network |
| Komputasi / business rule kompleks | `native` | Performa, type safety, debugging |
| File generation (PDF, image) | `native` | Butuh library Go |
| Integrasi dengan sistem legacy | `native` atau `sidecar` | Tergantung ekosistem |

Litmus test: kalau handler hanya membaca/menulis field resource sendiri
atau memanggil resource Forma lain, `script_ref` cukup. Kalau butuh
network, filesystem, atau library eksternal, harus `native`.

## 7. Config

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

Values are resolved per environment; secrets and environment definitions are
governed by Control Plane (`forma-control.md`). Scripts read via
`ctx.config.get("invoice_due_days")` — never via raw env vars.

**Environment awareness:** the active environment name is available at
runtime via `ctx.environment` (e.g. `"dev"`, `"staging"`, `"production"`).
Service-to-Mockup routing is resolved by the framework based on
environment-scoped config (see Core Extended: `kind: Mockup`). Handler code
MUST NOT branch on `ctx.environment` — the framework routes to mockup or
real connector transparently.

## 8. Tenancy Model — Workspace

Forma has exactly **one** multi-tenancy model (Foundation D29):

```
Workspace → App → Module → Resource
```

- **Applications are 100% tenancy-blind.** There is no tenancy switch, no
  single/multi mode, no tenant code in apps. `tenant_id` exists only as the
  runtime's internal isolation mechanism, keyed to the Workspace.
- Every Entity is workspace-isolated — **no exceptions, no global storage**
  (Foundation D26). Isolation is enforced at the query level; application
  code cannot bypass it. Cross-workspace access returns **404**.
- `characteristics: [reference]` is a domain marker only: data is seeded
  per-tenant, owned by the App Owner (shipped/updated via releases),
  read-only for Data Owners. Live/large shared datasets are **provider
  apps** publishing services, never shared tables.
- Installing multiple apps into one Workspace unifies tenant identity across
  them — the basis for cross-app grants (§4.4).
- Ownership rule (Foundation D27): data belongs to the owner of the
  Workspace where the resource runs.
- Large tenants wanting their own servers run their own Forma Cloud under an
  enterprise license (they become the Platform Operator) — not a different
  tenancy mode.

---

# Part II — Resource Definition

## 9. Entity & Service Anatomy

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity                       # or Service
metadata:
  name: invoice                    # singular, kebab-case
  module: billing
  description: Customer invoice    # required — used in docs
spec:
  version: v1                      # API version — "v1", "v2"
  plural: invoices                 # optional — auto-derived
  characteristics: [transaction]   # Entity only

  auth: {}                         # AuthSpec (Part III)
  audit: {}                        # AuditSpec (Part III)
  persist: {}                      # Entity only (Part IV)

  fields: []                       # Entity only — §10
  actions: []                      # §11
  events: []                       # §12
  state_machine: {}                # Entity only — §14
```

### 9.1 Entity characteristics

| Value | Nature | Consequences |
|---|---|---|
| `master` | Stable business data (customer, product) | standard audit |
| `transaction` | Append-heavy, grows (invoice, journal-entry) | time partitioning; natural key recommended |
| `reference` | Read-only seed data (country, tax-code) | seeded per tenant; content owned by App Owner, read-only for Data Owner (§8) |
| `summary` | System-managed projection (gl-balance) | no create/update/delete via API; updated via reliable events |

### 9.2 Naming conventions

| Element | Convention | Example |
|---|---|---|
| `metadata.name` | singular, kebab-case | `invoice`, `purchase-order` |
| field names | snake_case | `due_date` |
| action names | kebab-case | `mark-paid` |
| event names | kebab-case, past tense | `payment-received` |
| permission keys | dot notation, module-qualified | `billing.invoices.send` |

**Permission qualification:** every permission string is fully qualified as
`{module}.{key}`. Inside a manifest, the own-module prefix MAY be omitted
and MUST be auto-prefixed by the loader; references to other modules MUST
be written fully qualified.

## 10. Field Spec

Fields are valid only on `kind: Entity`.

```yaml
fields:
  - name: string          # required, snake_case
    type: FieldType       # required
    description: string
    required: bool        # default false
    immutable: bool       # default false
    unique: bool          # default false — unique per tenant
    natural_key: bool     # default false — §10.4
    default: any
    audited: bool         # default false
    index: bool           # default false
    rules: []             # §10.6
    relation: {}          # only type: relation
    child: {}             # only type: child
    enum_values: []       # only type: enum
```

### 10.1 Field types

`uuid` (v7 default — time-ordered), `string`, `integer` (64-bit),
`decimal` (arbitrary precision — **MUST be used for money, never float**),
`boolean`, `date` (`YYYY-MM-DD`), `datetime` (ISO 8601 with timezone),
`enum`, `json`, `child`, `relation`.

### 10.2 Child vs Relation — the modeling decision

**`child`** — no UUID of its own; key = `parent_id + sequence`; no identity,
lifecycle, or access outside the parent; created atomically with the parent.
Examples: invoice → line_items, order → items.

**`relation`** — a *separate* entity with its own UUID, independent identity
and lifecycle, associated via foreign key. NOT parent-child. Examples:
order → customer, customer → addresses, product → variants.

Decision test: does it have meaning outside the parent? Yes → relation.

### 10.3 Child Spec

```yaml
- name: items
  type: child
  child:
    storage: jsonb            # jsonb | table — default jsonb
    sequence_field: line_number
    fields:
      - { name: line_number, type: integer, immutable: true }
      - { name: product_id,  type: uuid,    rules: [required] }
      - { name: quantity,    type: integer, rules: [required, positive] }
      - { name: price,       type: decimal, rules: [required, positive] }
```

| | `jsonb` | `table` |
|---|---|---|
| Atomic with parent | always | same transaction |
| Direct query/index on child | no | yes |
| PK | — | composite (parent_id, sequence) — never UUID |
| Best for | few, simple items | many items, direct queries |

**When to use `jsonb` vs `table`:**

- **`jsonb`** (default) — child items are few (<100 per parent record),
  always accessed together with the parent, and never queried independently.
  Examples: order → line items, invoice → tax breakdown.

- **`table`** — child items are many (hundreds+ per parent) or need
  independent queries, indexes, or aggregation. Examples: journal entry →
  lines (query by account_id across all journals), stock movement → lines
  (aggregate quantity by product).

Rule of thumb: if you'd ever write `SELECT ... FROM child WHERE ...` without
joining through the parent, use `table`.

### 10.4 Natural key & generation

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
    # strategy: custom → script_ref returns the string;
    # atomicity still guaranteed by the framework (ctx.next_key)
```

Normative guarantees: counters live in a dedicated table
(`forma_natural_key_counters`, PK = tenant/resource/field/scope/period);
increments MUST be atomic, gap-free, duplicate-free. Implementations MUST
NOT derive the next value via `MAX()` scan. Reset boundaries evaluate in the
tenant's timezone. `ctx.next_key` is a helper over `ctx.lock` — it does not
expand the primitive closed set.

### 10.5 Relation Spec

```yaml
relation:
  type: belongs_to        # belongs_to | has_many | has_one
  resource: customer
  foreign_key: customer_id   # optional, auto-derived
```

FK lives in this entity's table for `belongs_to`; in the other entity's
table for `has_many`/`has_one`.

### 10.6 Field rules

Presence: `required`, `optional`. String: `min_length`, `max_length`,
`pattern`, `email`, `url`, `strip_html`. Numeric: `min`, `max`, `positive`,
`precision`. Date: `future`, `past`, `after: <field>`, `before: <field>`.
Collection: `min_items`, `max_items`. Cross-field: `unique`,
`exists: <resource>`. Escape hatch:

```yaml
- script: "value > resource.issued_date"
  message: "Due date must be after issued date"
```

## 11. Action Spec

```yaml
actions:
  - name: send
    description: Send invoice to customer      # required
    method: POST                               # default derived
    path: /send                                # override, default derived
    required_permission: billing.invoices.send # caller guard — or "public"
    uses:                                      # code's own access — §11.3
      resources: [customer.find]
      primitives: [queue]
    idempotent: false
    idempotency_key: { from: param, field: event_id }   # required when idempotent — §11.8
    audit: true
    emits: invoice-sent                        # event name (§12)
    call: sync                                 # sync | async
    expose: [http, websocket]                  # default both
    params: {}                                 # §11.4
    conditions: []                             # §11.5
    impl: { type: script_ref, ref: billing/invoice_send }
    runtime_script: {}                         # §11.6
    async: {}                                  # only when call: async — §11.7
```

### 11.1 Standard actions (Entity only)

Auto-generated; can be disabled (`disabled: true`) or overridden by
redeclaring the name.

| Action | Method | Path | Default permission |
|---|---|---|---|
| `list` | GET | `/` | `{module}.{plural}.list` |
| `find` | GET | `/:id` | `{module}.{plural}.view` |
| `create` | POST | `/` | `{module}.{plural}.create` |
| `update` | PATCH | `/:id` | `{module}.{plural}.update` |
| `delete` | DELETE | `/:id` | `{module}.{plural}.delete` |

`summary` entities: create/update/delete permanently disabled.

### 11.2 `impl` — discriminated form

```yaml
impl:
  type: native      # native | compiled | script | script_ref | sidecar
  ref: "InvoiceResource.Send"        # native: Go symbol
  # compiled: { ref, runtime: go_plugin|wasm, checksum: sha256:... }
  # script:   inline Starlark body under `source: |`
  # script_ref: "billing/invoice.send" — stored, versioned, rollback
  # sidecar:  { ref: container-name/handler }
```

Exactly one impl per action. Table & resolution priority: Part I §6.1.

### 11.3 `required_permission` + `uses` (normative — D20)

- `required_permission` guards the **caller**. `uses` declares what the
  **implementation** accesses: `db` (read/write per persist category),
  `resources` (other resources' actions, fully qualified across modules),
  `primitives` (lock, queue, pubsub, cache, storage, kvstore).
- Explicit for **all five impl types**. Runtime MUST reject `ctx.*` access
  outside declared `uses`. For script/script_ref, `forma validate` scans:
  undeclared usage → error, unused declaration → warning.
- `ctx.auth.has(p)` — `p` MUST be a declared permission somewhere in the
  module; otherwise validation fails.
- Module footprint = aggregate of all `required_permission` + `uses`
  (install-time consent, D21).

### 11.4 Params validation

```yaml
params:
  validate:
    - { field: customer_id, rules: [required, uuid, {exists: customer}] }
    - { field: items,       rules: [required, {min_items: 1}] }
    - script: "params.due_date > params.issued_date"
      message: "Due date must be after issued date"
```

### 11.5 Conditions (state validation)

```yaml
conditions:
  - script: "resource.status == 'draft'"
    message: "Only draft invoices can be sent"
```

### 11.6 `runtime_script` — observability overlay

Optional, runs **alongside** any impl (including native). Core Basic scope:
`after` timing only; receives `resource` + `result` **read-only**; cannot
modify results or control flow; same resource + action only; subject to the
same `uses` declaration. Purpose: instrumentation without recompiling.
Full Hook Spec (before/on_error, cross-resource, wildcard) is Extended.

### 11.7 Async actions

```yaml
call: async
async:
  result_delivery:
    - { channel: websocket, event: report-ready, session_from: header }
    - { channel: poll, enabled: true, ttl: 3600 }
  progress: { enabled: true, channel: websocket }
  timeout: 300
  retry: 3
  retry_backoff: exponential
```

### 11.8 Idempotency & optimistic concurrency (normative — D32)

**Idempotency is framework-enforced, never hand-rolled in handlers.**
`idempotent: true` REQUIRES an `idempotency_key` source:

```yaml
idempotency_key:
  from: header            # header (Idempotency-Key) | param | server
  field: event_id         # when from: param
```

- Runtime maintains an **idempotency store**:
  `(tenant, action, key) → status pending|completed + stored response`.
- Duplicate after completed → **replay the original response** (a retried
  webhook receives its 200 again — never a "already processed" error).
- Duplicate while pending → wait or `409`.
- `from: server` — two-step **prepare**: client requests a key
  (`POST /:resource/:action/prepare`), then submits with it; protects
  browser double-submit on create.
- Entries expire by retention policy, never deleted on commit — the replay
  window MUST outlive the transaction. Reliable-event delivery (§12.3/12.4)
  uses this same store on the outbox side.

**Optimistic concurrency via `version`** (column §20.3), default-on for all
Entities: `update` (standard and custom) accepts the version the client
read (`If-Match` header or `expected_version` param); mismatch →
`409 CONFLICT` with the current version in the response. `updated_at` is
audit metadata only — implementations MUST NOT use timestamps as
comparison tokens (clock resolution, NTP, non-monotonicity).

## 12. Event Spec

Entity and Service both emit events. Every event MUST be declared and
linked from an action via `emits` — names are never auto-derived.

```yaml
events:
  - name: invoice-sent
    description: Invoice was sent to the customer
    publish:
      durable: true          # default false
    payload:
      fields: [id, number, total, customer_id]   # Entity
      # schema: {...}                             # Service (no fields)
    deliver: []              # §12.3
```

### 12.1 Durability contract

`publish.durable: true` → event written to outbox **before** the action
returns. Entity: same DB transaction as the data change (invoice + outbox
record = atomic). Service: independent outbox, stored before returning, not
transactional with any entity data. `durable: false` → in-memory; lost on
crash; fine for UI refresh/cache invalidation.

Subscribers declare `durability: durable | non_durable`. Reliability
requires **both** sides:

| Publisher | Subscriber | Result |
|---|---|---|
| durable | durable | ✅ reliable |
| durable | non_durable | ⚠️ stored, subscriber may miss |
| non_durable | durable | ❌ impossible — implementation MUST warn |
| non_durable | non_durable | ✅ fire-and-forget by agreement |

### 12.2 Standard events (Entity only)

`created`, `updated`, `deleted` auto-emitted; default non-durable; make
durable by redeclaring with `publish.durable: true`.

### 12.3 Delivery channels

```yaml
deliver:
  - channel: audit_log
  - channel: websocket
    target: { scope: tenant }          # tenant | user (+ user_field)
  - channel: queue                     # event consequence as background job (D33)
    job: send-receipt-email            # handler receives the event payload
  - channel: reliable_event            # requires publish.durable: true
    target: { resource: gl.journal-entry, action: create }   # cross-module: qualified
    retry: { max: 10, backoff: exponential, initial_delay_ms: 1000 }
    dead_letter: { resource: failed-event, action: create }
    idempotency_key: "{resource_name}.{event_name}.{id}"
```

The declarative/imperative boundary (Foundation D33): consequences of an
event belong here in `deliver`; steps *inside* an action belong in its
`impl`. The deliver vocabulary is closed — new business cases never add
YAML syntax.

**Cross-module target qualification:** targets referencing a resource in
another module MUST use the fully qualified form `{module}.{resource}`
(e.g. `gl.journal-entry`). Targets within the same module MAY omit the
module prefix; the loader resolves unqualified names to the current module.

Realtime per-entity subscription convention (D10/PocketBase): Extended.

### 12.4 Outbox (normative)

Implementations MUST provide the `forma_outbox` table
(tenant_id, source_resource, nullable source_id, event_name, payload,
target, unique idempotency_key, status pending|delivered|dead_letter,
attempt, deliver_after) and a worker: poll pending → idempotency check →
**sync call** to target action → delivered, or backoff retry until
dead-letter. Sync call from the worker is what makes invoice → journal
consistency exact.

### 12.5 `kind: Subscription` — subscribing from outside (D35)

A consumer module reacts to another resource's event **without touching the
publisher's manifest** — the structural prerequisite for a module
ecosystem, since signed marketplace modules cannot be edited (D24).

```yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata:
  name: wa-on-order-paid
  module: notifications              # the CONSUMER module
spec:
  on: { resource: billing.order, event: paid }
  deliver:                           # exact same closed vocabulary as §12.3
    - { channel: queue, job: send-wa-notification }
```

Normative rules:

- **Ownership line:** consequences that are the publisher's own contract
  (e.g. billing promises a journal entry) stay in the publisher's
  `deliver`; optional, third-party, or added-later reactions are
  Subscriptions.
- **Resource qualification:** `on.resource` follows the same rules as
  `deliver` targets (§12.3): cross-module references MUST use
  `{module}.{resource}` (e.g. `billing.order`); same-module references
  MAY omit the module prefix.
- **Compiled fan-out:** `forma describe entity <name>` and the admin panel
  MUST display the merged consequence map — publisher `deliver` plus every
  Subscription targeting it, across all modules. Scattered files never mean
  scattered understanding.
- Subscriptions are part of the consumer module's **footprint**: shown at
  install-time consent (D20) as "this module reacts to billing.order.paid".
- The **two-sided durability contract (§12.1) applies unchanged**: a
  durable Subscription on a non-durable event MUST fail validation.
- Cross-module within an app = install consent; cross-app = the same
  concept elevated to a grant (D25).

## 13. Validation Spec

Three levels in Core Basic, evaluated in order; levels 4–6 (business rules,
cross-resource, consistency) are Extended.

1. **Field** — per-field `rules` (§10.6), automatic, before the handler.
2. **Input** — per-action `params.validate` (§11.4).
3. **State** — per-action `conditions` (§11.5), Entity only.

Error response (normative shape):

```json
{ "error": { "code": "VALIDATION_ERROR", "message": "Validation failed",
    "details": [ { "level": "field", "field": "email", "message": "..." },
                 { "level": "state", "message": "..." } ] },
  "meta": { "request_id": "req-uuid" } }
```

## 14. State Machine Spec

Entity only.

```yaml
state_machine:
  field: status
  initial: draft
  transitions:
    - from: draft                # string or list
      to: sent
      via: send                  # action name
      guard: "len(resource.items) > 0 and resource.total > 0"
    - from: [draft, sent]
      to: void
      via: void
```

Only declared transitions are allowed; anything else →
`STATE_TRANSITION_ERROR`. Guards are inline Starlark evaluated against the
current record.

**Approval boundary (D10):** guards answer "may this transition happen
now?"; they do NOT model "who must approve it". Role-based approval on
transitions is `kind: Workflow` (Extended), which attaches to an Entity's
state machine without modifying it — simple lifecycles stay inline, approval
chains live in Workflow.

---



# Part III — Runtime

## 15. Security Spec

### 15.1 Auth

```yaml
auth:
  required: bool              # default true
  strategies: [token, api_key, session]   # PASETO recommended for token
```

Anonymous access is never implicit: it requires
`required_permission: public` on the specific action.

### 15.2 Caller permissions & RBAC

`required_permission` (§11.3) is both declaration and publication: the
moment an action declares it, the string enters the module's permission
catalog for role assignment — implementations MUST NOT require a second
registration step. Roles are defined in the Module manifest and assigned
via `forma.core` role-assignment:

```yaml
roles:
  - name: billing-admin
    permissions: [billing.invoices.*, billing.payments.*]
  - name: billing-viewer
    permissions: [billing.invoices.list, billing.invoices.view]
```

Wildcards match within one module segment only.

### 15.3 `uses` vocabulary & enforcement

Full vocabulary for the `uses` block (§11.3). **One rule: if not declared,
it is blocked. Always** — enforced at runtime for all five impl types.

```yaml
uses:
  resources:                       # other resources' actions
    - customer.find                # same app; cross-module fully qualified
  db: { read: [billing], write: [billing] }        # module-scoped
  config:
    read: [billing.invoice_prefix, tenant.timezone]
    write: []                      # code-side writes need strong justification
  kvstore:
    - { scope: tenant, access: read_write, module: billing }
    - { scope: tenant, access: read_only,  module: inventory }  # cross-module: explicit
  primitives: [cache, queue, lock, storage, pubsub]
```

Enforcement (normative error codes): undeclared config read/write →
`CONFIG_ACCESS_DENIED` / `CONFIG_WRITE_DENIED`; config marked
`secret: true` is never readable via `ctx.config` (→
`CONFIG_SENSITIVE_ACCESS_DENIED`; access only via `ctx.secrets`, Extended);
kvstore outside declared module/scope → `KVSTORE_ACCESS_DENIED` (keys
auto-namespaced `{tenant}:{module}:{key}`); any other undeclared `ctx.*` →
`USES_VIOLATION`. Config writes from code are always audit-logged.
Finer-grained/pattern-based infrastructure scoping: Extended.

### 15.4 Workspace isolation

All operations — entity reads and service executions alike — are scoped to
the current workspace's tenant (§8). Enforcement MUST be at the query
level, not the application level. Cross-workspace access returns **404**,
never 403. Scripts see identity via `ctx.tenant.id`; they can never set it.

### 15.5 Cross-app grant enforcement (D25, D31)

- A cross-app call is valid only if a **grant** exists: signed by the
  provider's Data Owner (self-custodied key), covering the exact
  interface + actions, not revoked.
- Runtime MUST verify the grant before routing; calls to unpublished or
  ungranted interfaces return **404** (existence is not disclosed).
- Every cross-app call is metered against its grant (revenue-sharing
  basis, D22) and auditable by both sides.
- Grant documents are portable: both parties hold signed copies +
  transparency-log inclusion proof (D30/D31).

### 15.6 Audit

```yaml
audit: { enabled: true, immutable: true }   # defaults for Entity
```

Every action with `audit: true` records: who, what, resource, workspace,
before/after state, timestamp, IP, request ID.

## 16. Delivery Spec

- **HTTP (required):** `/api/{version}/{module}/{plural}` →
  `/api/v1/billing/invoices/:id/send`; child:
  `/api/v1/billing/invoices/:invoice_id/items`.
- **WebSocket (required):** all actions, protocol in §19.4.
- **Admin panel (recommended):** auto-generated from manifests; not
  required for Core Basic conformance.
- **Cross-app calls** ride the same protocols with grant verification
  (§15.5); wire details in `forma-plane-protocol.md`.

## 17. Communication Patterns

Three patterns; selection guide:

| Need | Pattern |
|---|---|
| Result needed now | `call: sync` (default) — blocking |
| Long-running (report, import, EOD, bulk, email) | `call: async` — job queue |
| Financial/critical event | reliable event (`publish.durable: true`, §12) |
| UI update, loss acceptable | WebSocket broadcast (non-durable event) |

### 17.1 Async job flow (normative wire shapes)

Request returns `202` immediately:

```json
{ "data": { "job_id": "job-uuid", "status": "pending" },
  "meta": { "track": { "websocket_event": "job.completed",
                        "poll_url": "/api/v1/jobs/job-uuid" } } }
```

Progress and completion push on channel `jobs` with
`event: progress|completed|failed` and payload
`{ job_id, progress?, message?, status?, result? }`. Handlers report via
`ctx.job.progress(pct, message)` and return the result object.

### 17.2 Reliable event delivery

Configuration and durability contract: §12. Runtime behavior: outbox worker
(§12.4) delivers via **sync call** to the target action with idempotency
check — this is what makes invoice → journal-entry consistency exact.
Applies to Entity (transactional outbox) and Service (independent outbox)
alike.

## 18. Registry Spec

Valkey/Redis, all keys prefixed `forma.`:

```
forma.service:{service-id}                  Hash — instance info + TTL
forma.resource:{name}:{version}             Hash — resource definition
forma.resource:{name}:{version}:instances   Set  — hosting instances
forma.service:{service-id}:resources        Set  — resources per instance
forma.instance:{service-id}:metrics         Hash — latency, error rate
forma.registry.events                       Pub/Sub — service.up / service.down
```

- **Registration** at boot: instance info (id, name, host, port, zone, pid)
  + all hosted resources.
- **Heartbeat** every 10s refreshes a 30s TTL; expiry = auto-deregistration;
  graceful shutdown = active deregistration + `service.down`.
- **Locality-aware routing** (mandatory order): same process (direct call,
  zero network) → same host (loopback) → same zone → any. Location
  transparency is mandatory: callers never know where a resource runs.

## 19. Convention Spec

Conventions are defaults and MUST be pluggable.

### 19.1 Query convention

```
GET /api/v1/billing/invoices
  ?page=1&per_page=25              # max 100
  &sort=created_at&direction=desc
  &fields=id,number,total
  &filter[status]=draft
  &filter[total][gte]=1000000
  &filter[created_at][between]=2024-01-01,2024-12-31
  &search=INV-001
  &include=customer,items
```

Filter operators: `eq neq gt gte lt lte between in nin like ilike null notnull`.

### 19.2 Response envelopes

List: `{ data: [], meta: {page, per_page, total, total_pages}, links: {first, prev, next, last} }`.
Single/mutation: `{ data: {}, meta: {request_id, timestamp} }`.
Error: `{ error: {code, message, details: []}, meta: {request_id, timestamp} }`.

### 19.3 Standard error codes

| Code | HTTP |
|---|---|
| `VALIDATION_ERROR` | 422 |
| `UNAUTHORIZED` | 401 |
| `FORBIDDEN` | 403 |
| `NOT_FOUND` (incl. cross-workspace & ungranted cross-app) | 404 |
| `CONFLICT` | 409 |
| `STATE_TRANSITION_ERROR` | 422 |
| `USES_VIOLATION`, `CONFIG_ACCESS_DENIED`, `KVSTORE_ACCESS_DENIED` | 500-class, logged |
| `INTERNAL_ERROR` (reference ID only, no internals) | 500 |

### 19.4 WebSocket message convention

Client→server: `{ id, channel, action, payload }`.
Result: `{ id, channel, action: "<name>.result", payload, error }`.
Broadcast: `{ id, channel, event, payload }`.
Job push: channel `jobs`, `event: progress|completed|failed`.

---

# Part IV — Data

## 20. Persist Spec

Hybrid storage: business data in JSONB; frequently queried fields get
generated columns for index performance.

```yaml
persist:
  table: string          # default "{module}_{plural}"
  soft_delete: true      # default
  category: operational  # operational | financial | compliance |
                         # analytics | master | archive
  indexes:
    - { field: status, type: btree }
```

### 20.1 Category routing

`persist.category` declares the data domain; the framework maps it to a
PostgreSQL schema (`financial` → schema `financial`). Developers MUST NOT
reference schema names. Dev: one instance, one schema per category,
auto-created. Production: infra may map categories to dedicated instances —
developer code identical. **Cross-category SQL joins are forbidden**
(→ `CROSS_CATEGORY_ACCESS_DENIED`) even on the same instance — cross-
category access goes through the resource API, so the constraint holds when
infra splits instances. Valkey/Redis topology (standalone/sentinel/cluster)
MUST be transparent to `ctx.*`.

### 20.2 Primary key (revised under D26/D29)

Since every entity is workspace-isolated, there is **one** PK strategy:
**UUID v7** (time-ordered — `gen_uuid_v7()` MUST be provided; v4 MUST NOT
be the default: B-tree fragmentation). Natural keys are always a unique
constraint per tenant — never the PK. Composite PKs with `tenant_id` are
never used. (v0.1.9 strategies 2–3 — integer PK and natural-key-as-PK for
global tables — are removed: global tables no longer exist.)

### 20.3 Table structure (normative)

```sql
CREATE TABLE {schema}.{module}_{plural} (
  id          uuid        PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid        NOT NULL,
  version     integer     NOT NULL DEFAULT 1,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  created_by  uuid, updated_by uuid,
  data        jsonb       NOT NULL DEFAULT '{}'
);
CREATE INDEX ON {schema}.{table} (tenant_id, deleted_at);
CREATE INDEX ON {schema}.{table} USING GIN (data);
-- per field with index:true → generated column + partial index:
--   _status varchar GENERATED ALWAYS AS (data->>'status') STORED;
--   INDEX (tenant_id, _status) WHERE deleted_at IS NULL
-- natural key → UNIQUE (tenant_id, _number) WHERE deleted_at IS NULL
```

`version` carries **optimistic-concurrency semantics** (§11.8): incremented
on every update, compared via CAS. `updated_at`/`created_at` are audit
metadata written by the DB — never comparison tokens.

## 21. Migration Spec

Three types, three owners:

1. **Structural — fully automatic.** Derived from Entity manifest diffs
   (add field, index, child table). Never hand-written; `forma migrate`
   detects and runs; `--dry-run` previews SQL.
2. **Custom DDL — `kind: Migration`** (§4.6): indexes, functions, triggers,
   extensions, materialized views. Runs after structural. `ctx.Exec()`
   rejects DML at runtime. Irreversible allowed with warning.
3. **Data migration** — scaffolded scripts
   (`forma migrate --generate data-migration <name>`), run/rollback by
   version, recorded in `forma_migrations`.

## 22. Workspace Provisioning

The runtime tracks each workspace's tenant record via `forma.core`
`workspace` entity. Lifecycle:
`create → provisioning → seed default roles + reference seeds → active`
(emits `workspace.activated`); `suspend ⇄ reactivate`; `terminate`.
Per-workspace config: `ctx.tenant.config("key", default)`. Row-level
isolation (`tenant_id` column) is the Core Basic strategy; storage tiering
is a Platform Operator concern.

## 23. forma.core Resources

All implementations MUST provide: `workspace`, `user`, `role`,
`role-assignment`, `api-key`, `session`, `job`, `audit-log` (append-only,
framework-written, read-only API), `setting` (entities); `health`,
`metrics` (services). Notable shapes: `job` tracks async execution
(status: pending|processing|completed|failed|dead_letter, progress, result,
attempt; actions find/retry/cancel with state conditions); `audit-log`
records resource, action, actor (user|api_key|system), before/after/diff,
IP, request ID.

---

# Part V — Operations

## 24. CLI Spec

```
MANIFESTS      forma apply|diff|delete -f <file|dir> [--dry-run]
               forma get <kind> [--output table|json|yaml|name]
               forma describe <kind> <name>
               forma validate                # incl. uses-honesty scan (§11.3)
DEV            forma new <app>               # scaffold kind: App
               forma dev [--port n] [--reset]
               forma generate [--resource <name>]
               forma repl [--context <name>] # Starlark console with ctx.* (D13)
MIGRATE        forma migrate [--dry-run|--status]
               forma migrate --generate data-migration <name>
               forma migrate --run|--rollback <version>
SEED           forma seed [--module|--resource] [--dev] [--reset]  # forma/seed
BACKUP         forma backup create [--module|--resource|--filter|--tenant-id
                 |--since <path>|--exclude|--dry-run] --output <path>
               forma backup inspect <path>
               forma restore --from <path> [--tenant-id|--module|--map-resource
                 |--transform <res=file>|--lookup|--on-conflict skip|overwrite|remap
                 |--incremental|--rebuild-summaries|--dry-run]
MODULE         forma module list|install <source>|uninstall <name>
                 # install presents derived permission footprint for consent (D20)
SCRIPT         forma script validate <file> · forma script test <ref> --input <json>
CONTEXT        forma context list|use|add <name> --server --token
```

Exit codes: 0 ok · 1 error · 2 validation · 3 auth · 4 network.
Config: `~/.forma/config.yaml` (contexts: server, token, environment).

## 25. Dev Environment

`forma dev` = complete local environment, one command, Docker Compose
managed by forma: **postgres:16** (one DB, schema per category +
`forma_control`), **valkey/redis** (registry, cache, queue, lock, pubsub),
**mailpit** (SMTP :1025, UI :8025), **minio** (S3 :9000, console :9001),
**forma-control:dev** (:8090).

Startup: check Docker → compose up → health checks → `forma migrate` →
`forma seed` → outbox worker → `forma-resource` with hot reload → watch
`*.yaml` → regenerate types on change → dashboard (API :8080, Admin
`/_admin`, Control :8090).

`forma-control` always runs as a separate process, even in dev (D3), with
fixed relaxed policy: auto-approve deployments, no signing, self-signed
mTLS, audit recorded but not enforced. Env vars are set by `forma dev`;
developers MUST NOT set them manually.

## 26. Backup & Restore

Normative because it carries the **Credible Exit Guarantee (D31)**: the
backup format is part of this open spec — any conforming implementation can
restore it, and backup/export can never be license-gated (D27).

- **Backup**: full or incremental (`--since`), filterable by
  module/resource/tenant/expression; storage files included; summaries
  excluded (rebuilt on restore). `inspect` shows contents without restoring.
- **Restore**: partial per module/resource; `--map-resource src=dst`;
  per-record **Starlark transform** (`transform(record, ctx)` — rename,
  mutate, `ctx.lookup`, `skip()`); conflict modes skip|overwrite|**remap**
  (UUIDs remapped with all FKs updated in one session); restore order
  auto-derived from `relation` dependency graph; `--dry-run` prints a full
  compatibility report (schema/version diffs, missing modules, estimated
  time) before touching anything.
- **Near-zero-downtime migration**: full backup → restore in background →
  incremental catch-ups → short write freeze → final increment → switch.

---

# Part VI — Developer Experience

## 27. Scripting Language

**Starlark** is the single scripting language — used for field rules,
conditions, guards, param validation, computed fields, and handlers.
Inline for single expressions; multi-line starting with `def` for
functions. Entry points: `def validate(resource, params, ctx)` returning
`ok()` / `fail(msg | {field, message})`; `def execute(params, ctx)`
returning a result object.

Sandbox limits (MUST enforce): 5000 ms, 64 MB, 100k iterations, no
network/filesystem/subprocess, ≤50 db queries and ≤1000 records read per
execution.

## 28. Script Context

**Resource API (preferred data access):** `invoice.load(id)`,
`invoice.find_by_number(nk)`, chainable
`invoice.query().where(...).include(...).order_by(...).get()/first()/count()`,
`invoice.new().set(...).save()`, `inv.call("mark-paid", {...})`,
`inv.field.total` / `inv.field.customer.name`; child helpers
`inv.add_child/update_child/remove_child("items", ...)` keyed by sequence
number; enum helpers `invoice.status.DRAFT`.

**Context:** `ctx.user.{id,role,permissions}`, `ctx.tenant.{id,name,config()}`,
`ctx.now()`, `ctx.job` (async only). **Built-ins:** `date.*`, `number.*`,
`string.*`, `array.*`, `log.*` (server-side only), `ok()/fail()`.

**Primitives (all gated by `uses`, §15.3):**

- `ctx.db` — escape hatch of last resort; resource API first. Read tier:
  SELECT only, tenant filter auto-injected, declared modules only. Write
  tier: full DML, tenant_id auto-injected on INSERT and enforced on
  UPDATE/DELETE, every query audited; discouraged in scripts, acceptable in
  native for measured hot paths. Signed Query Registry applies (Extended).
- `ctx.cache` — get/set(ttl)/delete/remember; keys auto-namespaced per
  tenant.
- `ctx.lock` — `with ctx.lock(key, ttl):` or `try_acquire`; and
  `ctx.next_key([field], [scope])` — the natural-key helper over
  `ctx.lock` implementing `natural_key_rule` (never `MAX()+1`).
- `ctx.queue` — `dispatch(action, payload, delay=, priority=)`.
- `ctx.pubsub`, `ctx.storage`, `ctx.kvstore`, `ctx.config`, `ctx.log` —
  per their specs; kvstore keys namespaced `{tenant}:{module}:{key}`.

REPL (`forma repl`, D13) exposes this exact context interactively; access
scope per environment is defined in `forma-control.md`.

## 29. Codegen

`forma generate` derives from manifests: typed client/server types (Go;
TS for frontend), constants for permissions and enum values, and OpenAPI
documents. Generated code is never edited by hand and never committed as
source of truth — manifests are.

---

# Part VII — Reference

## 30. Full Example — Order-to-Cash (canonical, D16)

Condensed here; the complete walkthrough lives in the Order-to-Cash
companion. Flow: `order` (Entity, items as child, natural key, state
machine draft→confirmed→fulfilled) → `confirm` action reserves stock under
`ctx.lock` → `payment-webhook` (Service, idempotent by gateway transaction
id) emits durable `payment-received` → reliable delivery creates `invoice`
and `journal-entry` via outbox sync-calls → summary entity
`sales-daily-summary` updated by durable events. Every pain in Foundation
§1.1 maps to one declared mechanism: race → `ctx.lock`; double webhook →
idempotency key; invoice numbering → `natural_key_rule`/`ctx.next_key`;
lost events → durable outbox; deploy → signed artifact + policy.

## 31. Conformance

An implementation is Core Basic-conforming when it provides:

1. Manifest loader: `apiVersion/kind/metadata/spec`, unknown
   apiVersion/kind rejected, multi-doc ≡ multi-file, discovery by scan.
2. Kinds: App, Module, Entity, Service, Config, Migration with specs as
   defined; derived surfaces (HTTP, WebSocket, docs) from Entity.
3. Fields incl. child (jsonb+table), relation, natural key with atomic
   gap-free generation.
4. Actions: standard set, five impl types, `required_permission` + `uses`
   with runtime enforcement and validate-time honesty scan;
   framework-enforced idempotency store with response replay and prepare
   step; optimistic concurrency via `version` CAS (§11.8).
5. Events: durability contract, transactional outbox + worker with
   idempotent sync delivery; `kind: Subscription` with compiled fan-out in
   `forma describe` and consent-time footprint (§12.5).
6. Validation levels 1–3 with normative error envelope; state machine with
   guards.
7. Security: auth strategies, RBAC catalog-on-declaration, workspace
   isolation at query level (404), cross-app grant verification, audit.
8. Persist: category schemas, UUID v7, normative table structure,
   cross-category SQL block.
9. Structural migration derivation; `kind: Migration` DDL-only execution.
10. forma.core resources; CLI verbs of §24 (incl. `repl`, `validate`,
    consent-on-install); dev environment contract; backup format +
    restore (transform, remap, dry-run) — never license-gated.
11. Starlark sandbox with stated limits and script context.

---



# Migration Map — v0.1.9 → v0.2.0

How every v0.1.9 section lands in the new structure. **Carry** = content
survives with only manifest-format conversion; **Rewrite** = substantive
change; **Move** = leaves this document.

| v0.1.9 section | Disposition |
|---|---|
| 1 Introduction, 2 Philosophy | **Rewritten** (Part I above) |
| 3 Resource Types | **Rewritten** → §4; `type:` field replaced by `kind:` manifest header |
| 4 Compilation Model | **Rewritten** → §6; `forma-server` renamed `forma-resource`; Compile Service removed; `sidecar` added as 5th impl (was Extended-only mention) |
| 5 Config Spec | **Rewritten** → §7 as `kind: Config` |
| 6 Tenancy Model | **Carry** → §8 |
| 7–12 (Anatomy, Field, Action, Event, Validation, State Machine) | **Done** → Part II (§9–14): manifest conversion; `required_permission`+`uses` per D20; module-qualified permissions; tenancy per D23; Workflow boundary note per D10; impl discriminated form with sidecar |
| 13 Security | **Done** → §15: unified permission block per-resource digantikan `uses` per-action (D20) dengan vocabulary penuh (config/kvstore enforcement carry); tenant isolation → workspace isolation (D29); **baru**: cross-app grant enforcement (D25/D31) |
| 14 Delivery | **Done** → §16 + pointer realtime subscription (Extended) & cross-app wire (plane-protocol) |
| 15 Communication, 16 Registry, 17 Convention | **Done** → §17–19 (carry, dipadatkan; error codes ditambah USES_VIOLATION dkk.) |
| 18 Module Spec | **Rewrite** → `kind: Module` manifest |
| 19 Persist, 20 Migration | **Done** → §20–21; **PK strategies 2–3 dihapus** (integer & natural-key-as-PK untuk tabel global — tabel global tidak ada lagi per D26/D29): satu strategi UUID v7 + natural key unique constraint |
| 21 Seed Spec | **Moved** → modul resmi `forma/seed` (D12); safety guarantees (dev seed exclusion via build tags, `--reset` blocked in production) tetap jadi syarat conformance |
| 22 Tenant Provisioning | **Done** → §22 Workspace Provisioning (rename per D28/D29); entity `tenant` → `workspace` di forma.core |
| 23 forma.core, 24 CLI, 25 Dev Env | **Done** → §23–25; CLI + `forma new`, `forma apply`, `forma validate`, `forma repl` (D13), consent saat `module install` (D20) |
| 26 Backup & Restore | **Done** → §26; kini normatif sebagai bagian Credible Exit Guarantee (D31): format backup = open spec, tidak pernah bisa digerbang lisensi (D27) |
| 27–29 Scripting, Script Context, Codegen | **Done** → §27–29; db tiers dilebur ke vocabulary `uses` (D20); `ctx.next_key` carry; REPL note (D13) |
| 30 Full Example | **Done** → §30 sebagai ringkasan Order-to-Cash (D16); walkthrough penuh di companion yang akan ditulis ulang |
| 31 Conformance | **Done** → §31: 11 butir, termasuk manifest-format, grant verification, dan backup-never-gated |

---

## Changelog

### v0.2.0 (Foundation realignment — in progress)
- Adopted Forma Manifest Format: `apiVersion/kind/metadata/spec` (Q11 resolved)
- **Workspace = the only tenancy model** (D29): apps 100% tenancy-blind,
  `FORMA_TENANCY` removed, no global storage (D26), `kind: App` added as
  root manifest with publish/request/grant cross-app model (D25) and
  license-degradation-to-read-only guarantee (D27)
- Kind catalog governed by concern map; three-layer extensibility via
  `KindDefinition` (D17, D18); built-ins: Module, Entity, Service, Config, Migration
- `kind: Module` specified: name = permission namespace, contents by scan,
  derived permission footprint with install-time consent (D19)
- Permission model: explicit `required_permission` + `uses` for all impl
  types; scan as honesty verifier only (D20)
- Declarative/imperative boundary normatized (D33): closed deliver
  vocabulary + `channel: queue`; `kind: Subscription` added for
  consumer-side event reactions with compiled fan-out (D35)
- Idempotency framework-enforced (store + response replay + prepare step)
  and optimistic concurrency via `version` CAS; `updated_at` demoted to
  audit metadata (D32, from Order-to-Cash test drive)
- Three-file-type rule made normative (yaml/script/asset)
- `forma-server` → `forma-resource`; two-process model normative incl. dev
- Compile Service removed; `impl.native` locally buildable (license: FSL, D8)
- `sidecar` promoted to Core Basic impl table (5 types)
- Seed Spec moved to official module `forma/seed`; scheduler/mail/notify
  declared official modules, out of spec scope
- Control Plane content extracted to `forma-control.md`; inter-plane contract
  to `forma-plane-protocol.md`
- Added `forma repl` and `forma apply` to CLI scope

*(v0.1.x changelog retained in archived document)*

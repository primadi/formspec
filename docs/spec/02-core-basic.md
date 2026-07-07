# Forma Core Basic Spec v0.2.0

**Status:** Draft — realigned under Forma Overview
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (Decisions D1–D50)

> This document defines the **minimum specification** required to build a conforming Forma implementation. Features not listed here are defined in Core Extended, Control Spec, Frontend Spec, Plane Protocol Spec, and Marketplace Spec.

---

## Part I — Foundation

### 1. Scope

Core Basic covers: multi-tenant CRUD applications with auto-generated API, admin panel, and docs; background job processing; transactional event delivery between resources; type-safe code generation from manifests; business rules via sandboxed scripting (Starlark).

**Not in Core Basic:** Workflow, Webhook, Mockup, Hooks, Query Builder, streaming, file fields → Core Extended. Control Plane governance → Control Spec. Frontend kinds → Frontend Spec. Scheduler, mail, notifications, seeding → official modules (not spec).

### 2. Core Philosophy

1. **Everything is a Resource.** Lifecycle: `Define → Persist → Act → Emit → Deliver`.
2. **One Definition, Many Protocols.** A manifest is the single source of truth for HTTP, WebSocket, admin panel, docs, and generated types.
3. **One Format for Everything.** Every concept uses `apiVersion/kind/metadata/spec` (Section 3). Tooling is generic.
4. **Three File Types Only.** `yaml` (description), `script` (logic), `asset` (static/custom UI). Nothing else.
5. **Convention over Configuration.** Sensible defaults; override only what you need.
6. **Security by Default.** Auth required; anonymous access must be explicit; tenant isolation automatic and non-bypassable; cross-tenant → 404.
7. **Location Transparency.** Callers never know where a resource runs — the registry resolves it.
8. **Contract before Implementation.** Manifest first, `impl` second.

---

## Part II — Manifest Format & Resource Kinds

### 3. The Forma Manifest Format

All Forma YAML files contain one or more **manifests**. A manifest MUST have exactly four top-level keys:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
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

PascalCase. Core Basic built-in kinds: `App`, `Module`, `Entity`, `Service`, `Config`, `Migration`, `Subscription`. Additional kinds are defined in other specs and registered by modules via `KindDefinition` (Extended). Unknown kinds MUST fail validation.

> **Guardrail:** application developers should almost never define new kinds. In 95% of cases, the right answer is an `Entity`.

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

#### 4.1 `kind: Entity`

Stateful, persisted, source of truth for business data. Supports CRUD, state machine, and events. `characteristics`: `master` (stable data), `transaction` (append-heavy, time-partitioned), `reference` (read-only seed data, owned by App Owner), `summary` (system-managed projection — no create/update/delete via API).

**API Exposure (D49):** Private by default. No external endpoint is created unless the entity opts in via `spec.expose` — a per-protocol declaration (`rest`, `grpc`, `ws`). Without `expose`, the entity is only accessible to internal callers (same-process services, Starlark scripts, events). See §11.1.

#### 4.2 `kind: Service`

Stateless, pure computation. MUST NOT hold internal state. External integrations (SFTP, third-party APIs) MUST be wrapped as Services — auth, permission, audit, and tenant isolation apply uniformly.

#### 4.3 Infrastructure primitives (closed set)

NOT kinds. Accessed only via `ctx`: `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage` (support: `ctx.config`, `ctx.kvstore`, `ctx.log`). Users cannot define new ones. Mail, notify, scheduler, seed are official modules built on top of primitives.

#### 4.4 `kind: App`

Root project manifest. Unit of deployment, trust boundary, and interface publication.

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: klinik-sehat
spec:
  version: 2.1.0
  vendor: acme-corp
  modules: [billing, acme-corp/general-ledger]
  publishes:                      # cross-app interfaces offered
    - service: icd-lookup
      actions: [search, find]
  consumes:                       # cross-app interfaces needed → triggers grant requests
    - app: bpjs-gateway
      service: claims
      actions: [submit-claim]
```

Default private. Cross-app access only via publish → request → **grant approved by Data Owner**, recorded, revocable, metered.

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
```

Module permission footprint is **derived** — the aggregate of `required_permission` + `uses` across all its manifests. `forma module install` MUST present this footprint for consent.

#### 4.6 `kind: Migration`

Developer-written **custom DDL only** (indexes, functions, triggers, extensions, materialized views). No DML — enforced at runtime. Structural migrations are derived automatically from Entity diffs.

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
  forma.yaml                      # kind: App (root)
  modules/
    billing/
      module.yaml                 # kind: Module
      entities/invoice.yaml       # kind: Entity
      services/tax-calculator.yaml
      scripts/invoice_send.star   # Starlark script
      assets/                     # static files, custom UI
  config/
    app.yaml                      # kind: Config
  impl/                           # Go source — build-time only, not deployed
    billing/
      invoice_handler.go
```

Folder names are convention. Loaders MUST discover by scanning `*.yaml`, not by path. The only hard rule: three file types (`.yaml`, `.star`, `assets/*`). `impl/` is build-time only, committed, excluded from deployment artifacts.

### 6. Compilation & Process Model

Two binaries, always — including development: `forma-resource` (Resource Plane runtime) and `forma-control` (Control Plane: governance, policy, signing — see Control Spec). Planes communicate via Plane Protocol (mTLS, policy pull on boot + 5-minute refresh, no write-back).

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
- Every Entity is workspace-isolated at the query level — no exceptions, no global storage. Cross-workspace → **404**.
- `characteristics: [reference]` is a domain marker: seeded per-tenant by App Owner, read-only for Data Owner. Live/large shared datasets → provider apps publishing services.
- Installing multiple apps into one Workspace unifies tenant identity — basis for cross-app grants.
- Data belongs to the owner of the Workspace where the resource runs. Expired module licenses degrade to read-only; `list/find/export/backup` MUST NOT be license-gateable.

---

## Part III — Resource Definition

### 9. Entity & Service Anatomy

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity                       # or Service
metadata:
  name: invoice                    # singular, kebab-case
  module: billing
spec:
  version: v1
  plural: invoices                 # optional, auto-derived
  characteristics: [transaction]   # Entity only
  auth: {}                         # §15
  audit: {}                        # §15
  persist: {}                      # §20
  expose: []                       # §11.1 — absent = no external access
  fields: []                       # Entity only — §10
  actions: []                      # §11
  events: []                       # §12
  state_machine: {}                # Entity only — §14
```

#### 9.1 Naming conventions

| Element | Convention | Example |
|---|---|---|
| `metadata.name` | singular, kebab-case | `invoice`, `purchase-order` |
| field names | snake_case | `due_date` |
| action names | kebab-case | `mark-paid` |
| event names | kebab-case, past tense | `payment-received` |
| permission keys | dot notation, module-qualified | `billing.invoices.send` |

### 10. Field Spec

Fields are valid only on `kind: Entity`.

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

- **`child`** — no own UUID; key = `parent_id + sequence`. No independent identity or lifecycle. Created atomically with parent. Example: invoice → line_items.
- **`relation`** — separate entity with own UUID, independent identity and lifecycle. Example: order → customer.

Decision test: does it have meaning outside the parent? Yes → relation.

#### 10.3 Child Spec

```yaml
- name: items
  type: child
  child:
    storage: jsonb            # jsonb | table
    sequence_field: line_number
    fields:
      - { name: line_number, type: integer, immutable: true }
      - { name: product_id,  type: uuid, rules: [required] }
      - { name: quantity,    type: integer, rules: [required, positive] }
```

| | `jsonb` | `table` |
|---|---|---|
| Atomic with parent | always | same transaction |
| Direct query/index on child | no | yes |
| Best for | <100 items, simple | many items, direct queries |

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
```

Counters live in `forma_natural_key_counters` (PK = tenant/resource/field/scope/period). Increments MUST be atomic, gap-free, duplicate-free. MUST NOT derive via `MAX()` scan. `ctx.next_key` is a helper over `ctx.lock`. Sequence numbers are allocated under `ctx.lock` inside the same transaction as the insert/update; if the transaction later fails its optimistic-concurrency (`version`) check and is retried, a gap MAY result — unless the entity declares gap-free mode, in which case the lock MUST be held until commit.

#### 10.5 Relation Spec

```yaml
relation:
  type: belongs_to        # belongs_to | has_many | has_one
  resource: customer
  foreign_key: customer_id   # optional, auto-derived
```

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

#### 11.1 Standard actions (Entity only)

**Disabled by default (D49).** Standard CRUD endpoints are only created when an entity opts in via `spec.expose`. When enabled, the following endpoints are generated per protocol:

| Protocol | Opt-in | Standard endpoints |
|---|---|---|
| REST | `expose: [{type: rest}]` | `/{plural}`, `/{plural}/:id`, etc. |
| gRPC | `expose: [{type: grpc, enabled: true}]` | Full gRPC service with matching RPCs |
| WebSocket | `expose: [{type: ws, enabled: true}]` | Subscription channel per entity + action |

**REST endpoint details (when enabled):**

| Action | Method | Path | Default permission |
|---|---|---|---|
| `list` | GET | `/` | `{module}.{plural}.list` |
| `find` | GET | `/:id` | `{module}.{plural}.view` |
| `create` | POST | `/` | `{module}.{plural}.create` |
| `update` | PATCH | `/:id` | `{module}.{plural}.update` |
| `delete` | DELETE | `/:id` | `{module}.{plural}.delete` |

The `actions` field inside `expose` filters which endpoints are generated; omit to enable all applicable (except `delete`). `expose: []` (empty) is equivalent to `expose` absent — no external surface is generated; both are valid ways to keep an entity internal-only. `summary` entities: create/update/delete permanently disabled even if listed.

Setting `disabled: true` on a standard action (`create`/`update`/`delete`/`find`) removes it from every surface — equivalent to the action never existing. Custom actions are simply omitted instead.

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

**Optimistic concurrency via `version`**, default-on for all Entities: update accepts version client read; mismatch → `409 CONFLICT` with current version. `updated_at` is audit metadata only.

#### 11.4 Async actions

`call: async` with `result_delivery` (websocket event, poll URL), `progress` (websocket), `timeout`, `retry`. Request returns `202` with job tracking info.

#### 11.5 `runtime_script` — observability overlay

Optional, runs alongside any impl. Core Basic scope: `after` timing only; read-only access to `resource` + `result`. Cannot modify results or control flow. Subject to same `uses`. Full Hook Spec is Extended.

### 12. Event Spec

Every event MUST be declared and linked from an action via `emits`.

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

`publish.durable: true` → event written to outbox before action returns. Entity: same DB transaction as data change (atomic). Service: independent outbox. Reliability requires **both** sides: publisher durable + subscriber durable = reliable. Non-durable publisher + durable subscriber = validation error.

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
- `forma describe entity <name>` MUST display merged fan-out (publisher deliver + all Subscriptions).
- Subscriptions enter consumer module's consent footprint.
- Two-sided durability contract applies unchanged.

### 13. Validation Spec

Three levels in Core Basic, evaluated in order:

1. **Field** — per-field `rules` (§10.6), automatic, before handler.
2. **Input** — per-action `params.validate`.
3. **State** — per-action `conditions`, Entity only.

Error response: `{ error: { code, message, details: [{level, field?, message}] }, meta: { request_id } }`.

### 14. State Machine Spec

Entity only.

```yaml
state_machine:
  field: status
  initial: draft
  transitions:
    - from: draft
      to: sent
      via: send                  # action name
      guard: "len(resource.items) > 0 and resource.total > 0"
```

Only declared transitions allowed; anything else → `STATE_TRANSITION_ERROR`. Guards are inline Starlark. Role-based approval on transitions is `kind: Workflow` (Extended).

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

- **Exposure model (D49):** deny-by-default. No external endpoint is created unless the entity opts in via `spec.expose`. Internal callers (same-process services, Starlark scripts, events) are unaffected and always have access.
- **Multi-protocol:** when an entity opts in via `spec.expose: [{type: rest} | {type: grpc} | {type: ws}]`, the router MUST generate protocol-specific routes for each declared type. REST and WebSocket are required transports; gRPC is recommended.
- **Workspace prefix:** every external route is prefixed with the workspace identifier. `workspace_slug` is a human-readable alias (configurable per Workspace); the router MUST fall back to the workspace UUID when no slug is set.
- **Internal dispatch:** same-process callers MUST bypass the network — the router detects registry locality and dispatches via direct function call. Cross-process callers use the configured protocol adapter.
- **Route scalability:** the router MUST use a radix-tree (or equivalent O(path segments)) lookup structure so that route count does not linearly degrade performance.
- **REST (required when exposed):** `/{workspace_slug}/api/{version}/{module}/{plural}/:id/:action`. Child: `/{workspace_slug}/api/.../items`.
- **WebSocket (required when exposed):** all actions; channel convention in §19.4.
- **Admin panel (recommended):** auto-generated from manifests.
- **Query conventions:** `?page&per_page&sort&direction&fields&filter[field][op]=value&search&include`. Filter operators: `eq neq gt gte lt lte between in nin like ilike null notnull`.
- **Pagination bounds:** `per_page` defaults to **20**, maximum **100**; values above the maximum are clamped to it; non-numeric or negative values → `400 VALIDATION_ERROR`. Implementations MAY lower the maximum per entity but MUST NOT raise it above 100.

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

Categories map to PostgreSQL schemas. Cross-category SQL joins forbidden (`CROSS_CATEGORY_ACCESS_DENIED`). Primary key: **UUID v7** (time-ordered) for all entities. Natural keys = unique constraint per tenant, never the PK.

Normative table structure:
```sql
CREATE TABLE {schema}.{module}_{plural} (
  id          uuid        PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid        NOT NULL,
  version     integer     NOT NULL DEFAULT 1,      -- optimistic concurrency
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  created_by  uuid, updated_by uuid,
  data        jsonb       NOT NULL DEFAULT '{}'
);
```

Indexed fields → generated columns: `_field VARCHAR GENERATED ALWAYS AS (data->>'field') STORED`. Natural key → `UNIQUE (tenant_id, _field) WHERE deleted_at IS NULL`.

### 20. Migration

Three types: (1) **Structural** — fully automatic from Entity diffs, never hand-written. (2) **Custom DDL** — `kind: Migration` for indexes, functions, triggers. DML rejected at runtime. (3) **Data migration** — scaffolded scripts, run/rollback by version.

Field rename MUST be declared via `renamed_from` on the field — otherwise the diff interprets it as drop+add; field removal requires two steps (deprecate, then remove) across two applied versions. Backfills belong to type-3 data-migration scripts.

### 21. Workspace Provisioning

Lifecycle: `create → provisioning → seed default roles + reference seeds → active` (emits `workspace.activated`); `suspend ⇄ reactivate`; `terminate`. Per-workspace config via `ctx.tenant.config("key", default)`.

### 22. forma.core Resources

All implementations MUST provide: `workspace`, `user`, `app-membership` (per-app membership + role assignments), `role`, `role-assignment`, `api-key`, `session`, `job`, `audit-log` (append-only, framework-written, read-only API), `failed-event` (append-only dead-letter store, framework-written — records event payload, target, error, attempt count; replayable via an admin action), `setting` (entities); `health`, `metrics` (services).

### 23. CLI

Key verbs: `forma apply|diff|delete|get|describe|validate`, `forma new <app>`, `forma dev`, `forma generate`, `forma repl`, `forma migrate`, `forma seed`, `forma backup create|inspect`, `forma restore`, `forma module list|install|uninstall` (install presents consent footprint), `forma script validate|test`.

### 24. Dev Environment

`forma dev` = one command, Docker Compose: Postgres 16, Valkey/Redis, Mailpit, MinIO, `forma-control:dev` (relaxed policy). Startup: health checks → migrate → seed → hot reload → regenerate types on YAML change → dashboard.

### 25. Backup & Restore

Normative (Credible Exit Guarantee D31): format is part of this open spec. Backup: full or incremental, filterable, storage files included, summaries excluded. Restore: partial, `--map-resource`, per-record Starlark transform, conflict modes (skip/overwrite/remap — UUIDs remapped with all FKs), `--dry-run` compatibility report. **Must never be license-gated (D27).**

---

## Part VI — Scripting & Codegen

### 26. Scripting (Starlark)

Single scripting language. Entry points: `def validate(resource, params, ctx)` → `ok()` / `fail(msg|{field, message})`; Entity action scripts `def execute(resource, params, ctx)` → result object (`resource` bound to the target record; absent for `create`-style actions); Service action scripts `def execute(params, ctx)`. Sandbox limits: 5000ms, 64MB, 100k iterations, no network/filesystem/subprocess, ≤50 db queries, ≤1000 records per execution.

Resource API: `invoice.load(id)`, `invoice.find_by_number(nk)`, chainable `invoice.query().where(...).include(...).get()/first()/count()`, `invoice.new().set(...).save()`, `inv.call("mark-paid", {...})`, field access `inv.field.total`, child helpers `inv.add_child/update_child/remove_child("items", ...)`.

Context: `ctx.user.{id,role,permissions}`, `ctx.tenant.{id,name,config()}`, `ctx.now()`, primitives (all gated by `uses`): `ctx.db`, `ctx.cache`, `ctx.lock` (incl. `ctx.next_key`), `ctx.queue`, `ctx.pubsub`, `ctx.storage`, `ctx.kvstore`, `ctx.config`, `ctx.log`.

### 27. Codegen

`forma generate` derives typed client/server types (Go; TypeScript for frontend), permission/enum constants, and OpenAPI documents. Generated code is never hand-edited; manifests are the source of truth.

---

## Part VII — Conformance

An implementation is Core Basic-conforming when it provides:

1. **Manifest loader:** `apiVersion/kind/metadata/spec`, unknown apiVersion/kind rejected, multi-doc ≡ multi-file, discovery by scan.
2. **Kinds:** App, Module, Entity, Service, Config, Migration with specs as defined; derived surfaces (HTTP, WebSocket, docs) from Entity.
3. **Fields:** incl. child (jsonb+table), relation, natural key with atomic gap-free generation.
4. **Actions:** standard set, five impl types, `required_permission` + `uses` with runtime enforcement and validate-time honesty scan; framework-enforced idempotency store with response replay and prepare step; optimistic concurrency via `version` CAS.
5. **Events:** durability contract, transactional outbox + worker with idempotent sync delivery; `kind: Subscription` with compiled fan-out and consent-time footprint.
6. **Validation:** levels 1–3 with normative error envelope; state machine with guards.
7. **Security:** auth strategies, RBAC catalog-on-declaration, workspace isolation at query level (404), cross-app grant verification, audit.
8. **Persist:** category schemas, UUID v7, normative table structure, cross-category SQL block.
9. **Migration:** structural derivation; `kind: Migration` DDL-only execution.
10. **Operations:** forma.core resources; CLI verbs; dev environment contract; backup format + restore (transform, remap, dry-run) — never license-gated.
11. **Scripting:** Starlark sandbox with stated limits and script context.

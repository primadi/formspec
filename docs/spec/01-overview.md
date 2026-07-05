# Forma Framework — Overview

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Audience:** Everyone — this is the first document to read about Forma.

---

## 1. What is Forma?

Forma is a **complete, spec-first ecosystem for building business applications**. Its reference implementation is written in Go, but the applications you build are not limited to a single language: business logic can be written in **Go** (`native`, `compiled` — for performance-critical code), **Starlark** (sandboxed scripting, editable from the admin panel without redeploy — a full app can be built entirely in Starlark), or **any language via the sidecar pattern** (PHP, Python, Node, Java — for leveraging existing ecosystems).

It is not just a framework — it is an open standard (CC0) with a reference implementation, tooling, and a marketplace.

Three characteristics that set Forma apart:

1. **Spec-first.** Forma is an open standard before it is code. The reference implementation is built on the spec, not the other way around. Anyone can build a conforming implementation.
2. **Declarative at the center.** A single YAML resource definition is the source of truth for the API, admin panel, frontend, documentation, data types, validation, state machine, and permissions.
3. **Designed for AI-assisted development.** A declarative structure + strict conventions + only three file types make Forma a guardrail for AI coding assistants: generated output is consistent, easy to review, and hard to get critically wrong.

> **Positioning:** _"If Laravel made PHP delightful for web apps, Forma makes Go practical for business software — but Forma goes further: spec-first, governance control plane, scripting runtime, multilingual logic, and a fully declarative surface."_

---

## 2. What is a "Business Application"?

A Forma business application is a **multi-user transactional system with domain rules**. Examples: multi-branch POS, inventory, billing/invoicing, clinic management, school management, HRM, order management.

### The Canonical Example: Order-to-Cash

The Order-to-Cash flow (order → reserve stock → payment → invoice → fulfillment) is used throughout all Forma documentation. It was chosen because it naturally surfaces every pain point that developers typically only discover after hitting production:

| Real pain point | Without an ecosystem | With Forma |
|---|---|---|
| Two cashiers deduct stock simultaneously (race condition) | Developer must recognize the problem and add manual locking | `ctx.lock` — an explicit convention |
| Payment webhook sent twice → duplicate invoice | Idempotency is often forgotten | Idempotency key by convention, enforced by the framework |
| Sequential invoice numbers under concurrency | Incorrect `SELECT MAX+1` | Managed sequence/lock |
| "Order paid" event lost on crash | Requires an outbox pattern, rarely done correctly | Built-in reliable events |
| Deploy new version + schema migration | Downtime, manual scripts | Signed artifacts + Control Plane policy |

---

## 3. Where Forma Comes From

Forma is not built from a blank slate. It synthesizes the best ideas from six proven projects, each with an explicit reason:

| Source | What Forma took from it |
|---|---|
| **Frappe** (ERPNext) | How to define business entities, workflows, and modules **declaratively**. DocType → `kind: Entity`. Vertical modules (accounting, HRM, inventory) as first-class ecosystem citizens. Form layout hints, print formats, and approval workflows attached to state machines. |
| **PocketBase** | The principle of **"One Definition, Many Protocols"** — from a single resource definition, you automatically get HTTP endpoints, WebSocket handlers, admin panel UI, API docs, and generated types. Auth required by default; anonymous access must be explicitly declared. DX benchmark: `forma dev` — one command, everything runs. Realtime subscription conventions. |
| **Dapr** | **Patterns, not a runtime dependency.** The sidecar contract for polyglot business logic. The component model as inspiration: the backend behind `ctx.*` primitives can be swapped (Valkey ↔ Redis ↔ NATS) without changing the contract. Dapr is not used as infrastructure because its building blocks are not tenant-aware, its sidecar breaks the single-binary story, and Forma needs semantics (registry, `tenant_affinity`, circuit breaker) that Dapr does not provide out of the box. |
| **OPA** (Open Policy Agent) | A **declarative policy engine for governance**. Rego as an auditable rule language. Forma embeds OPA as a Go library inside `forma-control` — applied strictly to governance (deployment, approval, key management), never to business data authorization. |
| **Laravel** | **Ecosystem completeness, DX, and business model.** Laravel proved that a framework wins because of the layers around it (Horizon, Pulse, Filament, Forge), not the runtime alone. Forma's equivalents: `forma.observe`, admin panel, module registry, Forma Cloud, Agent Skill. The business model — open code + paid services around it — is validated. The "Laravel → Forma" feature map (see Reference) is used as a completeness checklist. |
| **Kubernetes** | The **uniform declarative format** — `apiVersion` + `kind` + `metadata` + `spec`. Every Forma concept (entity, service, config, module, page, form, dashboard, menu) uses the same YAML format. The radical consequence: **an entire Forma project contains only three file types — `yaml` (description), `script` (logic), `asset` (static/custom UI).** |

---

## 4. Core Principles

1. **Everything is a Resource.** All concepts — entities, services, pages, dashboards, config, modules, policies — are modeled as resources using the same manifest format (`apiVersion/kind/metadata/spec`). `Entity` handles stateful data with CRUD and state machines; `Service` handles stateless computation. Many other kinds cover UI, governance, packaging, and configuration. External integrations must be wrapped as Services.
2. **One Definition, Many Protocols.** A resource YAML is the single source of truth for all surfaces: REST API, WebSocket, admin panel, documentation, and generated types.
3. **Convention over Configuration.** Sensible defaults for everything; override only what you need.
4. **Security by Default.** Auth is mandatory. Tenant isolation is automatic and cannot be bypassed. No implicit permissions. Cross-tenant access returns 404 (not 403 — existence is not leaked).
5. **Location Transparency.** Callers never need to know where a resource runs — the registry resolves it.
6. **Closed Set of Primitives.** Infrastructure access is only through six primitives: `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage` (with `ctx.config`, `ctx.kvstore`, `ctx.log` as supporting primitives). Users cannot define custom infrastructure services.
7. **Contract Before Implementation.** Always write the resource YAML first (fields, state machine, actions, events, permissions), then the `impl`.
8. **One Format for Everything (`kind`).** Backend, frontend, config, modules — all use the same `apiVersion/kind/metadata/spec` YAML format, splittable across folders/files per concern.
9. **Three Declarative File Types.** The declarative surface of a Forma project uses three file types: `yaml` (description — all manifests), `script` (logic — Starlark `.star` files), and `asset` (static files, custom UI component bundles — JS/CSS/images). Compiled code (Go in `impl/`, TypeScript source, PHP for sidecars) lives outside this surface: Go is compiled and fused into the binary at build time; sidecar containers are built and deployed as artifacts referenced from manifests but not part of the declarative project surface. The deployed application has no fourth declarative type — no `.env` files, no route files, no hand-written migration files.

---

## 5. Architecture at a Glance

Forma has **two planes**, running as two separate processes always — even in development:

```
┌──────────────────────────────────────────────────┐
│                 Control Plane                    │
│                 (forma-control)                  │
│                                                  │
│  Environments  ·  Policy (OPA/Rego)  ·  Keys     │
│  Signing  ·  Approval  ·  Transparency Log       │
│  Contracts (grants, consents, licenses)          │
│                                                  │
│  x Never reads business data                     │
│  x Never executes business logic                 │
└──────────────────┬───────────────────────────────┘
                   │  mTLS (pull policy every 5 min)
                   │  Desired-state channel ────────►
                   │  ◄────── Evidence channel
                   │  (append-only, never writes back)
┌──────────────────┴───────────────────────────────┐
│                Resource Plane                    │
│                (forma-resource)                  │
│                                                  │
│  CRUD  ·  Actions  ·  State Machine  ·  Events   │
│  Validation  ·  Permission Enforcement           │
│  REST API  ·  WebSocket  ·  Admin Panel          │
│                                                  │
│  Six primitives: ctx.db/cache/lock/queue/        │
│  pubsub/storage                                  │
└──────────────────────────────────────────────────┘
```

- **Control Plane** (`forma-control`): Governance and security. Defines environments, deployment policy (evaluated by embedded OPA/Rego), signing & key management (HSM/Vault/KMS), approval workflows (no-self-approval enforced), and an immutable, hash-chained audit store. **It never reads business data and never executes business handlers.**
- **Resource Plane** (`forma-resource`): Execution. CRUD, actions, validation, state machines. Data access only through the six closed primitives. Protocol surfaces (HTTP, WebSocket, docs) are derived automatically from resource definitions. Registers with the registry (Valkey) for load balancing and location transparency.

**Relationship:** The Resource Plane pulls policy at boot and every 5 minutes over mTLS. It **never writes back to the Control Plane** — it only appends evidence (deploy status, metering counts, audit anchors, violations). If the Control Plane is unreachable, the Resource Plane keeps serving on the last-known policy.

Control Plane compute is stateless (freely replicable). All state is delegated to a separate storage schema (`forma_control`), never shared with application schemas.

### Environment Identity

Every workspace has an environment flag — **dev** or **prod** — set by the Control Plane and immutable from the Resource Plane:

| | Dev Workspace | Prod Workspace |
|---|---|---|
| **Data reliability** | Not guaranteed — data can be wiped by the system | Guaranteed — backup, retention, restore |
| **Resources** | Shared free pool (limited, mixed workloads) | Production-grade — choice of shared-prod pool or exclusive |
| **Compute** | Shared pool (limited) | Shared-prod pool or exclusive (per-workspace instance) |
| **Data store** | Shared entity-store / kv-store (free, mixed with dev) | Shared-prod (isolated from dev, cost-efficient for small systems) or exclusive entity-store / kv-store / Postgres / Valkey |
| **Backup** | None | Scheduled, custom retention, external target |
| **Billing** | Free (capped) | Paid, per resource plan (shared-prod = lower cost; exclusive = higher cost) |
| **Policy** | Relaxed (dev-style) | Strict (approval, signing, promotion required) |
| **Logs** | No retention guarantee | Retention per plan |

**Resource pool separation:** the platform maintains three separate resource pools — dev-shared, prod-shared, and prod-exclusive. A prod-shared data store never mixes with dev workloads; it provides production guarantees (backup, retention, SLAs) at a lower cost than exclusive instances, suitable for small-to-medium systems that don't need dedicated hardware.

---

## 6. Who Runs Forma?

Forma has **three owner roles** at the platform level. These are not users of a business application — they are the people who operate the Forma platform itself. **One identity can hold multiple roles** (e.g., a solo developer building an app for their own use is both a Workspace Owner and an App Owner).

Each owner can appoint **admins** via delegation certificates: the admin has their own key, the owner signs a certificate (scope + validity window + revocable), and every admin action carries the certificate — the cryptographic chain always terminates at the owner key. Owner-only acts (accept/relinquish ownership, issue/revoke delegations, rotate owner key) cannot be delegated.

### A. Workspace Owner (Data Owner)

**Owns:** a workspace — where data, compute, and tenant identity live. The boundary of billing and security.

| Capability | Details |
|---|---|
| **Workspace** | Create workspace (dev or prod); manage name and metadata |
| **Apps & Modules** | Install/uninstall apps and modules from the marketplace; **review and approve required permissions** (consent footprint) at install time; re-consent when a version update changes the footprint |
| **Data sources** | Install and configure datasources per workspace: entity-store (built-in), kv-store, Postgres, Valkey/Redis |
| **Users & membership** | Manage workspace users; assign roles per app; invitation and removal |
| **Grants** | Approve/reject cross-app grant requests; revoke grants at any time |
| **Billing** | View invoices (infrastructure + module subscriptions); top up prepaid balance; set budget caps |
| **Backup & restore** | Schedule workspace backups; set retention and external target (e.g., own S3 bucket); restore data (requires owner signature) |
| **Logs & audit** | Access own workspace audit log; view consent history, grant history |
| **Data export** | Export data — guaranteed to never be license-gated |

**Tools:**

| Type | Name | Purpose |
|---|---|---|
| Web Console | `forma/console` | UI for all of the above capabilities |
| CLI | `forma` | `forma workspace create`, `forma backup`, `forma restore` |

### B. App/Module Owner

**Owns:** application or module artifacts — code, versions, and marketplace listings.

| Capability | Details |
|---|---|
| **Development** | Develop apps/modules locally with `forma dev`; write spec YAML + Starlark scripts + Go impl |
| **Versioning & release** | Sign artifacts with owner key; publish new versions to the registry |
| **Marketplace listing** | Set pricing (free / one_time / subscription / per_seat / per_call); set description, category; apply for Verified Badge |
| **Monitoring** | **View usage metrics:** who installed the app/module, which workspaces, user count, usage volume; data is sanitized — no access to tenant data contents |
| **Billing (as vendor)** | View marketplace revenue; track payouts |
| **License management** | Issue license tokens for enterprise customers; manage validity periods |

> **Important:** An App/Module Owner **can never access tenant data**. They see sanitized aggregate metrics only. Support access requires an impersonation grant signed by the Workspace Owner (time-boxed, fully audited).

**Tools:**

| Type | Name | Purpose |
|---|---|---|
| Web Console | `forma/studio` | UI for releases, marketplace listings, usage monitoring, vendor billing |
| CLI | `forma` | `forma dev`, `forma apply`, `forma sign`, `forma generate` |

### C. Cloud Owner (Platform Operator)

**Owns:** the Forma Cloud instance — infrastructure, global policy, key management.

| Capability | Details |
|---|---|
| **Infrastructure** | Provision and decommission workspaces; manage resource plans per workspace; set global deployment policy; manage the trusted module registry |
| **Key management** | Manage platform keys (HSM/KMS); rotate keys; verify owner public keys on registration |
| **Governance** | Set the policy floor (no self-approval, no unsigned artifact in non-dev); set impl trust tiers (unverified → sandbox only, Verified → +sidecar, scanned+approved → +native); emergency freeze |
| **Audit** | Full access to the transparency log; publish log checkpoints to third parties; verify log integrity |
| **Emergency** | `forma-ctl freeze`, revoke sessions, rotate keys; restart Resource Planes |
| **Billing (as operator)** | Manage billing infrastructure; marketplace settlement; metering verification |

> **Important:** The Cloud Owner **can never read tenant business data** — this is a normative prohibition in the Control Plane. There is no operator backdoor to tenant data. Impersonation is only possible via a grant signed by the Workspace Owner.

**Tools:**

| Type | Name | Purpose |
|---|---|---|
| Web Console | `forma/ops` | UI for provisioning, policy, audit dashboards |
| CLI (bedrock) | `forma-ctl` | Emergency control — conventional code inside the `forma-control` binary; does not depend on the platform it is fixing |

### D. Dogfooding

All three web console apps (`forma/console`, `forma/studio`, `forma/ops`) are **Forma applications built with Forma itself**. They run on a dedicated operations Resource Plane workspace and access the Control Plane through a bridge `kind: Service`. This proves that Forma can build complex UIs (dashboards, forms, tables, workflow approvals) using its own primitives.

`forma-ctl` is the exception — a conventional CLI inside the `forma-control` binary, serving as the emergency bedrock that never depends on the platform it repairs.

### E. Common Role Overlaps

| Scenario | Personas |
|---|---|
| Solo developer building an app for their own use | Workspace Owner + App Owner |
| SaaS startup building an app, selling on the marketplace, using Forma Cloud | App Owner + Module Vendor |
| Self-hosted enterprise tenant | Workspace Owner + Cloud Owner (via enterprise license) |
| IT consultant managing apps for a client | Admin (delegated by client Workspace Owner) + App Owner |

---

## 7. The Forma Manifest

Every concept in Forma is declared in the same manifest format:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: invoice
  module: billing
  description: Customer invoice
spec:
  # kind-specific body
```

| Key | Purpose |
|---|---|
| `apiVersion` | Schema version — `forma.dev/v1alpha1` for this version of the spec |
| `kind` | PascalCase resource kind: `Entity`, `Service`, `App`, `Module`, `Config`, `Migration`, `Subscription`, `Page`, `Form`, … |
| `metadata` | Identity: `name` (kebab-case, unique per kind+module), `module` (owning module), optional `description`, `labels`, `annotations` |
| `spec` | Kind-specific body, defined per kind in the relevant spec document |

A `.yaml` file MAY contain multiple manifests separated by `---`. Splitting across folders and files is purely a concern-separation choice. Tooling is generic — `forma validate`, `forma diff`, `forma apply` work on any manifest.

### Project File Types

A Forma project has two layers of files:

| Layer | Types | Purpose |
|---|---|---|
| **Declarative surface** (always deployed) | `.yaml` (manifests), `.star` (Starlark scripts), `assets/*` (static files, custom UI bundles — JS/CSS/images) | Everything the runtime needs to interpret and serve |
| **Build-time code** (compiled away, not deployed) | `impl/*.go` (native/compiled handlers), TypeScript source, PHP sidecar code, Dockerfiles | Source that compiles into the binary, WASM bundles, or sidecar containers |

There is no fourth declarative type. No `.env` files (config is a manifest). No route files (routes are derived from resources). No hand-written structural migration files (they are derived from Entity diffs).

---

## 8. Resource Kinds

All kinds share the manifest format. The catalog is governed by concern:

| Concern | Kind | Defined In |
|---|---|---|
| **Domain model** | `Entity`, `Service` | Core Basic |
| **Packaging** | `App`, `Module` | Core Basic |
| **Configuration** | `Config` | Core Basic |
| **Custom DDL** | `Migration` | Core Basic |
| **Cross-module reactions** | `Subscription` | Core Basic |
| **Business processes** | `Workflow` | Core Extended |
| **API surface** | `Api`, `Webhook`, `Mockup` | Core Extended |
| **Extension mechanism** | `KindDefinition` | Core Extended |
| **UI / Frontend** | `Page`, `Form`, `Table`, `Dashboard`, `Widget`, `Report`, `Menu`, `Print`, `Theme` | Frontend Spec |
| **Governance** | `Environment`, `Policy` | Control Spec |

**Derived by default:** CRUD API endpoints, the admin panel, and API documentation are generated automatically from an `Entity` manifest — no additional manifest is required. The `Page`, `Form`, `Table`, and `Menu` kinds exist only to **override** those defaults.

**Extensible in three layers:** (1) spec built-ins (above) → (2) official modules register kinds via `KindDefinition` (`Seed`, `Schedule`, `MailTemplate`…) → (3) third-party modules with namespaced kinds, subject to the Verified Badge.

> **Guardrail:** application developers should almost never need to define a new kind. Needing a new kind means extending the framework. In 95% of cases, the right answer is an `Entity`.

---

## 9. Six Primitives (`ctx.*`)

All infrastructure access goes through exactly six primitives — a closed set. Users cannot define custom infrastructure services:

| Primitive | Purpose | Example Use |
|---|---|---|
| `ctx.db` | Database access (raw SQL, scoped to the module) | Custom queries, reporting |
| `ctx.cache` | Volatile cache (backed by Valkey/Redis) | Session data, rate limiting |
| `ctx.lock` | Distributed lock | Natural key generation, mutual exclusion |
| `ctx.queue` | Reliable background jobs with retry | Email, PDF generation, heavy computation |
| `ctx.pubsub` | Non-durable publish/subscribe | Real-time dashboard updates |
| `ctx.storage` | Object storage (backed by S3/MinIO) | File uploads, receipts, attachments |

Supporting primitives: `ctx.config` (typed, environment-resolved configuration), `ctx.kvstore` (durable key-value store, used by the framework for idempotency), `ctx.log` (structured logging with automatic tenant/request/user context).

**The closed-set principle:** common needs that sit on top of primitives — mail, notifications, scheduled jobs, seeding — are built as **official modules** (`forma/mail`, `forma/notify`, `forma/scheduler`, `forma/seed`), not as new primitives. The closed set never grows for convenience. This keeps the runtime surface auditable and the trust boundary small.

---

## 10. Five Implementation Types

Every action declares how its logic is executed:

| Type | Form | Sandbox | Hot Update | Use When |
|---|---|---|---|---|
| `native` | Fused Go binary | No (full trust) | No | Performance-critical, stable logic |
| `compiled` | Go plugin `.so` / WASM | Partial (WASM) | Yes (load at runtime) | Hot-reload without restart |
| `script` | Inline Starlark | Yes (no network/FS, memory & time limits) | Yes | Prototypes, small rules |
| `script_ref` | Starlark stored in DB, versioned | Yes | Yes — editable from admin panel, with rollback | Business rules that change often |
| `sidecar` | Container (PHP/Python/Node/Java) via Unix socket | Container; trust = native | Yes (deploy container) | Need another language ecosystem |

The sidecar trust model is **equal to native** — identity proxy and Signed Query Registry protections apply regardless of the language. All five types are subject to the same `required_permission` and `uses` declarations.

---

## 11. Tenancy Model

Forma has exactly **one** multi-tenancy model: **Workspace**.

```
Workspace → App → Module → Resource
```

- **Applications are 100% tenancy-blind.** There is no tenancy switch, no single/multi mode, no tenant code in apps. `tenant_id` exists only as the runtime's internal isolation mechanism, keyed to the Workspace.
- Every Entity is workspace-isolated at the query level — **no exceptions, no global storage**. Application code cannot bypass this. Cross-workspace access returns **404** (not 403 — existence is not leaked).
- Installing multiple apps into one Workspace unifies tenant identity across them — this is the basis for cross-app grants.
- **Data always belongs to the owner of the Workspace where the resource runs.** A module you purchase never "owns" your data. Expired module licenses degrade to read-only, and `list/find/export/backup` can never be license-gated.
- Large tenants that want their own servers do not get a "different mode" — they purchase an **enterprise license and run their own Forma Cloud**, becoming the Platform Operator for themselves.

---

## 12. The Ecosystem

| Component | Description |
|---|---|
| **Module Registry** | Central hub for distributing modules, apps, themes, and widgets. Verified Badge (ed25519 trust chain) for vetted packages. |
| **Admin Panel** | Derived automatically from Entity definitions — zero setup. The PocketBase benchmark: a working CRUD UI the moment you define an entity. |
| **Official Modules** | `forma/scheduler` (cron jobs), `forma/mail` (email via `ctx.queue`), `forma/notify` (push/WA/SMS via `ctx.pubsub`), `forma/seed` (seeders & factories for dev/testing) — all built on top of the six primitives, not as new primitives. |
| **`forma` CLI** | `forma dev` (one-command dev environment via Docker Compose), `forma apply` (GitOps-style deployment), `forma validate`, `forma diff`, `forma generate` (codegen). |
| **`forma repl`** | Interactive Starlark console with `ctx.*` access — a first-class debugging tool and the surface for AI Agent Skills. |
| **Agent Skill** | Rules and conventions for AI coding assistants, ensuring generated code is consistent, complete, and respects Forma's structural requirements. |
| **Mockup System** | Simulate third-party integrations during development. Route calls to mockups by config, never by environment-branching in business code. |
| **`forma.observe`** | The official observability dashboard (Pulse/Horizon counterpart): jobs, metrics, audit, registry — itself built with Forma's Dashboard, Widget, and Report kinds. |
| **Forma Cloud** | Managed hosting. Optional — self-hosting is fully supported. |

---

## 13. Licensing & Business Model

| Component | License |
|---|---|
| **Specification** (all documents in `/docs/spec/`) | **CC0** — open standard, vendor-neutral, anyone can implement |
| **Reference Implementation** (`forma-resource`, `forma-control`) | **FSL** (Functional Source License) — source available, free to use for building any commercial application; the sole restriction: you may not sell Forma as a managed service competing with the official offering. Each version automatically becomes **Apache 2.0 after 2 years**. |
| **Enterprise Features** | Gated by a cryptographic **license token** — validated locally, no call-home required, air-gap safe. Gates governance features (HSM signing, immutable audit, SSO/SCIM), not code. |

### Three Monetization Paths

1. **Forma Cloud** — managed hosting with tiered resource plans (shared/exclusive compute and data stores)
2. **Enterprise License** — self-hosted enterprise governance features
3. **Marketplace** — revenue sharing on module/app/theme/widget sales, based on the dependency graph

---

## 14. Development Roadmap

Forma is currently being built by a solo developer. Target for MVP: **3–6 months**.

| Phase | Scope | Outcome |
|---|---|---|
| **MVP** (3–6 months) | Full Core Basic spec + `forma` CLI (`dev`, `apply`, `generate`) + derived admin panel + single-binary Resource Plane + Workspace (dev env) + Starlark scripting (`script_ref`) | Can build and deploy a simple app end-to-end — from spec to UI + backend (example: Order-to-Cash) |
| **Beta** | Core Extended (Workflow, Webhook, Mockup) + Control Plane (Policy, Signing) + Frontend kinds (Page, Form, Table) + Module Registry + Multi-workspace + Prod environment support | Usable by small teams; module distribution via registry |
| **v1.0** | Marketplace + Full Plane Protocol + Sidecar polyglot + Forma Cloud + `forma.observe` + Agent Skill + Console apps (dogfooding: `forma/console`, `forma/studio`, `forma/ops`) | Complete ecosystem; production-ready at medium scale |
| **v1.x** | K8s-native (CRDs, GitOps) + Enterprise features (HSM, SSO/SCIM) + Multi-Control federation | Enterprise scale |

---

## 15. Where to Go Next

Choose your path:

| Persona | Start here |
|---|---|
| **App Developer** — building a business application | [`02-core-basic.md`](./02-core-basic.md) → [`08-order-to-cash-tutorial.md`](./08-order-to-cash-tutorial.md) |
| **Module Vendor** — selling modules on the marketplace | [`02-core-basic.md`](./02-core-basic.md) → [`07-marketplace.md`](./07-marketplace.md) |
| **Platform Operator** — managing Forma infrastructure | [`04-control-plane.md`](./04-control-plane.md) → [`06-plane-protocol.md`](./06-plane-protocol.md) |
| **Investor** — business model & monetization strategy | §13 (Licensing & Business Model) above + [`07-marketplace.md`](./07-marketplace.md) |

For the full glossary, all design decisions (D1–D48), the Laravel → Forma feature map, and the complete kind catalog, see [`11-reference.md`](./11-reference.md).

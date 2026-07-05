# Forma Core Extended Spec v0.2.0 (draft)

**Status:** Draft — awaiting review
**License:** Creative Commons CC0
**Governed by:** Forma Foundation Document v2.0
**Requires:** Core Basic v0.2.0 · relates to Frontend v0.4.0, Control v0.1.0
**Supersedes:** Core Extended v0.1.4 (archived); incorporates the
Mockup/Webhook/environment-binding stub (2026)
**Resolves:** Q8 remainder (`Workflow`), Q14 (`KindDefinition`)

> Core Extended is everything a mature implementation adds on top of Core
> Basic conformance. Every construct here follows the same laws: manifest
> format (Core §3), closed vocabulary with scripted escape hatches (D33),
> explicit `uses` (D20), workspace isolation (D29).

---

## Part I — Extended Kinds

### 1. `kind: Workflow` — approval attached to a state machine (D10)

Simple lifecycles stay inline in the Entity (Core §14); role-based
approval lives here and **attaches without modifying the entity** — the
Subscription pattern applied to transitions.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Workflow
metadata: { name: journal-posting-approval, module: gl }
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:                              # sequential; each step needs quorum
    - { roles: [gl.supervisor], quorum: 1 }
    - { roles: [gl.controller], quorum: 1,
        when: "resource.amount > 100000000" }   # FormaExpr-compatible guard
  on_reject: { to: rejected }         # optional alternate transition
  escalation: { after: 48h, notify_roles: [gl.manager] }
```

Rules:
- The intercepted transition executes only after all applicable steps
  reach quorum; approvals are signed statements recorded in audit (and
  they surface in the entity's timeline UI).
- Approver eligibility = role membership per-app (D37); the submitter can
  never approve their own request (the D39 floor, applied to business
  workflows).
- A Workflow appears in `forma describe entity` merged output, like
  Subscriptions (D35) — attached behavior is always compiled, never
  hidden.

### 2. `kind: Api` — exposure override

Derived REST endpoints (Core §16) need no manifest. `Api` exists only to
deviate: custom paths, versioning, disabling, and external gRPC.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Api
metadata: { name: order-public, module: billing }
spec:
  for: billing.order
  rest:
    base_path: /public/orders        # replaces derived path
    version: v2
    disable: [delete, update]        # endpoints removed from this surface
  grpc:
    enabled: true                    # external gRPC surface
    package: acme.billing.v2
```

One resource may have multiple `Api` surfaces (public vs partner);
permissions are unchanged — surfaces never widen access (D38 logic:
exposure is presentation, enforcement stays on the resource).

### 3. `kind: Webhook` — verified inbound endpoints

(From the stub, normatized.) The framework verifies **before** the
handler runs; handlers only ever see verified payloads.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Webhook
metadata: { name: midtrans-webhook, module: billing }
spec:
  for: payment-gateway.webhook       # the Service action that handles it
  method: POST
  path: /webhooks/midtrans           # auto-derived when omitted
  auth:
    strategy: signature              # signature | token
    signature:
      algorithm: hmac-sha512         # hmac-sha256 | hmac-sha512 | rsa
      header: X-Midtrans-Signature
      key: { config: midtrans.server_key, secret: true }
      payload: raw_body
  idempotent: true                   # framework-enforced (Core §11.8/D32)
  idempotency_key: { from: payload, field: transaction_id }
```

Rules:
- `spec.for` MUST reference one Service action; failed verification →
  rejected before any handler, counted, alertable — never a handler
  concern.
- `token` strategy for simple internal webhooks; `signature` for
  cryptographic providers.
- This closes the Companion's acknowledged gap (finding #4).

### 4. `kind: Mockup` + environment binding

(From the stub, normatized.) A Mockup implements the same contract as the
real connector; callers never know which one answered.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Mockup
metadata: { name: midtrans-mock, module: billing }
spec:
  for: payment-gateway
  actions:
    - { name: create-session, impl: { type: script_ref, ref: billing/midtrans_mock_create_session } }
    - { name: webhook,        impl: { type: script_ref, ref: billing/midtrans_mock_webhook } }
  state: true                        # mock may keep per-tenant scratch state
                                     # in ctx.kvstore (simulated tx database)
  faults:                            # optional failure injection for tests
    - { action: create-session, rate: 0.0, error: TIMEOUT, delay_ms: 0 }
```

**Environment binding (normative):** business handlers never branch on
environment. Routing is config-driven:

1. `mock_enabled: true` (a `kind: Config` key; default true in dev/CI,
   false elsewhere) → calls to the Service route to its Mockup.
2. `false` or no Mockup → the real connector impl.
3. Neither exists → error.

`ctx.environment` exists for logging/diagnostics only; `forma validate`
SHOULD warn when business scripts branch on it. `forma dev` and
`forma test` enable all mockups automatically. Multiple mockups per
Service are allowed; `mock_ref` in config selects one (test scenarios).

### 5. `kind: KindDefinition` — the extension mechanism (resolves Q14)

How official and third-party modules register kinds (D18 layer 2–3):

```yaml
apiVersion: forma.dev/v1alpha1
kind: KindDefinition
metadata: { name: Seed, module: forma/seed }
spec:
  group: seed.forma.dev              # instances use apiVersion: seed.forma.dev/v1
  version: v1
  schema: { ... }                    # JSON Schema for the instance spec body
  handler: { type: native, ref: "FormaSeed.Apply" }   # apply/delete lifecycle
  scope: module                      # module | app
```

Rules:
- **Namespacing by apiVersion group** (the CRD lesson): built-in kinds own
  `forma.dev`; a module's kinds live under its own group
  (`seed.forma.dev`, `gl.acme-corp.dev`) — collisions are structurally
  impossible, and `forma validate` resolves schemas per group.
- The handler runs at `forma apply`/`delete` time under the module's
  declared `uses` — a KindDefinition grants no runtime power beyond its
  module's footprint.
- Published JSON Schemas feed the LSP/tooling ladder automatically (D34).
- Guardrail restated (D18): application developers almost never define
  kinds; 95% of the time the answer is an Entity.

---

## Part II — Resource Definition Extensions

### 6. Advanced field types & field-level security

```yaml
fields:
  - name: total_with_tax
    type: decimal
    computed: { script: "resource.total * (1 + resource.tax_rate)" }
  - name: account_number
    type: string
    encrypted: true                  # at rest
    masked: true                     # "****1234" in output
    required_permission: invoices.view_sensitive   # extra read permission
  - name: internal_notes
    type: string
    exclude: [public_api, audit_log, webhook]      # per-surface exclusion
  - name: attachment
    type: file                       # §9 Storage
```

Field-level `required_permission` composes with action permissions;
`classification` per resource is governance metadata evaluated by the
Control Plane (Control §5.3).

### 7. Validation levels 4–6

Core Basic stops at level 3; these run in declared order after it. This
also gives the Companion's blacklist check its declarative home
(finding #2 closed):

```yaml
business_rules:                      # Level 4 — may read other resources
  - script_ref: "billing/credit-check"        # runtime-editable, versioned
cross_validate:                      # Level 5 — same DB transaction as action
  - handler: "InventoryValidator.CheckStock"
consistency:                         # Level 6 — immediately before persist
  - script: "resource.total == sum([i.qty * i.price for i in resource.items])"
    message: "Total must equal sum of line items"
```

All three are subject to `uses` (a level-4 rule reading `customer` must
declare it) and may be script / script_ref / handler.

### 8. Hook Spec

```yaml
hooks:
  - { on: before | after | on_error, action: "send" | "*",
      impl: { type: script_ref, ref: ... }, priority: 10 }
  - { on: before_deliver | after_deliver, event: "paid" | "*", ... }
```

- `before` may modify params or abort (`fail()`); `before_deliver` may
  suppress delivery.
- **Cross-module hooks are declared in the hooking module** (never by
  editing the target — the D35 ownership pattern) and appear in
  `forma describe` merged output and in the module's consent footprint.
- Core Basic's `runtime_script.after` is the deliberate minimal subset;
  full-Hook implementations MAY alias it onto `on: after`.

### 9. Storage Spec (file fields)

```yaml
- name: photos
  type: file_list                    # or: file
  storage:
    allowed_types: [jpg, png, webp]
    max_size_mb: 5
    max_count: 10
    visibility: private              # private | public
    signed_url_ttl: 300
    cdn: true
    transform: [ { resize: { width: 800, height: 600, fit: cover } },
                 { format: webp } ]
```

Drivers (local / s3 / gcs) are deployment config — never manifest
concerns. Upload rides `POST /:resource/:id/{field}` (multipart) and the
frontend upload tray (Frontend §7.1). Files are tenant-scoped in
`ctx.storage` and included in workspace backups (D41/D45).

---

## Part III — Communication Extensions

### 10. Extended deliver channels

Core Basic's deliver set (audit_log, websocket, queue, reliable_event)
gains:

```yaml
deliver:
  - channel: webhook                 # outbound to registered subscriber endpoints
    target: { scope: subscribers }
    sign: true                       # HMAC payload signature
    retry: { max: 5, backoff: exponential }
  - channel: notification            # delegates to forma/notify (D12)
    target:
      recipient_field: customer_id
      channels: [email, in_app]      # forma/notify NotificationChannel names
      template: invoice-paid
    when: "resource.total > 0"
  - channel: pubsub                  # explicit non-durable at-most-once
    target: { scope: tenant }
```

The notification channel is a thin bridge: templates, channel providers
(email/WA/push), and preferences live in the official `forma/notify` +
`forma/mail` modules — the deliver vocabulary stays closed while the
channel ecosystem grows as modules (D12).

### 11. Async result via webhook callback

Extends Core §11.7 `result_delivery` with
`{ channel: webhook, url_from: header, sign: true, retry: {...} }` —
caller passes `X-Callback-URL`; results are HMAC-signed.

### 12. Subscription Tier 2 — streaming (harmonized with D35)

`kind: Subscription` (Core §12.5) gains a durable streaming mode for
high-volume fan-out:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata: { name: analytics-on-paid, module: analytics }
spec:
  on: { resource: billing.order, event: paid }
  durability: durable
  durable:
    store: redis_stream              # redis_stream | kafka
    retention: 30d
    position: latest                 # latest | earliest | offset
    max_retry: 3
    dead_letter: { resource: failed-event, action: create }
  filter: "event.total > 0"
  transform: |
    def transform(event, ctx):
      return { "event_type": "order_paid", "amount": event.total }
  deliver:
    - { channel: queue, job: record-analytics }
```

| | Tier 1 (Core — outbox) | Tier 2 (streaming) |
|---|---|---|
| Storage | Postgres outbox | Redis Stream / Kafka |
| Consistency | transactional | at-least-once, positioned replay |
| Fan-out | one target per entry | many subscribers |
| Use | GL, billing, inventory | analytics, audit, monitoring |

Consumer position is tracked per subscription per tenant; idempotency key
= `{subscription_id}:{event_sequence}` through the Core §11.8 store.
**Dynamic subscriptions** (created at runtime via API/admin panel) are
*data, not manifests* — the D-principle from Frontend §5.2: manifest
subscriptions define what ships with a module; dynamic ones record what an
operator chose, live in forma.core, and are equally visible in
`forma describe` merged fan-out.

### 13. Query Builder

For aggregation beyond the Core resource API — `sum/count/avg/min/max`,
multi-`group_by`, `having`, `date_trunc`, window functions
(`rank/row_number/sum.over(partition_by, order_by, frame)`), plus the
hard rule carried from Core §20.1: **no cross-schema SQL, ever** —
cross-resource queries load independently and merge at app level.
Permissions are the resources' own; cross-resource requires `uses`
declarations. Extended implementations upgrade `include()` from lazy
(N+1) to batch loading transparently; Core Basic implementations SHOULD
warn on N+1 above a threshold.

---

## Part IV — Runtime Extensions

### 14. Rate limiting

`rate_limit: { max, per, scope: user|tenant|ip|global, strategy:
sliding_window|token_bucket }` at resource level with per-action
overrides.

### 15. Load balancing & circuit breaker

Registry-driven (Core §18): strategies `round_robin | least_connections |
latency_aware | tenant_affinity`; per-instance circuit breaker
(closed → open on 5 consecutive failures or >50% errors/60s → half-open
probe at 30s), metrics in `forma.instance:{id}:metrics`.

### 16. `ctx.secrets`

The only path to `secret: true` config values (Core §15.3): declared via
`uses: { secrets: [midtrans.server_key] }`, values are never loggable,
never enter script string interpolation warnings unchecked, and every
read is audited. Backends (env/file/Vault/KMS) are deployment config.

### 17. Summary Spec

`characteristics: [summary]` entities (Core §9.1) are populated
exclusively by durable events; Extended adds the rebuild contract:
`forma summary rebuild <entity> [--tenant]` replays the source event
stream (Tier 1 outbox history or Tier 2 stream) into a fresh projection —
which is why backups exclude summaries (Core §26).

### 18. i18n

Labels/messages accept keys (`label: { key: invoice.title }`) resolved
against per-module translation assets; locale precedence: user → workspace
→ app default. Validation messages and enum labels participate. Plain
strings remain valid (single-locale apps pay nothing).

### 19. Kubernetes integration

Three modes: Standalone · K8s-aware (environment from namespace labels
`forma.dev/environment`) · K8s-native (CRDs mirroring Forma manifests,
admission webhook bridging `forma apply` into the Control-Plane approval
flow, GitOps via ArgoCD/Flux). The admission webhook is a *client* of the
plane protocol — K8s never becomes a second source of governance truth.

### 20. forma.observe

The official observability app (Pulse/Horizon counterpart, D43-style
dogfooded): dashboards over `forma.core` metrics/jobs/audit + registry
metrics — itself built from Dashboard/Widget/Report kinds (Frontend
§5).

---

## Migration Map — v0.1.4 → v0.2.0

| v0.1.4 | Disposition |
|---|---|
| 1 Hooks | **Done** → §8 (impl discriminated form; cross-module = consent footprint) |
| 2 Advanced fields, 9 Field-level security | **Done** → §6 |
| 3 Validation 4–6 | **Done** → §7 (Companion finding #2 closed) |
| 4–5 Webhook/notification delivery, async webhook | **Done** → §10–11; notification delegates to `forma/notify` (D12) |
| 6 Streaming Tier 2, 7 pubsub | **Done** → §12 (harmonized with `kind: Subscription` D35; dynamic subs = data) + §10 |
| 8 Query Builder | **Done** → §13 |
| 10–12 Rate limit, LB, circuit breaker | **Done** → §14–15 |
| 13–15 Control Plane, Permission model, Deployment | **Moved** → `forma-control.md` + `forma-plane-protocol.md`; permission-as-code superseded by `uses` (D20); classification → Control §5.3 |
| 16 K8s | **Done** → §19 (webhook = plane-protocol client) |
| 17 Storage | **Done** → §9 |
| 18 Notification | **Moved** → official module `forma/notify` (D12); bridge channel in §10 |
| 19 Inbound webhook | **Superseded** → `kind: Webhook` §3 (stub) |
| 20 Adv. tenant provisioning | **Superseded** by D29 (workspace) + D42 (plans) |
| 21 Module Registry | **Moved** → `forma-marketplace.md` (D42) |
| 22 Extended forma.core | folded into owning sections (subscription records §12, webhook endpoints §3/§10) |
| 23 forma-ctl | **Moved** → Control Spec §11 |
| 24 ctx.secrets | **Done** → §16 |
| 25 Mockup System | **Superseded** → `kind: Mockup` §4 (stub, + state/faults) |
| 26 Summary | **Done** → §17 |
| 27 i18n | **Done** → §18 |
| 28 forma.observe | **Done** → §20 |
| — (new) | `kind: Workflow` §1 (Q8), `kind: Api` §2, `kind: KindDefinition` §5 (Q14) |

## Extended Conformance (delta over Core Basic)

1. Five extended kinds (Workflow, Api, Webhook, Mockup, KindDefinition)
   incl. group-namespaced kind registration and schema publication.
2. Workflow interception with signed approvals, no self-approval, merged
   `describe` visibility.
3. Webhook verification before handler; Mockup routing by config with no
   environment branching in business code.
4. Validation 4–6 in order, under `uses`; hooks incl. cross-module with
   footprint visibility.
5. File fields with transforms, signed URLs, tenant scoping, backup
   inclusion.
6. Extended deliver channels; Tier-2 streaming Subscriptions with
   positioned replay and dynamic-subscription-as-data.
7. Query Builder with the no-cross-schema-SQL rule; batch `include()`.
8. Rate limiting, LB strategies incl. tenant_affinity, circuit breaker.
9. `ctx.secrets` audited access; summary rebuild; i18n resolution chain.

## Open questions

| # | Question |
|---|---|
| E1 | Webhook replay/retry for missed inbound deliveries (stub Q) — framework store-and-retry vs provider responsibility |
| E2 | Standalone `kind: Webhook` targeting an Entity action directly, without a Service intermediary (stub Q) |
| E3 | Mockup fault-injection vocabulary final shape (stub Q — sketched in §4) |
| E4 | Pattern-scoped `uses` for cache/storage (sketch in old §14) — glob syntax + migration |
| E5 | Workflow: delegation of approval (vacation), parallel step groups |
| E6 | FeatureFlag kind — candidate (Appendix B) |

## Changelog

### v0.2.0
- Full realignment under Foundation v2.0 + manifest format
- New kinds: Workflow (Q8), Api, KindDefinition (Q14); Webhook & Mockup
  normatized from stub with state/faults and environment binding
- Tier-2 streaming folded into `kind: Subscription` (D35); dynamic
  subscriptions = data
- Control/permission/deployment content extracted to Control & Plane
  Protocol specs; notification/registry extracted to modules/marketplace

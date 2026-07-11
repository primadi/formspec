# Forma Core Extended Spec v0.2.0

**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Overview · Forma Reference (Decisions D10, D20, D33, D35, D46, D48, D49)
**Requires:** Core Basic v0.2.0
**Related:** Frontend Spec · Control Spec

> Core Extended is everything a mature implementation adds on top of Core Basic conformance. Every construct here follows the same laws: manifest format (Core §3), closed vocabulary with scripted escape hatches (D33), explicit `uses` (D20), and workspace isolation (D29).

---

## Part I — Extended Kinds

### 1. `kind: Workflow` — Approval Attached to a State Machine

Simple lifecycles stay inline in the Document (Core §14). Role-based approval lives in Workflow and **attaches without modifying the document** — the Subscription pattern (D35) applied to transitions.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Workflow
metadata: { name: journal-posting-approval, module: gl }
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:
    - { roles: [gl.supervisor], quorum: 1 }
    - { roles: [gl.controller], quorum: 1,
        when: "resource.amount > 100000000" }
  on_reject: { to: rejected }
  escalation: { after: 48h, notify_roles: [gl.manager] }
```

**Rules:**
- The intercepted transition executes only after all applicable steps reach quorum. Approvals are signed statements recorded in audit.
- Approver eligibility = role membership per-app (Core D37). The submitter can never approve their own request (the D39 floor applied to business workflows).
- A Workflow appears in `forma describe document` merged output — attached behavior is always compiled, never hidden.

### 2. `kind: Api` — Exposure Override

A document must first opt into exposure via `spec.expose` (§Core 11.1, D49). `kind: Api` only overrides **already-exposed** surfaces — it cannot create access where `expose` has not been set. Used for: custom paths, versioning, disabling specific endpoints, and gRPC configuration.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Api
metadata: { name: order-public, module: billing }
spec:
  for: billing.order
  rest:
    base_path: /public/orders
    version: v2
    disable: [delete, update]
  grpc:
    enabled: true
    package: acme.billing.v2
```

One resource may have multiple `Api` surfaces (public vs partner). Permissions are unchanged — surfaces never widen access beyond what `expose` allows. Exposure is presentation; enforcement stays on the resource.

### 3. `kind: Webhook` — Verified Inbound Endpoints

The framework verifies **before** the handler runs; handlers only ever see verified payloads.

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
      algorithm: hmac-sha512
      header: X-Midtrans-Signature
      key: { config: midtrans.server_key, secret: true }
      payload: raw_body
  idempotent: true
  idempotency_key: { from: payload, field: transaction_id }
```

**Rules:** `spec.for` MUST reference one Service action. Failed verification → rejected before any handler, counted, alertable. `token` strategy for simple internal webhooks; `signature` for cryptographic providers.

### 4. `kind: Mockup` — Simulated Third-Party Integration

A Mockup implements the same contract as the real connector; callers never know which one answered.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Mockup
metadata: { name: midtrans-mock, module: billing }
spec:
  for: payment-gateway
  actions:
    - { name: create-session, impl: { type: script_ref, ref: billing/midtrans_mock_create_session } }
    - { name: webhook,        impl: { type: script_ref, ref: billing/midtrans_mock_webhook } }
  state: true                        # may keep per-tenant scratch state in ctx.kvstore
  faults:                            # failure injection for tests
    - { action: create-session, rate: 0.0, error: TIMEOUT, delay_ms: 0 }
```

**Environment binding (normative):** business handlers never branch on environment. Routing is config-driven: `mock_enabled: true` (a `kind: Config` key; default true in dev/CI) → calls route to Mockup; `false` → real connector. `ctx.environment` exists for logging only; `forma validate` SHOULD warn on business-script branching.

### 5. `kind: KindDefinition` — The Extension Mechanism

How official and third-party modules register new kinds (D18, layers 2–3):

```yaml
apiVersion: forma.dev/v1alpha1
kind: KindDefinition
metadata: { name: Seed, module: forma/seed }
spec:
  group: seed.forma.dev              # instances use apiVersion: seed.forma.dev/v1
  version: v1
  schema: { ... }                    # JSON Schema for the instance spec body
  handler: { type: native, ref: "FormaSeed.Apply" }
  scope: module                      # module | app
```

**Rules:**
- **Namespacing by apiVersion group** (CRD pattern): built-in kinds own `forma.dev`; module kinds live under their own group (`seed.forma.dev`, `gl.acme-corp.dev`) — collisions structurally impossible.
- The handler runs under the module's declared `uses` — a KindDefinition grants no runtime power beyond its module's footprint.
- Published JSON Schemas feed the LSP/tooling ladder automatically.
- Application developers almost never define kinds; 95% of the time the answer is a Document.

### 5. `kind: Integrator` — Cross-Module Bridge (NEW in v0.3.0)

Bridges two Documents/Modules that **do not directly know each other** — consistent with the principle "Modules do not directly know each other."

```yaml
kind: Integrator
name: invoice-to-gl
listen:
  resource: billing.Invoice
  event: before_cancel        # or on_paid, etc. — event name from the publisher's Contract
call:
  resource: gl.GLJournal
  action: cancel
compensate: recreate_gl_journal   # optional; the framework decides whether to call it
```

`listen.resource` and `call.resource` are resolved through the registry (`forma.resource:{name}:{version}`) — an Integrator never `import`s the definition of Invoice or GLJournal directly.

**Mandatory rule:** every Integrator that creates a side-effect from one event MUST also provide a symmetric handler for its cancellation event — otherwise, cancel on the source side will permanently block because the generic reference guard always blocks without anyone knowing how to open the path.

**Config:**

```yaml
# Global defaults
integrator_defaults:
  retry:
    max_attempts: 5
    backoff: exponential
    base_delay_ms: 500
    max_delay_ms: 30000
  outcome_unknown_after: 5

# Per-Integrator override
kind: Integrator
name: invoice-to-payment-gateway
retry:
  max_attempts: 10      # third-party gateway: more lenient
```

The `compensate` field lists an action on the target resource that reverses the effect. At runtime, the framework detects whether the call resolves within the same database transaction or crosses a boundary (§Core 14d). If same-transaction: compensate is never called (ACID rollback handles it). If cross-boundary: compensate is registered in the Saga log.

---

## Part II — Resource Definition Extensions

### 6. Advanced Field Types & Field-Level Security

```yaml
fields:
  - name: total_with_tax
    type: decimal
    computed: { script: "resource.total * (1 + resource.tax_rate)" }
  - name: account_number
    type: string
    encrypted: true                  # at rest
    masked: true                     # "****1234" in output
    required_permission: invoices.view_sensitive
  - name: internal_notes
    type: string
    exclude: [public_api, audit_log, webhook]   # per-surface exclusion
  - name: attachment
    type: file                       # §9
```

Field-level `required_permission` composes with action permissions. `classification` per resource (Control §5.3) is governance metadata.

### 7. Validation Levels 4–6

Core Basic stops at level 3. These run in declared order after it:

```yaml
business_rules:                      # Level 4 — may read other resources
  - script_ref: "billing/credit-check"        # runtime-editable, versioned
cross_validate:                      # Level 5 — same DB transaction as action
  - handler: "InventoryValidator.CheckStock"
consistency:                         # Level 6 — immediately before persist
  - script: "resource.total == sum([i.qty * i.price for i in resource.items])"
    message: "Total must equal sum of line items"
```

All three are subject to `uses`. A level-4 rule reading `customer` must declare it in its `uses` block.

### 8. Hook Spec

The `hooks:` block lives at the top level of a Document or Service `spec` — a sibling of `actions`/`events`.

```yaml
hooks:
  - { on: before | after | on_error, action: "send" | "*",
      impl: { type: script_ref, ref: ... }, priority: 10 }
  - { on: before_deliver | after_deliver, event: "paid" | "*", ... }
```

- `before` may modify params or abort (`fail()`); `before_deliver` may suppress delivery.
- **Cross-module hooks are declared in the hooking module** (never by editing the target — the D35 ownership pattern). They appear in `forma describe` merged output and the consent footprint.

### 9. Storage Spec (File Fields)

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

Drivers (local/S3/GCS) are deployment config, never manifest concerns. Upload via `POST /:resource/:id/{field}` (multipart). Files are tenant-scoped in `ctx.storage` and included in workspace backups.

---

## Part III — Communication Extensions

### 10. Extended Deliver Channels

Core Basic's deliver set gains:

```yaml
deliver:
  - channel: webhook                 # outbound to registered subscribers
    target: { scope: subscribers }
    sign: true                       # HMAC payload signature
    retry: { max: 5, backoff: exponential }
  - channel: notification            # delegates to forma/notify
    target:
      recipient_field: customer_id
      channels: [email, in_app]
      template: invoice-paid
    when: "resource.total > 0"
  - channel: pubsub                  # explicit non-durable at-most-once
    target: { scope: tenant }
```

The notification channel is a thin bridge to `forma/notify`. Templates and channel providers live in official modules — the deliver vocabulary stays closed while the channel ecosystem grows as modules.

### 11. Tier-2 Streaming Subscriptions

`kind: Subscription` (Core §12.3) gains a durable streaming mode:

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
    position: latest
    max_retry: 3
    dead_letter: { resource: failed-event, action: create }
  filter: "event.total > 0"
  transform: |
    def transform(event, ctx):
      return { "event_type": "order_paid", "amount": event.total }
  deliver:
    - { channel: queue, job: record-analytics }
```

In `filter` and `transform`, `event` exposes the declared event payload fields (Core §12 payload contract) plus envelope metadata `event.name`, `event.resource_id`, `event.occurred_at`.

| | Tier 1 (Core — outbox) | Tier 2 (streaming) |
|---|---|---|
| Storage | Postgres outbox | Redis Stream / Kafka |
| Consistency | transactional | at-least-once, positioned replay |
| Fan-out | one target per entry | many subscribers |
| Use | GL, billing, inventory | analytics, audit, monitoring |

**Dynamic subscriptions** (created at runtime via API/admin panel) are *data, not manifests*. Manifest subscriptions define what ships with a module; dynamic ones record what the operator chose, live in `forma.core`.

### 12. Query Builder

For aggregation beyond the Core resource API: `sum/count/avg/min/max`, multi-`group_by`, `having`, `date_trunc`, window functions (`rank/row_number/sum.over`). **Hard rule from Core §19: no cross-schema SQL, ever** — cross-resource queries load independently and merge at app level. Extended implementations upgrade `include()` from lazy (N+1) to batch loading transparently.

---

## Part IV — Runtime Extensions

### 13. Rate Limiting

`rate_limit: { max, per, scope: user|tenant|ip|global, strategy: sliding_window|token_bucket }` at resource level with per-action overrides.

### 14. Load Balancing & Circuit Breaker

Registry-driven (Core §18): strategies `round_robin | least_connections | latency_aware | tenant_affinity`. Per-instance circuit breaker: closed → open on 5 consecutive failures or >50% errors/60s → half-open probe at 30s.

### 15. `ctx.secrets`

The only path to `secret: true` config values. Declared via `uses: { secrets: [midtrans.server_key] }`. Values never loggable, never enter script interpolation warnings unchecked. Every read audited. Backends (env/file/Vault/KMS) are deployment config.

### 16. Summary Spec

`characteristics: [summary]` documents are populated exclusively by durable events. Extended adds the rebuild contract: `forma summary rebuild <document>` replays the source event stream into a fresh projection — which is why backups exclude summaries.

### 17. i18n

Labels/messages accept keys (`label: { key: invoice.title }`) resolved against per-module translation assets. Locale precedence: user → workspace → app default. Plain strings remain valid (single-locale apps pay nothing).

### 18. Kubernetes Integration

Three modes: **Standalone** · **K8s-aware** (environment from namespace labels `forma.dev/environment`) · **K8s-native** (CRDs mirroring Forma manifests, admission webhook bridging `forma apply` into Control-Plane approval flow, GitOps via ArgoCD/Flux). The admission webhook is a *client* of the plane protocol — K8s never becomes a second source of governance truth.

### 19. forma.observe

The official observability app: dashboards over `forma.core` metrics/jobs/audit + registry metrics — itself built from Dashboard, Widget, and Report kinds (Frontend Spec).

---

## Conformance (Delta over Core Basic)

An implementation is Core Extended-conforming when it additionally provides:

1. Five extended kinds (Workflow, Api, Webhook, Mockup, KindDefinition) with group-namespaced kind registration and schema publication.
2. Workflow interception with signed approvals, no self-approval, merged `describe` visibility.
3. Webhook verification before handler; Mockup routing by config with no environment branching in business code.
4. Validation levels 4–6 in order, under `uses`; hooks incl. cross-module with footprint visibility.
5. File fields with transforms, signed URLs, tenant scoping, backup inclusion.
6. Extended deliver channels (webhook, notification, pubsub); Tier-2 streaming Subscriptions with positioned replay and dynamic-subscription-as-data.
7. Query Builder with no-cross-schema-SQL rule; batch `include()`.
8. Rate limiting, LB strategies incl. tenant_affinity, circuit breaker.
9. `ctx.secrets` audited access; summary rebuild; i18n resolution chain.

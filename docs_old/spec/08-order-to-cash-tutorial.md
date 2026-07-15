# Order-to-Cash Tutorial

**Status:** Draft
**Audience:** App Developers — first-time Forma users
**Prerequisites:** Read [Forma Overview](./01-overview.md) and [Core Basic Spec](./02-core-basic.md)

> This tutorial walks you through building a mini Order-to-Cash application with Forma, step by step. By the end, you will have: a working order management system with sequential order numbers, payment gateway integration, a transactional journal entry on payment, and email/WA notifications — all from declarative manifests with minimal code.

---

## 1. Requirements (FR1–FR10)

Our Order-to-Cash flow: customer checkout → pay via payment gateway → after successful payment: PDF receipt generated, receipt emailed, WA notification sent, accounting journal automatically created, admin dashboard shows live payments.

| # | Requirement | Why it's hard without conventions |
|---|---|---|
| FR1 | Sequential order numbers (`ORD-2026-000123`), no duplicates under concurrency | Classic race condition |
| FR2 | Payment gateway (Midtrans/Xendit) with simulation mode in dev | No standard pattern for external integrations |
| FR3 | Webhook may be sent multiple times — must not process twice | Persistent idempotency, not cache |
| FR4 | After payment, journal entry **must** exist — no loss, no duplicates | Transactional outbox, not fire-and-forget |
| FR5 | Email + WA may be delayed, must not silently fail | Reliable background jobs with retry |
| FR6 | Dashboard live payment ticker | Loss acceptable if admin is offline |
| FR7 | Membership tier discount cached, invalidated on rule change | Volatile cache + manual invalidation |
| FR8 | Number prefix & notification template per workspace, no redeploy | Dynamic config |
| FR9 | All steps logged with structure, one correlation ID | Consistent observability |
| FR10 | PDF receipt stored, re-downloadable | Object storage, not container filesystem |

---

## 2. Project Structure

Create the project:

```bash
forma new tokoku
```

This generates a standard Forma project structure (Core §5):

```
tokoku/
  forma.yaml                    # kind: App (root manifest)
  modules/
    billing/                    # kind: Module
      module.yaml
      entities/
      services/
      scripts/
    gl/                         # kind: Module (double-entry accounting)
      module.yaml
      entities/
    notifications/              # kind: Module
      module.yaml
      subscriptions/
  impl/                         # Go source — build-time only
  config/                       # kind: Config
```

---

## 3. Step 1 — App Manifest

Create `forma.yaml`:

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: tokoku
  description: Mini order-to-cash
spec:
  version: 1.0.0
  vendor: acme-corp
  modules: [billing, gl, notifications]
```

This declares our app composes three modules. No `publishes` or `consumes` — this app doesn't offer cross-app interfaces and the payment gateway is an external integration (wrapped as a Service), not another Forma app.

---

## 4. Step 2 — Customer & Order Entities

First, the customer entity the order will reference. Create `modules/billing/entities/customer.yaml`:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: customer
  module: billing
  description: Customer master record
spec:
  version: v1
  characteristics: [master]
  fields:
    - { name: name,           type: string,  rules: [required] }
    - { name: email,          type: string }
    - { name: is_blacklisted, type: boolean, default: false }
```

Now the order itself. Create `modules/billing/entities/order.yaml`:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: order
  module: billing
  description: Customer order from checkout to completed
spec:
  version: v1
  characteristics: [transaction]

  fields:
    # FR1 — Sequential number with natural_key_rule
    - name: number
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      natural_key_rule:
        strategy: sequence
        format: "{prefix}-{year}-{seq:06d}"
        prefix: { config: billing.order_number_prefix, default: "ORD" }
        reset: yearly

    - name: customer_id
      type: relation
      relation: { type: belongs_to, resource: customer }

    # Child items — ordered line items without own identity
    - name: items
      type: child
      child:
        storage: jsonb
        sequence_field: line_number
        fields:
          - { name: line_number, type: integer, immutable: true }
          - { name: product_id,  type: uuid, rules: [required] }
          - { name: quantity,    type: integer, rules: [required, positive] }
          - { name: price,       type: decimal, rules: [required, positive] }

    - name: total
      type: decimal
      rules: [required, positive]

    - name: member_tier
      type: enum
      enum_values: [regular, silver, gold]

    - name: status
      type: enum
      enum_values: [draft, awaiting_payment, paid, fulfilled, void]
      index: true

    - name: gateway_reference
      type: string

    - name: paid_at
      type: datetime

  state_machine:
    field: status
    initial: draft
    transitions:
      - { from: draft,            to: awaiting_payment, via: checkout,
          guard: "len(resource.items) > 0 and resource.total > 0" }
      - { from: awaiting_payment, to: paid,             via: mark-paid }
      - { from: [draft, awaiting_payment], to: void,    via: void }

  actions:
    - name: update
      conditions:
        - script: "resource.status == 'draft'"
          message: "Checked-out orders cannot be edited — use 'void'"

    - name: delete
      disabled: true              # Transaction records must keep audit trail

    - name: checkout
      description: Generate order number & create payment session
      required_permission: orders.checkout       # → billing.orders.checkout
      audit: true
      conditions:
        - script: "not customer.load(resource.customer_id).is_blacklisted"
          message: "Customer is blacklisted, cannot checkout"
      uses:
        # payment-gateway is a Service (defined in Step 5)
        resources: [payment-gateway.create-session, customer.find]
        config: { read: [billing.order_number_prefix] }
        primitives: [lock]
      impl: { type: script_ref, ref: billing/order_checkout }

    - name: mark-paid
      description: Transition to paid — called by payment gateway webhook
      required_permission: orders.mark-paid
      idempotent: true            # FR3 — framework-enforced
      idempotency_key: { from: param, field: event_id }
      audit: true
      emits: paid
      params:
        validate:
          - { field: gateway_reference, rules: [required] }
      impl: { type: script_ref, ref: billing/order_mark_paid }

  events:
    - name: paid
      description: Order successfully paid
      publish:
        durable: true             # FR4 — mandatory for financial events
      payload:
        fields: [id, number, total, customer_id, paid_at]
      deliver:
        - channel: audit_log
        - channel: websocket
          target: { scope: tenant }               # FR6 — dashboard ticker
        - { channel: queue, job: generate-receipt }        # FR10 + FR7
        - { channel: queue, job: send-receipt-email }      # FR5 — receipt = billing promise
        - channel: reliable_event                 # FR4 — journal, no loss
          target: { resource: gl.journal-entry, action: create }   # defined in Step 6
          retry: { max: 10, backoff: exponential, initial_delay_ms: 1000 }
          # failed-event is the built-in forma.core dead-letter entity
          # (Core Basic §22) — nothing to define in this tutorial
          dead_letter: { resource: failed-event, action: create }
          idempotency_key: "order.paid.{id}"
```

**What's happening here:**
- FR1 (sequential numbers): `natural_key_rule` with `strategy: sequence` and `ctx.next_key()` — the framework handles locking, reset periods, and formatting. No `MAX()+1` ever.
- FR3 (idempotency): `idempotent: true` on the `mark-paid` action. The framework maintains an idempotency store and replays the original response on duplicates.
- FR4 (reliable events): `publish.durable: true` + `deliver.reliable_event`. The order `paid` event and the journal entry creation are atomic (same DB transaction for entities). The outbox worker delivers idempotently.
- The state machine allows `draft → awaiting_payment` via `checkout`, `awaiting_payment → paid` via `mark-paid`. All other transitions are blocked.
- `update` is restricted to `draft` status only. `delete` is disabled for audit integrity.

---

## 5. Step 3 — Checkout Script

Create `modules/billing/scripts/order_checkout.star`:

```python
def execute(resource, params, ctx):
    # FR1 — Natural key: ctx.next_key reads the field's natural_key_rule,
    # handles locking, reset, and formatting. Never MAX()+1 manually.
    number = ctx.next_key("number")
    resource.set("number", number).save()

    # FR2 — Gateway called ONLY via the declared Service wrapper (defined in Step 5).
    # In dev: auto-routes to mockup. In prod: real connector.
    session = payment_gateway.call("create-session", {
        "order_id": resource.id,
        "amount":   resource.total,
    })

    ctx.log.info("order.checkout", {"order_id": resource.id, "number": number})
    return ok({"payment_url": session.payment_url})
```

---

## 6. Step 4 — Mark-Paid Script

Create `modules/billing/scripts/order_mark_paid.star`:

```python
def execute(resource, params, ctx):
    # FR3 is already handled BEFORE this line runs:
    # the framework rejects duplicate event_ids and replays the original response.
    resource.set("gateway_reference", params.gateway_reference)
    resource.set("paid_at", ctx.now())
    resource.save()   # Transitions awaiting_payment→paid AND writes the "paid"
                      # event to the outbox — ONE DB transaction (answers the
                      # second Scenario A bug: non-atomic status+journal).
    return ok()
```

That's 3 lines of business logic. All consequences of payment (journal, receipt, email, dashboard) are in the `deliver` block — declarative, auditable, and never forgotten.

---

## 7. Step 5 — Payment Gateway Service

Create `modules/billing/services/payment-gateway.yaml`:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Service
metadata:
  name: payment-gateway
  module: billing
  description: Wrapper for Midtrans/Xendit (mockup in dev)
spec:
  version: v1
  actions:
    - name: create-session
      required_permission: payment-gateway.create-session
      impl: { type: native, ref: "PaymentGateway.CreateSession" }

    - name: webhook
      description: Callback from gateway — verify, then forward to order
      required_permission: payment-gateway.webhook
      idempotent: true
      idempotency_key: { from: param, field: event_id }
      uses:
        resources: [order.mark-paid]
      impl: { type: native, ref: "PaymentGateway.Webhook" }
```

Because this is a `kind: Service`, it automatically gets auth, permission enforcement, audit, and workspace isolation — no way to "forget to secure" an external integration.

---

## 8. Step 6 — Journal Entry Entity (Receiver)

Create `modules/gl/entities/journal-entry.yaml`:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: journal-entry
  module: gl
spec:
  version: v1
  characteristics: [transaction]
  fields:
    - { name: source,     type: string, immutable: true, index: true }
    - { name: source_id,  type: uuid,   immutable: true, index: true }
    - { name: amount,     type: decimal, rules: [required] }
    - { name: entry_date, type: date,    rules: [required] }
  actions:
    - name: create
      required_permission: journal-entries.create
      idempotent: true      # Outbox worker may retry — duplicates rejected here too
      # §11.3: idempotent requires a key source. The outbox worker passes the
      # publisher's delivery key ("order.paid.{id}") as the source_ref param.
      idempotency_key: { from: param, field: source_ref }
      params:
        validate:
          - { field: source_ref, rules: [required] }
      audit: true
```

The outbox worker calls `create` **synchronously** with idempotency checks on both sides — the journal never vanishes and never duplicates, even on retry.

---

## 9. Step 7 — WA Notification via Subscription (D35)

Real scenario: the system has been running for months. Someone wants WA notifications on order payment. **`order.yaml` is not touched** — especially if `billing` is a signed marketplace module that can't be edited.

Create `modules/notifications/subscriptions/wa-on-order-paid.yaml`:

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

The dividing line (D35): journal stays in `order`'s `deliver` because it's a billing promise (FR4). WA is `notifications` module's concern — Subscription. `forma describe entity billing.order` shows the merged fan-out (publisher deliver + all Subscriptions).

---

## 10. Step 8 — Run It

```bash
forma dev
```

This starts the complete local environment: Postgres, Valkey, Mailpit, MinIO, `forma-control` (dev/relaxed policy), and `forma-resource` with hot reload.

- Visit `http://localhost:8080/_admin` — the admin panel is derived automatically from your entities
- Create a customer, create an order, checkout → the payment gateway auto-routes to the mockup in dev
- Trigger the mock webhook → order transitions to `paid` → journal entry created, receipt PDF generated, email sent

---

## 11. What Just Happened — Concept Map

| Requirement | Forma Construct | Why It Works |
|---|---|---|
| FR1 — Sequential numbers | `natural_key_rule` + `ctx.next_key()` | Locking, reset, formatting handled by framework |
| FR2 — Payment gateway | `kind: Service` | External integration wrapped; mockup in dev via config |
| FR3 — Webhook idempotency | `idempotent: true` | Framework-enforced store + response replay |
| FR4 — Reliable journal | `publish.durable` + `deliver.reliable_event` | Outbox in same DB transaction, sync delivery with idempotency |
| FR5 — Email reliability | publisher `deliver channel: queue` | Background job with retry — email is a billing promise, so it lives in `order`'s deliver |
| FR5 — WA reliability | `kind: Subscription` (Step 7) | Same queue-with-retry guarantee, added later without touching `order.yaml` |
| FR6 — Live dashboard | `deliver channel: websocket` | Non-durable — loss acceptable |
| FR7 — Cached discount | `ctx.cache` + manual invalidation | Cache is volatile by contract — a discount rule change explicitly invalidates, so stale tiers never linger |
| FR8 — Config per workspace | `kind: Config` + `ctx.config()` | Values are read per workspace at runtime — prefix and templates change without redeploy |
| FR9 — Structured logging | `ctx.log` — tenant/request/user auto-injected | One correlation ID follows the whole flow — nothing to wire by hand |
| FR10 — PDF storage | `ctx.storage.write()` | Object storage, not filesystem |

---

## Next Steps

- Read the [Order-to-Cash Companion](./09-order-to-cash-companion.md) for the deep technical analysis, including the "without Forma vs with Forma" comparison
- Explore [Core Extended](./03-core-extended.md) for Workflow (approval chains), Webhook signature verification, and Mockup environment binding
- See [Entity Extension](./10-entity-extension.md) to learn how to add custom fields to marketplace modules without forking

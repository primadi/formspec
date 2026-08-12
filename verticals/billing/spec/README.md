# billing — Spec

**Klasifikasi:** **App** standalone, independently installable — see [`docs/architecture/07-vertical-modules.md`](../../../docs/architecture/07-vertical-modules.md).
**Spec target:** FormSpec Core Basic v0.2.0 + Core Extended stub.

> Formerly split across two examples that turned out to be **the same module all along** (`module: billing` in both `module.yaml`s — see `SPEC-COMPATIBILITY-NOTES.md` T3): `examples/Customer` (customer/address entities + customer-management UI) and `examples/Order-to-Cash/spec/modules/billing` (order/checkout/payment-gateway, plus its own copy of customer/address). This merge is the fix for that duplication — one canonical `billing` module. Order-to-Cash's copy was kept as canonical (fuller); Customer's non-duplicate customer-management UI (`forms/customer-*`, `pages/customer-*`, `tables/customer-*`, `config/member-discounts.yaml`) was folded in. The `notifications` and `gl` modules that used to live alongside this in Order-to-Cash's `spec/modules/` are now their own apps (`verticals/notifications`, `verticals/gl`).

## Struktur

```
verticals/billing/
├── spec/
│   ├── README.md
│   ├── formspec.yaml                                    # kind: App "billing", publishes: order-events
│   ├── config/app.yaml
│   └── modules/
│       └── billing/
│           ├── module.yaml                           # kind: Module "billing"
│           ├── entities/
│           │   ├── order.yaml                        # [transaction] — inti order-to-cash
│           │   ├── customer.yaml                     # [master] — data pelanggan
│           │   └── address.yaml                      # [master] — alamat
│           ├── forms/, pages/, tables/, widgets/      # UI — order + customer management
│           ├── services/                             # payment-gateway, notify-wa
│           ├── mockups/, webhooks/                    # payment-gateway-mock, midtrans-webhook
│           ├── menus/
│           ├── config/                                # midtrans.yaml, member-discounts.yaml
│           └── scripts/
│
└── impl/
    └── billing/
        ├── order_handler.go            # OrderResource.UpdateDiscountRule
        ├── payment_gateway.go          # PaymentGateway.CreateSession, .Webhook
        ├── notify_wa.go                # NotifyWA.Send
        ├── generate_receipt.go         # Job: generate-receipt
        └── send_receipt_email.go       # Job: send-receipt-email
```

## App Identity

- **Name:** `billing`
- **Vendor:** `formspec-dev`
- **Modules:** `billing`
- **Permission namespace:** `billing.*` (e.g. `billing.orders.checkout`)
- **Publishes:** `order-events` (approximate mapping — see the architecture doc's gap list: `consumes`/`publishes` is only spec'd for `kind: Service`, not for a plain entity event like `order.paid`) — consumed by `notifications`, `sales-inventory-integrator`, `sales-gl-integrator`

## Relasi dengan vertical lain

`order.yaml`'s `paid` event has **two separate integration mechanisms live at once**, both already present before this reorg:

1. A direct `deliver.reliable_event` targeting `gl.journal-entry` (create), inline on the event itself — retry/dead-letter/idempotency declared right there.
2. Three other apps each independently `consumes` `billing`'s order events via their own `kind: Subscription`: `notifications` (WhatsApp), `sales-inventory-integrator` (stock movement), `sales-gl-integrator` (a *second*, Subscription-driven path to the same gl outcome as #1).

This overlap (gl gets notified both ways) was already present in the original examples and is left as-is here — see the architecture doc for discussion of which pattern (inline reliable_event vs. separate Subscription app) is preferable for future integrations.

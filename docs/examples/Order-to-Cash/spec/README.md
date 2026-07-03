# Order-to-Cash — Spec

**Klasifikasi:** **App** (`tokoku`) — contoh kanonik Forma (Foundation D16).
**Spec target:** Forma Core Basic v0.2.0 + Core Extended stub.

## Struktur

```
Order-to-Cash/
├── spec/
│   ├── README.md
│   ├── forma.yaml                                          # kind: App "tokoku"
│   ├── modules/
│   │   ├── billing/
│   │   │   ├── module.yaml                                 # kind: Module
│   │   │   ├── entities/
│   │   │   │   ├── order.yaml                              # [transaction] — inti O2C
│   │   │   │   ├── customer.yaml                           # [master] — data pelanggan
│   │   │   │   └── address.yaml                            # [master] — alamat
│   │   │   ├── services/
│   │   │   │   ├── payment-gateway.yaml                    # kind: Service — wrapper Midtrans
│   │   │   │   └── notify-wa.yaml                          # kind: Service — wrapper WA
│   │   │   ├── mockups/
│   │   │   │   └── payment-gateway-mock.yaml               # kind: Mockup — simulasi dev
│   │   │   ├── webhooks/
│   │   │   │   └── midtrans-webhook.yaml                   # kind: Webhook — verifikasi signature
│   │   │   ├── config/
│   │   │   │   └── midtrans.yaml                           # kind: Config — server_key, mock_enabled
│   │   │   └── scripts/
│   │   │       ├── order_checkout.star
│   │   │       ├── order_mark_paid.star
│   │   │       ├── customer_blacklist.star
│   │   │       ├── customer_update_tier.star
│   │   │       ├── midtrans_mock_create_session.star
│   │   │       └── midtrans_mock_webhook.star
│   │   ├── notifications/
│   │   │   ├── module.yaml
│   │   │   └── subscriptions/
│   │   │       └── wa-on-order-paid.yaml                   # kind: Subscription — reaksi dari luar
│   │   └── gl/
│   │       ├── module.yaml
│   │       ├── entities/
│   │       │   ├── journal-entry.yaml                      # [transaction] — jurnal akuntansi
│   │       │   └── gl-balance.yaml                         # [summary] — saldo per akun
│   │       ├── subscriptions/
│   │       │   └── order-to-journal.yaml                   # kind: Subscription → order.paid
│   │       └── scripts/
│   │           └── journal_post.star
│   └── config/
│       └── app.yaml                                        # kind: Config — order_number_prefix
│
└── impl/
    ├── billing/
    │   ├── order_handler.go                                # OrderResource.UpdateDiscountRule
    │   ├── payment_gateway.go                              # PaymentGateway.CreateSession, .Webhook
    │   ├── notify_wa.go                                    # NotifyWA.Send
    │   ├── generate_receipt.go                             # Job: generate-receipt
    │   └── send_receipt_email.go                           # Job: send-receipt-email
    ├── notifications/
    │   └── send_wa_notification.go                         # Job: send-wa-notification
    └── gl/
        └── create_sales_journal.go                         # Job: create-sales-journal
```

## App Identity

- **Name:** `tokoku`
- **Vendor:** `acme-corp`
- **Modules:** `billing`, `notifications`, `gl`
- **Permission namespaces:** `billing.*`, `notifications.*`, `gl.*`

## Alur Bisnis Utama

```
Customer checkout → Buat sesi pembayaran → Customer bayar →
Webhook Midtrans → Verifikasi signature → order.mark-paid →
┌──────────────────── deliver ──────────────────────┐
│ • audit_log                                         │
│ • websocket (ticker admin, boleh hilang)            │
│ • queue → generate-receipt (PDF nota, FR10)         │
│ • queue → send-receipt-email (email nota, FR5)      │
│ • reliable_event → gl.journal-entry.create (FR4)    │
└─────────────────────────────────────────────────────┘
  + Subscription → WA notification (reaksi dari luar)
```

## Konsep yang di-cover

| Konsep | Lokasi | FR |
|---|---|---|
| `natural_key_rule: { strategy: sequence }` | order.number | FR1 |
| `child: { storage: jsonb }` | order.items | — |
| `kind: Service` — integrasi eksternal | payment-gateway, notify-wa | FR2 |
| `kind: Webhook` — verifikasi signature | midtrans-webhook.yaml | FR3 |
| `kind: Mockup` — simulasi dev | payment-gateway-mock.yaml | FR2 |
| `idempotent: true` + `idempotency_key` | order.mark-paid, webhook | FR3 |
| `publish.durable: true` | order.paid event | FR4 |
| `deliver` — 4 kelas jaminan | audit_log, websocket, queue, reliable_event | FR4-FR6 |
| `conditions` — precondition deklaratif | order.checkout | — |
| `uses` — deklarasi akses eksplisit | order.checkout, mark-paid | D20 |
| `ctx.cache` + invalidasi | generate-receipt, update-discount-rule | FR7 |
| `kind: Config` — per-workspace | order_number_prefix | FR8 |
| `ctx.log` — correlation ID otomatis | semua script | FR9 |
| `ctx.storage.write` | generate-receipt (PDF) | FR10 |
| `kind: Subscription` (D35) | wa-on-order-paid, order-to-journal | FR5 |
| `deliver.reliable_event` cross-module | order.paid → gl.journal-entry | FR4 |

## Temuan Test Drive Spec v0.2.0

1. **`idempotent: true` semantics** — SELESAI (D32 + §11.8), handler mark-paid kini 3 baris
2. **Cross-resource validation (blacklist)** — di Extended; di Core Basic, rumahnya `conditions` di action
3. **Cross-module target qualified** — `resource: gl.journal-entry` bukan nama polos
4. **Webhook signature verification** — Extended `kind: Webhook`, tempatnya sudah pasti
5. **Garis deklaratif vs imperatif (D33)** — fakta/jaminan → YAML, prosedur → handler, konsekuensi → `deliver`
6. **`kind: Subscription` (D35)** — reaksi dari luar tanpa sentuh `order.yaml`

## Relasi dengan Example Lain

| Example | Relasi |
|---|---|
| **Customer** | `order.customer_id` → `customer.id`; `conditions` cek blacklist |
| **Midtrans PG** | `payment-gateway` Service + Mockup + Webhook ada di sini |
| **General Ledger** | `order.paid` → reliable_event → `gl.journal-entry.create` |
| **Inventory** | `order.paid` → Subscription → stock-movement (out) |

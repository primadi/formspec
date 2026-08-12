# Midtrans Payment Gateway — Spec

**Klasifikasi:** Module (`billing`), Service wrapper untuk Midtrans API.
**Spec target:** FormSpec Core Basic v0.2.0 + Core Extended stub (Mockup, Webhook, Environment Binding).

## Struktur

```
Midtrans-Payment-Gateway/
├── spec/
│   ├── README.md
│   ├── module.yaml                     # kind: Module — namespace "billing"
│   ├── services/
│   │   └── midtrans.yaml               # kind: Service (konektor asli)
│   ├── mockups/
│   │   └── midtrans-mock.yaml          # kind: Mockup (simulasi dev/CI)
│   ├── webhooks/
│   │   └── midtrans-webhook.yaml       # kind: Webhook (verifikasi HMAC-SHA512)
│   ├── config/
│   │   └── midtrans.yaml               # kind: Config (server_key, mock_enabled)
│   └── scripts/
│       ├── midtrans_create_session.star # STUB — harus native
│       ├── midtrans_webhook.star        # Penerus ke order.mark-paid
│       ├── midtrans_mock_create_session.star
│       └── midtrans_mock_webhook.star
│
└── impl/
    └── billing/
        └── midtrans.go                 # PaymentGateway.CreateSession, .Webhook, .CheckStatus
```

## Module Identity

- **Name:** `billing`
- **Permission namespace:** `billing.*` (contoh: `billing.midtrans.create-session`)
- **Dependencies:** `formspec/core`

## Konsep yang di-cover

| Konsep | Lokasi | Spec Source |
|---|---|---|
| `kind: Service` — integrasi eksternal | midtrans.yaml | Core Basic §4.2 |
| `kind: Mockup` — simulasi dev | midtrans-mock.yaml | Extended §1 |
| `kind: Webhook` — verifikasi signature | midtrans-webhook.yaml | Extended §2 |
| `kind: Config` — per-environment values | midtrans.yaml | Core Basic §7 |
| Environment binding (mock vs real) | mock_enabled config | Extended §3 |
| `idempotent: true` + `idempotency_key` | midtrans.yaml action webhook | Core Basic §11.8/D32 |
| `uses.resources` — deklarasi dependency | midtrans.yaml (order.mark-paid) | Core Basic §4.7/D20 |
| `impl: { type: native }` | midtrans.yaml → impl/billing/midtrans.go | Core Basic §6.1 |

## Environment Routing

```
ctx.resource.call("midtrans", "create-session", ...)
        │
        ▼
Framework: cek config "billing.midtrans.mock_enabled"
        │
    ┌───┴───┐
    ▼       ▼
  true    false
    │       │
    ▼       ▼
Mockup   Service
(spec)   (impl)
```

Handler checkout TIDAK PERNAH menulis `if ctx.environment == "dev"`.

## Temuan Test Drive

1. **HTTP client di Starlark tidak tersedia** — `midtrans_create_session.star` adalah STUB. Panggilan HTTP ke Midtrans API harus melalui `impl: native`. Lihat `impl/billing/midtrans.go`.
2. **Satu Service, dua implementasi** — Mockup dan Real dipisahkan ke `kind: Mockup` + `kind: Service` dengan kontrak identik. Framework yang routing berdasarkan `mock_enabled` config.
3. **Verifikasi signature butuh raw body** — Detail implementasi framework, bukan spec.
4. **Semua `impl: native` method punya Go stub** di `impl/billing/midtrans.go` dengan `// TODO: implement`.

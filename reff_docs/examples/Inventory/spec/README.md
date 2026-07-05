# Inventory — Spec

**Klasifikasi:** **App** standalone. Bisa di-install ke workspace client, atau di-compose sebagai module oleh App lain.
**Spec target:** Forma Core Basic v0.2.0.

## Struktur

```
Inventory/
├── spec/
│   ├── README.md
│   ├── forma.yaml                                    # kind: App "inventory"
│   ├── modules/
│   │   └── inventory/
│   │       ├── module.yaml                           # kind: Module
│   │       ├── entities/
│   │       │   ├── product.yaml                      # [master] — produk
│   │       │   ├── warehouse.yaml                    # [master] — gudang
│   │       │   ├── stock-movement.yaml               # [transaction] — pergerakan stok
│   │       │   └── stock-level.yaml                  # [summary] — level stok real-time
│   │       ├── subscriptions/
│   │       │   └── order-to-movement.yaml            # kind: Subscription → order.paid
│   │       ├── scripts/
│   │       │   ├── movement_confirm.star
│   │       │   ├── movement_apply.star
│   │       │   └── stock_level_update.star
│   │       └── config/
│   │           └── inventory.yaml                    # kind: Config
│   └── config/
│       └── app.yaml
│
└── impl/
    └── inventory/
        └── create_out_movement.go                    # native Go handler untuk Subscription job
```

## App Identity

- **Name:** `inventory`
- **Vendor:** `forma-dev`
- **Modules:** `inventory`
- **Permission namespace:** `inventory.*` (contoh: `inventory.stock-movements.apply`)

## Konsep yang di-cover

| Konsep | Lokasi | Spec Source |
|---|---|---|
| `kind: App` — standalone deployable | forma.yaml | Core Basic §4.4 |
| `characteristics: [master]` | product, warehouse | Core Basic §9.1 |
| `characteristics: [transaction]` | stock-movement | Core Basic §9.1 |
| `characteristics: [summary]` — system-managed | stock-level | Core Basic §9.1 |
| `ctx.lock` — race condition prevention | movement_apply.star, stock_level_update.star | Core Basic §4.3 |
| `child: { storage: table }` — large volume | stock-movement.lines | Core Basic §10.3 |
| `natural_key_rule: { strategy: sequence }` | stock-movement.number | Core Basic §10.4 |
| `guard` — would_cause_negative_stock | stock-movement state_machine | Core Basic §14 |
| `deliver.reliable_event` | stock-movement events | Core Basic §12.3 |
| Cross-module `kind: Subscription` | order-to-movement.yaml | Core Basic §12.5/D35 |

## Pola Race Condition

```
TANPA LOCK (salah):
  Kasir A: baca stok = 10            Kasir B: baca stok = 10
  Kasir A: jual 7 → tulis stok = 3   Kasir B: jual 6 → tulis stok = 4
  Hasil: stok = 4 (seharusnya -3, tidak mungkin)

DENGAN LOCK (benar):
  Kasir A: lock("stock:p-001:w-01")   Kasir B: lock("stock:p-001:w-01") → TUNGGU
  Kasir A: baca stok = 10
  Kasir A: jual 7 → tulis stok = 3
  Kasir A: release lock
                                      Kasir B: lock DIDAPAT
                                      Kasir B: baca stok = 3
                                      Kasir B: jual 6 → FAIL (stok tidak cukup)
                                      Kasir B: release lock
```

## Relasi dengan Example Lain

| Example | Hubungan |
|---|---|
| **Order-to-Cash** | `order.paid` → Subscription → stock-movement (type=out), mengurangi stok |
| **General Ledger** | Tidak langsung |

## impl/

Satu Go stub: `impl/inventory/create_out_movement.go` — handler untuk Subscription job `create-out-movement`.

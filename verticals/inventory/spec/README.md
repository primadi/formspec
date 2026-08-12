# Inventory — Spec

**Klasifikasi:** **App** standalone, independently installable — see [`docs/architecture/07-vertical-modules.md`](../../../docs/architecture/07-vertical-modules.md).
**Spec target:** FormSpec Core Basic v0.2.0.

> Formerly `examples/Inventory`. Moved to `verticals/inventory`; the `order-to-movement` Subscription that used to live here was extracted to its own app, `verticals/sales-inventory-integrator` — inventory no longer reaches into `billing` itself. `warehouse` now has a `branch_id` relation to `company.branch` for multi-branch support (see the architecture doc for why that's a plain field, not new framework machinery).

## Struktur

```
verticals/inventory/
├── spec/
│   ├── README.md
│   ├── formspec.yaml                                    # kind: App "inventory", publishes: stock-movements,
│   │                                                  #   consumes: company (branch-directory)
│   ├── menus/, widgets/, reports/, tables/            # App-level UI
│   ├── modules/
│   │   └── inventory/
│   │       ├── module.yaml                           # kind: Module
│   │       ├── entities/
│   │       │   ├── product.yaml                      # [master] — produk
│   │       │   ├── warehouse.yaml                    # [master] — gudang, punya branch_id
│   │       │   ├── stock-movement.yaml               # [transaction] — pergerakan stok
│   │       │   └── stock-level.yaml                  # [summary] — level stok real-time
│   │       ├── scripts/
│   │       │   ├── movement_confirm.star
│   │       │   ├── movement_apply.star
│   │       │   └── stock_level_update.star
│   │       └── config/
│   │           └── inventory.yaml                    # kind: Config
│   └── config/
│       └── app.yaml
│
└── impl/                                             # (none currently — the one Go stub moved to
                                                        #  sales-inventory-integrator/impl/ with the subscription)
```

## App Identity

- **Name:** `inventory`
- **Vendor:** `formspec-dev`
- **Modules:** `inventory`
- **Permission namespace:** `inventory.*` (contoh: `inventory.stock-movements.apply`)
- **Publishes:** `stock-movements` service (`create`, `apply`) — consumed by `sales-inventory-integrator`
- **Consumes:** `company`'s `branch-directory` service (`read`) — for `warehouse.branch_id`

## Konsep yang di-cover

| Konsep | Lokasi | Spec Source |
|---|---|---|
| `kind: App` — standalone deployable | formspec.yaml | Core Basic §4.4 |
| `characteristics: [master]` | product, warehouse | Core Basic §9.1 |
| `characteristics: [transaction]` | stock-movement | Core Basic §9.1 |
| `characteristics: [summary]` — system-managed | stock-level | Core Basic §9.1 |
| `ctx.lock` — race condition prevention | movement_apply.star, stock_level_update.star | Core Basic §4.3 |
| `child: { storage: table }` — large volume | stock-movement.lines | Core Basic §10.3 |
| `natural_key_rule: { strategy: sequence }` | stock-movement.number | Core Basic §10.4 |
| `guard` — would_cause_negative_stock | stock-movement state_machine | Core Basic §14 |
| `deliver.reliable_event` | stock-movement events | Core Basic §12.3 |
| Cross-app `belongs_to` relation | warehouse.branch_id → company.branch | Core Basic §10.5 |

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

## Relasi dengan vertical lain

| Vertical | Hubungan |
|---|---|
| **company** | `warehouse.branch_id` → cross-app relation ke `company.branch` |
| **billing** | Tidak langsung sekarang — dulu via Subscription `order-to-movement` yang sudah dipindah ke app terpisah `sales-inventory-integrator` |
| **gl** | Tidak langsung |

## impl/

Kosong — satu Go stub yang dulu ada di sini (`create_out_movement.go`) sekarang di `verticals/sales-inventory-integrator/impl/`.

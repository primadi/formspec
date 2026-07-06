# Customer Module — Spec

**Klasifikasi:** Module (`billing`), bagian dari App `tokoku` (Order-to-Cash).
**Spec target:** Forma Core Basic v0.2.0.

## Struktur

```
spec/
├── README.md
├── module.yaml                  # kind: Module — namespace "billing"
├── entities/
│   ├── customer.yaml            # kind: Entity, characteristics: [master]
│   └── address.yaml             # kind: Entity, has_many dari customer
└── scripts/
    ├── customer_blacklist.star  # Action: blacklist customer
    └── customer_update_tier.star# Action: update membership tier
```

## Module Identity

- **Name:** `billing`
- **Permission namespace:** `billing.*` (contoh: `billing.customers.blacklist`)
- **Dependencies:** `forma/core`

## Entities

### `customer` (master)
Data induk pelanggan. Field penting:
- `email` — unique + index (bisa dipakai `find`)
- `is_blacklisted` — dicek di `order.checkout` sebagai precondition
- `member_tier` — snapshot saat checkout, tidak berubah retroaktif

### `address` (master)
Alamat pelanggan, `belongs_to` customer.

## Konsep yang di-cover

| Konsep | Lokasi |
|---|---|
| `characteristics: [master]` | customer, address |
| `type: relation` (belongs_to) | address.customer_id |
| `unique` + `index` | customer.email |
| `required_permission` | blacklist, update-tier actions |
| `audit: true` | blacklist, update-tier |
| `ctx.log` terstruktur | kedua script handler |
| `pattern` validation | customer.phone |

## Relasi dengan Example Lain

| Example | Hubungan |
|---|---|
| **Order-to-Cash** | `order.customer_id` → `customer.id`; `order.checkout` conditions cek `customer.is_blacklisted` |
| **Midtrans PG** | Tidak langsung — payment gateway adalah Service di module `billing` yang sama |

## impl/

Tidak ada `impl/` Go code. Semua action menggunakan `impl: { type: script_ref }` → pure Starlark, hot-updatable dari admin panel.

## Validasi

```bash
# Jika forma CLI sudah tersedia:
forma validate --spec spec/
```

# Service Demo — Service Runtime + Validation Rules

Contoh project FormSpec yang mendemonstrasikan dua fitur backend:

1. **`kind: Service` runtime (todo 7.1)** — komputasi stateless murni via script.
2. **Validation rules L1–L3 lengkap (todo 7.9.6)** — `length`, `in`, `script`, `unique`.

## Menjalankan

```bash
cd examples/service-demo
formspec dev --dev-ui
```

Login dev: `admin` / `admin`.

## Yang didemonstrasikan

### 1. Service runtime (`spec/modules/demo/services/tax-calculator.yaml`)

`kind: Service` adalah komputasi stateless — tidak ada state yang dipersist.
Dua action:

- **`calculate`** (sync) — `POST /api/v1/demo/tax-calculator/calculate`
  dengan body `{"amount":100,"rate":0.1}` → `{"tax":10,"total":110}`.
  Diimplementasikan via `impl: script_ref` (`demo/calculate_tax`).
- **`notify`** (`call: async`) — `POST /api/v1/demo/tax-calculator/notify`
  → `202 Accepted` (fire-and-forget, tanpa job_id/progress/result).

Script service di-resolve module-scoped: `spec/modules/demo/scripts/*.star`.
Ref memakai **slash notation** (`demo/calculate_tax`) agar `resolveScript`
mencocokkan pola `{basePath}/modules/{module}/scripts/{name}.star`.

### 2. Validation rules (`spec/modules/demo/master/product/entity.yaml`)

Entity `product` mendemonstrasikan rule L1–L3 yang lengkap:

| Field | Rule | Contoh gagal |
|---|---|---|
| `sku` | `length: 8` | `"TEH0001"` (7 char) → "length must be exactly 8" |
| `sku` | `unique` | duplikat `"KOPI0001"` → "value must be unique per tenant" |
| `status` | `in: [active, inactive, discontinued]` | `"banned"` → "value must be one of ..." |
| `min_stock` | `script: "value >= 0"` | `-1` → "script rule failed" |

Catatan: rule `unique` memakai format objek `{name: unique, value: true}`
(bukan `{name: unique}` tanpa value) — schema `ValidationRule` mewajibkan
`value` untuk rule ini.

## Struktur

```
spec/
  apps/service-demo.yaml          # App manifest
  modules/demo/
    module.yaml                   # Module manifest
    services/tax-calculator.yaml  # kind: Service
    scripts/calculate_tax.star    # script service sync
    scripts/notify_async.star     # script service async
    master/product/entity.yaml    # Entity dengan validation rules
```

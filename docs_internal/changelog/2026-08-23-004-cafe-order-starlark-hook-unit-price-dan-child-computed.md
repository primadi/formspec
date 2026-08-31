# 2026-08-23-004 — Cafe Order: Starlark Hook Auto-Fill unit_price + Child-Level Computed (line_total, total_amount)

## Apa yang diubah

Menjawab pertanyaan user: "bisakah dibuat sederhana (get function) dan pakai
Starlark, bukan JS?" — Ya. Solusi framework-native tanpa custom widget:

### Cafe example (`examples/cafe`)

`spec/modules/cafe-order/transaction/order/entity.yaml`:

- Child field `line_total` ditambahkan kembali sebagai **computed field**
  deklaratif: `computed: { formula: "quantity * unit_price" }` — "get function"
  yang diminta user.
- `total_amount` formula diubah ke `sum([i["line_total"] for i in items])`
  (bracket syntax server-side, spec §5.1).
- `hooks:` baru — `on: before, action: create|update` → script
  `fill_order_unit_price` (Starlark) yang auto-fill `unit_price` dari
  `menu_item_id` via `resource.fetch("cafe-master.menu-item", id)`.
- Action `create`/`update` dideklarasikan dengan
  `uses.resources: [cafe-master.menu-item]` — format `{module}.{entity}`
  (BUKAN `{module}.{entity}.{action}`) agar hook boleh baca lintas-module.

`spec/modules/cafe-order/transaction/order/scripts/fill_order_unit_price.star`
(baru) — hook Starlark: iterasi items, fetch harga menu item bila `unit_price`
kosong, tulis balik `items`.

### Backend framework fixes

- `renderers/jsonb-persist/crud.go` — `evaluateComputed` kini mengevaluasi
  **child-level computed** (per child record) sebelum top-level, sehingga
  `line_total` per baris dan agregat parent (`sum` atas child) bekerja.
- `internal/starlark/evaluator.go` — builtin **`sum`** ditambahkan (sebelumnya
  `sum(...)` gagal "undefined: sum"). Catatan: dict Starlark tidak mendukung
  akses atribut (`i.line_total`), jadi formula server memakai bracket
  (`i["line_total"]`) — sesuai spec §5.1; frontend FormSpecExpr tetap dot.

### Test

- `internal/starlark/evaluator_test.go` — `TestEvalExpr_SumOverChildItems`.
- `renderers/jsonb-persist/crud_test.go` — `TestEntityStore_ChildComputedField`
  (line_total per baris + total_amount agregat). 303 test pass.

## Kenapa diubah

Use case dasar (order line items: harga otomatis dari master, subtotal per
baris, total) harus bisa ditangani FormSpec tanpa custom widget JS. Solusi:
computed field deklaratif untuk turunan murni (line_total, total_amount) +
Starlark hook untuk logika yang butuh fetch (unit_price dari menu_item_id).

## File terdampak

- `examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml`
- `examples/cafe/spec/modules/cafe-order/transaction/order/scripts/fill_order_unit_price.star` (baru)
- `renderers/jsonb-persist/crud.go`
- `renderers/jsonb-persist/crud_test.go`
- `internal/starlark/evaluator.go`
- `internal/starlark/evaluator_test.go`

## Referensi

- `docs/spec/backend/01-core-basic.md` §5 (action impl, uses), §15 hook
- `docs/spec/backend/02-core-extended.md` §15 Hook Spec
- `docs/spec/backend/05-field-types.md` §5.1 computed field (bracket syntax)

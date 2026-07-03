# Forma Example: Inventory Module

**Status:** Draft — contoh modul vertikal dengan race condition `ctx.lock`,
`child: { storage: table }` untuk volume besar, dan summary entity.
**Spec target:** Forma Core Basic v0.2.0.

---

## 1. Kebutuhan Bisnis: Inventory Management

**Alur:** barang masuk (purchase) / keluar (sales) → stock movement
tercatat → stock level ter-update real-time → tidak boleh negatif.

| # | Requirement | Kenapa sulit tanpa konvensi |
|---|---|---|
| FR1 | Dua kasir kurangi stok bersamaan — tidak boleh negatif | Race condition klasik |
| FR2 | Stock movement bervolume besar (ratusan ribu per hari) | Child table, bukan jsonb |
| FR3 | Stock level real-time per produk per gudang | Summary entity via reliable event |
| FR4 | Multi-warehouse — stok per gudang terisolasi | Workspace isolation + warehouse entity |
| FR5 | Nomor movement urut: `MOV-{year}-{seq}` | Sequence + lock |

---

## 2. Struktur project

```
inventory/
  module.yaml
  entities/
    product.yaml
    warehouse.yaml
    stock-movement.yaml
    stock-level.yaml          # summary
  subscriptions/
    order-to-movement.yaml    # kind: Subscription → billing.order.confirmed
  scripts/
    movement_confirm.star
    stock_level_update.star
```

---

## 3. Entity `product`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: product
  module: inventory
  description: Produk yang di-stock
spec:
  version: v1
  characteristics: [master]

  fields:
    - name: sku
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      rules: [required]
    - name: name
      type: string
      rules: [required]
    - name: unit
      type: enum
      enum_values: [pcs, kg, liter, box, pack]
      default: pcs
    - name: is_active
      type: boolean
      default: true
```

---

## 4. Entity `warehouse`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: warehouse
  module: inventory
  description: Gudang / lokasi penyimpanan
spec:
  version: v1
  characteristics: [master]

  fields:
    - name: code
      type: string
      natural_key: true
      immutable: true
      unique: true
      rules: [required]
    - name: name
      type: string
      rules: [required]
    - name: is_active
      type: boolean
      default: true
```

---

## 5. Entity `stock-movement` — Transaction

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: stock-movement
  module: inventory
  description: Pergerakan stok — masuk, keluar, transfer antar gudang
spec:
  version: v1
  characteristics: [transaction]

  fields:
    - name: number
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      natural_key_rule:
        strategy: sequence
        format: "MOV-{year}-{seq:06d}"
        prefix: { value: "MOV" }
        reset: yearly
    - name: type
      type: enum
      enum_values: [in, out, transfer]
      index: true
      immutable: true
    - name: warehouse_id
      type: relation
      relation: { type: belongs_to, resource: warehouse }
      immutable: true
    - name: reference
      type: string
      description: Dokumen sumber — "ORD-2026-000123", "PO-2026-000045"
    - name: status
      type: enum
      enum_values: [draft, confirmed, applied]
      index: true
    - name: lines
      type: child
      child:
        storage: table              # volume besar → perlu query & index
        sequence_field: line_number
        fields:
          - { name: line_number, type: integer, immutable: true }
          - { name: product_id, type: uuid, rules: [required, {exists: product}] }
          - { name: quantity,  type: decimal, rules: [required, positive] }
          - { name: unit,      type: string }
          - { name: notes,     type: string }

  state_machine:
    field: status
    initial: draft
    transitions:
      - { from: draft,     to: confirmed, via: confirm }
      - { from: confirmed, to: applied,   via: apply,
          guard: "not would_cause_negative_stock(resource)" }
      # applied = final

  actions:
    - name: confirm
      description: Konfirmasi movement — belum mempengaruhi stok
      required_permission: stock-movements.confirm
      audit: true
      impl: { type: script_ref, ref: inventory/movement_confirm }

    - name: apply
      description: Terapkan movement ke stok — cek & update stock level
      required_permission: stock-movements.apply
      audit: true
      emits: stock-applied
      uses:
        primitives: [lock]           # FR1 — lock untuk cegah race condition
      conditions:
        - script: "resource.status == 'confirmed'"
          message: "Hanya movement confirmed yang bisa di-apply"
      impl: { type: script_ref, ref: inventory/movement_apply }

  events:
    - name: stock-applied
      description: Movement berhasil diterapkan — update stock level
      publish:
        durable: true
      payload:
        fields: [id, number, type, warehouse_id]
      deliver:
        - channel: reliable_event
          target: { resource: inventory.stock-level, action: update }
          retry: { max: 5, backoff: exponential }
          idempotency_key: "stock-level.{id}"
```

---

## 6. Entity `stock-level` — Summary

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: stock-level
  module: inventory
  description: Level stok per produk per gudang — system-managed
spec:
  version: v1
  characteristics: [summary]

  fields:
    - name: product_id
      type: relation
      relation: { type: belongs_to, resource: product }
      immutable: true
      index: true
    - name: warehouse_id
      type: relation
      relation: { type: belongs_to, resource: warehouse }
      immutable: true
      index: true
    - name: quantity_on_hand
      type: decimal
      default: 0
      description: Stok fisik saat ini
    - name: quantity_reserved
      type: decimal
      default: 0
      description: Stok yang sudah di-reserve (order confirmed, belum shipped)
    - name: quantity_available
      type: decimal
      default: 0
      description: on_hand - reserved — yang bisa dijual
    - name: last_movement_at
      type: datetime

  actions:
    - name: update
      description: Dipanggil outbox worker — tambah/kurangi stok
      idempotent: true
      uses:
        primitives: [lock]           # lock per (product, warehouse)
      impl: { type: script_ref, ref: inventory/stock_level_update }
```

---

## 7. Script Handler

### 7.1 `movement_apply.star` — dengan lock untuk cegah race condition

```python
# modules/inventory/scripts/movement_apply.star

def execute(resource, params, ctx):
    # FR1 — lock mencegah race condition: dua kasir kurangi
    # stok bersamaan. Lock diambil untuk setiap produk di movement.
    locked_keys = []
    for line in resource.field.lines:
        key = "stock:" + line.product_id + ":" + resource.field.warehouse_id
        ctx.lock.acquire(key, ttl=10)
        locked_keys.append(key)

    # Semua lock didapat → cek apakah stok cukup
    # Guard `would_cause_negative_stock` dicek ulang di sini
    # dengan data terkini (setelah lock)
    for line in resource.field.lines:
        level = stock_level.query() \
            .where("product_id", line.product_id) \
            .where("warehouse_id", resource.field.warehouse_id) \
            .first()

        if resource.field.type == "out":
            available = level.field.quantity_available if level else 0
            if available < line.quantity:
                # Release semua lock sebelum return fail
                for k in locked_keys:
                    ctx.lock.release(k)
                return fail("Stok tidak cukup untuk produk " + line.product_id)

    # Stok cukup → transisi status + tulis outbox
    resource.set("status", "applied")
    resource.save()    # transisi confirmed→applied + outbox stock-applied

    # Lock akan auto-release saat TTL habis atau context berakhir
    ctx.log.info("movement.applied", {
        "movement_id": resource.id,
        "number": resource.field.number,
        "type": resource.field.type,
    })
    return ok()
```

### 7.2 `stock_level_update.star`

```python
# modules/inventory/scripts/stock_level_update.star

def execute(resource, params, ctx):
    movement = stock_movement.load(params.id)

    for line in movement.field.lines:
        key = "stock:" + line.product_id + ":" + movement.field.warehouse_id
        ctx.lock.acquire(key, ttl=5)

        level = stock_level.query() \
            .where("product_id", line.product_id) \
            .where("warehouse_id", movement.field.warehouse_id) \
            .first()

        if not level:
            level = stock_level.new() \
                .set("product_id", line.product_id) \
                .set("warehouse_id", movement.field.warehouse_id) \
                .set("quantity_on_hand", 0) \
                .set("quantity_available", 0)

        if movement.field.type == "in":
            level.set("quantity_on_hand",
                level.field.quantity_on_hand + line.quantity)
        elif movement.field.type == "out":
            level.set("quantity_on_hand",
                level.field.quantity_on_hand - line.quantity)

        level.set("quantity_available",
            level.field.quantity_on_hand - level.field.quantity_reserved)
        level.set("last_movement_at", ctx.now())
        level.save()

        ctx.lock.release(key)

    ctx.log.info("stock_level.updated", {
        "movement_id": movement.id,
        "lines": len(movement.field.lines),
    })
    return ok()
```

---

## 8. Subscription — reaksi ke order.confirmed

```yaml
# modules/inventory/subscriptions/order-to-movement.yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata:
  name: order-to-movement
  module: inventory
spec:
  on: { resource: billing.order, event: paid }
  deliver:
    - channel: queue
      job: create-out-movement
      # Job handler (native Go) menerima PaidEvent,
      # membuat stock-movement type=out untuk setiap item di order,
      # warehouse default, reference = order.number.
      # Status: langsung confirmed (auto-apply via job).
```

---

## 9. Pola Race Condition — Sebelum vs Sesudah `ctx.lock`

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
                                      Kasir B: return error ke user
```

---

## 10. Pemetaan ke Primitif

| Primitif | Dipakai di | FR |
|---|---|---|
| `ctx.lock` | movement.apply, stock_level.update | FR1 |
| `ctx.cache` | (opsional) frequently-accessed stock levels | — |

---

## 11. Yang di-cover oleh contoh ini

| Konsep | Dimana |
|---|---|
| `child: { storage: table }` untuk volume besar | stock-movement.lines |
| `ctx.lock` untuk cegah race condition | movement.apply, stock_level.update |
| Guard + lock = double protection | would_cause_negative_stock + lock |
| `characteristics: [summary]` | stock-level entity |
| Reliable event ke summary | stock-applied → stock-level.update |
| `kind: Subscription` cross-module | order-to-movement |
| `natural_key_rule: sequence` | stock-movement.number, product.sku, warehouse.code |
| Pattern: lock → cek → tulis → release | seluruh handler apply + update |

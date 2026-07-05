# Forma Example: Customer Module (stub)

**Status:** Stub — melengkapi contoh Order-to-Cash yang mereferensikan
`customer` entity tanpa mendefinisikannya.
**Spec target:** Forma Core Basic v0.2.0.
**Fungsi:** referensi untuk `order.customer_id` dan `conditions` blacklist
di Order-to-Cash.

---

## 1. Entity `customer`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: customer
  module: billing
  description: Data pelanggan — dipakai order, invoice, dll.
spec:
  version: v1
  characteristics: [master]

  auth: { required: true, strategies: [token] }

  fields:
    - name: name
      type: string
      rules: [required, min_length: 1]
    - name: email
      type: string
      unique: true
      index: true
      rules: [required, email]
    - name: phone
      type: string
      rules: [pattern: "^\\+?[0-9]{7,15}$"]
    - name: is_blacklisted
      type: boolean
      default: false
      description: True jika customer diblokir — dicek di order.checkout
    - name: member_tier
      type: enum
      enum_values: [regular, silver, gold]
      default: regular
    - name: notes
      type: string

  actions:
    - name: find
      # standard find works — by id or by unique field (email)

    - name: blacklist
      description: Tandai customer sebagai blacklisted
      required_permission: customers.blacklist  # → billing.customers.blacklist
      audit: true
      impl: { type: script_ref, ref: billing/customer_blacklist }

    - name: update-tier
      description: Admin menaikkan/turunkan tier membership
      required_permission: customers.manage-tier
      audit: true
      params:
        validate:
          - { field: member_tier, rules: [required] }
      impl: { type: script_ref, ref: billing/customer_update_tier }
```

---

## 2. Entity `address` (has_many dari customer)

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: address
  module: billing
  description: Alamat pelanggan — billing, shipping, dll.
spec:
  version: v1
  characteristics: [master]

  fields:
    - name: customer_id
      type: relation
      relation: { type: belongs_to, resource: customer }
      immutable: true
    - name: type
      type: enum
      enum_values: [billing, shipping, both]
      default: shipping
    - name: label
      type: string
      description: Nama label alamat — "Rumah", "Kantor"
    - name: street
      type: string
      rules: [required]
    - name: city
      type: string
      rules: [required]
    - name: postal_code
      type: string
    - name: is_default
      type: boolean
      default: false
```

---

## 3. Script `customer_blacklist`

```python
# modules/billing/scripts/customer_blacklist.star

def execute(resource, params, ctx):
    resource.set("is_blacklisted", True)
    resource.save()
    ctx.log.info("customer.blacklisted", {"customer_id": resource.id})
    return ok()
```

---

## 4. Script `customer_update_tier`

```python
# modules/billing/scripts/customer_update_tier.star

def execute(resource, params, ctx):
    old_tier = resource.field.member_tier
    resource.set("member_tier", params.member_tier)
    resource.save()
    ctx.log.info("customer.tier_updated", {
        "customer_id": resource.id,
        "old_tier": old_tier,
        "new_tier": params.member_tier,
    })
    return ok()
```

---

## 5. Yang di-cover oleh contoh ini

| Konsep | Dimana |
|---|---|
| `characteristics: [master]` | customer, address |
| `has_many` relation (implisit) | customer → addresses |
| `belongs_to` relation | address → customer |
| Field validation: email, phone pattern | customer fields |
| `unique` + `index` pada email | customer.email |
| Action dengan `required_permission` | blacklist, update-tier |
| `audit: true` pada action sensitif | blacklist, update-tier |
| `ctx.log` terstruktur | kedua script handler |
| Entity stub pendek — cukup jadi referensi | seluruh dokumen |

---

## 6. Hubungan dengan Order-to-Cash

| O2C element | Customer module |
|---|---|
| `order.customer_id` — `type: relation, belongs_to: customer` | `customer.id` |
| `order.checkout` — `conditions: "not customer.load(...).is_blacklisted"` | `customer.is_blacklisted` |
| `order.member_tier` — snapshot tier saat checkout | `customer.member_tier` |

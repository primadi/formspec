# Subscription

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `SubscriptionSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Subscription` adalah **reaksi lintas-module** terhadap event resource lain — tanpa mengubah publisher.

**Kapan memakai Subscription:**
- Module lain bereaksi ke event resource milik module ini (mis. `billing.invoice` → `on_submit`)
- Reliable event delivery (durable publisher + durable subscriber)

**Kapan TIDAK pakai Subscription:**
- Reaksi in-process sesama module → cukup event `on_*` di Entity itu sendiri
- Jembatan reaktif request/response antar module → `kind: Integrator`

**Sumber kontrak:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §7.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Subscription
metadata:
  name: on-invoice-submitted
  module: general-ledger
spec:
  events:
    - resource: billing.invoice
      action: submit
  handler:
    type: native
    ref: "GLHandler.OnInvoiceSubmitted"
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `events` | []`string` | — | billing.invoice.on_submit |  |
| `handler` | `ImplDecl` | ✅ |  |  |
| `store` | `string` | — | redis |  |
| `durable` | `string` | — |  | Tier 2: durability mode |
| `retry` | `RetryDecl` | — |  | Tier 2 |
| `position` | enum (latest · earliest) | — | latest |  |
| `filter` | `string` | — |  | Tier 2: Starlark filter over event payload |
| `transform` | `string` | — |  | Tier 2: Starlark transform over event payload |
| `dead_letter` | `DeliveryTarget` | — |  | Tier 2 |
| `max_retry` | `integer` | — |  | Tier 2 |
| `retention` | `string` | — |  | Tier 2: stream retention duration |
| `delivery` | `SubDeliveryDecl` | — |  | Tier 2: delivery channel |

<!-- /generated:attributes -->

## Gotchas

- **Reliabilitas mensyaratkan kedua sisi**: publisher durable + subscriber durable = reliable. Publisher non-durable + subscriber durable = error validasi.
- **`before_*` = sync, `on_*` = async** — konvensi penamaan mengunci tipe event. Event di luar pola wajib deklarasi `type` eksplisit.
- **Subscription masuk consent footprint module konsumen**.
- **Cross-ref:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §7 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

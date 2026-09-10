# Entity

<!-- generated:meta -->
| | |
|---|---|
| Grup | `data` |
| Plane | `resource` |
| Spec struct | `EntitySpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Entity` adalah **kind terpenting** FormSpec — merepresentasikan data
bisnis stateful yang dipersist. **95% kasus pembuatan aplikasi jawabannya
Entity.** Needing kind lain berarti memperluas framework, bukan membangun app.

Pilih karakteristik yang tepat (mutually exclusive — `formspec apply` menolak
lebih dari satu):

| Karakteristik | Arti | Wajib |
|---|---|---|
| `master` | Data referensi stabil (Customer, Product) | Boleh punya lifecycle atau tidak |
| `transaction` | Append-heavy, time-partitioned (Invoice, Journal Entry) | Wajib field `transaction_date` |
| `reference` | Seed data read-only (Provinsi, Tarif Pajak) | — |
| `summary` | Projeksi terkelola sistem (GL Balance) | CUD permanen nonaktif via API |

**Kapan TIDAK pakai Entity:**
- Komputasi tanpa state → `kind: Service`
- Hanya butuh UI override → tambah `kind: Form` / `kind: Table` (Entity tetap ada)

**Sumber kontrak:** [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md) §1.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: arisan-group
  module: arisan-master
  description: "Grup arisan — kumpulan anggota dengan iuran bulanan tetap"
spec:
  version: v1
  characteristic: master
  lifecycle: plain_crud
  display_field: name
  plural: arisan-groups
  fields:
    - name: code
      type: string
      required: true
      unique: true
      title: "Kode Grup"
    - name: name
      type: string
      required: true
      title: "Nama Grup"
    - name: monthly_amount
      type: money
      required: true
      title: "Iuran Bulanan"
  state_machine:
    field: status
    initial: active
    states:
      - { name: active, label: "Aktif" }
      - { name: completed, label: "Selesai" }
    transitions:
      - { from: active, to: completed, via: complete }
  actions:
    - name: submit
      disabled: true
    - name: complete
      description: "Tandai grup selesai"
      required_permission: arisan-master.arisan-group.complete
      audit: true
  expose:
    - type: rest
      actions: [list, find, create, update, delete]
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `version` | `string` | ✅ | v1 |  |
| `plural` | `string` | — | invoices |  |
| `characteristic` | enum (master · transaction · reference · summary) | — |  |  |
| `auth` | [`EntityAuth`](../../spec/backend/01-core-basic.md) | — |  |  |
| `persist` | [`PersistSpec`](../../spec/backend/04-persist-backend.md) | — |  |  |
| `fields` | [][`Field`](../../spec/backend/05-field-types.md) | — |  |  |
| `actions` | [][`Action`](../../spec/backend/01-core-basic.md) | — |  |  |
| `state_machine` | [`StateMachine`](../../spec/backend/02-core-extended.md) | — |  |  |
| `events` | [][`EventDecl`](../../spec/backend/01-core-basic.md) | — |  |  |
| `deliver` | [][`DeliveryDecl`](../../spec/backend/02-core-extended.md) | — |  |  |
| `indexes` | [][`IndexDecl`](../../spec/backend/01-core-basic.md) | — |  |  |
| `extend_storage` | [`ExtendStorage`](../../spec/backend/03-entity-extension.md) | — |  |  |
| `expose` | []`ExposeConfig` | — |  |  |
| `backdate_policy` | [`BackdatePolicy`](../../spec/backend/02-core-extended.md) | — |  |  |
| `forward_date_policy` | [`ForwardDatePolicy`](../../spec/backend/02-core-extended.md) | — |  |  |
| `hooks` | [][`HookDecl`](../../spec/backend/02-core-extended.md) | — |  |  |
| `rate_limit` | [`RateLimitSpec`](../../spec/backend/02-core-extended.md) | — |  | 1.4.1 resource-level rate limit (02-core-extended.md §17) |
| `soft_deactivate` | [`SoftDeactivateDecl`](../../spec/backend/02-core-extended.md) | — |  | 1.4.10 |
| `cache` | `CacheSpec` | — |  | Cache opts this entity into the framework read-through cache on |
| `lifecycle` | enum (two_step_autosave · two_step_manual · plain_crud) | — | plain_crud |  |
| `display_field` | `string` | — | name |  |

<!-- /generated:attributes -->

## Referensi Struct

Nama struct di kolom **Tipe** di atas adalah tipe Go di `pkg/spec/entity.go`
(dan `pkg/spec/spec.go`). Kontrak normatifnya didokumentasikan di `docs/spec/backend/`:

| Struct | Dokumentasi normatif |
|---|---|
| `Field` | [`05-field-types.md`](../../spec/backend/05-field-types.md) — katalog tipe (§1), money (§2), validasi (§3), tree (§4), keamanan & computed (§5) |
| `EntityAuth` | [`01-core-basic.md`](../../spec/backend/01-core-basic.md) §1.4 |
| `Action` | [`01-core-basic.md`](../../spec/backend/01-core-basic.md) §5 |
| `EventDecl` | [`01-core-basic.md`](../../spec/backend/01-core-basic.md) §7 |
| `StateMachine` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §1 |
| `DeliveryDecl` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §3 |
| `BackdatePolicy` / `ForwardDatePolicy` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §9 |
| `HookDecl` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §15 |
| `RateLimitSpec` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §17 |
| `SoftDeactivateDecl` | [`02-core-extended.md`](../../spec/backend/02-core-extended.md) §19 |
| `PersistSpec` | [`04-persist-backend.md`](../../spec/backend/04-persist-backend.md) |
| `ExtendStorage` | [`03-entity-extension.md`](../../spec/backend/03-entity-extension.md) |

## Gotchas

- **`spec.version: v1` wajib** di setiap Entity — `formspec apply` menolak tanpa ini.
- **Reserved fields** (`owner`, `created_at`, `modified`, `doc_status`, `amends`,
  `amended_by`, `version`) tidak boleh dipakai ulang sebagai nama field custom.
- **`lifecycle` adalah string enum** — bukan map `{doc_status: true}`. Nilai
  yang valid ada di tabel Atribut di atas.
- **`expose` adalah array** `{type, actions}` — shorthand `all`/`read`/`none`
  tidak ada. Omit expose = UI only (external API → 404).
- **`target:` di field relation diam-diam diabaikan** → dangling relation. Pakai
  `relation: { type: belongs_to, resource: <module.entity> }`.
- **Update setelah `submit` selalu ditolak, tanpa pengecualian.** Perubahan
  pasca-submit lewat custom action bernama.
- **`transaction` WAJIB** punya field `transaction_date` eksplisit.
- **`delete` guard absolut** (setara `ON DELETE RESTRICT`), tanpa
  `override_permission`.
- Dua lapis state berjalan paralel: `doc_status` (framework) + custom
  `state_machine` (developer) — lihat `docs/spec/backend/01-core-basic.md` §1.6.
- **Cross-ref:** [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)
  · [`docs/spec/backend/01-core-basic.md`](../spec/backend/01-core-basic.md)

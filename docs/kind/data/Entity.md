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
| `auth` | `EntityAuth` | — |  |  |
| `persist` | `PersistSpec` | — |  |  |
| `fields` | []`Field` | — |  |  |
| `actions` | []`Action` | — |  |  |
| `state_machine` | `StateMachine` | — |  |  |
| `events` | []`EventDecl` | — |  |  |
| `deliver` | []`DeliveryDecl` | — |  |  |
| `indexes` | []`IndexDecl` | — |  |  |
| `extend_storage` | `ExtendStorage` | — |  |  |
| `expose` | []`ExposeConfig` | — |  |  |
| `backdate_policy` | `BackdatePolicy` | — |  |  |
| `forward_date_policy` | `ForwardDatePolicy` | — |  |  |
| `hooks` | []`HookDecl` | — |  |  |
| `rate_limit` | `RateLimitSpec` | — |  | 1.4.1 resource-level rate limit (02-core-extended.md §17) |
| `soft_deactivate` | `SoftDeactivateDecl` | — |  | 1.4.10 |
| `lifecycle` | enum (two_step_autosave · two_step_manual · plain_crud) | — | plain_crud |  |
| `display_field` | `string` | — | name |  |

<!-- /generated:attributes -->

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

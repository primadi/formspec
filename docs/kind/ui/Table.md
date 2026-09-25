# Table

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `TableSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Table` adalah **override list/browse view** untuk satu Entity — kolom terderivasi dari entity, bisa di-override.

**Kapan memakai Table:**

- Pilih persis kolom + urutan (`columns`)
- Sort/filter/paginate (`filters`, `default_sort`, `search`)
- Inline/batch edit, row/bulk actions, realtime

**Kapan TIDAK pakai Table:**

- Entity cukup dengan default table → jangan deklarasi
- Status sebagai dimensi kerja utama → `kind: Kanban`
- Waktu sebagai narasi utama → `kind: Timeline`

**Prinsip 3-layer:** Table dan Form adalah bentuk override di layer yang sama (di atas Entity, di bawah Page).

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §3.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Table
metadata: { name: order-list, module: billing }
spec:
  public: true
  entity: billing.order
  columns:
    - { field: number, link: order-detail }
    - { field: customer.name }
    - { field: total, format: currency }
    - { field: status, widget: badge }
  filters:
    - { field: status, label: Status, type: select }
    - { field: created_at, label: "Created", type: date_range }
  default_sort: -created_at # "field" = asc, "-field" = desc
  search: true
  realtime: true
  row_actions: [mark-paid, void]
  bulk_actions: [export]
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/table/<name> is auto-generated. Set false for embed-only tables. |
| `entity` | `string` | ✅ | billing.order |  |
| `columns` | []`TableColumn` | — |  |  |
| `default_sort` | `string` | — | -created_at |  |
| `page_size` | `integer` | — |  |  |
| `search` | `boolean` | — |  |  |
| `realtime` | `boolean` | — |  |  |
| `row_actions` | []`TableAction` | — |  |  |
| `bulk_actions` | []`TableAction` | — |  |  |
| `filters` | [][`FilterSpec`](../../spec/frontend/06-page-kinds.md) | — |  |  |
| `fixed_filters` | [][`FilterSpec`](../../spec/frontend/06-page-kinds.md) | — |  |  |
| `batch_edit` | []`string` | — |  | Batch editing: fields editable across a multi-row selection. Framework runs update per row, partial failure reported per row. |

<!-- /generated:attributes -->

## Gotchas

- **`align`/`width` mengikat, bukan hiasan** — keduanya diterapkan ke sel header **dan** body kolom itu (`docs/spec/frontend/06-page-kinds.md` §3.1.1). `align` adalah himpunan tertutup (`left`/`center`/`right`); nilai lain ditolak validasi. Kolom yang diderivasi tidak menebak perataan dari tipe field.
- **`format` tidak diturunkan dari tipe field** — `decimal`/`integer` sama seringnya pengenal (`line_number`) maupun kuantitas, jadi kolom yang ingin pemisah ribuan menulis `format: number`. Skala `decimal` milik field menang atas `settings.decimal_scale` (§3.1.2).
- **Kolom relasi menampilkan label, bukan UUID** — `field: branch_id` (`belongs_to`) dirender sebagai nama record terkait, sama seperti dot-path `field: branch.name`. Keduanya sah; keduanya benar (§3.1.2).
- **Derivasi kolom tidak boleh membuang field diam-diam** — field sisa tetap terjangkau via row detail/expand (no-silent-drop). `columns` eksplisit menang penuh.
- **`inline_edit`** — field `readonly`/`compute`/`immutable` atau di luar permission `update` tidak editable. Commit = action `update` + `version` (CAS → 409 kalau stale). Baris `submitted` menolak inline-edit.
- **`batch_edit`** — partial failure dilaporkan per baris; tak pernah all-or-nothing diam-diam.
- **`default_sort` wajib mereferensikan field yang ada** di entity — framework-managed fields selalu valid; custom seperti `modified` tidak ada kecuali dideklarasi.
- **Kontrak filter (dipakai Table & Kanban)** — `filters` (bisa diubah user) vs `fixed_filters` (immutable server-side, tidak dirender).
- **`public: false`** → embed-only.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §3 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

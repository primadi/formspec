# Form

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `FormSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Form` adalah **override layout input/edit** untuk satu Entity — menggantikan form hasil derivasi otomatis.

**Kapan memakai Form:**
- Urutan/label/hide field berbeda dari default
- Grouping field per section (`sections`), multi-kolom
- `visible_when` / `readonly_when` / `required_when` / `compute` (FormSpecExpr)
- Ubah container: `render` modal / drawer / separate_page

**Kapan TIDAK pakai Form:**
- Entity cukup dengan default → jangan deklarasi Form sama sekali
- Komposisi multi-entity → `kind: Page`

**Prinsip 3-layer:** Form/Table adalah layer tengah antara Entity dan Page. Table = bentuk lain dari Form (sama-sama override di layer yang sama).

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §2.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: order-edit
  module: billing
spec:
  public: true
  entity: billing.order
  mode: edit                  # create | edit | view
  render: { mode: separate_page }   # modal | drawer | separate_page
  sections:
    - title: Customer
      columns: 2
      fields:
        - { field: customer_id, widget: relation-picker }
        - { field: member_tier, read_only: true }
    - title: Totals
      fields:
        - { field: total, read_only: true,
            compute: "sum([i.quantity * i.price for i in fields.items])" }
  actions:
    - { action: checkout, label: "Checkout", style: primary }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/form/<name> is auto-generated. Set false for embed-only forms. |
| `entity` | `string` | ✅ | billing.order |  |
| `mode` | enum (create · edit · view) | — | edit |  |
| `sections` | []`FormSection` | — |  |  |
| `actions` | []`FormAction` | — |  |  |
| `submit` | `FormSubmit` | — |  |  |
| `render` | `FormRenderDecl` | — |  |  |
| `context` | []`ContextDecl` | — |  | Context declares render-context variables injected into this form's |

<!-- /generated:attributes -->

## Gotchas

- **Tiap `field` wajib ada di Entity**; tiap `action` wajib ada + permission-gated otomatis.
- **Vocabulary perilaku client TERTUTUP**: `visible_when`, `readonly_when`, `required_when`, `compute` — butuh efek imperatif → custom widget (`asset`), bukan FormSpecExpr.
- **`render` = keputusan container design-time**, bukan runtime. `modal` (≤5 field), `drawer` (5–12), `separate_page` (12+ field / child table / butuh deep-link). Form kedua dengan render lain = deklarasi terpisah.
- **Validasi server tetap otoritas** — `rules` di client untuk UX, bukan keamanan.
- **`public: false`** → embed-only (no route); hanya tampil di Page authored.
- **Pola UI dipilih dari `submit` aktif/tidak** (bukan `characteristic`): 2-step+autosave (default), 2-step manual, atau 1-step `create-submit`.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

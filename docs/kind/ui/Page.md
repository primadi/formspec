# Page

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `PageSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Page` adalah layar ber-route yang menyusun blok (form, table,
component). Ini kind dasar tier page — Form/Table sendiri TIDAK independen
routable; mereka tampil sebagai blok di dalam Page ATAU lewat auto-Page wrapper
yang di-derive framework (route `/<module>/form/<name>`) saat `public: true`.

**Kapan memakai Page (bukan auto-derived):**
- Komposisi multi-entity dalam satu layar (master-detail, tabs, multi-block)
- Full-custom page (`mode: custom` atau satu blok `component:`)
- Pola Tabbed Resources — kelompokkan master-data kecil terkait di satu tab Page
- Pola Configuration Page — `kind: Page` ber-tabs atas Entity `characteristic: reference`

**Kapan TIDAK pakai Page:**
- Hanya ubah tampilan satu entity → `kind: Form` / `kind: Table` (lebih kecil)
- Data entry sederhana → cukup `kind: Entity` (auto-derived)

**Prinsip 3-layer:** Entity → Form/Table → Page. Page = komposisi; bukan untuk
mengubah tampilan satu entity. Lihat `docs/spec/frontend/06-page-kinds.md` §14.

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §1.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Page
metadata:
  name: order-detail
  module: billing
spec:
  route: /orders/:id
  title: "Order {order.number}"
  blocks:
    - form:  { ref: order-edit, id: ":id", mode: view }
    - table: { ref: order-payments, param: { order_id: ":id" } }
  layout: { columns: 2 }

---
# Varian tabs — beberapa sub-layar dalam satu route
spec:
  route: /settings
  tabs:
    - { label: General,  form:  { ref: settings-general } }
    - { label: Tax,      form:  { ref: settings-tax } }
    - { label: Products, table: { ref: product-list } }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route is generated for this page. Set false to restrict to embedding only. |
| `route` | `string` | ✅ | /orders/:id |  |
| `title` | `string` | ✅ | Order {order.number} |  |
| `title_visible` | `boolean` | — |  | Render the page title heading (default true) |
| `icon` | `string` | — |  |  |
| `description` | `string` | — |  |  |
| `permissions` | []`string` | — |  |  |
| `blocks` | []`PageBlock` | — |  |  |
| `tabs` | []`PageTab` | — |  |  |
| `layout` | `PageLayout` | — |  |  |
| `mode` | enum ( · custom) | — |  | Page mode. `custom` hands all rendering to an asset component; empty means blocks/tabs composition. |
| `asset` | `string` | — |  | Asset is the module-relative asset path for `mode: custom` |
| `binds` | `PageBinds` | — |  | Binds is the backend footprint (entities/actions/subscribe) a custom |
| `renderer` | `string` | — |  | Renderer is the per-instance renderer override (frontend/03-renderer- |

<!-- /generated:attributes -->

## Gotchas

- **`route` unik per App**; `:params` satu-satunya sintaks route dinamis
  (`/orders/:id`).
- **`blocks` dan `tabs` mutually exclusive** — tidak bisa dua-duanya.
- **Route yang dibutuhkan** — Page dengan `public: false` tidak punya route
  mandiri; hanya bisa di-embed sebagai blok di Page lain.
- **Render per-blok di-permission-check independen** — enforcement tetap
  per-blok, komposisi tidak melonggarkan gating.
- **`layout.mode: split`** (master-detail / `binds`) masih **Open** — belum
  didukung skema `PageSpec`/`BlockRef` maupun renderer (tracking di
  `docs_internal/plan/todo.md`). Saat ini master-detail via `param` + route `:id`.
- **`mode: custom`** (custom page, `binds` footprint) juga **Open** — Page saat
  ini hanya `blocks`/`tabs`. Kontrak di `docs/spec/frontend/06-page-kinds.md`
  §13 adalah target desain.
- **Full-custom page** = satu entry `component:` tanpa `blocks`/`tabs`.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §1, §14 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

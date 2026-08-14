# Widget

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `WidgetSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Widget` adalah **satu tile di Dashboard** — komponen pengisi slot. Bisa di-attach ke Entity untuk data binding.

**Kapan memakai Widget:**
- Metrik stat (revenue hari ini, count pending)
- Chart / table / list dalam dashboard

**Kapan TIDAK pakai Widget:**
- Layar penuh → `kind: Dashboard` (yang mereferensikan widget) atau `kind: Page`
- Widget dasar form input → base component library (bukan kind)

**Sumber kontrak:** [`docs/spec/frontend/07-component-kinds.md`](../spec/frontend/07-component-kinds.md) §2–3.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1
kind: Widget
metadata: { name: sales-today-stat, module: billing }
spec:
  title: "Today's Revenue"
  type: metric                       # metric | chart | table | list
  entity: sales-daily-summary
  config: { field: total }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/widget/<name> is auto-generated. |
| `title` | `string` | ✅ | Today's Revenue |  |
| `type` | enum (metric · chart · table · list) | ✅ | metric |  |
| `entity` | `string` | — | sales-daily-summary |  |
| `query` | `string` | — |  |  |
| `refresh_secs` | `integer` | — |  |  |
| `size` | `WidgetLayout` | — |  |  |
| `config` | map | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Widget didefinisikan terpisah**, Dashboard mereferensikan by name (nama saja, bukan module-qualified).
- **`type`**: `metric` | `chart` | `table` | `list`. `config` specifik per type.
- **Visibilitas katalog widget derived dari permission**; mekanisme customizable dashboard.
- **Cross-ref:** [`docs/spec/frontend/07-component-kinds.md`](../spec/frontend/07-component-kinds.md) §2 · [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §7 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

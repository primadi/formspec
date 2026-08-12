# Dashboard

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `DashboardSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Dashboard` adalah **kanvas grid widget** — di-deklarasi level module (bukan per-entity). Dashboard **mereferensikan** widget by name; widget didefinisikan terpisah sebagai `kind: Widget`.

**Kapan memakai Dashboard:**
- Ringkasan eksekutif / operasional (sales hari ini, cashflow)
- Multi-widget dengan layout grid

**Kapan TIDAK pakai Dashboard:**
- Satu metrik → `kind: Widget` saja (tapi widget tetap butuh dashboard untuk ditampilkan)

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §7.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Dashboard
metadata: { name: sales-today, module: billing }
spec:
  customizable: true
  defaults: [sales-today-stat, gl-cashflow-chart]
  refresh: 60                          # atau realtime: true
  widgets:
    - ref: sales-today-stat
      layout: { x: 0, y: 0, w: 4, h: 2 }
    - ref: gl-cashflow-chart
      layout: { x: 4, y: 0, w: 8, h: 4 }
      config: { range: 30d }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/dashboard/<name> is auto-generated. |
| `title` | `string` | ✅ |  |  |
| `description` | `string` | — |  |  |
| `customizable` | `boolean` | — |  |  |
| `defaults` | []`string` | — |  |  |
| `refresh` | `integer` | — |  |  |
| `realtime` | `boolean` | — |  |  |
| `widgets` | []`DashboardWidget` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **Widget `ref` pakai nama widget saja** — `ref: doc-in-progress`, BUKAN `ref: crc-report.doc-in-progress`. Registry index widget by `metadata.name` only; module-qualified refs gagal validasi.
- **`DashboardWidget`** = `{ ref, layout: {x,y,w,h}, config }`; **`WidgetSpec`** = `{ title, type, entity?, query?, refresh_secs?, size?, config? }`.
- **Visibilitas katalog widget derived dari permission**; mekanisme customizable.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §7 · [`docs/spec/frontend/07-component-kinds.md`](../spec/frontend/07-component-kinds.md) §2 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

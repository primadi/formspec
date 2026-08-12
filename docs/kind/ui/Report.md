# Report

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `ReportSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Report` adalah **output tabular terparameterisasi** — di-deklarasi level module.

**Kapan memakai Report:**
- Laporan dengan parameter (rentang tanggal, filter kategori)
- Agregasi + grouping + totals
- Export (xlsx, csv)

**Kapan TIDAK pakai Report:**
- Dokumen cetak satu entity → `kind: Print`
- Dashboard metrik → `kind: Dashboard` + `kind: Widget`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §8.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Report
metadata: { name: sales-by-category, module: billing }
spec:
  title: "Sales by Category"
  entity: billing.order
  required_permission: reports.sales-by-category
  parameters:
    - { field: date_from, label: "Dari", type: date, required: true }
    - { field: date_to,   label: "Sampai", type: date, required: true }
  columns:
    - { field: number, label: "No." }
    - { field: category, label: "Kategori" }
    - { field: total, label: "Total", aggregate: sum, format: currency }
  groups:
    - { field: category, label: "Kategori" }
  totals:
    - { label: "Total", field: total, fn: sum }
  export: [xlsx, csv]
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/report/<name> is auto-generated. |
| `title` | `string` | ✅ | Sales by Category |  |
| `entity` | `string` | ✅ | billing.order |  |
| `required_permission` | `string` | — |  |  |
| `parameters` | []`ReportParam` | — |  |  |
| `columns` | []`ReportColumn` | — |  |  |
| `groups` | []`ReportGroup` | — |  |  |
| `totals` | []`ReportTotal` | — |  |  |
| `export` | []`string` | — |  | pdf \| csv \| xlsx |

<!-- /generated:attributes -->

## Gotchas

- **`entity` selalu query entity, permission-checked** — Report tidak pernah meng-embed SQL.
- **Export berjalan sebagai async job** — file mendarat di download tray.
- **`source.filter` masih Open** — parameter saat ini dikirim sebagai filter query `?<field>=<value>` per `parameters[]`.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §8 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

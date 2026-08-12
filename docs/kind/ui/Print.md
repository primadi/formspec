# Print

<!-- generated:meta -->
| | |
|---|---|
| Grup | `ui` |
| Plane | `resource` |
| Spec struct | `PrintSpec` |

<!-- /generated:meta -->

## Kapan Memakai

`kind: Print` adalah **dokumen cetak untuk satu entity**, multi-target output.

**Kapan memakai Print:**
- Invoice, struk POS, surat jalan, label
- Output `pdf` / `thermal` / `dotmatrix` / `html`

**Kapan TIDAK pakai Print:**
- Laporan tabular terparameterisasi → `kind: Report`

**Sumber kontrak:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §8.

## Contoh Manifest

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: billing.order
  output:
    format: pdf                   # pdf | thermal | dotmatrix | html
    paper: { size: A5, margin: 12mm }
  header: { logo: true, title: "Receipt {order.number}" }
  body:
    - fields: [number, paid_at, customer.name]
    - child_table: { field: items, columns: [product_id, quantity, price] }
    - totals: { field: total, format: currency }
  footer: { text: "Thank you — {tenant.name}" }
```

## Atribut

<!-- generated:attributes -->
| Atribut | Tipe | Wajib | Contoh | Deskripsi |
|---|---|---|---|---|
| `public` | `boolean` | — | true | If true (default), a route /module/print/<name> is auto-generated. |
| `entity` | `string` | ✅ | billing.order |  |
| `template` | `string` | — |  |  |
| `output` | `PrintOutput` | — |  |  |
| `header` | `PrintHeader` | — |  |  |
| `body` | []`PrintBodyItem` | — |  |  |
| `footer` | `PrintFooter` | — |  |  |

<!-- /generated:attributes -->

## Gotchas

- **`output.paper.size` divalidasi terhadap format** — `thermal_58mm` cuma valid dengan `format: thermal`.
- **Semua format kecuali `html` render server-side** → hasil ke download tray. `html` render di browser (`window.print()` + CSS `@media print`).
- **Kertas custom** `custom: { width, height, unit }` divalidasi saat `formspec validate`.
- **Print programatik**: `ctx.print(entity_id, "receipt")` — format per-manifest Print, bukan per-panggilan.
- **Cross-ref:** [`docs/spec/frontend/06-page-kinds.md`](../spec/frontend/06-page-kinds.md) §8 · [`ai_skills/formspec-kinds`](../../ai_skills/formspec-kinds/SKILL.md)

# 2026-08-23-005 — ChildTable Paritas dengan Parent Table: Sorting, Computed per Baris, readonly_when

## Catatan tambahan (saat verifikasi)

- **FormSpecExpr hanya mendukung string double-quote** — ekspresi
  `readonly_when` yang memakai `''` (single-quote) gagal parse dan diam-diam
  dievaluasi `false`. Di YAML pakai single-quoted string yang membungkus
  double-quote: `readonly_when: 'fields.menu_item_id != ""'`.
- Frontend `Field` type (`types/manifest.ts`) diselaraskan dengan Go struct:
  `visible_when`, `readonly_when`, `required_when`.

## Apa yang diubah

Menjawab pertanyaan user: "parent table mendukung apa saja? child table harus
mendukung juga." ChildTable kini memparalelkan fitur parent Table yang relevan
untuk grid lokal yang bisa diedit:

### Frontend (`renderers/react-shadcn`)

`widgets/ChildTable.tsx` (ditulis ulang):

- **Client-side sorting** — header kolom clickable (asc → desc → clear),
  data child lokal jadi tidak perlu round-trip server.
- **Per-row `computed`** — child field ber-`computed` (mis. `line_total`)
  dirender sebagai span display-only (bukan input), nilai dihitung live dari
  baris via FormSpecExpr `evalCompute`.
- **Per-row `readonly_when`** — child field bisa mengunci sel berdasarkan
  nilai baris lain (mis. `unit_price` read-only begitu `menu_item_id` terisi),
  via `evalReadonlyWhen`.
- **Shared cell rendering** — kolom child dirender identik dengan parent table
  (badge/boolean/currency/date) lewat `renderCellValue` + `cellHintsForField`.

`lib/renderCell.tsx` (baru) — `renderCellValue` + `cellHintsForField`
diextract dari TableRenderer agar parent & child table berbagi satu vocabulary
rendering. `kinds/table/TableRenderer.tsx` kini import dari module ini.

### Backend (`pkg/spec`)

`pkg/spec/entity.go` — struct `Field` ditambah `visible_when`, `readonly_when`,
`required_when` (client-behavior vocabulary FormSpecExpr). Biasanya properti
Form manifest; pada entity field ia menjadi default — dan satu-satunya cara
mengonfigurasi perilaku per-kolom pada child field (ChildTable membaca
`child.fields` dari entity, bukan dari Form manifest). JSON Schema
diregenerate (`make generate-schema` → `schemas/`).

### Cafe example

`spec/modules/cafe-order/transaction/order/entity.yaml` — child field
`unit_price` diberi `readonly_when: "fields.menu_item_id != None and fields.menu_item_id != ''"`
— read-only begitu menu item dipilih; harga diisi otomatis oleh hook
`fill_order_unit_price.star` saat save.

## Kenapa diubah

Use case dasar order line items butuh child grid yang berperilaku seperti
parent table: kolom bisa diurutkan, subtotal per baris dihitung live, dan
harga satuan terkunci setelah item dipilih — tanpa custom widget JS.

## File terdampak

- `renderers/react-shadcn/src/widgets/ChildTable.tsx`
- `renderers/react-shadcn/src/lib/renderCell.tsx` (baru)
- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx`
- `renderers/react-shadcn/src/types/manifest.ts`
- `pkg/spec/entity.go`
- `schemas/` (regenerate)
- `examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml`

## Referensi

- `docs/spec/frontend/06-page-kinds.md` (Form, FormSpecExpr)
- `docs/spec/frontend/08-formspec-expr.md` (visible_when/readonly_when/compute)

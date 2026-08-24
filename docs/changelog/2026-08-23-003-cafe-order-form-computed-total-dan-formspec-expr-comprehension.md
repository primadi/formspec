# 2026-08-23-003 — Cafe Order Form: Computed Total, Child Labels, Status Hidden + FormSpecExpr List Comprehension

## Apa yang diubah

### Cafe example (`examples/cafe`)

`spec/modules/cafe-order/transaction/order/entity.yaml`:

- Child fields `items` diberi `title` (No., Menu Item, Qty, Harga Satuan) —
  ChildTable merender `f.title ?? f.name` sebagai header kolom.
- `line_total` dihapus dari child — child-level `computed` belum didukung
  renderer; total dihitung langsung dari `quantity * unit_price`.
- `total_amount` menjadi `computed: { formula: "sum([i.quantity * i.unit_price for i in items])" }`
  — dievaluasi server-side saat read (jsonb-persist `evaluateComputed`).
- `status` diberi `exclude: [ui]` — disembunyikan dari UI turunan; default
  tetap `open` via `default` + `state_machine.initial`.

`spec/modules/cafe-order/transaction/order/forms/create.yaml` (baru) — authored
Form `order-create` yang meng-override derived form: status tidak ditampilkan,
`total_amount` dihitung live client-side via `compute` FormSpecExpr, label
eksplisit per field.

### Frontend fixes (`renderers/react-shadcn`)

- `widgets/RelationPicker.tsx` — `resolveRelatedEntity` kini mendukung notasi
  dot `module.entity` (dipakai backend & semua contoh) selain slash
  `module/name`. Sebelumnya relation lintas-module (mis. `cafe-master.menu-item`)
  gagal resolve → URL API salah → 404.
- `hooks/useSelectFilterOptions.ts` — lookup relation resource memakai
  `resolveEntityRef` (dot/slash) + URL memakai `relatedEntity.name`, bukan
  resource mentah.
- `lib/formspec-expr/parser.ts` + `eval.ts` — implementasi **list comprehension**
  `[expr for var in iterable]` yang selama ini ada di spec & contoh
  (`visit-edit.yaml`) tapi belum diimplementasikan interpreter → `compute`
  `sum([... for ... in ...])` gagal diam-diam. Ditambah 3 test di
  `formaexpr.test.ts` (87 test pass).

## Kenapa diubah

Permintaan user untuk form create order kafe: label child field, `unit_price`
read-only + auto-fill (butuh custom widget — belum dikerjakan), `line_total`
dan `total_amount` otomatis, serta `status` disembunyikan dengan default `open`.
Selama verifikasi ditemukan dua bug framework (relation dot-notation & list
comprehension) yang memblokir alur tersebut.

## File terdampak

- `examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml`
- `examples/cafe/spec/modules/cafe-order/transaction/order/forms/create.yaml` (baru)
- `renderers/react-shadcn/src/widgets/RelationPicker.tsx`
- `renderers/react-shadcn/src/hooks/useSelectFilterOptions.ts`
- `renderers/react-shadcn/src/lib/formspec-expr/parser.ts`
- `renderers/react-shadcn/src/lib/formspec-expr/eval.ts`
- `renderers/react-shadcn/src/lib/formspec-expr/formaexpr.test.ts`

## Referensi

- `docs/spec/backend/05-field-types.md` §5.1 (computed field), §5.3 (exclude: [ui])
- `docs/spec/frontend/06-page-kinds.md` (Form, compute FormSpecExpr)
- `docs/spec/frontend/08-formspec-expr.md` (grammar list comprehension)

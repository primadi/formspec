# Plan: Cafe Order Child Items — Auto-Fill, read_only, Dropdown Flip

**Status**: ✅ Complete (2026-08-24)
**Sumber**: `docs/spec/frontend/` (FormSpecExpr, ChildTable), `docs/spec/backend/05-field-types.md`
**Referensi changelog**: `docs/changelog/2026-08-24-001-cafe-order-child-autofill-readonly-dropdown.md`

## Masalah (dari user)

Di child items order (form `order-create`, widget `child-grid`):

1. Setelah pilih menu item "Kopi Susu", **Harga Satuan tidak langsung update** —
   harga hanya diisi server-side oleh hook `fill_order_unit_price.star` saat
   create/update, bukan client-side.
2. **ReadOnly di Harga Satuan harusnya selalu read-only** — tidak perlu
   `readonly_when` yang bergantung pada nilai `menu_item_id`.
3. **Dropdown relation picker di dalam table terpotong** — saat memilih item
   paling bawah, dropdown tidak terlihat; harus scroll table dulu.

## Solusi

### 1. Auto-fill client-side (`auto_fill`)

Vocabulary baru pada child field (dan field entity umumnya):

```yaml
- name: unit_price
  type: money
  read_only: true
  auto_fill:
    from: menu_item_id # relation field di baris yang sama (pemicu)
    field: price # field pada entity terkait yang disalin
```

Saat `menu_item_id` berubah (user pilih record di RelationPicker), record
lengkap sudah ada di tangan picker → `onSelectRecord` membawa record tersebut →
ChildTable menyalin `price` → `unit_price` dalam satu `updateRow`. Saat relation
dikosongkan (tombol X), target auto-fill ikut dikosongkan.

### 2. `read_only` pada child field

`Field` (Go + TS) mendapat `read_only: bool`. `ChildTable.isCellReadonly`
menghormati `f.read_only` (selalu read-only, dirender sebagai span display-only
via `renderCellValue`). `readonly_when` tetap didukung untuk kasus kondisional.

### 3. Dropdown RelationPicker tidak terpotong

Dropdown di-portal ke `<body>` (`createPortal`) dengan `position: fixed`,
di-anchor ke bounding rect picker. Tidak lagi ter-clip oleh `overflow-x-auto`
table. Saat ruang di bawah tidak cukup (< 240px), dropdown **flip ke atas**
(`bottom` di-set, `top` di-unset). Reposisi pada scroll/resize.

## File yang diubah

| File                                                                  | Perubahan                                                                              |
| --------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                                                  | `Field.ReadOnly bool`, `Field.AutoFill *AutoFillDecl`, tipe `AutoFillDecl`             |
| `internal/genjsonschema/generator.go`                                 | `AutoFillDecl` masuk daftar `sharedTypes` (agar `$defs` ter-emit)                      |
| `schemas/formspec.schema.json` (+ `schemas/kinds/*`)                  | Regenerasi via `make generate-schema`                                                  |
| `renderers/react-shadcn/src/types/manifest.ts`                        | `Field.read_only`, `Field.auto_fill`, `AutoFillDecl`                                   |
| `renderers/react-shadcn/src/widgets/ChildTable.tsx`                   | `read_only` di `isCellReadonly`; map auto-fill; handler select/clear; `onSelectRecord` |
| `renderers/react-shadcn/src/widgets/RelationPicker.tsx`               | prop `onSelectRecord`; dropdown portal + flip + reposition                             |
| `examples/cafe/spec/modules/cafe-order/transaction/order/entity.yaml` | `unit_price`: `readonly_when` → `read_only: true` + `auto_fill`                        |

## Level of effort

- `pkg/spec` + schema: small
- Frontend ChildTable + RelationPicker: medium
- Cafe example: small

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass; `go test ./...` 736 pass.
- Browser (dev server): pilih "Kopi Susu" → Harga Satuan auto-fill `$15,000.00`,
  read-only (span); Qty 2 → Subtotal `$30,000.00`, Total `30000`; clear menu item
  → harga ikut kosong; dropdown baris terakhir flip ke atas (tidak terpotong).

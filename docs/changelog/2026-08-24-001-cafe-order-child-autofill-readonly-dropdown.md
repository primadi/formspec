# 2026-08-24-001 — Cafe Order Child Items: Auto-Fill unit_price, read_only, Dropdown Flip

## Apa yang diubah

Menjawab 3 pertanyaan user tentang child items di form `order-create`
(`examples/cafe`): (1) Harga Satuan tidak langsung update setelah pilih menu
item, (2) ReadOnly Harga Satuan harus selalu read-only tanpa `readonly_when`,
(3) dropdown relation picker di dalam table terpotong saat pilih item paling
bawah.

### Framework (`pkg/spec` + frontend)

- `pkg/spec/entity.go` — `Field` ditambah `read_only: bool` dan
  `auto_fill: *AutoFillDecl` (`{from, field}`). `AutoFillDecl` adalah
  client-side lookup: saat relation field `from` (di baris child yang sama)
  berubah, field pada record terkait (`field`) disalin ke field ini.
- `internal/genjsonschema/generator.go` — `AutoFillDecl` ditambahkan ke daftar
  `sharedTypes` agar `$defs/AutoFillDecl` ter-emit (tanpa ini `formspec
validate` gagal: "json-pointer ... AutoFillDecl not found").
- `schemas/` — regenerasi via `make generate-schema` (125 shared defs).
- `types/manifest.ts` — `Field.read_only`, `Field.auto_fill`, `AutoFillDecl`.
- `widgets/ChildTable.tsx` — `isCellReadonly` menghormati `f.read_only`
  (selalu read-only, dirender sebagai span display-only). Map auto-fill
  dibangun dari `auto_fill`; saat relation dipilih (`onSelectRecord`) id + field
  target disalin dalam satu `updateRow`; saat relation dikosongkan, target ikut
  dikosongkan. Type error lama `{ fields: row }` diperbaiki dengan cast
  `RuntimeObject`.
- `widgets/RelationPicker.tsx` — prop `onSelectRecord` (membawa record lengkap,
  dipakai ChildTable untuk auto-fill; `handleSelect` memanggilnya menggantikan
  `onChange` bila disediakan). Dropdown kini **di-portal ke `<body>`**
  (`createPortal` + `position: fixed`, anchor ke bounding rect) sehingga tidak
  lagi ter-clip oleh `overflow-x-auto` table, dan **flip ke atas** saat ruang di
  bawah tidak cukup (< 240px). Reposisi pada scroll/resize.

### Cafe example

`spec/modules/cafe-order/transaction/order/entity.yaml` — child field
`unit_price`: `readonly_when` diganti `read_only: true` + `auto_fill:
{from: menu_item_id, field: price}`. Hook Starlark `fill_order_unit_price.star`
tetap sebagai otoritas server-side (verifikasi ulang saat create/update).

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass; `go test ./...` 736 pass.
- Browser: pilih "Kopi Susu" → Harga Satuan auto-fill `$15,000.00` (read-only
  span); Qty 2 → Subtotal `$30,000.00`, Total `30000`; clear menu item → harga
  ikut kosong; dropdown baris terakhir flip ke atas (tidak terpotong).

## Referensi

- Plan: `docs/plan/cafe-order-child-autofill-readonly-dropdown.md`
- Lanjutan dari: `docs/changelog/2026-08-23-003/004/005` (cafe-order form,
  hook unit_price, ChildTable paritas)

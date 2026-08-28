---
name: form-layout
description: >
  Gunakan skill ini saat membuat atau mengubah Form kind — layout mode
  (modal/drawer/separate page), pemilihan widget per field type, computed
  field, dan ekspresi FormSpecExpr (visible_when, readonly_when, required_when,
  compute). Trigger: percakapan menyebut "form", "layout input", "form create",
  "field computed", "visible when", atau UX input data.
applies_to_kind: [Form]
min_core_spec_version: "0.2.0"
metadata:
  version: "1.0"
  source: docs/spec/frontend/ + docs/plan/fase5-completion.md
---

# Form Layout

## Prinsip

- **Derived by default** — setiap Entity otomatis mendapat Form create/edit.
  Tulis `kind: Form` eksplisit hanya untuk mengoverride default.
- **Design-time layout locking** — mode penyajian ditentukan di manifest dan
  TIDAK bisa di-switch di runtime:
  - `modal` — data kecil, konteks cepat (default).
  - `drawer` — form sedang, tetap melihat list di belakang.
  - `separate_page` — form panjang/kompleks (banyak section/child table).

## Widget per Tipe Field

| Field type | Widget default | Catatan |
|---|---|---|
| string | TextInput | |
| text | Textarea | |
| integer | NumberInput (`integer: true`) | blokir pecahan |
| decimal | NumberInput (`scale`) | dibatasi `scale` desimal |
| money | MoneyInput | format mata uang otomatis |
| boolean | Switch | |
| enum | Select | `type-ahead` untuk opsi banyak |
| date / datetime | DateInput | format ikut global settings |
| relation | RelationPicker | dropdown + pencarian; portal anti-terpotong |
| child table | ChildTable | untuk baris item (order items, dll.) |

## FormSpecExpr

Ekspresi dipakai untuk perilaku dinamis — subset Starlark, murni fungsi dari
field lain di record yang sama:

- `visible_when` — sembunyikan field sampai kondisi terpenuhi.
- `readonly_when` — tampilkan tapi terkunci.
- `required_when` — wajib kondisional.
- `compute` — nilai dihitung dari field lain (mis. total = qty × price).

Aturan: ekspresi TIDAK boleh punya side effect; hanya membaca field di record
yang sama. Logika lintas-record → taruh di action hook (Starlark), bukan expr.

## Child Table (baris item)

- Field child bisa `auto_fill` dari record relation (mis. `unit_price`
  terisi otomatis dari `menu_item_id → price`).
- Field turunan harga biasanya `read_only: true` (dihitung, bukan diketik).
- Computed di child (subtotal) dan di parent (total) memakai `compute`.

## Checklist Sebelum Draft

1. Semua field entity punya widget yang tepat?
2. Field uang pakai money, bukan decimal?
3. Computed tidak duplikat dengan logika hook server-side?
4. Layout mode sesuai panjang form?
5. Draft ditulis via `propose_spec_file` (validasi otomatis)?

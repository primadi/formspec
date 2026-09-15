# Plan — Generalisasi S1: `picker` pada child field (menggantikan blok `order_builder`)

**Status**: ✅ selesai (2026-09-15) — changelog `docs_internal/changelog/2026-09-15-004-child-field-picker.md`
**Menggantikan**: `docs_internal/plan/order-builder-block.md` (blok `order_builder`, changelog `2026-09-15-003`)
**Keputusan pemilik**: opsi 2 (`child.picker`) + adopsi **kafe dan inventory/gl sekaligus**

---

## 1. Kenapa

`order_builder` mencampur satu pola yang **umum** dengan kosakata yang **spesifik
pesanan** — dan, lebih buruk, ia **menduplikasi jalur tulis** (POST sendiri),
sehingga tidak mendapat idempotency, validasi rules, `action:`/lifecycle,
redirect, maupun event yang sudah dimiliki `Form`.

Polanya sendiri muncul di banyak domain (dibuktikan dari spec yang ada):

| Entity                                                                    | Child   | Sumber baris                        |
| ------------------------------------------------------------------------- | ------- | ----------------------------------- |
| `cafe-order.order`                                                        | `lines` | `menu-item` (join harga per cabang) |
| `cafe-stock.purchase-order`                                               | lines   | `ingredient`                        |
| `cafe-stock.stock-opname`                                                 | lines   | `ingredient` (qty hasil hitung)     |
| `inventory.stock-movement`                                                | lines   | `product` (qty + unit)              |
| `gl.journal-entry`                                                        | `lines` | akun                                |
| `billing.order`, `prescription`, `visit.treatments`, `checklist-document` | lines   | katalog/jasa/obat/item              |

Semua sama: **"isi child field ini dengan memilih dari entity lain"**, plus qty
dan snapshot.

## 2. Bentuk umum

**`picker` hidup di deklarasi child field**, bukan di blok Page — jadi berlaku di
**Form mana pun** (juga Wizard step), dan submit/validasi/idempotency/event tetap
milik `Form`:

```yaml
# entity: cafe-order.order
- name: lines
  type: child
  child:
    storage: jsonb
    sequence_field: line_no
    fields: [...]
    picker:
      entity: cafe-master.menu-item # WAJIB — sumber baris
      filter: { is_available: "true" } # opsional, interpolasi {token}
      display: # apa yang dilihat user (tile)
        name_field: name # default "name"
        image_field: photo
        description_field: description
        category_field: menu_category_id
        # Harga boleh dari entity lain (mis. per cabang) → join client-side
        price_entity: cafe-master.menu-item-price
        price_match_field: menu_item_id
        price_field: price
        price_filter: { branch_id: "{session.branch_id}" }
        columns: 3
        search: true
        empty_text: "Menu belum tersedia"
      map: # apa yang DITULIS ke baris
        ref_field: menu_item_id # WAJIB — relasi ke sumber
        name_field: name_snapshot # snapshot (D2)
        price_field: unit_price_snapshot # snapshot
        quantity_field: quantity
        note_field: note # catatan bebas per baris
        max_quantity: 20
```

**Tiga primitif pelengkap yang ikut dibutuhkan** (dan berguna di luar picker):

| Primitif                                | Di mana               | Kenapa                                                                                                                                               |
| --------------------------------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `FormField.default_from: "{template}"`  | Form/Wizard           | Nilai awal field dari render context (`{session.branch_id}`, `{now}`, `{today}`) — pengganti `checkout.defaults`. Berlaku umum, bukan khusus picker. |
| `widget: hidden`                        | kosakata widget (S10) | Field ikut tersubmit tanpa dirender — field seed (cabang, sesi, waktu) tidak boleh tampil ke tamu.                                                   |
| `FormRenderDecl.picker_panel: inline \\ | aside`                | `render:` Form                                                                                                                                       | Tata letak: `aside` menaruh tile katalog di kolom sendiri (di samping panel pilihan + submit). Default `inline` (di atas child grid). |

**Yang hilang dari kosakata**: `catalog`, `lines`, `checkout`, `defaults`,
`submit_label`, `success_message`, `reset_after_submit`, blok `order_builder`.
Submit/label/sukses/reset mengikuti `Form` + `FormSubmit` yang sudah ada.

## 3. Perubahan per file

| File                                      | Perubahan                                                                                                                                           | Effort |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `pkg/spec/entity.go`                      | `ChildDecl.Picker *PickerDecl` (+`PickerDisplay`/`PickerMap`) + validasi di `ValidateEntitySpec`                                                    | medium |
| `pkg/spec/frontend.go`                    | `FormField.DefaultFrom`, `FormRenderDecl.PickerPanel`; **hapus** `PageBlock.OrderBuilder`                                                           | medium |
| `pkg/spec/order_builder.go`               | **dihapus** (digantikan `picker.go`)                                                                                                                | —      |
| `pkg/spec/widget.go`                      | `WidgetHidden` masuk `FormWidget`                                                                                                                   | small  |
| `internal/genjsonschema`                  | daftar `sharedTypes` disesuaikan                                                                                                                    | small  |
| `schemas/**`                              | regenerasi                                                                                                                                          | small  |
| `types/manifest.ts`                       | tipe baru; hapus tipe `order_builder`                                                                                                               | medium |
| `lib/orderBuilder.ts` → `lib/picker.ts`   | logika murni: `priceIndex`, `catalogTiles`, `interpolateTemplate`, `newRowFromSource`, `addOrIncrement`, `setQuantity`, `removeLine`, total display | medium |
| `kinds/form/FormRenderer.tsx`             | context (`spec.context` + prop dari Page), seed `default_from`, `widget: hidden`, render picker (`inline`/`aside`)                                  | large  |
| `kinds/page/blocks/OrderBuilderBlock.tsx` | **dihapus** + dispatch di `PageRenderer.tsx`                                                                                                        | small  |
| `kinds/page/PageRenderer.tsx`             | teruskan `context` ke blok Form                                                                                                                     | small  |
| Test                                      | `lib/picker.test.ts`; parity widget (`hidden`); `pkg/spec` picker + `default_from`                                                                  | medium |

## 4. Adopsi (kafe + inventory/gl sekaligus)

| Spec                                                          | Yang ditambah                                                                                                                                               |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cafe-order.order` + Form baru `order-form-qr`                | picker pada `lines` (join harga per cabang), seed field `hidden`+`default_from`, `picker_panel: aside`; halaman `menu-catalog.yaml` jadi Page + blok `form` |
| `cafe-stock.purchase-order` + Form baru `purchase-order-form` | picker pada lines (sumber `ingredient`)                                                                                                                     |
| `cafe-stock.stock-opname` (Form sudah ada)                    | picker pada lines                                                                                                                                           |
| `inventory.stock-movement` + Form baru                        | picker pada lines (sumber `product`)                                                                                                                        |
| `gl.journal-entry` + Form baru                                | picker pada lines (sumber akun)                                                                                                                             |

## 5. Verifikasi

```bash
make generate-schema && go test ./... && cd renderers/react-shadcn && npx vitest run && npx tsc -b
go run ./cmd/formspec validate --spec examples/kafe/spec --schema schemas   # 0 problem
go run ./cmd/formspec check -f examples/kafe/spec                          # 0 error
# + validate untuk verticals/inventory dan verticals/gl
```

**Bukti accept** (E2E, DB segar): form QR membuat order lewat **Form pipe**
(`POST` payload yang sama) — `lines` + snapshot dari picker, `default_from`
mengisi `branch_id`/`transaction_date`/`table_session_id` dari context, field
seed tidak tampil, `line_total`/`subtotal` terhitung server. Ditambah: spec
purchase-order/stock-movement/journal-entry memakai `picker` yang sama dan lolos
`formspec validate`/`check`.

## 6. Risiko & catatan

- **Blok yang baru dikirim dihapus** — perubahan kontrak; halaman kafe dimigrasi
  di langkah yang sama supaya tidak ada spec yang menggantung.
- Form kini perlu `spec.context` (FormRenderer belum me-resolve-nya) → diteruskan
  dari Page, dengan fallback resolve sendiri saat Form dirender mandiri.
- `widget: hidden` mengubah kosakata widget (S10) → enum ter-regenerasi + test
  paritas akan memaksa `case`-nya ada.
- Yang **tetap** di luar: perhitungan total (tetap `computed` entity), layar POS
  kasir & numpad uang (2.14), token QR → sesi (2.2).

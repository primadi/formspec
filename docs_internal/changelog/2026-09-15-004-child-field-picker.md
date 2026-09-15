# 2026-09-15-004 — `picker` pada child field (menggantikan blok `order_builder`)

**Menggantikan** changelog `2026-09-15-003` (blok Page `order_builder`, item kafe
1.5). Plan: `docs_internal/plan/child-field-picker.md`.
**Keputusan pemilik:** opsi 2 — pindahkan picker ke child field, blok Page jadi
tidak perlu; adopsi kafe **dan** inventory/gl sekaligus.

**Masalahnya.** Blok `order_builder` mencampur satu pola yang **umum** dengan
kosakata yang **spesifik pesanan** — dan, lebih buruk, ia **menduplikasi jalur
tulis**: POST sendiri, sehingga tidak mendapat validasi `rules`, permission,
idempotency, `action:` lifecycle, redirect, maupun event yang sudah dimiliki
`Form`. Polanya sendiri (pilih baris dari sumber + qty + snapshot) muncul di
banyak entity yang sudah ada: `cafe-order.order`, `cafe-stock.purchase-order`,
`cafe-stock.stock-opname`, `inventory.stock-movement`, `gl.journal-entry`,
`prescription`, `visit.treatments`, `checklist-document`.

**Bentuk umumnya.** `picker` hidup di **deklarasi child field**:

```yaml
- name: lines
  type: child
  child:
    picker:
      entity: cafe-master.menu-item
      filter: { is_available: "true" }
      display:
        {
          name_field,
          image_field,
          description_field,
          category_field,
          price_entity,
          price_match_field,
          price_field,
          price_filter,
          columns,
          search,
          empty_text,
        }
      map:
        {
          ref_field,
          name_field,
          price_field,
          quantity_field,
          note_field,
          max_quantity,
        }
```

Konsekuensinya: berlaku di **Form mana pun** (dan langkah Wizard) yang mengedit
entity itu; baris yang dipilih adalah baris child biasa di state form, jadi
submit tetap milik Form. Tidak ada blok Page, tidak ada jalur tulis kedua.

**Tiga primitif pelengkap** (umum, bukan khusus picker):

| Primitif                                | Guna                                                                                                                                                                      |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FormField.default_from: "{template}"`  | Mengisi nilai awal field dari render context (`{session.branch_id}`, `{now}`, `{today}`) — pengganti `checkout.defaults`. Token tak terselesaikan dibiarkan **verbatim**. |
| `widget: hidden`                        | Field ikut tersubmit tanpa dirender (branch, sesi, waktu). Masuk kosakata widget S10 → enum + test paritas ikut menjaganya.                                               |
| `FormRenderDecl.picker_panel: inline \\ | aside`                                                                                                                                                                    | `inline` = tile di atas child grid (grid tetap editor baris); `aside` = tile **dan** editor baris di kolom sendiri, child grid tidak dirender dua kali. |

**Aturan normatif** (ditegakkan `ValidateEntitySpec`): `map.ref_field` wajib;
setiap field di `map` harus ada di `child.fields`; `quantity_field` wajib
disertai `max_quantity` (batas harus dipilih, tidak boleh tak terbatas);
`price_entity` wajib disertai `price_match_field` + `price_field`; baris tanpa
harga tampil **tidak bisa dipilih** (bukan terkirim `0`). Picker **tidak pernah**
menghitung/menyimpan total — itu urusan `computed` entity.

**Yang dihapus**: `pkg/spec/order_builder.go` (4 tipe + validator),
`PageBlock.OrderBuilder`, tipe TS-nya, `kinds/page/blocks/OrderBuilderBlock.tsx`,
`lib/orderBuilder.ts` → `lib/picker.ts`, dan dispatch di `PageRenderer.tsx`.
`PageBlock` kembali ke closed set semula.

**Yang diubah/ditambah di renderer**: `FormRenderer` kini me-resolve
`spec.context` (dulu hanya Page yang bisa) — menerima `context` dari Page dan
menggabungkannya, sehingga Form mandiri juga jalan tanpa fetch ganda; seed
`default_from` (hanya field yang belum terisi — record yang dimuat selalu
menang); `widget: hidden`; `PickerPanel.tsx` baru (tile + kategori + cari +
editor baris) yang membaca/menulis **state form yang sama** dengan child grid,
jadi grid, panel, dan payload tidak bisa berbeda.

**Adopsi** (semuanya memakai konstruk yang sama):

| Spec                                                                | Yang ditambah                                                                                   |
| ------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `cafe-order.order`                                                  | picker pada `lines` (join harga per cabang)                                                     |
| `cafe-order/forms/order-form-qr.yaml` **(baru)**                    | `picker_panel: aside`, 5 field `hidden` + `default_from`, `context` sesi                        |
| `cafe-order/pages/menu-catalog.yaml`                                | jadi Page biasa: `blocks: [{form: {ref: order-form-qr, mode: create}}]`                         |
| `cafe-stock.purchase-order` + `purchase-order-form.yaml` **(baru)** | picker pada `lines` (sumber `ingredient`) — domain berbeda, kode sama                           |
| `cafe-stock.stock-opname`                                           | picker pada `lines`; `quantity_field: counted_qty` (yang diakumulasi = hasil hitung fisik)      |
| `inventory.stock-movement` + `stock-movement-form.yaml` **(baru)**  | picker pada `lines` (sumber `product`)                                                          |
| `gl.journal-entry` + `journal-entry-form.yaml` **(baru)**           | picker pada `lines` **tanpa** `quantity_field` (satu akun per baris; debit/kredit diisi manual) |

**Bukti.**

- `go test ./...` **35 paket ok**; `vitest` **250 lulus** (`lib/picker.test.ts`
  25 kasus: aturan baris, mapping snapshot, template, join harga);
  `tsc -b` bersih.
- `formspec validate` (schema lokal): kafe **0 problem** (72 manifest, termasuk
  5 picker + 3 Form baru); inventory/gl **jumlah problem identik sebelum-sesudah**
  (4 dan 3 — drift schema lama; dengan schema pra-perubahan angka "before"
  justru lebih tinggi karena `picker` belum dikenal). Tidak ada error yang
  menyebut `picker`.
- E2E sungguhan (spec kafe, DB segar) — payload yang sama di-POST lewat **Form
  pipe**: `number=ORD-2026-00001`, `channel=qr_table` (dari `default_from`),
  `table_session_id`/`branch_id` dari context, catatan per baris terbawa,
  `line_total` **{50000,12500}** dan `subtotal` **{62500}** terhitung server.

**Tetap terbuka** (tidak berubah dari sebelumnya): token QR → sesi masih dua
langkah (TODO 2.2); widget uang/POS (2.14); menulis `menu-item-price` lewat API
terhalang guard script (#30/#31 = 4.5); nomor pesanan per cabang (1.6).

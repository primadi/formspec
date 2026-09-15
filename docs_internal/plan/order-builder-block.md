# Plan — S1: blok transaksional `order_builder` (katalog + keranjang + checkout)

**Status**: ⛔ **digantikan** (2026-09-15) oleh `docs_internal/plan/child-field-picker.md`.
Blok `order_builder` terlalu sempit dan menduplikasi jalur tulis Form; pickernya
pindah ke deklarasi **child field** (`child.picker`) sehingga berlaku di Form
mana pun. Changelog pengganti: `2026-09-15-004`.
**TODO item**: `examples/kafe/gaps_found/TODO.md` **1.5** (prioritas §F #5)
**Menutup**: gap **#5** (`03-pos-dan-public-ordering.md`) · S1 di
`13-kelengkapan-spec-untuk-kafe.md` §A
**Sumber spec**: `docs/spec/frontend/06-page-kinds.md` §1 (Page blocks)
**Prasyarat selesai**: S7 (aritmetika `money`), S3 (akses publik per-entity),
S10 (kosakata widget), 2.1 (`lifecycle: none` / plain_crud)

---

## 1. Masalah

`PageBlock` adalah himpunan tertutup `{form, table, component, widget, html,
section}` — tidak ada blok transaksional. `Listing` read-only tanpa aksi baris;
`SectionBlock` murni presentasi **tanpa data binding**. Akibatnya alur pelanggan
QR ("scan → lihat menu → pilih → kirim pesanan") hanya bisa _dipaksakan_ dengan
`Form` + `ChildTable`, yang menghasilkan **form admin**, bukan UX memesan.

## 2. Keputusan — Page block `order_builder`

Dipilih di antara tiga opsi gap doc (`widget: image` di Listing · blok
transaksional · `kind: Pos` baru). Alasan:

- **Blok, bukan kind baru.** Page sudah mengomposisi blok; kind baru menuntut
  registrasi kind, rute (`internal/api/generator.go`), permission, docs-kind,
  dan dispatcher SPA — jauh lebih besar untuk nilai yang sama.
- **Blok baru, bukan memperluas `Listing`.** `ListingSpec` sengaja read-only
  (katalog publik, tanpa tulis); menambah aksi baris ke sana akan merusak
  kontraknya sendiri.
- Blok ini **punya data binding** (katalog → keranjang → payload create),
  berbeda dari `SectionBlock`.

### Bentuk deklaratif

```yaml
kind: Page
spec:
  route: /menu/:guest_token
  public: true
  context:
    - {
        name: session,
        source: entity,
        entity: cafe-order.table-session,
        id: "{route.guest_token}",
      }
  blocks:
    - order_builder:
        catalog:
          entity: cafe-master.menu-item
          name_field: name # default "name"
          price_field: price # money field (default "price")
          image_field: photo # opsional: file/attachment atau URL string
          description_field: description # opsional
          category_field: menu_category_id # opsional: relation/enum → chip filter
          filter: { is_available: true } # opsional: pre-filter kesetaraan
          columns: 3 # 2–4, default 3
          search: true # filter client-side atas name_field
          empty_text: "Menu belum tersedia"
        lines:
          field: lines # child field pada entity checkout
          item_field: menu_item_id
          quantity_field: quantity
          price_field: unit_price_snapshot
          name_field: name_snapshot
          note_field: note # opsional (catatan per baris)
          max_quantity: 20 # default 99
        checkout:
          entity: cafe-order.order
          title: "Keranjang"
          submit_label: "Kirim pesanan"
          fields: # FormField[] — widget vocabulary (S10) berlaku
            - {
                field: guest_note,
                widget: textarea,
                label: "Catatan untuk dapur",
              }
          defaults: # diseed ke payload; interpolasi `{token}`
            channel: qr_table
            transaction_date: "{now}"
            table_session_id: "{session.id}"
            branch_id: "{session.branch_id}"
          success_message: "Pesanan terkirim!"
          reset_after_submit: true # default true
```

**Interpolasi `defaults`.** Nilai string boleh memakai `{dotted.path}` — konvensi
yang **sudah** dipakai `ContextDecl.id` (`"{user.vendor_id}"`) dan teks section.
Scope-nya = render context halaman (`spec.context` + slot standar `user`/`route`)
**plus dua token blok**: `{now}` (ISO datetime) dan `{today}` (ISO date). Token
ini blok-lokal (tidak menambah kontrak render-context global).

**Yang tidak dilakukan blok:** tidak menghitung/menyimpan total pesanan. Total
adalah kontrak Entity (`computed`), bukan UI — blok hanya menampilkan subtotal
keranjang. (Justru inilah yang membuat adopsi kafe butuh `computed`, §5.)

**Ditunda (dicatat, bukan dilupakan):** `price_entity` (harga per cabang dari
`menu-item-price` — butuh join + scope cabang, S5/3.5), kasir/POS numpad &
pembayaran (`kind: Pos` + widget uang, item 2.14), dan `{session.*}` otomatis
untuk branch (butuh atribut sesi, S5/1.8).

## 3. Perubahan per file

| File                                                                            | Perubahan                                                                                                                                                  | Effort |
| ------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `pkg/spec/frontend.go`                                                          | `OrderBuilderBlock` + 4 sub-struct (`OrderCatalogDecl`, `OrderLinesDecl`, `OrderCheckoutDecl`) + `PageBlock.OrderBuilder` + validasi di `ValidatePageSpec` | medium |
| `internal/genjsonschema/generator.go`                                           | daftar `sharedTypes` + `KindMapping` (kalau perlu) agar `$ref` resolve                                                                                     | small  |
| `schemas/**`                                                                    | regenerasi (`make generate-schema`)                                                                                                                        | small  |
| `renderers/react-shadcn/src/types/manifest.ts`                                  | tipe blok + `PageBlock.orderBuilder`                                                                                                                       | small  |
| `renderers/react-shadcn/src/lib/orderBuilder.ts` **(baru)**                     | logika murni: add/setQty/remove, subtotal, interpolasi `{token}`, `buildSubmitPayload`                                                                     | medium |
| `renderers/react-shadcn/src/kinds/page/blocks/OrderBuilderBlock.tsx` **(baru)** | grid katalog + keranjang + checkout                                                                                                                        | large  |
| `renderers/react-shadcn/src/kinds/page/PageRenderer.tsx`                        | dispatch `if (block.orderBuilder)`                                                                                                                         | small  |
| `docs/spec/frontend/06-page-kinds.md`                                           | §1 daftar blok + section `order_builder`                                                                                                                   | small  |
| `examples/kafe/spec/**`                                                         | halaman QR baru (`cafe-order/pages/menu-catalog.yaml`) + `computed` total pesanan (§5)                                                                     | medium |
| Test                                                                            | `pkg/spec/*_test.go`, `lib/orderBuilder.test.ts`, `OrderBuilderBlock` render test                                                                          | medium |

## 4. Verifikasi

```bash
make generate-schema && go test ./... && cd renderers/react-shadcn && npx vitest run
go run ./cmd/formspec validate --spec examples/kafe/spec --schema schemas   # 0 problem
go run ./cmd/formspec check -f examples/kafe/spec                          # 0 error
```

**Bukti accept** — payload yang dibangun `buildSubmitPayload` (diuji unit,
bukan dikira-kira) di-POST nyata ke `/_ui/entity/cafe-order/order`:

- 201 + `lines` tersimpan dengan `line_total` **terhitung server** (S7),
  `subtotal`/`total_amount` ikut terhitung, `number` ter-generate,
  `channel: qr_table`, `table_session_id`/`branch_id` dari hasil interpolasi.
- Halaman QR = `blocks: [order_builder]` (bukan `form` + `child-grid`).

## 5. Adopsi kafe (bagian dari item ini)

1. **Halaman QR** `cafe-order/pages/menu-catalog.yaml` (route `/menu/:guest_token`,
   `public: true`, `context: session → table-session`) memakai `order_builder`.
2. **`computed` untuk nilai pesanan** — sekarang bisa karena S7 (dan komentar
   `GAP-02` di entity itu sudah kedaluwarsa): child `line_total =
quantity * unit_price_snapshot`, parent `subtotal = sum([i["line_total"] …])`,
   `total_amount = subtotal - discount_amount + …`. Ini yang membuat pesanan QR
   punya angka, bukan `null`.
3. Tolak: **tidak** mengubah `pos-workbench` menjadi POS numpad — itu butuh
   widget uang (2.14) dan layar POS adalah pembahasan terpisah (`kind: Pos`).
   Marker GAP-05 di `pos-workbench` diperbarui: bagian pelanggan tertutup,
   bagian kasir masih terbuka.

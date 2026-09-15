# 2026-09-15-003 — S1: blok Page `order_builder` (katalog + keranjang + checkout)

Item `examples/kafe/gaps_found/TODO.md` 1.5 (prioritas §F #5) — menutup gap **#5**
untuk sisi pelanggan. Plan: `docs_internal/plan/order-builder-block.md`.

**Masalahnya.** `PageBlock` adalah himpunan tertutup
(`form|table|component|widget|html|section`) — tidak ada blok transaksional.
`Listing` read-only tanpa aksi baris, `SectionBlock` tanpa data binding. Alur
"pelanggan scan QR → lihat menu → pilih → kirim pesanan" hanya bisa dipaksakan
dengan `Form` + `ChildTable`, yang menghasilkan **form admin**, bukan UX memesan.

**Keputusan: blok Page, bukan kind baru.** Page sudah mengomposisi blok;
`kind: OrderBuilder` menuntut registrasi kind, rute, permission, docs-kind, dan
dispatcher SPA untuk nilai yang sama. `Listing` juga tidak diperluas — katalog
publik read-only adalah kontraknya sendiri.

**Bentuknya.**

```yaml
- order_builder:
    catalog:  { entity, name_field, price_field, image_field, category_field, filter, columns, search }
    lines:    { field, item_field, quantity_field, price_field, name_field, note_field, max_quantity }
    checkout: { entity, fields[], defaults{}, submit_label, success_message, reset_after_submit }
```

- `price_entity`/`price_match_field`/`price_filter` — harga boleh tinggal di
  entity terpisah (kafe menyimpannya per cabang di `menu-item-price`); join
  client-side, dan item tanpa harga tampil **tidak bisa dipesan** alih-alih
  dikirim sebagai `0`.
- `defaults` diinterpolasi `{dotted.path}` (konvensi yang sudah dipakai
  `ContextDecl.id` dan teks section) terhadap render context halaman +
  token blok-lokal `{now}`/`{today}`. Token tak terselesaikan dibiarkan
  **verbatim** sehingga terlihat di payload — bukan string kosong yang menyamar
  sebagai field wajib terisi.
- `checkout.fields` adalah `FormField`, jadi kosakata widget S10 berlaku di
  dalamnya (dan divalidasi).
- Blok **tidak** menghitung/menyimpan total pesanan: itu kontrak Entity
  (`computed`, [`docs/spec/backend/05-field-types.md`](../../docs/spec/backend/05-field-types.md) §2.1).
- `defaults` yang menyentuh `lines.field` ditolak validator — keranjang yang
  memiliki field itu.

**Yang diubah.**

- `pkg/spec/order_builder.go` (baru) — `OrderBuilderBlock` + `OrderCatalogDecl`
  - `OrderLinesDecl` + `OrderCheckoutDecl` + `ValidateOrderBuilderBlock`;
    `PageBlock.OrderBuilder`; `ValidatePageSpec` memvalidasinya (sekaligus
    merapikan rantai `if` blok yang dulu `continue` satu sama lain).
- `internal/genjsonschema/generator.go` — 4 tipe baru masuk `sharedTypes`;
  schema ter-regenerasi (`$defs/OrderBuilderBlock*`, `PageBlock.order_builder`).
- `renderers/react-shadcn/src/lib/orderBuilder.ts` (baru) — logika murni:
  add/set/remove line, clamp qty, subtotal display, `priceIndex`+`catalogTiles`
  (join harga), `interpolateDefaults`/`interpolateFilter`, `buildLinePayload`,
  `buildSubmitPayload`. **25 test** (`orderBuilder.test.ts`), termasuk payload
  persis yang dipakai E2E.
- `renderers/react-shadcn/src/kinds/page/blocks/OrderBuilderBlock.tsx` (baru) —
  grid katalog (gambar/kategori/cari), keranjang (qty, catatan per baris,
  subtotal), checkout (field header dari kosakata widget, submit, sukses, reset);
  dispatch di `PageRenderer.tsx`.
- `examples/kafe`: halaman baru `cafe-order/pages/menu-catalog.yaml`
  (route `/menu/:session_id`, `public: true`), `line_total` + `subtotal` kini
  `computed` di entity `order` (S7 membuatnya bisa; komentar GAP-02 kedaluwarsa
  dihapus), marker GAP-05 diperbarui (sisi pelanggan tertutup, kasir terbuka).
- `docs/spec/frontend/06-page-kinds.md` §1 — bagian "Blok `order_builder:`".

**Tiga bug engine ditemukan selama pengerjaan — semuanya kelas "diam-diam
salah", jadi diperbaiki di sini:**

1. **Filter boolean tidak pernah cocok.** `?is_available=true` mengembalikan
   **nol baris** tanpa error: nilai string `"true"` dibandingkan dengan kolom
   hasil cast numerik (`CAST(json_extract(data,'$.f') AS INTEGER) = 'true'`).
   Katalog QR akan tampil kosong tanpa gejala. Kini
   `coerceFilterValue` menerima `true/false/1/0/yes/no` untuk field boolean
   (dipakai `List`, `Aggregate`, `Window`). Test:
   `TestList_BooleanFilterAcceptsTrueString`.
2. **Baris tanpa `created_by`/`updated_by` merusak seluruh pembacaan entity.**
   Semua kolom audit nullable, tapi `scanEntityRecord` memindainya ke `string` →
   baris hasil seed/migrasi/operator SQL membuat **setiap** `list`/`find` entity
   itu 500 (`converting NULL to string is unsupported`). Kini `sql.NullString`.
   Ketemu karena seed harga di E2E.
3. **Gerbang permission render-context salah nama.** `source: entity` memeriksa
   `{module}.{entity}.view` (singular), sedangkan permission terdaftar
   `{module}.{plural}.{action}` → deklarasi `context` entity **tidak pernah**
   resolve kecuali pemanggil memegang `*` (seed dev memegangnya, karena itu tak
   terlihat). Ditambah: permukaan `public: true` kini melewati pra-cek (pelanggan
   anonim tidak punya permission apa pun, dan server tetap otoritas). Ini yang
   membuat halaman QR publik bisa menyelesaikan sesi mejanya. Test:
   `useRenderContext.test.ts`.

**Bukti runtime** (spec kafe, `formspec dev`, DB segar):

| Langkah                                   | Hasil                                                                                                                                                                          |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Anonim baca katalog (`is_available=true`) | `['Kopi Susu','Roti Bakar']`                                                                                                                                                   |
| Anonim baca baris harga cabang            | 2 baris `{amount, currency}`                                                                                                                                                   |
| Context halaman (sesi dari route)         | `session.id`, `branch_id`, `table_id`                                                                                                                                          |
| POST payload `buildSubmitPayload()`       | **201** — `number=ORD-2026-00001`, `channel=qr_table`, `guest_note` + `note` per baris terbawa, `line_total` **{50000,12500}** dan `subtotal` **{62500}** dihitung server (S7) |

`go test ./...` **35 paket ok**; `vitest` **250 lulus** (dari 221);
`tsc -b` bersih; `formspec validate --spec examples/kafe/spec --schema schemas`
**0 problem** (70 manifest); `formspec check` **0 error**.

**Tetap terbuka (dicatat, bukan ditutupi).**

- **Token QR → sesi** masih dua langkah (aplikasi membuat/menemukan sesi lebih
  dulu): `GET /entity/{id}` me-resolve ID dan natural key, sedangkan
  `guest_token` bukan keduanya. Menjadikan token sebagai kunci milik tamu adalah
  **2.2** (sisa #45, scope per-permukaan). Halaman mendokumentasikannya, tidak
  berpura-pura sudah bisa.
- **Menulis harga lewat API masih gagal**: `guard_menu_item_price_unique.star`
  memakai `ctx.db().query` dengan nama kolom mentah (`branch_id`) yang tidak ada
  (tabel menyimpan JSONB + kolom turunan `_branch_id`) — gap #30/#31, item 4.5.
  Verifikasi menyisipkan baris harga lewat SQL langsung (halaman QR hanya
  membaca).
- **Nomor pesanan bertabrakan di cabang kedua pada DB yang sama**
  (`ORD-2026-00001` dobel): `scope_field` per cabang + index unik global → item
  **1.6**/#9/3.6. Verifikasi memakai DB segar.
- **Sisi kasir**: numpad uang/kembalian menunggu widget uang (**2.14**); layar
  POS sebagai kind tersendiri belum ada (`pos-workbench` tetap bentuk sementara
  dengan marker GAP-05 yang diperbarui).

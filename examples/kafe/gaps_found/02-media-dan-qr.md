# Gap #3 & #4 — QR Code dan Gambar Produk

Dua hal yang disebut langsung oleh pemilik proyek: *"menampilkan gambar menu,
menampilkan qrcode"*. Ternyata keduanya punya status yang **berbeda**: gambar
setengah tersedia, QR tidak ada sama sekali.

---

## Gap #3 — Tidak ada QR code, sama sekali ✅ Pasti

### Bukti

- **Tidak ada widget** — `renderers/react-shadcn/src/widgets/index.ts` (barrel)
  hanya berisi: `TextInput, TextareaInput, RichText, FileInput, NumberInput,
  Select, Switch, Badge, RelationPicker, DateInput, JsonInput, ChildTable,
  RadioGroup, Combobox, PasswordInput, SliderInput, TagsInput`. Tidak ada
  QR/barcode.
- **Tidak ada kind** — katalog kind resmi berjumlah **34** dan tidak satu pun
  berkaitan dengan QR/barcode. `docs/kind/` hanya punya 4 grup:
  curation (3), data (11), ui (15), infra (5).
- **Tidak ada field type** — `FieldType` (pkg/spec/entity.go) adalah himpunan
  tertutup: `string, text, richtext, integer, decimal, money, boolean, enum,
  date, datetime, time, uuid, json, file, attachment, relation, child, number`.
  Tidak ada `qr`.
- **`PageBlock` tertutup** — `renderers/react-shadcn/src/types/manifest.ts`:
  `PageBlock { form, table, component, widget, html, section }`. Tidak ada slot
  untuk widget presentasional non-bisnis.
- **`SectionBlock` marketing-only** — tipe tertutupnya
  `hero | feature_grid | card | carousel | cta | banner | alert | notice`.
  Tidak ada kanvas bebas.
- **Tidak ada primitive QR di engine** — pencarian di repo untuk `qr`/`qrcode`
  tidak menemukan apa pun. `ctx.*` primitives adalah closed set 9:
  `db, cache, lock, queue, pubsub, storage, kvstore, config, log`.

### Dampak ke aplikasi kafe: **BLOCKER**

Kebutuhan bisnis yang tidak bisa dinyatakan dalam YAML sama sekali:

1. **QR code per meja** — untuk dicetak lalu ditempel di meja. Butuh
   *generate* QR dari sebuah URL/string, lalu tampilkan + cetak.
2. **QR statis di struk/nota** (opsional) — QR ke halaman struk digital.
3. **QR/barcode scanning** — kalau nanti produk punya barcode untuk kasir cepat.
4. **QR untuk pembayaran** — QRIS. Saat ini `examples/Midtrans-Payment-Gateway`
   ada, tapi itu wrapper Service, bukan generator QR.

### Kenapa ini terasa "seharusnya sudah ada"

QR code adalah kebutuhan generik dan berulang di aplikasi bisnis nyata (tiket
antrean, label rak, QR meja, QR pembayaran, e-tiket). Contoh `Clinic-UI-Showcase`
bahkan sudah punya `kind: Print` **tiket antrean** — yang seharusnya wajar
memuat QR.

### Usulan (pilih salah satu, makin ke bawah makin besar)

| Tingkat | Usulan | Cakupan |
| --- | --- | --- |
| Kecil | Widget `qrcode` yang bisa dipakai di Form (read-only, nilai = string yang di-encode) + blok Page | Meja, struk, e-tiket |
| Sedang | Tambah tipe `SectionBlock` (`qr`) atau blok `PageBlock` baru untuk konten presentasional ber-data | Halaman publik |
| Besar | `Service` bawaan `formspec/qrcode` (`input: string, output: file`) sehingga hasilnya jadi objek `ctx.storage` — bisa dipakai di Print, Listing, dan API | Semua kanal |

Yang **paling mendesak** untuk kafe: widget read-only yang meng-encode sebuah
field `string`/URL menjadi gambar QR, supaya `kind: Print` bisa mencetaknya di
kertas thermal.

---

## Gap #4 — Upload gambar ADA, tampilan gambar TIDAK ✅ Pasti

Ini temuan yang lebih halus dan justru lebih menjebak: separuh fitur sudah ada,
separuh lagi belum — sehingga mudah diasumsikan "sudah bisa".

### Yang sudah ada (jangan dibangun ulang)

| Kemampuan | Bukti |
| --- | --- |
| Field type `file` / `attachment` (alias, dinormalisasi) | `pkg/spec/entity.go`: `FieldFile`, `FieldAttachment` |
| Metadata storage (`allowed_types`, `max_size_mb`, `max_count`, `visibility`, `cdn`, `transform` resize/thumbnail) | `StorageSpec` + `StorageTransform` di `pkg/spec/entity.go`; `05-field-types.md` §1.3 |
| Widget upload dengan **preview gambar** | `renderers/react-shadcn/src/widgets/FileInput.tsx` — `isImage` → `<img className="h-12 w-12 rounded border object-cover">`, plus tombol Replace/Remove |
| Route upload | `internal/api/file.go` → `HandleFileUpload()`, `POST /{module}/{entity}/{id}/{field}`, multipart, permission = `update` |
| Pemetaan tipe→widget | `derive.ts` → `case "file": return "fileinput"` |
| Sanitasi nama file + key objek ter-scope | `{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}` |

Jadi **mengunggah foto menu sudah bisa hari ini.**

### Yang TIDAK ada — gambar tidak pernah dirender sebagai gambar

Renderer selain form memakai `renderCellValue()` bersama
(`renderers/react-shadcn/src/lib/renderCell.tsx`), dan fungsi itu **tidak punya
cabang untuk `file`**:

```ts
if (widget === "badge") ...
if (widget === "boolean") ...
if (format === "currency" && typeof value === "number") ...
if (format === "date" && typeof value === "string") ...
if (format === "relative" && typeof value === "string") ...
if (typeof value === "object") return JSON.stringify(value)
return String(value)   // ← field `file` mendarat di sini
```

Akibatnya, sebuah field `menu-item.photo` (objek key) dirender sebagai **teks
path** di:

| Renderer | File | Hasil untuk field `file` |
| --- | --- | --- |
| **Table** | `kinds/table/TableRenderer.tsx` | teks path objek |
| **Listing** (katalog publik!) | `kinds/listing/ListingRenderer.tsx` | teks path objek |
| **Report** | `kinds/report/ReportRenderer.tsx` | teks/gambar tidak muncul |
| **DetailPage** | `kinds/page/DetailPage.tsx` | **link download** dengan ikon `FileText` + nama file — bukan `<img>` |
| **ChildTable** | `widgets/ChildTable.tsx` | lewat `renderCellValue` → teks |
| **Print** | `kinds/print/PrintRenderer.tsx` | `resolveCellValue()` → `String(value)` |

Jadi **"menampilkan gambar menu" tidak bisa dilakukan** di katalog publik
(QR order), di daftar menu POS, di laporan menu terlaris, maupun di struk —
walaupun upload-nya mulus.

### Dampak ke aplikasi kafe: **BLOCKER**

Menu kafe tanpa foto adalah menu yang jauh lebih lemah secara komersial.
Yang paling parah: **katalog publik untuk pelanggan QR tidak bisa menampilkan
foto menu**, padahal itulah halaman yang paling butuh gambar.

### Usulan

1. `renderCellValue()` / `cellHintsForField()` perlu mengenal `file`:
   - kalau `allowed_types` mengandung gambar → render `<img>` dengan URL
     download (`/{ws}/_ui/entity/{module}/{entity}/{id}/{field}` — persis yang
     sudah dipakai `FileInput`), ukuran kecil di Table, besar di Listing.
   - selain gambar → tombol download (perilaku sekarang).
2. Tambah opsi eksplisit di manifest: `TableColumn`/`Listing` sudah punya
   `widget` dan `format` — cukup tambah nilai `widget: image`
   (mis. `{ field: photo, widget: image }`). Ini jalur termurah karena
   infrastrukturnya sudah ada.
3. `DetailPage` untuk `field.type === "file"` + gambar → render `<img>`
   sebelum link download.
4. `PrintRenderer.resolveCellValue()` → kalau field `file` gambar, ambil URL
   absolut (Print butuh URL, bukan key).
5. Perhatikan **`transform` thumbnail** — sudah dideklarasikan di spec
   (`resize`/`thumbnail` digenerate server-side saat upload). Cek apakah
   benar-benar jalan; kalau ya, Table bisa memakai thumbnail, bukan gambar
   penuh.

### Catatan

Sudah ada preseden di kode bahwa penulisnya sadar gambar itu penting:
`FileInput` deteksi `isImage` lewat ekstensi
(`/\.(png|jpe?g|gif|webp|svg)$/i`) dan punya mode readonly yang merender `<img>`.
Jadi logikanya **sudah ada di satu tempat** — hanya belum dipakai bersama oleh
renderer lain.

---

## Gap #4b — Format `storage.allowed_types` ambigu ✅ Pasti

Ditemukan saat menulis `menu-item.photo`.

### Bukti

Tiga sumber menyebut bentuk yang **berbeda**:

| Sumber | Bentuk yang dicontohkan |
| --- | --- |
| `docs/spec/backend/05-field-types.md` §1.3 + `docs_old/spec/03-core-extended.md` | `allowed_types: [jpg, png, webp]` — ekstensi **tanpa titik**, plus `file_list` sebagai tipe terpisah |
| `renderers/react-shadcn/src/widgets/FileInput.tsx` → `allowedFileType()` | mencocokkan `".ext"` (**dengan** titik), atau `contentType` persis (`image/jpeg`), atau pola `image/*` |
| `internal/api/file.go` | memakai `allowedFileType(...)` yang sama di server (menegakkan `allowed_types`) |

Akibatnya, `allowed_types: [jpg]` (bentuk yang **didokumentasikan**) tidak akan
cocok dengan klien, karena `jpg` bukan `.jpg`, bukan mime, bukan `image/*`.

Selain itu `docs_old` menyebut tipe `file_list` yang **tidak ada** di himpunan
`FieldType` sekarang (`file` + `attachment` saja) — jadi cara menyatakan
"banyak file" juga tidak jelas (`max_count > 1`, bukan tipe terpisah).

### Dampak ke aplikasi kafe: **MEDIUM**

Foto menu adalah satu-satunya bentuk `allowed_types` yang dibutuhkan kafe, dan
salah bentuk berarti tombol upload menolak file yang seharusnya sah.

### Usulan

1. Pilih **satu** kanonik dan dokumentasikan eksplisit. Rekomendasi: ekstensi
   tanpa titik (`[jpg, png, webp]`) karena itu yang dipakai dokumen, lalu
   normalisasi di klien (`t` tanpa titik → cocokkan ekstensi).
2. `formspec validate` memvalidasi isi `allowed_types` (tolak nilai tak dikenal)
   sehingga salah bentuk tertangkap di validasi, bukan di UI.
3. Jelasakan cara "banyak file": `max_count > 1` atau tipe tersendiri.

### Bentuk yang dipakai di spec ini

Supaya aman, `menu-item.photo` mendeklarasikan **dua bentuk sekaligus**
(`[".jpg", ".jpeg", ".png", ".webp", "image/jpeg", "image/png", "image/webp"]`)
— kompromi yang tidak seharusnya perlu.

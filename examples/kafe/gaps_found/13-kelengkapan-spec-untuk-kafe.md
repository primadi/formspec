# Kelengkapan Spec FormSpec untuk Aplikasi Kafe

**Pertanyaan yang dijawab dokumen ini:** apa yang **kurang dari bahasa spec
FormSpec** (permukaan YAML) untuk membangun aplikasi kafe?

Fokusnya **spesifikasi, bukan implementasi.** Bug engine, fitur belum dikode, dan
salah dokumen dipisahkan ke bagian akhir (`§E`) karena perbaikannya dilakukan di
project FormSpec, bukan di sini.

Semua "tersedia sekarang" di bawah **diverifikasi dari JSON Schema resmi**
(`schemas/v1/`, cache lokal `formspec v0.0.8`) atau dari pesan validator yang
nyata — bukan dari pembacaan prosa.

---

## §0. Ringkasan — sepuluh hal yang tidak bisa dinyatakan di YAML

> Ringkasan S1–S10 di bawah; pembahasan tiap item ada di **§A**. Enam item
> tambahan yang bisa dinyatakan tapi belum lengkap ada di **§B** (S11–S16).

Diurutkan berdasarkan apakah aplikasi bisa dibangun.

| #       | Yang tidak bisa dinyatakan                  | Akibat untuk kafe                                                 |
| ------- | ------------------------------------------- | ----------------------------------------------------------------- |
| **S1**  | Halaman **katalog + keranjang** pelanggan   | Pelanggan **tidak bisa memesan** dari QR                          |
| **S2**  | **Filter bernilai dari sesi/route**         | Kasir lihat semua cabang; pelanggan bisa lihat pesanan orang lain |
| **S3**  | **Akses publik per-entity**                 | Publik dapat `list` shift, kas, dan nomor HP member               |
| **S4**  | **QR code**                                 | QR meja & QR struk tidak bisa dibuat                              |
| **S5**  | **Scope cabang deklaratif**                 | Multi-outlet tanpa isolasi yang dijamin                           |
| **S6**  | **Pemetaan payload Integrator**             | Integrasi akuntansi tidak bisa dinyatakan penuh                   |
| **S7**  | **Aritmetika & agregasi `money`**           | Numpad uang, kembalian, total, laporan margin                     |
| **S8**  | **Unique parsial / index atas relasi**      | Satu shift terbuka, satu harga per cabang                         |
| **S9**  | **Approval pada transisi multi-state-asal** | Persetujuan void bisa dilewati                                    |
| **S10** | **Kosakata `widget` tidak divalidasi**      | Salah ketik == fitur belum ada (tak terbedakan)                   |

Sisanya (S11–S16) penting tapi punya jalan sementara.

---

## §A. Tidak bisa dinyatakan sama sekali

### S1 — Halaman pemesanan pelanggan (katalog + keranjang)

> **✅ Selesai 2026-09-15 (TODO 1.5).** Dipilih **Opsi 1**: blok Page
> `order_builder` (bukan kind baru) — `catalog` + `lines` + `checkout`, termasuk
> join harga dari entity terpisah (harga per cabang) dan interpolasi `defaults`
> (`{context}`, `{now}`, `{today}`). Halaman QR kafe
> (`cafe-order/pages/menu-catalog.yaml`) memakainya; verifikasi runtime:
> anonim baca katalog + harga → keranjang → POST → order dengan `line_total`
> dan `subtotal` terhitung server. Sisi kasir (numpad uang, `kind: Pos`) tetap
> terbuka — lihat 2.14.

**Kebutuhan kafe.** Pelanggan memindai QR meja → melihat menu bergambar →
menambah beberapa item → mengirim pesanan.

**Tersedia sekarang** (terverifikasi):

```json
"PageBlock": {
  "properties": { "component", "form", "table", "widget", "html", "section" },
  "additionalProperties": false
}
```

```json
"ListingSpec": { "properties": { "columns", "entity", "filters", "search" },
                 "additionalProperties": false }
```

`ListingSpec` didokumentasikan sebagai _"similar to table-list but **without
Auth-wrap assumptions and without row_actions/bulk_actions**"_.

**Kenapa tidak cukup.**

- `PageBlock` adalah himpunan tertutup — tidak ada blok transaksional.
- `Listing` tidak punya aksi baris, jadi tidak ada tempat menaruh "Tambah".
- `SectionBlock` adalah **presentasi murni tanpa pengikatan data** — schema
  menyatakannya eksplisit: _"pure presentation, **no data binding**, no auth,
  and zero styling fields"_. Jadi section tidak bisa dijadikan kanvas ber-data,
  dan `SectionType` hanya pemasaran (`hero | feature_grid | card | carousel |
cta | banner | alert | notice`).
- Menyusun dari blok yang ada menghasilkan **form admin** (Form + ChildTable),
  bukan UX memesan.

**Konstruk yang dibutuhkan** — salah satu:

```yaml
# Opsi 1: blok transaksional
blocks:
  - order_builder:
      catalog: { ref: menu-catalog, layout: grid, columns: 3 }
      cart: { add_action: add-line, show_subtotal: true }
      checkout: { ref: order-form-qr }
```

```yaml
# Opsi 2: kind baru (tier page)
kind: OrderBuilder
spec:
  catalog_entity: cafe-master.menu-item
  price_entity: cafe-master.menu-item-price
  cart_entity: cafe-order.order
  line_field: lines
  modifiers: [note, quantity]
```

---

### S2 — Filter yang nilainya berasal dari sesi / route

**Kebutuhan kafe.**

- Kasir hanya melihat pesanan **cabangnya**.
- Pelanggan melihat **pesanannya sendiri** lewat token di URL.
- Supervisor melihat shift **dirinya**.

**Tersedia sekarang** (terverifikasi, `FilterSpec` lengkap):

```
properties: all_label, default, field, label, op, show_all, type
```

- `op` enum: `eq neq gt gte lt lte between in nin like ilike null notnull` ✅ lengkap
- `type` enum: `text select date …` ✅
- `default`: _"supports `today` / `today()`"_ — **hanya itu**.

**Kenapa tidak cukup.** Nilai filter hanya bisa **statis saat penulisan spec**
atau `today()`. Tidak ada cara menyatakan _"nilainya diambil dari pengguna yang
sedang login"_ atau _"dari parameter route"_.

Konsekuensi penting: `fixed_filters` memang **immutable dan di-merge server-side**
— jadi ia bukan sekadar UI. Tapi karena **nilainya dibekukan di manifest**, ia
tidak bisa berarti "cabang milik pengguna ini". Untuk kasir cabang A, filter itu
tidak membatasi apa pun.

**Konstruk yang dibutuhkan:**

```yaml
fixed_filters:
  # nilai dari atribut pengguna
  - { field: branch_id, op: eq, from: session, attr: branch_id }
  # dari parameter route
  - { field: guest_token, op: eq, from: route, param: guest_token }
  # dari record yang ditugaskan padanya
  - { field: assigned_to, op: eq, from: session, attr: principal_id }
```

Ini **satu konstruk** yang menyelesaikan dua kebutuhan sekaligus: isolasi
multi-outlet (S5) **dan** akses pelanggan ke pesanannya sendiri (S3).

---

### S3 — Akses publik per-entity

**Kebutuhan kafe.** Publik boleh: baca menu & harga, buat pesanan.
Publik tidak boleh: baca shift, kas, nomor HP member, data karyawan.

**Tersedia sekarang** (terverifikasi dari perilaku runtime):

| Entity                      | Module di-mount App publik? | Anonim                |
| --------------------------- | --------------------------- | --------------------- |
| `cafe-master/menu-category` | ✅                          | `create` **berhasil** |
| `cafe-master/promo`         | ✅                          | `create` **berhasil** |
| `cafe-stock/ingredient`     | ❌                          | `401 Unauthorized`    |

`App.spec.modules` meng-mount **seluruh module**, dan `access: public` memberi
anonim `list`/`find`/`create` untuk **semua entity** di module itu.

**Kenapa tidak cukup.** Granularitasnya module. Untuk mengizinkan pelanggan
melihat menu **dan** membuat pesanan, module yang di-mount harus memuat keduanya
— dan `cafe-order` juga memuat `shift` dan `cash-movement`, sementara
`cafe-master` memuat `member` (nomor HP) dan `employee`.

**Konstruk yang dibutuhkan:**

```yaml
# App publik
access: public
public_entities:
  - { entity: cafe-master.menu-category, actions: [list, find] }
  - { entity: cafe-master.menu-item, actions: [list, find] }
  - { entity: cafe-master.menu-item-price, actions: [list] }
  - { entity: cafe-master.dining-table, actions: [find] }
  - { entity: cafe-order.table-session, actions: [create, find] }
  - { entity: cafe-order.order, actions: [create, find] }
```

Plus pasangan field-level (S3b) untuk menutup field sensitif tanpa memindahkan
entity ke module lain:

```yaml
- name: phone
  type: string
  exclude: [public_api] # sudah ada di schema; belum ditegakkan
```

---

### S4 — QR code

> **🟡 Sebagian 2026-09-16 (TODO 2.6).** Widget `qrcode` kini ada di **dua**
> kosakata tertutup (S10): `FormWidget` dan `TableCellWidget` — komponen
> `src/widgets/QrCode.tsx` (SVG, tajam saat dicetak), cabang di `FormFieldWidget`
> **dan** `renderCellValue`, enum schema ter-regenerasi. **Belum:** jalur cetak
> (`Print` memakai `resolveCellValue()` yang mengubah nilai jadi teks) dan adopsi
> di spec kafe (QR yang bisa dipindai butuh URL absolut, sedangkan `dining-table`
> hanya menyimpan kode meja). Scanning (barcode/QRIS) di luar cakupan.

**Kebutuhan kafe.** QR per meja (untuk dicetak dan ditempel), dan QR di struk
untuk membuka struk digital.

**Tersedia sekarang.** Tidak ada apa pun:

- tidak ada widget QR di barrel widget manapun
- tidak ada `kind` terkait (katalog 34 kind: curation/data/ui/infra)
- tidak ada `FieldType` QR (himpunan tertutup)
- tidak ada tipe `SectionBlock` presentasional
- `ctx.*` adalah himpunan tertutup 9 primitive

**Kenapa tidak cukup.** Kebutuhan generik dan berulang di aplikasi bisnis
(tiket antrean, label rak, QR meja, QR pembayaran, e-tiket). Contoh
`Clinic-UI-Showcase` bahkan punya `kind: Print` **tiket antrean** yang wajar
memuat QR.

**Konstruk yang dibutuhkan** — pilihan termurah:

```yaml
- name: qr_token
  type: string
- name: qr_image
  type: qrcode # field type baru: turunan dari field lain, read-only
  derived_from: qr_token
```

atau widget read-only:

```yaml
fields:
  - { field: qr_token, widget: qrcode } # merender QR dari nilai field
```

---

### S5 — Scope cabang deklaratif

> **✅ Selesai 2026-09-15 (TODO 1.8).** Tiga konstruk menggantikan satu: `scope:
{dimension, field, required}` (entity **dipartisi** — fakta data, tidak
> memfilter), `row_scope` (filter yang **ditegakkan server**; nama ini dulunya
> `scope`, diganti karena satu nama tidak bisa dua bentuk), dan `assignments:
[{dimension, field, principal_field}]` (**dari mana** nilai dimensi seorang
> principal berasal — inilah "pengguna ini bertugas di cabang X" yang
> sebelumnya tidak bisa dinyatakan). Nilai `from: session` diselesaikan dari
> `assignments` bila token tidak membawanya, dan `formspec validate` menolak
> `row_scope` yang atributnya tak punya sumber — mencegah bentuk yang 403
> selamanya sementara manifest terlihat benar. Penyalaan penyaringan otomatis
> tetap item **3.5**, sesuai urutan. Normatif:
> `docs/spec/backend/01-core-basic.md` §1.7.

**Kebutuhan kafe.** Satu kafe, banyak cabang. Harga beda per cabang, stok per
cabang, nomor pesanan per cabang, kasir hanya cabangnya.

**Tersedia sekarang.** Yang **bisa**: field relasi biasa bernama `branch_id`
(konvensi), `natural_key_rule.scope_field` untuk penomoran per-cabang.
Yang **tidak bisa**: menyatakan bahwa sebuah entity **ter-scope**, dan dari mana
scope pengguna berasal.

`TenantDecl{Isolated bool}` ada di `EntitySpec.Tenant` tetapi **dormant** (nol
konsumen) dan hanya **flag boolean**, bukan deskriptor dimensi.

**Kenapa tidak cukup.** Bahasa spec tidak punya cara menyatakan:

- "entity ini data per-cabang" → engine tak tahu untuk memfilter
- "pengguna ini bertugas di cabang X" → tak ada sumber nilai filter
- hierarki cabang (region → cabang) → tak ada tipe hierarkis

Konsekuensinya: **isolasi cabang sepenuhnya tanggung jawab aplikasi**, dan tidak
ada gerbang otomatis yang menangkap kebocoran.

**Konstruk yang dibutuhkan:**

```yaml
# pada entity
spec:
  version: v1
  scope: { dimension: branch, field: branch_id, required: true }
```

```yaml
# pada penugasan pengguna
spec:
  assignments:
    - { dimension: branch, entity: cafe-master.employee, field: branch_id }
```

Ini juga memberi `scope_field` natural key sesuatu untuk ditemukan secara generik,
dan membuat multi-outlet jadi **aturan spec**, bukan disiplin penulis.

---

### S6 — Pemetaan payload pada Integrator

**Kebutuhan kafe.** Pesanan lunas → jurnal: omzet → kredit 4-1000, pajak →
kredit 2-2000, kas → debit 1-1000.

**Tersedia sekarang** (terverifikasi):

```json
"IntegratorSpec": { "properties": { "listen", "call", "compensate" },
                    "required": ["listen", "call"], "additionalProperties": false }
"IntegratorCall": { "properties": { "resource", "action" },
                    "required": ["resource", "action"], "additionalProperties": false }
```

**Kenapa tidak cukup.** Kedua sisi tidak akan pernah punya nama field sama:

| Sisi kafe (`order`)                  | Sisi akuntansi (`gl.journal-entry`)        |
| ------------------------------------ | ------------------------------------------ |
| `total_amount`, `tax_amount` (money) | `debit_account`, `credit_account`          |
| `subtotal`                           | `entry_date`, `memo`, `lines[]` (akun COA) |

Pemetaan _"omzet → kredit 4-1000"_ adalah **pengetahuan akuntansi**, bukan
penamaan field. Tanpa tempat menyatakannya, ia harus hidup di script milik `gl`
(vertical pihak ketiga) atau di action kustom sisi kafe — dan yang kedua membuat
`kind: Integrator` kehilangan gunanya.

**Konstruk yang dibutuhkan:**

```yaml
call:
  resource: gl.journal-entry
  action: create
  map:
    entry_date: "{order.paid_at}"
    memo: "Penjualan {order.number}"
    lines:
      - { account: "1-1000", debit: "{order.total_amount}" }
      - { account: "4-1000", credit: "{order.subtotal}" }
      - { account: "2-2000", credit: "{order.tax_amount}" }
```

---

### S7 — Aritmetika & agregasi `money`

> **✅ Selesai 2026-09-15 (TODO 1.3).** Bentuk kanonik yang dipilih: **uang
> beroperasi langsung** — `computed: { formula: "tendered - amount" }` tetap
> ditulis apa adanya, dan engine-nya yang menyusul (`money - money` → money,
> `number × money` → money, `sum([money…])` → money). Operand tidak sah
> (objek non-money, list, teks) **error**, bukan `0`; agregasi `money`
> menjumlahkan `.amount`-nya dan field non-numerik ditolak `formspec check`.
> Normatif: `docs/spec/backend/05-field-types.md` §2.1 +
> `docs/spec/frontend/08-formspec-expr.md` §5. Jadi **tidak ada** sintaks
> `amount(x)` yang diwajibkan seperti sketsa di bawah — `amount(x)` hanya
> disediakan untuk kasus yang memang butuh skalar (mis. membandingkan dengan
> angka mentah).

**Kebutuhan kafe.** Kembalian, selisih kas, subtotal baris, total pesanan,
omzet harian, margin per menu.

**Tersedia sekarang.** `money` adalah tipe kelas satu, dan **nilainya objek** —
terverifikasi runtime:

```json
"min_purchase": { "amount": "50000", "currency": "IDR" }
```

**Kenapa tidak cukup.** Dua konstruk yang sudah ada dan terlihat cocok, keduanya
bekerja atas **skalar**:

```yaml
computed: { formula: "tendered - amount" } # money - money ?
columns: [{ field: total_amount, aggregate: sum }] # SUM atas objek ?
```

`formspec check` melaporkan **0 error** untuk keduanya — jadi bentuk ini
_disetujui_, tetapi semantiknya atas objek belum ditetapkan. Untuk data uang,
"belum ditetapkan" sama dengan "tidak bisa diandalkan".

**Konstruk yang dibutuhkan** — tetapkan satu kanonik:

```yaml
computed: { formula: "amount(tendered) - amount(amount)" }
```

atau

```yaml
computed:
  formula: "tendered - amount"
  money: { compare: amount, result: money }
```

dan untuk agregasi, nyatakan eksplisit bahwa `sum` atas `money` menjumlahkan
`.amount` dan menolak (bukan diam-diam salah) bila field bukan numerik.

---

### S8 — Unique parsial dan index atas relasi

**Kebutuhan kafe.** Tiga aturan keunikan:

| Aturan                                 | Bentuk                                                   | Bisa dinyatakan? |
| -------------------------------------- | -------------------------------------------------------- | ---------------- |
| Satu harga per menu per cabang         | `unique (branch_id, menu_item_id)`                       | ❌               |
| Satu baris stok per (cabang, bahan)    | `unique (branch_id, ingredient_id)`                      | ❌               |
| Satu shift terbuka per (cabang, kasir) | `unique (branch_id, cashier_id) **WHERE status='open'**` | ❌               |

**Tersedia sekarang** (terverifikasi):

```json
"IndexDecl": { "properties": { "fields": {"type":"array"}, "unique": {"type":"boolean"} },
               "additionalProperties": false }
```

**Kenapa tidak cukup.** Dua kekurangan **di bahasa spec**:

1. **Tidak ada predikat parsial.** `IndexDecl` hanya `{fields, unique}` — tidak
   bisa menyatakan `WHERE status = 'open'`.
2. **Relasi tidak bisa diindeks.** Index dihasilkan dari kolom turunan
   `_field GENERATED ALWAYS AS (json_extract(data,'$.field'))`, dan **field
   `relation` tidak menghasilkan kolom turunan** (diverifikasi: pencarian
   `_branch_id` di seluruh DDL → nol hasil).

`kind: Migration` bisa menutupinya dengan DDL mentah — tetapi itu justru bukti
bahwa **konsepnya tidak ada di bahasa spec**: jalan keluarnya adalah keluar dari
bahasa spec.

**Konstruk yang dibutuhkan:**

```yaml
indexes:
  - fields: [branch_id, menu_item_id]
    unique: true
  - fields: [branch_id, cashier_id]
    unique: true
    where: "status = 'open'" # predikat parsial
```

---

### S9 — Workflow pada transisi dengan banyak state asal

> **✅ Selesai 2026-09-15 (TODO 1.7).** Pemicu workflow merujuk transisi lewat
> `name:` (`via`), sehingga satu workflow mengawal **seluruh** state asal;
> pasangan `from`/`to` yang hanya mencakup sebagian transisi kini **ditolak**
> `formspec validate` dengan pesan yang menyebut `name:`. Normatif:
> `docs/spec/backend/02-core-extended.md` §6.

**Kebutuhan kafe.** Void pesanan yang sudah dibayar butuh persetujuan supervisor,
dan void bisa terjadi dari 4 status: `paid`, `in_kitchen`, `ready`, `served`.

**Tersedia sekarang** (terverifikasi):

| Tipe                                      | Field `from`                                                  |
| ----------------------------------------- | ------------------------------------------------------------- |
| `TransitionDecl` (state machine)          | **daftar boleh** — `"draft"` atau `[draft, awaiting_payment]` |
| `WorkflowTransitionRef` (pemicu workflow) | **string tunggal** (`"type": "string"`)                       |

**Kenapa tidak cukup.** Satu workflow hanya bisa mengawal **satu** state asal.
Menulis `from: paid` berarti void dari `in_kitchen`/`ready`/`served`
**tidak melewati approval sama sekali** — dan itu justru kasus tersering
(makanan sudah dibuat). **Tanpa error, tanpa log, tanpa peringatan.**
`formspec validate` tetap hijau karena validator tidak tahu transisi aslinya
punya empat state asal.

**Konstruk yang dibutuhkan** — pilihan terkuat: pemicu merujuk **transisi**,
bukan pasangan state, karena transisi sudah punya identitas unik (`via`):

```yaml
on:
  transition: cafe-order.order.void-order # nama transisi, bukan from/to
```

Alternatif minimal: `from` menerima daftar seperti `TransitionDecl.from`.

---

### S10 — Kosakata `widget` tidak divalidasi

> **✅ Selesai 2026-09-15 (TODO 1.4).** Dua himpunan tertutup, satu per permukaan:
> `FormWidget` (**24 nilai**) dan `TableCellWidget` (**4 nilai**: `badge`,
> `boolean`, `image`, `qrcode`). Sumber kebenaran `pkg/spec/widget.go`; enum masuk
> schema sebagai `$defs/FormWidget`/`$defs/TableCellWidget`. Anti-drift dijaga
> `src/widgets/catalog.test.tsx`. Menutup akar #1 ("salah ketik == fitur belum
> ada").

> **✅ Selesai 2026-09-15 (TODO 1.4).** Dua himpunan tertutup menggantikan string
> bebas: `FormWidget` (20 nama) dan `TableCellWidget` (`badge`, `boolean`) di
> `pkg/spec/widget.go` → enum di JSON Schema (`$defs/FormWidget`,
> `$defs/TableCellWidget`) → `formspec validate` menolak salah ketik dan editor
> dapat autocomplete. Paritas schema ↔ katalog ↔ implementasi dijaga
> `src/widgets/catalog.test.tsx`. Nama dokumen lama
> (`textinput`/`numberinput`/`dateinput`/`toggle`/`json-editor`) ternyata **tidak
> pernah ada** di renderer — `07-component-kinds.md` §1 kini memakai nama
> sebenarnya. Enam properti lain di tabel bawah (ReportParam.type, ReportColumn,
> EventDeliveryDecl.channel, PrintOutput.format, WorkflowStep.mode) tetap string
> bebas — itu **8.5**, di luar 1.4.

**Tersedia sekarang** (terverifikasi):

```json
"FormField": { "properties": { "field", "label", "help", "placeholder",
  "read_only", "readonly_when", "required_when", "visible_when", "compute",
  "widget": { "type": "string" } } }
```

**`widget` adalah string bebas — tanpa enum.**

**Kenapa ini gap, bukan sekadar ketidaknyamanan.** Akibatnya:

1. `widget: money-input` **lolos validasi** dan tidak melakukan apa pun.
2. **Salah ketik tidak bisa dibedakan** dari "widget belum ada".
   `widget: relaion-picker` sama validnya dengan `widget: relation-picker`.
3. Tidak ada katalog widget resmi yang bisa dibaca dari schema — penulis spec
   harus menebak, atau membaca kode renderer.
4. Editor tidak bisa menyarankan (tidak ada `enum` → tidak ada autocomplete).

Pola yang sama muncul di tempat lain dengan konsistensi yang tidak merata:

| Properti                                  | Status                                          |
| ----------------------------------------- | ----------------------------------------------- |
| `FilterSpec.op`                           | ✅ enum 13 nilai                                |
| `FilterSpec.type`                         | ✅ enum                                         |
| `FormField.widget` / `TableColumn.widget` | ❌ string bebas                                 |
| `ReportParam.type`                        | ❌ string bebas                                 |
| `ReportColumn.format` / `.aggregate`      | ❌ string bebas                                 |
| `EventDeliveryDecl.channel`               | ❌ string bebas (deskripsi menyebut 4 nilai)    |
| `PrintOutput.format`                      | ❌ string bebas (deskripsi menyebut 4 nilai)    |
| `WorkflowStep.mode`                       | ❌ string bebas (deskripsi: all/any/sequential) |

**Konstruk yang dibutuhkan:** jadikan himpunan tertutup sebagai `enum`, dan
untuk widget — generate katalognya dari barrel renderer supaya schema dan
implementasi tidak bisa berbeda.

---

## §B. Bisa dinyatakan, tapi tidak lengkap

### S11 — Tidak ada model pajak / service charge

> **🟡 Sebagian 2026-09-15 (TODO 1.8, versi minimal).** Yang ditambahkan: field
> type **`percent`** — secara numerik `decimal`, tetapi ditandai sebagai
> persentase sehingga renderer/format menampilkannya sebagai `%` alih-alih
> menebak. `branch.tax_percent`, `branch.service_charge_percent`, dan
> `promo.percent` di kafe kini memakainya. **Belum** ada model pajak penuh
> (dasar pengenaan, harga-termasuk-pajak, pembulatan pajak, pelaporan) — itu
> tetap tanggung jawab spec aplikasi. Normatif:
> `docs/spec/backend/05-field-types.md` §1.5.

**Kebutuhan kafe.** PB1 10% dapat diatur, service charge opsional per cabang,
ditampilkan terpisah di struk.

**Tersedia sekarang.** Tidak ada konsep pajak: tidak ada field type, tidak ada
kind, tidak ada atribut. Yang bisa dilakukan hanyalah menaruh `tax_percent` di
`branch` + field `tax_amount` di `order` + perhitungan manual.

**Kenapa tidak cukup.** Pajak penjualan adalah kebutuhan **universal** aplikasi
bisnis, dan bentuknya berulang: tarif, dasar pengenaan, apakah harga termasuk
pajak, pembulatan, pelaporan. Menyerahkannya ke penulis spec berarti setiap
aplikasi mengarang modelnya sendiri, dan **tidak ada dua aplikasi yang
hasilnya sama** — termasuk soal pembulatan.

Alternatif yang lebih sederhana: setidaknya **field type `percent`** dan cara
menyatakan "field ini pajak" agar renderer/report memperlakukannya konsisten.

### S12 — Satuan dan konversi

> **✅ Sebagian 2026-09-15 (TODO 1.8).** Deklarasi satuan kini ada:
> `unit: {base, convertible}` pada field yang nilainya adalah nama satuan,
> dengan `base`/`convertible` divalidasi terhadap `enum_values` — jadi satuan
> menjadi **data**, bukan konvensi di dalam script. Di kafe,
> `ingredient.unit` dan `recipe.lines[].unit` menyatakan grup gram↔kg (`ml` dan
> `pcs` sengaja di luar grup: dimensi lain, bukan error). **Konversi dan
> ledakan resep belum dihitung engine** — itu item **4.6**. Normatif:
> `docs/spec/backend/05-field-types.md` §1.6.

**Kebutuhan kafe.** Resep dalam **gram**; pembelian dalam **kg**; sebagian bahan
dalam **pcs** dan isi per kemasan (mis. "1 dus = 24 pcs").

**Tersedia sekarang.** `ingredient.unit` dan `recipe.lines[].unit` adalah `enum`
biasa yang saya tulis sendiri (`gram | ml | pcs`) — **tidak ada konsep satuan di
FormSpec**. Tidak ada konversi, tidak ada dimensi.

**Kenapa tidak cukup.** Ledakan resep (recipe explosion) membutuhkan aritmetika
lintas satuan. Tanpa itu, penulis spec harus memaksa satu satuan dasar dan
melakukan konversi di script — yang berarti **HPP bergantung pada disiplin
script**, bukan pada data.

**Konstruk yang dibutuhkan** (minimal):

```yaml
- name: quantity
  type: decimal
  unit: { base: gram, convertible: [kg, ounce] }
```

### S13 — Event tidak terikat pada transisi

> **🔴 Masih terbuka (2026-09-18).** `TransitionDecl` = `{From, To, Via, Guard}`
> (`pkg/spec/entity.go:1168`) — **tidak ada** `emit`. D3 (transisi tidak
> memancarkan event otomatis) sudah ditetapkan normatif, tetapi keterkaitan
> eksplisit transisi↔event belum bisa dinyatakan. Item **6.1**.

**Tersedia sekarang.** `EventDecl` ada di entity (`name`, `type`, `publish`,
`deliver`, `payload`). Tetapi **tidak ada cara menyatakan** "transisi X
memancarkan event Y" — keterkaitannya hanya tersirat dari penamaan
(dugaan saya: nama event = nama state), dan itu **belum pasti**.

**Kenapa tidak cukup.** Integrasi stok & jurnal bergantung penuh pada ini. Kalau
keterkaitannya tidak dinyatakan, tidak ada yang bisa memverifikasi bahwa
`order.paid` benar-benar terpancar saat transisi ke `paid` — termasuk
`formspec validate`.

**Konstruk yang dibutuhkan:**

```yaml
transitions:
  - from: awaiting_payment
    to: paid
    via: confirm-payment
    emit: on_paid # ← keterkaitan eksplisit
```

### S14 — `summary` tanpa kontrak pemelihara

> **✅ Selesai 2026-09-15 (TODO 1.8).** `maintained_by` menyebut script
> pemelihara (validator menolak referensi yang tak bisa di-resolve/dikompilasi,
> dicek bersama `impl.ref`), dan `invariants: [{unique, message}]` menyatakan
> invarian yang **wajib** ditopang unique index yang benar-benar dideklarasikan
> — jadi yang menegakkan adalah database, bukan disiplin script. Efek
> sampingnya penting: memasang `hooks:` di entity `summary` (yang tidak pernah
> dipanggil) tidak lagi bisa terlihat sebagai perlindungan, karena invariannya
> harus punya penopang nyata. Tiga summary kafe kini menyatakan invariannya.
> **Sisa:** `hooks`/`conditions` pada `summary` tetap tidak dipanggil — item
> **4.2**. Normatif: `docs/spec/backend/02-core-extended.md` §6.1.

**Tersedia sekarang.** `characteristic: summary` menonaktifkan create/update/delete
permanen. Tetapi tidak ada cara menyatakan **siapa** yang memeliharanya.

**Kenapa tidak cukup.** Konsekuensi yang saya temukan langsung: karena penulisan
`summary` tidak lewat action pipeline, **`hooks:` dan `conditions:` pada
`summary` tidak pernah dipanggil**. Jadi invarian seperti "satu baris per
(cabang, bahan)" tidak bisa dijaga dari manifest — hanya dari dalam script.

Lebih buruk: memasang hook-nya membuat manifest **terlihat** terlindungi padahal
tidak.

**Konstruk yang dibutuhkan:**

```yaml
spec:
  characteristic: summary
  maintained_by: cafe-stock/scripts/stock_level_apply # deklarasi + bisa divalidasi
  invariants:
    - {
        unique: [branch_id, ingredient_id],
        message: "satu saldo per cabang/bahan",
      }
```

### S15 — Tampilan tugas approval

**Tersedia sekarang** (terverifikasi):

```json
"WorkflowStep": { "properties": { "approvers", "escalation", "mode", "roles", "when" } }
```

Tidak ada `title`, tidak ada `description` (ditulis → `schema: /spec/steps/0: validation failed`).
Bandingkan `WizardStep` yang **punya** `title` + `description` — jadi ini
inkonsistensi antar-kind, bukan keterbatasan yang disengaja.

**Kenapa tidak cukup.** `ApprovalInbox` bersifat zero-config: sumbernya langkah
workflow yang menunggu. Karena langkah tidak punya label, tidak ada apa pun yang
bisa ditampilkan sebagai deskripsi tugas — dan tidak ada tempat mendeklarasikan
**field pendukung** (nomor pesanan, total, alasan void) agar approver bisa
memutuskan tanpa membuka record sendiri.

**Konstruk yang dibutuhkan:**

```yaml
steps:
  - title: "Persetujuan Void Pesanan"
    description: "Periksa nomor pesanan dan alasan sebelum menyetujui."
    display_fields: [number, total_amount, void_reason]
    roles: [cafe-order.supervisor]
```

### S16 — Kolom: dua konsep dengan kemampuan berbeda

> **🔴 Masih terbuka (2026-09-18).** `ReportColumn` = `{Field, Label, Aggregate,
> Format}` (`pkg/spec/frontend.go:590`) — **tidak ada** `widget`, dan
> `Aggregate`/`Format` masih string bebas (bukan enum). Item **7.2**.

Terverifikasi:

| Properti                             | `TableColumn` | `ReportColumn` |
| ------------------------------------ | ------------- | -------------- |
| `field`, `label`                     | ✅            | ✅             |
| `format`, `aggregate`                | `format` ✅   | ✅             |
| `widget`                             | ✅            | ❌ **ditolak** |
| `sortable`, `width`, `align`, `link` | ✅            | ❌             |

Keduanya `additionalProperties: false`. Nama sama, kemampuan berbeda, dan tidak
ada halaman referensi yang menaruh keduanya berdampingan — sehingga penulis spec
wajar menyalin bentuk satu ke yang lain (saya melakukannya, dan 6 report gagal
validasi sekaligus).

---

## §C. Yang sudah cukup — jangan diubah

Penting dicatat supaya tidak "memperbaiki" yang sudah benar:

| Kemampuan                                          | Bukti cukup                                                                                 |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| **State machine + action + guard**                 | 8 status `order`, transisi multi-asal, `conditions`, custom action — semua tervalidasi      |
| **`child` (jsonb) untuk baris dokumen**            | Baris pesanan & baris resep; `sequence_field`; field `relation` di dalam child              |
| **Relasi + denormalisasi finansial (`snapshot`)**  | Justru mewajibkan praktik yang benar (harga dibekukan)                                      |
| **`natural_key_rule` + `scope_field`**             | Nomor per cabang tanpa duplikasi                                                            |
| **`soft_deactivate`**                              | Field `is_active` di-inject otomatis + action deactivate/reactivate (terverifikasi runtime) |
| **`Config` `keys` + `settings`**                   | Namespace presentasi global (currency/locale/timezone/rounding)                             |
| **Realtime Table/Kanban/Dashboard**                | KDS berbasis Kanban siap pakai                                                              |
| **`kind: Migration`**                              | Escape hatch DDL; menyelamatkan tiga aturan keunikan                                        |
| **`kind: Workflow` + `ApprovalInbox` zero-config** | Multi-approver, quorum, mode, eskalasi                                                      |
| **Aturan simetri cancel Integrator (7.7.2)**       | **Aturan bagus** — memaksa jalur pembalik dipikirkan sejak awal                             |
| **`kind: Wizard` multi-langkah**                   | Tutup shift 3 langkah                                                                       |
| **`kind: Print` html/pdf**                         | Struk digital                                                                               |
| **System fields**                                  | `doc_status`, `version`, `created_at/by`, soft delete, audit                                |
| **`kind: Subscription` Tier 1/2**                  | Outbox → streaming                                                                          |

---

## §D. Semantik yang belum ditetapkan spec

Ini bukan "konstruk kurang", tapi "spec belum menjawab". Penting karena penulis
spec **tidak bisa menebak** dan validasi tetap hijau.

| #      | Pertanyaan                                                               | Kenapa penting                                                                                                                          | Yang diamati            |
| ------ | ------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| **D1** | Apakah `create` **selalu** menghasilkan `doc_status: draft`?             | Record `draft` **tidak bisa direferensikan** (diverifikasi: _"relation target is draft (must be submitted or lifecycle-free)"_)         | Ya — `create` → `draft` |
| **D2** | Bedanya `lifecycle: plain_crud` dengan tidak mendeklarasikan lifecycle?  | Keduanya tampak memakai `doc_status`; kalau sama, opsi itu tidak berguna                                                                | Keduanya `draft`        |
| **D3** | Apakah transisi state machine memancarkan event otomatis?                | Seluruh integrasi stok & jurnal bergantung padanya                                                                                      | Belum diketahui         |
| **D4** | Konvensi nama event: nama **state** atau prefix `on_*`?                  | Dokumen vertical pakai `billing.order.paid`; validator mewajibkan `on_*`; dan untuk konteks integritas yang dikenali justru `on_cancel` | Kontradiktif            |
| **D5** | Permission = `module.**entity**.action` atau `module.**plural**.action`? | Engine menghasilkan `cafe-master.menu-category.update` (**singular**); contoh dokumen memakai plural (`orders.checkout`)                | Kontradiktif            |
| **D6** | Apakah `submit` harus butuh permission `update`?                         | Menentukan siapa yang boleh menyelesaikan record                                                                                        | Ya (403 tanpa `update`) |
| **D7** | Di mana `settings` hidup & siapa membacanya?                             | Field `money` tidak bisa dipakai tanpa mata uang; tapi `settings` tidak disebut di dokumen tipe field                                   | Config level App        |

**D1 + D6 paling berdampak:** bersama-sama mereka berarti data yang dibuat lewat
permukaan publik tidak akan pernah bisa direferensikan — dan itu belum
dinyatakan di mana pun.

> **✅ Semuanya dijawab 2026-09-15.** Jawabannya diambil dari kode
> (`decisions-needed.md`, Fase 0) lalu **ditulis normatif** di
> `docs/spec/backend/01-core-basic.md` — D1/D2 di §1.2, D3/D4 di §7, D5/D6 di
> §8.6, D7 di §11. Ringkas: `create` → `draft` kecuali entity lifecycle-free
> (dan `lifecycle:` adalah hint UI, bukan penentu); transisi **tidak**
> memancarkan event otomatis; nama state polos bukan konvensi; permission
> `{module}.{plural}.{action}` dengan `submit` ber-permission sendiri; `settings`
> hidup di Config level App dan dibaca framework (mata uang `money`; ragu =
> tolak, bukan tebak).

---

## §E. Bukan gap spec (perbaikan di project FormSpec)

Dikeluarkan dari fokus dokumen ini, dicatat supaya tidak hilang:

| Kategori                 | Gap                                                                                                                                                          |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Belum diimplementasi** | Print `thermal`/`dotmatrix` (handler hanya PDF); `indexes:` tidak dihasilkan; cross-app grant; SyncAgent→router                                              |
| **Perilaku runtime**     | `money` tidak divalidasi/dinormalisasi (3 bentuk diterima); `settings.currency` tidak diterapkan                                                             |
| **Validator**            | Referensi menggantung tidak ditangkap (`App.spec.modules`, `view:`, `impl.ref`); shorthand `render: drawer` ditolak walau deskripsi schema menyebut diterima |
| **Dokumentasi/skill**    | `Config.spec.data` (bentuk salah); daftar widget di `03-kind-renderers.md` kedaluwarsa; `realtime.md` kedaluwarsa; aturan 7.7.2 tidak terdokumentasi         |

---

## §F. Urutan perbaikan spec yang saya sarankan

Berdasarkan "berapa banyak aplikasi yang terbuka":

| Prioritas | Item                                                                                                                                                                                 | Membuka apa                                                         |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------- |
| **1**     | **S2** filter bernilai dari sesi/route ✅                                                                                                                                            | Multi-outlet **dan** akses pelanggan — satu konstruk, dua kebutuhan |
| **2**     | **S3** akses publik per-entity + `exclude` ditegakkan ✅                                                                                                                             | Aplikasi publik yang aman                                           |
| **3**     | **S7** semantik `money` ✅                                                                                                                                                           | Semua aplikasi transaksional: POS, kas, laporan                     |
| **4**     | **S10** kosakata `widget` jadi enum ✅                                                                                                                                                  | Menghentikan kelas bug "salah ketik == fitur belum ada"             |
| **5**     | **S1** blok/kind transaksional ✅                                                                                                                                                       | Pemesanan mandiri pelanggan                                         |
| **6**     | **S8** unique parsial + index relasi ✅                                                                                                                                                 | Integritas data tanpa keluar ke DDL mentah                          |
| **7**     | **S9** workflow atas nama transisi ✅                                                                                                                                                   | Approval yang tidak bisa dilewati                                   |
| **8**     | **D1–D7** tetapkan semantik ✅                                                                                                                                                       | Menghilangkan tebakan yang berujung data rusak                      |
| **9**     | **S5** scope cabang deklaratif ✅ (konstruk; penyalaan = 3.5 ✅)                                                                                                                        | Multi-cabang yang dijamin, bukan didisiplinkan                      |
| **10**    | **S6** pemetaan Integrator 🔴, **S11** pajak 🟡 (tipe `percent`), **S12** satuan ✅ (konversi = 4.6), **S13** event↔transisi 🔴, **S14** summary ✅, **S15** label approval 🔴, **S16** kolom 🔴 | Melengkapi                                                          |

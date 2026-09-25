# Field Types & Validation

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Katalog tipe di sini normatif untuk
> semua `kind: Entity`; storage-agnostic — layout fisik tiap tipe adalah
> urusan PersistBackend
> ([`../../renderers/jsonb-persist/02-schema-strategies.md`](../../renderers/jsonb-persist/02-schema-strategies.md)).

Field hanya valid di `kind: Entity` (§9 anatomi dokumen). Nama field reserved
([`01-core-basic.md`](01-core-basic.md) §1.2) tidak boleh dipakai ulang. Tiap
field mendeklarasikan `type` dari katalog tertutup di §1.

## 1. Katalog Tipe

Tipe adalah **himpunan tertutup** — `formspec apply` menolak `type` di luar daftar
ini. Tiap tipe punya representasi kanonik yang identik di seluruh PersistBackend
dan protokol; perbedaan hasil antar backend adalah non-konformansi.

### 1.1 Tipe Primitif

| Type       | Domain                            | Representasi kanonik           | Catatan                                                                                                                                                      |
| ---------- | --------------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `string`   | Teks pendek satu baris            | UTF-8                          | Kandidat generated column/index; batasi panjang via rule `max_length`                                                                                        |
| `text`     | Teks panjang multi-baris          | UTF-8                          | Tidak diindeks penuh secara default; untuk deskripsi, catatan                                                                                                |
| `richtext` | Teks kaya (markup)                | HTML/Markdown ter-**sanitasi** | Backend/engine **wajib** menyanitasi di server sebelum simpan — payload dari klien tidak pernah dipercaya mentah ([`01-core-basic.md`](01-core-basic.md) §3) |
| `integer`  | Bilangan bulat 64-bit             | int64                          | Nama kanonik `integer` (bukan `int`)                                                                                                                         |
| `decimal`  | Bilangan eksak presisi-arbitrer   | string desimal                 | **Wajib** untuk kuantitas numerik non-uang yang butuh eksak; **tidak pernah** float. Semantik `precision`/`scale` — §1.2                                     |
| `percent`  | Persentase                        | string desimal (0–100)         | Secara numerik = `decimal`; bedanya rendering/format (`10` berarti 10%, bukan 0.1) — §1.5                                                                    |
| `money`    | Uang = jumlah + mata uang         | `{ amount, currency }`         | Tipe first-class — §2                                                                                                                                        |
| `boolean`  | `true`/`false`                    | bool                           | —                                                                                                                                                            |
| `date`     | Tanggal kalender                  | `YYYY-MM-DD` (ISO-8601)        | Tanpa zona waktu                                                                                                                                             |
| `datetime` | Titik waktu                       | ISO-8601 dengan offset zona    | Disimpan UTC, disajikan per `settings.timezone`                                                                                                              |
| `time`     | Waktu-hari                        | `HH:MM:SS` (ISO-8601)          | Tanpa tanggal/zona                                                                                                                                           |
| `uuid`     | Identitas 128-bit                 | UUID v7 (time-ordered)         | Default untuk PK ([`01-core-basic.md`](01-core-basic.md) §2)                                                                                                 |
| `enum`     | Satu nilai dari himpunan tertutup | string                         | **Wajib** `enum_values: [...]`; nilai di luar himpunan → `VALIDATION_ERROR`                                                                                  |
| `json`     | Struktur bebas                    | JSON                           | Tanpa skema yang ditegakkan framework; hindari untuk data yang butuh query/validasi                                                                          |
| `file`     | Referensi objek tersimpan         | pointer object storage         | Isi hidup di `ctx.storage`, bukan di baris — §1.3. `attachment` adalah alias `file`                                                                          |

Format tanggal/waktu, zona, dan default lain yang lintas-komponen dibaca dari
**global settings** (`settings.*`, [`01-core-basic.md`](01-core-basic.md) §10) —
komponen tidak pernah menebak.

### 1.1.1 `options` — himpunan pilihan berlabel (multi-nilai)

`enum_values` membawa **nilai saja**. Untuk field yang nilainya **daftar**
(`json` menyimpan array, `string` menyimpan daftar dipisah koma), nilai telanjang
tidak bisa dibaca: `[1, 2]` tidak pernah terbaca sebagai "Senin, Selasa".

```yaml
- name: days_of_week
  type: json
  title: "Hari Berlaku"
  options:
    - { value: 1, label: "Senin" }
    - { value: 2, label: "Selasa" }
    - { value: 7, label: "Minggu" }
```

- `options` adalah properti **data** (nilai mana yang sah) — ditaruh di Entity,
  sama alasannya dengan `enum_values`. Satu deklarasi dipakai ulang oleh form
  (widget `select-multi-tag`, [`../frontend/07-component-kinds.md`](../frontend/07-component-kinds.md) §1.3),
  sel tabel, dan halaman detail.
- `value` mempertahankan **tipe skalar** yang benar-benar disimpan
  (`value: 1` → angka `1`, bukan `"1"`), supaya array `json` tetap array angka.
- `label` opsional; tanpa `label`, nilai di-humanise jadi caption
  (`in_progress` → "In Progress").
- Hanya sah pada field yang **memegang himpunan** (`json`, `string`). Pada
  field skalar (`integer`, `date`, …) `options` adalah klaim yang tidak bisa
  dieksekusi apa pun; pada `enum` pesan errornya mengarahkan ke `enum_values`.

Aturan yang ditegakkan saat validasi (bukan konvensi): `value` wajib ada dan
skalar, dan **tidak boleh duplikat** (termasuk lintas ejaan — `1` dan `"1"`
adalah pilihan yang sama) sebab pilihan duplikat tidak akan pernah bisa dipilih
dua kali, yang terbaca sebagai widget rusak.

### 1.2 `decimal` — Precision & Scale

`decimal` menyimpan angka **eksak**, bukan floating-point biner. Dua atribut
opsional membentuk shape-nya:

```yaml
- { name: quantity, type: decimal, precision: 12, scale: 3 }
```

- `precision` — total digit signifikan (kiri + kanan koma).
- `scale` — digit di belakang koma.

Aturan normatif:

- Tanpa `precision`/`scale`, backend menyimpan nilai secara eksak apa adanya
  (arbitrary-precision) — **tidak ada** pembulatan diam-diam.
- Dengan `scale` diset, nilai dengan digit pecahan melebihi `scale` dibulatkan
  memakai **rounding mode global** (§2, banker's rounding default).
- Nilai yang melampaui `precision` adalah `VALIDATION_ERROR` (422) — bukan
  dibulatkan diam-diam.
- `decimal` **tidak** membawa mata uang. Untuk uang, pakai `money` (§2) —
  jangan menyandingkan `decimal` + field `currency` manual, karena itu membuat
  tiap komponen menebak skala dan pembulatannya sendiri.

### 1.3 `file` / `attachment`

Field `file` menyimpan **pointer** ke objek di `ctx.storage`, bukan byte-nya di
baris Entity. Metadata kanonik yang disajikan: `key`, `filename`,
`content_type`, `size`, `checksum`. Upload/download lewat jalur `ctx.storage`
yang tunduk `uses` dan isolasi tenant; file ikut ter-backup
([`04-persist-backend.md`](04-persist-backend.md) §3). Byte mentah tidak pernah
disimpan di luar `ctx.storage` (larangan storage tak terkelola,
[`04-persist-backend.md`](04-persist-backend.md) §5).

**Storage Spec.** Sebuah field `file`/`attachment` mendeklarasikan batasan dan
kebijakan aksesnya lewat blok `storage`:

```yaml
- name: photo
  type: file
  storage:
    allowed_types: [image/png, image/jpeg, .pdf] # allowlist MIME/ekstensi
    max_size_mb: 10
    max_count: 5 # hanya untuk field multi-file
    visibility: private # public | private | signed
    signed_url_ttl: 15m # TTL URL akses terbatas-waktu (visibility: signed)
    one_time: false # true = objek dihapus setelah link didownload
    ttl: 168h # objek dihapus bila tidak didownload dalam durasi ini
    max_download_mb: 50 # batas ukuran download (efektif = min global, field)
    cdn: true # opsional — passthrough CDN untuk objek public
    transform: # opsional — turunan resize/thumbnail image
      - { name: thumb, width: 200, height: 200, fit: cover }
```

Aturan normatif:

- `allowed_types` — allowlist MIME type / ekstensi. Ditegakkan **server-side**
  pada upload; tipe di luar allowlist → `VALIDATION_ERROR` (422). Deteksi tipe
  memakai content sniffing, bukan sekadar percaya `Content-Type`/ekstensi klien.
- `max_size_mb` / `max_count` — batas ukuran per objek dan jumlah objek (khusus
  field multi-file). Ditegakkan server-side; pelanggaran → `VALIDATION_ERROR`.
- `visibility` — `public` (objek dapat diakses lewat URL publik/CDN),
  `private` (hanya lewat jalur `ctx.storage` ber-auth), `signed` (diakses lewat
  signed URL berdurasi terbatas). Default `private`.
- `signed_url_ttl` — masa berlaku signed URL untuk `visibility: signed`
  (durasi, mis. `15m`, `24h`). Dibaca dari default global bila tidak diset
  ([`01-core-basic.md`](01-core-basic.md) §10) — komponen tidak menebak.
- `one_time` — link download berlaku **satu kali**: setelah objek berhasil
  didownload via link, objek dihapus dari storage dan link berikutnya → `410
GONE`. Penegakan atomic di server (budget download), bukan konvensi klien.
- `ttl` — objek yang **tidak pernah didownload** dalam durasi ini dihapus
  otomatis oleh sweeper (format durasi Go, mis. `168h`).
- `max_download_mb` — batas ukuran objek yang boleh dilayani via route
  download/link. Limit efektif = min(limit global `FORMSPEC_DOWNLOAD_MAX_MB`,
  nilai per-field); pelanggaran → `413 FILE_TOO_LARGE` **sebelum** objek
  dimuat (cek ukuran via stat, todo 7.17.7).
- `cdn` — passthrough CDN untuk objek `public`; detail delivery adalah urusan
  primitif storage ([`../platform/06-datastore.md`](../platform/06-datastore.md) §2).
- `transform` — spesifikasi turunan image (resize/thumbnail). Turunan
  dibangkitkan server-side saat upload dan diperlakukan sebagai objek tersimpan
  yang sama disiplin isolasi/backup-nya dengan objek asli.

**Konvensi route upload.** Upload ke field file spesifik pada record yang sudah
ada memakai `POST /:resource/:id/{field}` (di bawah workspace prefix,
[`01-core-basic.md`](01-core-basic.md) §8.5). Route ini menerima byte, menerapkan
`allowed_types`/`max_size_mb`/`max_count`, menyimpan ke `ctx.storage`, lalu
menautkan pointer ke field. Widget frontend yang mengonsumsi konvensi ini adalah
`fileinput` ([`../frontend/07-component-kinds.md`](../frontend/07-component-kinds.md) §1.1).

**`allowed_types` — bentuk kanonik: ekstensi tanpa titik** (gap #4b). Nilainya
`[jpg, png, webp]`, bukan `[".jpg"]` dan bukan `["image/jpeg"]`. Tiga bentuk lain
**diterima** dan diperlakukan sama oleh matcher klien maupun server, supaya spec
yang sudah ada tetap jalan:

| Bentuk             | Contoh       | Arti                       |
| ------------------ | ------------ | -------------------------- |
| ekstensi (kanonik) | `jpg`        | ekstensi file, tanpa titik |
| ekstensi bertitik  | `.jpg`       | sama, ejaan lama           |
| MIME type          | `image/jpeg` | tipe konten persis         |
| MIME wildcard      | `image/*`    | seluruh keluarga tipe      |

Alasan bentuk kanonik ditegaskan: bentuk yang didokumentasikan (`[jpg]`) dulu
**tidak cocok dengan apa pun** — ia bukan `.jpg`, bukan MIME, bukan wildcard —
sehingga upload yang seharusnya sah ditolak dengan "File type not allowed".
`formspec validate` kini menolak entri di luar keempat bentuk itu (mis. `JPG`,
`*.jpg`, `"jpg, png"`), jadi salah bentuk tertangkap saat validasi, bukan saat
pengguna memilih file.

**Banyak file** dinyatakan dengan `max_count > 1`, bukan tipe field tersendiri:
nilainya menjadi array objek key, dan `max_count: 1` (default) berarti satu key
string. Renderer yang menampilkan gambar memakai `widget: image` —
[`../frontend/07-component-kinds.md`](../frontend/07-component-kinds.md) §1.

### 1.4 Tipe Struktural

`relation` dan `child` bukan tipe data biasa — keduanya memodelkan hubungan
antar-dokumen dan didefinisikan penuh di [`01-core-basic.md`](01-core-basic.md)
§1.3 (garis pembeda = kepemilikan lifecycle; `relation.on_delete`;
`child.storage`; `child.sequence_field` untuk line-ordering eksplisit). Katalog
ini tidak mengulanginya; §3 menambahkan satu marker normatif (`tree`) di atas
`relation` self-referential.

### 1.5 `percent` — Persentase (S11)

`percent` menandai field sebagai **persentase**, bukan rasio. Nilainya adalah
angka persentase itu sendiri (`10` = 10%), disimpan sebagai `decimal` — jadi
presisi, `scale`, pembulatan, dan aturan validasi numeriknya identik dengan
§1.2. Yang berbeda hanya **interpretasi**: tanpa tipe ini, `10` tidak bisa
dibedakan dari "10×" dan setiap komponen menebak sendiri apakah harus
mengalikan dengan 100 saat menampilkan atau menghitung.

```yaml
- { name: tax_percent, type: percent, scale: 2, default: 10 }
```

Aturan:

- Field `percent` **tidak** menyimpan mata uang dan **tidak** menggantikan
  `money` untuk nominal.
- Agregasi numerik (`sum`, `avg`, `min`, `max`) berlaku seperti `decimal`.
- Renderer menampilkan satuan `%`; format kanoniknya `format: percent`
  ([`../frontend/07-component-kinds.md`](../frontend/07-component-kinds.md)).
- Pajak/service charge **belum** punya model tersendiri: tarif dinyatakan
  sebagai field `percent`, sedangkan dasar pengenaan, apakah harga termasuk
  pajak, dan pelaporannya masih tanggung jawab spec aplikasi.

### 1.6 `unit` — Dimensi Satuan (S12)

Field yang **namanya adalah nama satuan** (biasanya `enum`) boleh
declare dimensi satuannya:

```yaml
- name: unit
  type: enum
  enum_values: [gram, kg, ml, pcs]
  unit: { base: gram, convertible: [kg], factors: { kg: 1000 } }
```

- `base` — satuan **penyimpanan**. Nilai field ini yang menjadi acuan.
- `convertible` — satuan lain yang boleh dikonversi ke/dari `base`.
- `factors` — berapa `base` yang setara dengan **satu** satuan itu
  (`{kg: 1000}` = 1 kg = 1000 gram). Setiap entri `convertible` **wajib** punya
  faktor; tanpa itu deklarasinya hanya mengatakan "satuan ini berhubungan"
  tanpa mengatakan **bagaimana** — persis konvensi-di-dalam-script yang ingin
  dihapus deklarasi ini.
- Keduanya **wajib** ada di `enum_values` (kalau field-nya `enum`): konversi ke
  satuan yang tidak bisa dinyatakan field itu adalah salah ketik, bukan fitur.
- Satuan di luar grup `base`+`convertible` (mis. `ml`, `pcs` di atas) berarti
  "dimensi lain" — sengaja tidak dikonversi, bukan error.

Deklarasinya ada supaya satuan menjadi **data**, bukan konvensi yang hidup di
dalam script: tanpa ini, setiap aplikasi memaksa satu satuan dasar dan
melakukan konversi di Starlark, sehingga hasil (mis. HPP) bergantung pada
disiplin penulis script, bukan pada data. Konversi dihitung engine lewat
`ctx.unit.convert(entity, field, value, from, to)` — konversi melewati `base`
(`value × factor(from) ÷ factor(to)`), dan satuan di luar grup **error**, bukan
`0`.

## 2. Money (Normatif)

`money` adalah tipe first-class: pasangan **jumlah eksak** (`decimal` dengan
skala tetap per mata uang) dan **kode mata uang** (ISO-4217, mis. `IDR`, `USD`).
Uang bukan "decimal biasa dengan asumsi tersembunyi" — memodelkannya sebagai
tipe tersendiri memaksa jumlah dan mata uang selalu terbawa bersama, hilang satu
tidak mungkin.

```yaml
# Warisi mata uang default dari global settings
- { name: total, type: money }

# Kunci mata uang eksplisit di field ini — non-default WAJIB sertakan decimal_places (§2 di bawah)
- { name: fee, type: money, currency: USD, decimal_places: 2 }
```

**Sumber mata uang — jangan pernah menebak** ([`01-core-basic.md`](01-core-basic.md)
§10). Urutan resolusi:

1. `currency` eksplisit di deklarasi field, kalau ada.
2. Kalau tidak, `settings.currency` global.
3. Kalau keduanya tidak tersedia dan tidak ada default standar → **error**,
   bukan tebakan. Komponen/renderer/backend dilarang menyimpulkan mata uang
   dari heuristik (mis. "field bernama `price` pasti currency default").

**Skala tetap per mata uang — dideklarasikan, bukan diturunkan dari katalog.**
Core **tidak** menyertakan tabel metadata mata uang (bukan ISO-4217 built-in) —
katalog mata uang (kode, nama, simbol, kalau app butuh daftar/dropdown) adalah
**Entity bisnis biasa** yang dimodelkan app developer sendiri kalau perlu,
tidak berbeda dari Entity lain, dan tidak diistimewakan framework. Yang wajib
dideklarasikan eksplisit (bukan ditebak, bukan di-lookup) adalah skala minor-unit
tiap mata uang yang benar-benar dipakai:

```yaml
# Global — mata uang default workspace beserta skalanya
settings:
  currency:
    code: IDR
    decimal_places: 0
    symbol: "Rp"          # opsional, hanya untuk format tampilan

# Field yang override ke mata uang lain WAJIB ikut menyertakan skalanya
- { name: fee, type: money, currency: USD, decimal_places: 2, symbol: "$" }
```

Field `money` yang meng-override `currency` ke kode selain
`settings.currency.code` **wajib** menyertakan `decimal_places` sendiri di
deklarasi field — tidak ada katalog untuk di-lookup, jadi tidak mendeklarasikan
skala pada mata uang non-default adalah `VALIDATION_ERROR` saat `formspec apply`,
bukan tebakan diam-diam (mis. asumsi "2 digit untuk semua mata uang").

**Rounding normatif.** Default **banker's rounding** (round-half-to-even) — dipilih
karena netral secara statistik pada agregasi finansial. Bisa di-override di satu
tempat lewat `settings.rounding` global (mis. `half_up`); komponen tidak pernah
memilih mode pembulatan sendiri. Setiap operasi yang menghasilkan digit di luar
skala mata uang dibulatkan dengan mode ini.

**Format tampilan** (simbol, posisi, pemisah ribuan) mengikuti `settings.locale`

- `symbol` yang dideklarasikan di `settings.currency`/field itu sendiri — bukan
  hard-code per komponen, dan bukan hasil query ke Entity katalog (kalau app
  punya Entity katalog mata uang, itu murni data tampilan/pilihan-user, di luar
  jalur baca tipe `money`).

**Open — FX & multi-currency.** Konversi antar mata uang (rate table, tanggal
rate, revaluasi, gain/loss selisih kurs) **di luar scope core**. Ia menjadi
domain modul resmi `formspec/currency` (rate table + operasi konversi), yang belum
ditetapkan kontraknya. Yang masuk core sekarang: kebenaran **single-currency** —
penyimpanan jumlah+mata uang, pembulatan, dan format. Menjumlahkan dua `money`
dengan kode mata uang berbeda tanpa konversi eksplisit adalah error, bukan
operasi diam-diam.

### 2.1 Aritmetika & agregasi `money` (Normatif)

Nilai `money` di wire dan di penyimpanan selalu objek `{amount, currency}`.
Operasi atasnya **tidak** butuh sintaks pembungkus: nilai `money` adalah operand
biasa. Operand diklasifikasikan menjadi **money** atau **skalar** (number,
string numerik, boolean).

| Ekspresi                   | Hasil   | Syarat                                             |
| -------------------------- | ------- | -------------------------------------------------- |
| `m + m`, `m - m`           | `money` | mata uang sama; berbeda → error                    |
| `m * n`, `n * m`, `m / n`  | `money` | `n` skalar; pembagi 0 → error                      |
| `m1 / m2`                  | number  | rasio (mis. margin); mata uang sama                |
| `-m`                       | `money` |                                                    |
| `m1 <op> m2` (`< <= > >=`) | boolean | mata uang sama                                     |
| `m1 == m2`, `m1 != m2`     | boolean | kesetaraan **nilai** (deep), bukan identitas objek |
| `amount(m)`                | number  | ekstraksi eksplisit komponen jumlah                |
| `currency(m)`              | string  | kode ISO-4217                                      |
| `sum([m…])`                | `money` | semua elemen money & securrency                    |

**Tidak ada koersi diam-diam.** Operand yang bukan money dan bukan skalar —
objek non-money, list, atau string non-numerik — **error evaluasi**, bukan `0`.
Operan `money` yang digabung dengan angka mentah juga error (angkanya tidak punya
satuan): untuk membandingkan atau mencampurnya dengan skalar, ekstrak dulu
komponennya (`amount(m)`), supaya perbedaan satuan terlihat di spec, bukan di
laporan keuangan.

**Presisi.** Aritmetika `money` eksak (desimal), bukan floating point: hasilnya
dirender pada skala operand tanpa nol tak bermakna di belakang koma
(`2 × 25000` → `50000`, `0.1 + 0.2` → `0.3`, `25000 * 0.5` → `12500`). Nol
belakang sengaja dibuang supaya hasil yang eksak tidak "mengaku" punya presisi
yang lebih tinggi daripada field-nya: `12500.0` akan ditolak field IDR
(`decimal_places: 0`) padahal nilainya tepat. Hasil yang melebihi
`decimal_places` field tetap ditolak saat validasi nilai (§3), bukan dipotong
diam-diam.

**Agregasi.** `sum`/`avg`/`min`/`max` atas field `money`
(`columns[].aggregate`, `totals[].fn`, widget `config.aggregate`) mengagregasi
**komponen `.amount`**-nya. Field yang bukan numerik dan bukan money ditolak:
`formspec check` melaporkannya sebagai error statis, dan runtime menolaknya
sebagai error — bukan total `0` yang terlihat meyakinkan. `count` bebas (boleh
atas field apa pun, boleh tanpa field).

Aturan yang sama berlaku di semua tempat ekspresi dievaluasi — guard/when
Starlark server ([`../runtimes/`](../../runtimes/)), `computed` field, dan
FormSpecExpr di renderer ([`../frontend/08-formspec-expr.md`](../frontend/08-formspec-expr.md) §5).

### 2.2 Kolom turunan `money` (Normatif)

Field `money` yang di-`index: true` (atau `unique`/`natural_key`) mendapat
**kolom turunan numerik yang membaca `.amount`**, bukan teks dari objeknya:

```sql
-- SQLite
_price numeric(20,8) GENERATED ALWAYS AS (CAST(json_extract(data, '$.price.amount') AS REAL)) STORED
-- PostgreSQL
_price numeric(20,8) GENERATED ALWAYS AS (data->'price'->>'amount') STORED
```

Alasannya persis seperti operasi lain atas `money` (§2.1): bandingkan nilainya,
jangan teks JSON-nya. Kolom turunan yang menyimpan objek membuat `9000` dianggap
**lebih besar** dari `10000` (perbandingan leksikografis), sehingga sortir harga,
filter rentang, dan laporan "margin tertinggi" salah **tanpa gejala apa pun** —
dan index di atasnya hanya mempercepat jawaban yang salah.

Konsekuensi yang perlu diketahui penulis spec:

- Urutan/rentang/sortir atas `money` kini numerik **di kedua driver**, dan
  `columnRefExpr` memakai kolom turunan itu ketika ada (kalau tidak, index-nya
  tidak akan terpakai).
- Nilai kanonik tetap objek `{amount, currency}` di payload; yang dinumerikkan
  hanya **proyeksi** untuk sortir/agregasi. Pada SQLite cast-nya `REAL`, jadi
  presisi eksak tetap milik payload — jangan jadikan kolom turunan sebagai
  sumber kebenaran nilai uang.
- `money` tanpa index tetap dilayani ekspresi `.amount` saat query (agregasi §2.1
  dan sortir memakai jalur yang sama), jadi kebenarannya tidak bergantung pada
  ada-tidaknya kolom turunan.

## 3. Validasi Field

`rules` di sebuah field adalah **himpunan tertutup** kosakata di bawah. Rule
dievaluasi **server-side, selalu** — pada setiap jalur masuk (HTTP, script,
event) sebelum handler action berjalan (level "Field",
[`01-core-basic.md`](01-core-basic.md) §3). Duplikasi rule di frontend murni
untuk UX; ia tidak pernah menjadi satu-satunya penjaga — payload yang melewati
frontend tetap tertahan di backend.

| Kategori     | Rule                               | Arti                                                                                                                                           |
| ------------ | ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Presence     | `required`                         | Nilai wajib ada (non-null, non-empty)                                                                                                          |
|              | `optional`                         | Eksplisit boleh kosong (default)                                                                                                               |
| String       | `min_length` / `max_length`        | Batas panjang                                                                                                                                  |
|              | `length`                           | Panjang persis                                                                                                                                 |
|              | `pattern`                          | Cocok regex                                                                                                                                    |
|              | `email`                            | Format email                                                                                                                                   |
|              | `url`                              | Format URL                                                                                                                                     |
| Numeric      | `min` / `max`                      | Batas nilai                                                                                                                                    |
|              | `positive`                         | > 0                                                                                                                                            |
|              | `precision`                        | Batas digit signifikan (untuk `decimal`, §1.2)                                                                                                 |
| Enum/set     | `in`                               | Nilai termasuk himpunan yang diberikan                                                                                                         |
| Date         | `future` / `past`                  | Relatif terhadap `ctx.now()`                                                                                                                   |
|              | `after:<field>` / `before:<field>` | Relatif terhadap field lain di dokumen yang sama                                                                                               |
| Collection   | `min_items` / `max_items`          | Batas jumlah elemen (child/list)                                                                                                               |
| Cross-record | `unique`                           | Unik per tenant ([`01-core-basic.md`](01-core-basic.md) §2 — unique constraint, bukan PK)                                                      |
|              | `exists:<resource>`                | Nilai wajib menunjuk record yang ada                                                                                                           |
| Escape hatch | `script` + `message`               | Starlark inline untuk aturan di luar kosakata; pesan pakai format `code`+`params` bernamespace App ([`01-core-basic.md`](01-core-basic.md) §9) |

Kegagalan validasi mengembalikan envelope error normatif
([`01-core-basic.md`](01-core-basic.md) §8.5) dengan `details: [{level, field?,
message}]`.

## 4. Tree / Hierarki

Banyak data bisnis berbentuk pohon: chart of accounts, unit organisasi, kategori
produk, bill-of-materials. **Model penyimpanannya adalah `relation`
self-referential** — sebuah field `relation` yang menunjuk ke dokumen yang sama
(mis. `parent_id` di `account` menunjuk `account`). Ini bukan tipe baru; ia
`relation` biasa ([`01-core-basic.md`](01-core-basic.md) §1.3) yang targetnya
dokumen sendiri.

Di atas itu, satu marker normatif mengaktifkan dukungan hierarki:

```yaml
- name: parent_id
  type: relation
  relation: { type: belongs_to, resource: account } # self-referential
  tree: true
```

`tree: true` **hanya** valid pada `relation` self-referential (`resource` = nama
dokumen sendiri); `formspec apply` menolaknya di tempat lain. Marker ini
mewajibkan PersistBackend menyediakan **query hierarki** — bukan sekadar lookup
parent satu tingkat.

**Operator query hierarki (aditif).** Kontrak operator filter dasar tertutup di
[`01-core-basic.md`](01-core-basic.md) §6 (`eq`, `in`, `between`, dst.). Field
ber-`tree: true` menambah operator berikut **khusus untuk field itu** — bukan
memperluas himpunan dasar untuk field lain:

| Operator        | Arti                                 | Contoh                                  |
| --------------- | ------------------------------------ | --------------------------------------- |
| `descendant_of` | Semua turunan (rekursif) dari id     | `filter[parent_id][descendant_of]=<id>` |
| `ancestor_of`   | Semua leluhur (rekursif) dari id     | `filter[parent_id][ancestor_of]=<id>`   |
| `child_of`      | Anak langsung (satu tingkat) dari id | `filter[parent_id][child_of]=<id>`      |
| `root`          | Node akar (parent null)              | `filter[parent_id][root]=true`          |

Semantiknya **identik di seluruh PersistBackend** — cara backend memenuhinya
(recursive CTE, closure table, materialized path, dll.) adalah detail
implementasi ([`04-persist-backend.md`](04-persist-backend.md) §2, kemampuan
"Query resolution"). Kontraknya cuma: hasil traversal ancestor/descendant benar
dan konsisten.

**Integritas tree.** Siklus (node menjadi leluhur dirinya sendiri) ditolak
server-side pada `create`/`update`. `relation.on_delete`
([`01-core-basic.md`](01-core-basic.md) §1.3) tetap berlaku pada edge parent —
mis. `restrict` mencegah penghapusan node yang masih punya anak.

## 5. Keamanan Field & Computed Field

Sampai §4 sebuah field diperlakukan sebagai data yang klien tulis dan baca bebas
dalam batas `rules`. Bagian ini menambahkan marker **per-field** untuk dua hal
di luar itu: nilai yang diturunkan server (§5.1) dan kerahasiaan/governance
(§5.2–§5.4). Semua marker di bagian ini ditegakkan **server-side, selalu** —
sejalan dengan level "Field" ([`01-core-basic.md`](01-core-basic.md) §3); klien
tidak pernah menjadi penjaganya.

### 5.1 Computed field

```yaml
- name: line_total
  type: money
  computed: { formula: "quantity * unit_price" }
```

`computed` menandai field yang nilainya **diturunkan server-side**, bukan ditulis
klien. Marker ini menegakkan:

- **Never client-writable** — nilai `computed` di payload klien diabaikan (bukan
  error diam-diam yang menimpa hasil hitung); satu-satunya sumber nilai adalah
  evaluasi formula.
- **Recomputed on save** — formula dievaluasi ulang pada tiap `create`/`update`
  sebelum persist, sehingga nilai tersimpan selalu konsisten dengan input
  terkini. Ia bukan default sekali-tulis.

`formula` adalah **ekspresi Starlark inline** yang dievaluasi server-side
terhadap data dokumen yang sama (context = field record, dibaca via nama field
atau `data["field"]`) — bukan named `script_ref`. Ia boleh membaca field lain di
dokumen yang sama; ia tidak boleh menghasilkan efek samping di luar nilai yang
dikembalikan. Untuk logika multi-pernyataan yang lebih besar, gunakan custom
action `impl: script_ref` (escape hatch validasi
[`01-core-basic.md`](01-core-basic.md) §9) yang menulis hasilnya ke field, atau
kerjakan via hook.

### 5.2 Enkripsi & Masking at Rest

```yaml
- name: national_id
  type: string
  encrypted: true
  masked: true
```

- `encrypted: true` — field **dienkripsi at rest**; hanya didekripsi untuk
  pembacaan yang terotorisasi. Penyimpanan ciphertext dan manajemen kunci adalah
  tanggung jawab PersistBackend/primitif storage; kontraknya: nilai plaintext
  tidak pernah tersimpan mentah di baris maupun index.
- `masked: true` — nilai **diobscure di respons API dan log** kecuali pemanggil
  punya permission elevated; representasi ter-mask menampilkan bentuk parsial
  yang aman (mis. hanya 4 digit terakhir). Masking berlaku di **semua** surface
  keluaran secara default — termasuk log terstruktur, sejalan disiplin PII
  ([`../platform/09-observability.md`](../platform/09-observability.md) §2.2).

`encrypted` dan `masked` ortogonal: `encrypted` melindungi data at rest,
`masked` melindungi data saat disajikan. Sebuah field boleh salah satu, keduanya,
atau tidak sama sekali.

### 5.3 Field-Level Permission & Surface Exclusion

`required_permission` pada action ([`01-core-basic.md`](01-core-basic.md) §5)
adalah guard bagi si pemanggil untuk memanggil action. Field bisa memasang
guard **lebih halus** di atasnya:

```yaml
- name: salary
  type: money
  required_permission: hr.view_salary
  exclude: [public_api, audit_log, webhook]
```

- `required_permission` (level field) — **berbeda** dari yang di level action.
  Pemanggil boleh diizinkan memanggil `update`/`read`, tapi tetap tidak boleh
  melihat atau menyetel field sensitif ini tanpa permission tambahan. Tanpa
  permission: field di-strip dari respons (read) dan penyetelannya di payload
  ditolak (write).
- `exclude` — daftar surface keluaran tempat field ini **dihilangkan** meski ada
  secara internal. Nilai: `public_api` (respons permukaan external `/api/v1/` —
  lihat [`01-core-basic.md`](01-core-basic.md) §8.2), `audit_log`
  (entri audit bisnis, [`02-core-extended.md`](02-core-extended.md) §11),
  `webhook` (payload webhook keluar, [`02-core-extended.md`](02-core-extended.md)
  §4), `ui` (form/list hasil derivasi Layer 0 dan Menu App — field tetap ada di
  permukaan `public_api`/`/_ui/entity/` mentah, hanya tidak ditampilkan render
  visual; dipakai untuk field internal/computed/API-only, mis. field tambahan
  dari Entity Extension yang memang tidak boleh pernah terlihat — lihat
  [`03-entity-extension.md`](03-entity-extension.md) §5). Field tetap ada dan
  dapat dipakai logika internal/script; ia hanya tidak bocor ke surface yang
  disebut.

### 5.4 Classification

```yaml
- name: email
  type: string
  classification: pii # pii | financial | internal
```

`classification` adalah **label governance** — bukan penjaga akses, melainkan tag
untuk pelaporan dan disiplin data. Nilai kanonik minimal: `pii`, `financial`,
`internal`. Label ini menjadi dasar:

- pelaporan governance (inventarisasi field ber-PII/finansial per App/module);
- disiplin PII observability — field ber-`classification: pii` tidak pernah
  masuk log mentah ([`../platform/09-observability.md`](../platform/09-observability.md)
  §2.2), berpasangan dengan `masked`/`exclude: [audit_log]` di atas;
- keterkaitan audit trail bisnis ([`02-core-extended.md`](02-core-extended.md)
  §11) — perubahan pada field terklasifikasi dapat diperlakukan dengan retensi
  atau disiplin redaksi yang berbeda.

`classification` bersifat deskriptif dan aditif; ia melengkapi — bukan
menggantikan — `encrypted`/`masked`/`required_permission`/`exclude` yang
menegakkan perlindungan aktual.

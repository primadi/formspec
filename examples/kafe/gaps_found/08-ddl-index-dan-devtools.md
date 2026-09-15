# Gap #22, #23, #24 — DDL, Indeks, dan Dev Tooling

Ditemukan lewat **verifikasi runtime** (bukan pembacaan kode): menjalankan
`formspec validate`, `formspec check`, dan `formspec migrate plan` pada spec
kafe yang sudah ditulis.

Semua bukti di bawah adalah keluaran nyata CLI pada spec ini.

---

## Ringkasan

| # | Temuan | Dampak |
| --- | --- | --- |
| **22** | `indexes:` (IndexDecl) **tidak menghasilkan index apa pun**; `index: true` pada field `relation` juga tidak menghasilkan kolom | **HIGH** — dua aturan bisnis tidak ditegakkan DB |
| **23** | Kolom turunan untuk `money` bertipe `text` → urutan/rentang atas uang **leksikografis**, bukan numerik | **HIGH** — sortir & laporan berbasis uang tidak andal |
| **24** | `formspec dev` **menolak start di Windows** meski port bebas; `--force` tidak menembus | **HIGH (DX)** — verifikasi runtime mustahil lewat dev server |

---

## Gap #22 — `indexes:` dan indeks relasi tidak jalan ✅ Pasti

### Bukti 1 — IndexDecl diabaikan

`spec/modules/cafe-master/master/menu-item-price/entity.yaml` mendeklarasikan:

```yaml
indexes:
  - fields: [branch_id, menu_item_id]
    unique: true
```

`formspec migrate plan --spec spec` menghasilkan DDL untuk
`cafe_master_menu_item_prices` **tanpa satu pun index komposit**. Daftar index
yang benar-benar dibuat (seluruh spec, 22 tabel) hanya berisi:

- `idx_uq_<tabel>_<field>` — dari `unique: true` / `natural_key: true` pada field
- `idx_<tabel>_<field>` — dari `index: true` pada field

Tidak ada satu pun index dari `indexes:`. Pencarian `CREATE .*INDEX` atas
seluruh DDL tidak menemukan `idx_uq_cafe_master_menu_item_prices_*`.

### Bukti 2 — field `relation` tidak bisa diindeks

Hipotesis pertama: mungkin IndexDecl butuh field-nya ditandai `index: true`
dulu. Diuji dengan menambahkan `index: true` pada `branch_id` **dan**
`menu_item_id`. Hasil: **tetap tidak ada kolom turunan** dan tetap tidak ada
index — tabelnya masih hanya berisi kolom framework + `data`.

Bukti pemungkas: pencarian `_branch_id` atas seluruh DDL → **nol hasil**.
Bandingkan dengan field skalar yang memang dapat kolom turunan:

```
_code       text        GENERATED ALWAYS AS (json_extract(data, '$.code'))       STORED
_name       text        GENERATED ALWAYS AS (json_extract(data, '$.name'))       STORED
_status     varchar(50) GENERATED ALWAYS AS (json_extract(data, '$.status'))     STORED
_transaction_date timestamptz GENERATED ALWAYS AS (json_extract(data,'$.transaction_date')) STORED
_qr_token   text        GENERATED ALWAYS AS (json_extract(data, '$.qr_token'))   STORED
```

Tidak ada padanan untuk `relation`. Jadi **field relasi tidak menghasilkan kolom
turunan**, sehingga tidak bisa diindeks — dan index komposit atas relasi
mustahil dinyatakan.

### Dampak ke aplikasi kafe: **HIGH**

Dua aturan bisnis yang **bergantung pada keunikan komposit atas relasi** tidak
ditegakkan di database:

| Aturan | Deklarasi di spec | Kenyataan di DB |
| --- | --- | --- |
| Satu harga per menu per cabang (D1) | `menu-item-price` — `indexes: [branch_id, menu_item_id] unique` | **Tidak ada unique index** |
| Satu baris stok per (cabang, bahan) (D3) | `stock-level` — `indexes: [branch_id, ingredient_id] unique` | **Tidak ada unique index** |

Konsekuensinya: dua baris harga untuk menu yang sama di cabang yang sama **bisa
masuk**, dan `stock-level` bisa terduplikasi. Keduanya merusak perhitungan HPP
dan tampilan harga secara diam-diam — bukan error, tapi angka salah.

`stock-level` dan `menu-cost` adalah entity `summary` yang hanya ditulis script,
jadi duplikasi di sana bisa memaksa script melakukan upsert manual (cari dulu,
baru insert/update) — race-prone tanpa `ctx.lock`.

### Usulan

1. **Hormati `IndexDecl`** untuk field skalar (`fields` yang sudah punya kolom
   turunan) — minimal ini menutup index komposit biasa.
2. **Dukung indeks atas field `relation`** (buat kolom turunan untuk relation,
   mis. `_branch_id`). Ini yang membuka keunikan komposit berbasis relasi.
3. **`formspec validate` harus MENOLAK `indexes:` yang tidak bisa diwujudkan** —
   jangan diam-diam mengabaikan. Menyatakan constraint yang tidak ada lebih
   buruk daripada tidak menyatakannya, karena penulis spec akan mengira dirinya
   sudah terlindungi.

---

## Gap #23 — Kolom turunan `money` bertipe `text` ✅ Pasti

### Bukti

Diuji dengan menambahkan `index: true` pada field `price` (type `money`).
DDL yang dihasilkan:

```sql
_price text GENERATED ALWAYS AS (json_extract(data, '$.price')) STORED
CREATE INDEX idx_cafe_master_menu_item_prices_price ON cafe_master_menu_item_prices (_price);
```

Tipe `text` berasal dari `renderers/jsonb-persist/ddl.go` → `fieldTypeToSQL()`
yang **tidak punya `case spec.FieldMoney`**, sehingga jatuh ke
`default: return "text"`.

Karena `money` adalah objek (`{amount, currency}`), `json_extract` atasnya
menghasilkan **teks JSON**, mis. `{"amount":"25000","currency":"IDR"}`.

### Dampak ke aplikasi kafe: **HIGH**

- **Sortir harga** di daftar menu membandingkan string, bukan angka:
  `"9000"` dianggap **lebih besar** dari `"10000"` (perbandingan leksikografis).
- **Filter rentang** (`harga > X`) tidak dapat diandalkan.
- Laporan "menu dengan margin tertinggi" yang mengandalkan urutan di DB akan
  salah tanpa gejala.
- Penyebabnya tidak muncul di `formspec validate` (hijau) maupun `formspec check`
  (0 error, 0 warning) — konsisten dengan pola gagal senyap yang lain.

### Koreksi terhadap Gap #2 (penting)

Sebelumnya (Gap #2) saya menduga `money` disimpan di kolom `text` sehingga
bentuknya bermasalah. **Verifikasi DDL membantah separuh dugaan itu:**

- ✅ **Storage aman.** `price` hidup di dalam kolom JSONB `data`, bukan kolom
  terpisah. JSONB memang bisa memuat objek — tidak ada masalah penyimpanan.
- ❌ **Indeks/urutan bermasalah.** Begitu field ditandai `index: true`, kolom
  turunannya `text` — di sinilah masalahnya.

Jadi masalahnya **bukan "money disimpan salah"**, tapi **"money tidak bisa
diurutkan/dibandingkan secara numerik"**. Sisa Gap #2 yang masih relevan hanya
sisi renderer (apakah `renderCellValue` menangani objek) — dan itu perlu UI,
bukan DDL.

### Usulan

1. Tambah `case spec.FieldMoney` di `fieldTypeToSQL()` → kolom turunan numerik
   (`numeric(20,8)` untuk PostgreSQL), dengan nilai diambil dari
   `json_extract(data, '$.price.amount')`.
2. Sebagai langkah minimal: **tolak `index: true` pada field `money`** sampai
   pemetaan tipe benar — lebih baik gagal keras daripada memberi index yang
   menyesatkan.

---

## Gap #24 — `formspec dev` tidak bisa start di Windows ⛔ DIBATALKAN

> **DIBATALKAN setelah verifikasi runtime (2026-09-14).** Klaim di bawah
> **salah**. `formspec dev` berjalan normal — yang terjadi hanyalah port yang
> saya pilih sedang terpakai, dan saya menyimpulkan "bug" dari satu perintah
> `Get-NetTCPConnection` yang mengembalikan kosong. Nol hasil dari satu
> pemeriksaan bukan bukti port bebas; cara yang benar adalah **mencoba bind** ke
> port itu, atau sekadar mencoba port lain.
>
> Di port 18100 (diverifikasi bisa di-bind) server langsung naik:
> `engine loaded: 155 routes`, `SPA embedded — open http://localhost:18100/default/_admin`,
> `REST API on :18100`, plus empat worker (outbox, workflow-escalation,
> subscription-stream, subscription-dynamic).
>
> **Yang tetap berlaku — severity turun ke LOW:** pesan errornya tidak
> menuntun. *"cannot identify the owner"* tidak menyebut "coba port lain",
> tidak menyertakan flag, dan tidak menyebutkan port mana yang bebas.
>
> **Pelajaran metodologis:** ini klaim "bug framework" ketiga yang gugur
> setelah diuji (lihat juga Gap #2 storage dan Gap #18 `target:`). Ketiganya
> berasal dari inferensi atas bukti yang terlalu tipis. Hasil verifikasi
> runtime lengkap ada di `12-hasil-verifikasi-runtime.md`.

### Bukti

Semua upaya start gagal dengan pesan yang sama:

```
[formspec] using config: formspec-app.yaml
port 18080 is in use but cannot identify the owner: cannot determine owner of port 18080
```

Padahal port itu **bebas**:

```
Get-NetTCPConnection -LocalPort 18080  → (kosong)
netstat -ano | Select-String ":18080"  → (kosong)
```

Yang menarik: saat mencoba port `18099`, port itu memang terpakai — tetapi oleh
**VS Code sendiri** (`Code.exe`, PID 19704), bukan proses lain yang relevan.
Jadi pesan errornya benar untuk kasus itu, dan **salah** untuk 18080.

`--force` (`Force kill previous instance on same ports`) **tidak menembus**
penolakan ini — pesannya identik.

Pola yang terbaca: deteksi pemilik port gagal di Windows (kemungkinan butuh
`netstat`/API yang tidak tersedia atau butuh elevasi), lalu gagal-ke-aman dengan
menganggap port terpakai. Pesannya sendiri mengaku: *"cannot determine owner"* —
artinya "saya tidak bisa membuktikan ini bebas", bukan "ini terpakai".

### Dampak ke aplikasi kafe: **HIGH** untuk DX, BLOCKER untuk verifikasi

Seluruh verifikasi runtime lewat dev server — termasuk pengujian yang justru
paling dibutuhkan (apakah `money` dirender benar, apakah gambar tampil, apakah
Kanban KDS live-update) — **tidak bisa dijalankan di Windows**.

Yang **masih bisa** dipakai sebagai gantinya (terverifikasi jalan):

| Perintah | Hasil pada spec ini |
| --- | --- |
| `formspec validate --spec spec` | 29 manifest, 0 problem |
| `formspec check -f spec` | 0 error, 0 warning |
| `formspec migrate plan --spec spec` | 22 tabel + index (lihat Gap #22/#23) |

Jadi verifikasi bisa dilakukan sampai lapisan **DDL & statik**, tapi tidak
sampai lapisan **HTTP & UI**.

### Usulan

1. Perbaiki deteksi pemilik port (atau turunkan jadi peringatan, bukan
   penolakan) ketika pemilik tidak bisa ditentukan.
2. Sediakan flag eksplisit seperti `--ignore-port-check` yang benar-benar
   memaksa. `--force` saat ini hanya menangani "kill instance sebelumnya",
   bukan "lewati pengecekan".
3. Sertakan nama proses pemilik di pesan error bila berhasil dideteksi — pada
   kasus 18099 itu langsung menjelaskan masalahnya (VS Code), sedangkan pada
   18080 pesannya buntu.

---

## Yang Terverifikasi **Bagus** (jangan diabaikan)

Verifikasi runtime juga mengonfirmasi banyak hal berjalan benar:

| Aspek | Bukti |
| --- | --- |
| DDL tergenerate untuk seluruh 22 entity | 22 `CREATE TABLE` |
| Enum ditegakkan di DB | `CHECK (json_extract(data, '$.prep_station') IN ('bar','kitchen','both'))` |
| `unique` / `natural_key` jalan | `idx_uq_..._code`, `_phone`, `_qr_token`, `_number`, `_username` |
| `index: true` pada field skalar jalan | `idx_cafe_order_orders_status`, `_transaction_date`, dll. |
| Composite `unique` pada field skalar (per-field) jalan | `(tenant_id, _code) WHERE deleted_at IS NULL` |
| Hanya `tenant_id` yang auto-inject | Setiap tabel punya `tenant_id`, **tidak ada** `branch_id` — bukti langsung Gap #8 |
| Strategi JSONB hybrid konsisten | Bisnis data di `data`; kolom turunan hanya untuk yang diindeks |
| `formspec check` bersih | 0 error, 0 warning — termasuk untuk `computed` aritmetika money (`tendered - amount`) |

---

## Gap #27 — Pemetaan tipe SQL tidak driver-aware (tipe Postgres bocor ke SQLite) ✅ Pasti

Ditemukan di DDL yang sama, satu baris di bawah temuan Gap #22.

### Bukti

`fieldTypeToSQL()` (`renderers/jsonb-persist/ddl.go`) **tidak menerima parameter
`driver`**:

```go
// fieldTypeToSQL maps a FormSpec FieldType to SQL type.
// params: enumValues for FieldEnum
func fieldTypeToSQL(ft spec.FieldType, _ []string) string {
```

Sementara `GenerateEntityDDL(meta, entity, driver)` memang menerima driver dan
memakai `dialectFor(driver)` untuk hal lain. Akibatnya, saat dijalankan dengan
driver **SQLite**, DDL yang dihasilkan memuat tipe khas PostgreSQL:

```sql
_transaction_date timestamptz GENERATED ALWAYS AS (json_extract(data,'$.transaction_date')) STORED
_status           varchar(50)  GENERATED ALWAYS AS (json_extract(data,'$.status')) STORED
```

(`timestamptz`, `varchar(50)`, `numeric(20,8)` semuanya tipe PostgreSQL.)

### Dampak ke aplikasi kafe: **LOW–MEDIUM**

SQLite bertipe dinamis dan menerima nama tipe apa pun (dipetakan ke *type
affinity*), jadi DDL-nya **tetap jalan** — `formspec migrate plan` dan
`migrate apply` tidak gagal. Yang dirugikan:

- **Kebingungan pembaca.** DDL untuk SQLite yang menyebut `timestamptz`
  membuat developer mengira ada pemetaan tipe yang benar, padahal tidak.
- **Risiko diam.** Begitu ada backend ketiga (MySQL, dsb.) yang lebih ketat,
  tipe yang salah baru terasa — setelah pola ini tertanam.
- Bandingkan SQLite: `date` dipetakan `date`, `boolean` → `boolean` (lihat
  `_is_below_min`), sementara `datetime` → `timestamptz`. Tidak konsisten.

### Usulan

- Berikan `driver` ke `fieldTypeToSQL()` (atau pindahkan pemetaan ke
dialect/dialect-per-driver), lalu pakai tipe yang benar per driver:
  SQLite `text`/`integer`/`real`/`numeric`; PostgreSQL `timestamptz`,
  `varchar`, `numeric`.
- Tambahkan tes DDL yang memeriksa **tidak ada tipe PostgreSQL** muncul di
  keluaran driver SQLite — murah, dan langsung menangkap regresi.

---

## ✅ UPDATE GAP #22 — ada jalan keluar yang bekerja: `kind: Migration`

Saat menulis `kind: Migration` untuk aturan bisnis #10, ternyata **GAP-22 bisa
 ditutup hari ini tanpa menunggu engine diperbaiki.**

`MigrationSpec` (`schemas/v1/kinds/Migration.schema.json`) menerima DDL mentah:

```json
"MigrationSpec": {
  "properties": {
    "ddl":    { "type": "string" },
    "module": { "type": "string" }
  },
  "required": ["ddl"],
  "additionalProperties": false
}
```

Karena itu index komposit atas field `relation` — yang tidak bisa dihasilkan
`IndexDecl` — dapat dibuat sebagai **expression index atas JSONB**:

```sql
CREATE UNIQUE INDEX idx_uq_cafe_master_menu_item_prices_branch_menu
ON cafe_master_menu_item_prices (
  tenant_id,
  json_extract(data, '$.branch_id'),
  json_extract(data, '$.menu_item_id')
)
WHERE deleted_at IS NULL
```

### Terverifikasi berjalan

```
$ formspec migrate plan  --spec spec
24 pending migration(s):
  CREATE UNIQUE INDEX idx_uq_cafe_master_menu_item_prices_branch_menu ON ...
  CREATE UNIQUE INDEX idx_uq_cafe_order_shifts_open ON ...
  CREATE UNIQUE INDEX idx_uq_cafe_stock_stock_levels_branch_ingredient ON ...

$ formspec migrate apply --spec spec --dsn sqlite:.formspec/gap22-test.db
Applied 24 structural + 3 custom migration(s).
```

Ketiga migration juga lolos `formspec validate` (50 manifest, 0 problem).

### Tiga keunikan yang sekarang punya penutup

| Aturan | Migration | Termasuk partial? |
| --- | --- | --- |
| Satu harga per menu per cabang (D1) | `menu-item-price-unique` | — |
| Satu baris stok per (cabang, bahan) (D3) | `stock-level-unique` | — |
| Satu shift terbuka per (cabang, kasir) (#10) | `shift-open-unique` | ✅ `WHERE status = 'open'` |

> **Konsekuensi untuk guard script:** ketiganya berpindah status menjadi
> **lapis kedua**, bukan pengaman utama. Constraint database lebih kuat karena
> berlaku untuk **semua** jalur tulis (API, script, seed, operator), sementara
> guard hanya berlaku pada jalur yang melewatinya. Rencana: hapus guard setelah
> migration terbukti terpasang di semua environment.

> **Yang belum terverifikasi:** DDL-nya di-*apply* tanpa error, tapi apakah
> constraint benar-benar **menolak** baris duplikat saat runtime belum diuji —
> butuh jalur tulis. Uji termurah: `kind: Seed` dengan dua baris duplikat, karena
> `formspec seed` menulis lewat EntityStore yang sama tanpa butuh HTTP.

---

## Gap #35 — `MigrationSpec.ddl` hanya satu string: DDL tidak bisa portabel ✅ Terverifikasi

### Bukti

`MigrationSpec` punya **satu** field `ddl` bertipe string — tidak ada peta
per-driver. Padahal ekspresi untuk membaca JSONB berbeda antar driver:

| Driver | Ekspresi untuk `data.branch_id` |
| --- | --- |
| SQLite (dev) | `json_extract(data, '$.branch_id')` |
| PostgreSQL (produksi) | `(data ->> 'branch_id')` |

Jadi DDL di atas **benar untuk dev dan salah untuk produksi**. Ini bukan kasus
tepi: setiap migration yang menyentuh kolom `data` (yang berarti **semua**
migration index/constraint di strategi JSONB hybrid) menghadapi masalah sama.

### Dampak ke aplikasi kafe: **HIGH**

Aplikasi ini secara eksplisit menargetkan **SQLite dev + PostgreSQL produksi**
(`overview.md`). Artinya:

- Migration yang bekerja di dev belum tentu bekerja di produksi — dan
  kegagalannya baru muncul **saat deploy**, bukan saat pengembangan.
- Tidak ada cara menyatakan kedua varian dalam satu spec. Pilihannya: satu
  migration per driver (bagaimana memilihnya?), atau DDL yang kebetulan sama di
  keduanya (sangat terbatas).

### Usulan

1. Dukung varian per-driver, mis. `ddl: {default: "...", postgres: "...", sqlite: "..."}`.
2. Atau sediakan ekspresi portabel yang diterjemahkan engine (mis. helper
   `json_get('branch_id')` yang dikompilasi ke sintaks driver masing-masing).
3. Minimal: `formspec validate` **memperingatkan** bila `ddl` memuat fungsi
   khas driver (`json_extract`/`->>`) sehingga penulis sadar tidak portabel.

---

## Gap #36 — Hanya DDL yang diizinkan: migration tidak bisa merapikan data ✅ Terverifikasi

### Bukti

Deskripsi `MigrationSpec` di schema:

> "MigrationSpec defines a custom DDL migration (01-core-basic.md §4).
> **Only DDL statements are allowed — DML is rejected at runtime.**"

### Dampak ke aplikasi kafe: **HIGH**

Konsekuensinya praktis dan tajam: **`CREATE UNIQUE INDEX` akan GAGAL bila tabel
sudah punya baris duplikat** — dan justru itulah kondisi yang terjadi di
lapangan sebelum constraint-nya ada.

Skenario nyata:

1. Kafe berjalan sebulan **tanpa** `menu-item-price-unique`, karena GAP-22
   membuat `indexes:` tidak terpasang.
2. Operator menambahkan harga kedua untuk menu yang sama di cabang yang sama —
   **tidak ada yang mencegah**.
3. Migration di-deploy → `CREATE UNIQUE INDEX` gagal.
4. Diperlukan `UPDATE`/`DELETE` untuk merapikan duplikat — yaitu **DML, yang
ditolak** oleh migration.

Jadi perbaikan data harus dilakukan di luar migration (SQL manual oleh operator),
dan tidak ada jejaknya di spec. Untuk audit, itu lubang.

### Usulan

1. Tambah langkah **data repair** yang eksplisit dan terdeklarasi, mis.
   `pre_ddl_queries` atau `kind: Migration` dengan blok `repair:` yang boleh DML
   **hanya** pada tabel yang sama dan **dicatat di audit log**.
2. Atau minimal: `formspec migrate plan` **memeriksa** apakah masih ada duplikat
   dan melaporkannya sebagai masalah sebelum `apply` — supaya deploy tidak gagal
   setengah jalan.
3. Dokumentasikan prosedur resmi "menambahkan unique index pada tabel yang sudah
   berisi data" — termasuk cara merapikan duplikat.

# Plan — `total_amount` sebagai `computed` (kafe 10.65)

Menutup **kafe 10.65** (major): `order.total_amount` tidak punya penulis, sehingga
jurnal GL, laporan penjualan, struk, dan kolom Total kosong untuk setiap pesanan
yang lahir lewat aplikasi.

## Keputusan yang sudah diambil

Pemilik memilih **opsi (a)**: turunkan `total_amount` sebagai `computed` di entity
`order`.

## Temuan pembatas (dibaca dari kode, bukan diasumsikan)

1. **`computed` tidak bisa membaca entity lain.** `hydrateAndCompute`
   (`renderers/jsonb-persist/crud.go:731`) memanggil `evaluateComputed`
   **SEBELUM** `resolveRelations`, dan `evaluateComputed` menyusun env dari field
   record sendiri + `backdate_limit_days`. Jadi `branch.tax_percent` **tidak
   tersedia** saat formula berjalan. → Persentase harus **masuk ke record** lewat
   mekanisme yang sudah ada: `relation.snapshot` (denormalisasi finansial, todo
   7.10).
2. **Formula tidak bisa menyebut field yang absen.** Env hanya memuat key yang
   ADA; identifier yang tak terdefinisi = error compile → `evaluateComputed`
   menelan error itu (`continue`) dan field hasilnya **absen tanpa sinyal**.
   Field opsional (`discount_amount`, `manual_discount_amount`,
   `points_value`) justru absen secara normal. → Perlu jalan baca-aman.
3. **Aritmetika `money` menolak campuran.** `money ± number` = error (S7 /
   05-field-types.md §2.1: "tidak ada koersi diam-diam"), jadi cabang "tidak
   ada diskon" tidak boleh menghasilkan angka `0` polos.
4. **`computed` dievaluasi pada SEMUA jalur baca** — `GetByID`
   (`hydrateAndCompute`), `List` (`crud.go:1966`), `FindByField`,
   `FindByFields`. Report & widget dashboard meng-agregasi **di klien**
   (`renderers/react-shadcn/src/lib/aggregate.ts`), jadi nilai turunan ikut
   terhitung. Event `on_paid` dibangun dari `execParams.Resource` =
   `current.Data` hasil `GetByID` + body → **ikut membawa nilai turunan**,
   sehingga `gl/journalize` menerima `total_amount`.

## Perubahan

### 1. Engine — `resource` di env `computed` (kecil)

`renderers/jsonb-persist/crud.go` `evaluateComputed`: suntikkan
`resource` (dan `data`) sebagai `starlark.NewFieldMap(data)` — **persis yang
sudah dilakukan `EvaluateGuard`** (`internal/starlark/guard.go:117`).

Alasan: `FieldMap.Attr` mengembalikan `None` untuk field yang tidak ada
(`internal/starlark/fieldmap.go:36`), sehingga formula bisa **menguji
keberadaan** operand opsional. Tanpa ini, formula yang menyebut field opsional
gagal senyap — kelas "absen tanpa sinyal" yang sudah menelan beberapa bug di repo
ini.

### 2. Engine — builtin `money_zero(x)` (kecil)

`internal/starlark/money.go` `moneyBuiltins()`: tambah `money_zero(value)` →
money bernilai 0 dengan mata uang milik `value`. Melengkapi `amount()`/
`currency()` yang sudah ada (bisa membongkar money, belum bisa membangunnya).

Alasan: cabang kondisional butuh "nol money", dan alternatifnya
(`subtotal * 0`) adalah idiom yang tidak terbaca di manifest. `money_zero`
menjaga aturan "mata uang tidak pernah ditebak" — nol diambil dari mata uang
operand, bukan hardcode `IDR`.

### 3. Manifest — `cafe-order` order entity (sedang)

- `branch_id` relation dapat `snapshot:` untuk `tax_percent`,
  `service_charge_percent`, `apply_service_charge`.
- Tiga field snapshot itu **dideklarasikan** di entity (wajib: snapshot menulis
  field, dan `validateKnownFields` menolak field yang tidak dideklarasikan).
- `computed` untuk rantai nilai:
  - `service_charge_amount` = `subtotal × service_charge_percent / 100` bila
    `apply_service_charge`, selain itu `money_zero`.
  - `tax_amount` = `(subtotal + service_charge_amount) × tax_percent / 100`.
  - `total_amount` = `subtotal + service_charge_amount + tax_amount − Σ
    (diskon opsional)`.
- Komentar lama yang menyatakan `total_amount` "BELUM diturunkan" dihapus
  (menjadi salah begitu perubahan ini mendarat).

### 4. ⚠️ Perubahan angka akuntansi yang harus disetujui pemilik

Formula di komentar entity berbunyi
`tax_percent × (subtotal + service charge)`, sedangkan verifikasi lama
(`o2c_e2e_test.go`, catatan 9.4 skenario 8) memakai **`tax_percent × subtotal`**
(143750 = 125000 + 12500 + 6250). Angka lama itu **disuplai tangan oleh test**
(test menulis `total_amount`/`tax_amount` sendiri), jadi ia tidak mengunci aturan
apa pun — tetapi ia satu-satunya angka yang pernah "terverifikasi".

Rencana: ikuti **komentar entity** (basis pajak = subtotal + service charge) dan
perbarui test, lalu **laporkan perubahan angkanya** (143750 → 144375) supaya
pemilik bisa membalik satu baris bila maksudnya basis = subtotal.

### 5. Test

- Unit `internal/starlark`: `money_zero` (currency diambil dari operand; nol
  money terlibat aritmetika `money ± money`).
- Unit `renderers/jsonb-persist`: formula yang membaca field opsional lewat
  `resource.<field>` → hadir saat diisi, tidak gagal saat absen.
- e2e kafe (`resource/`): pesanan lewat **payload form QR** (tanpa angka uang) →
  `total_amount` terisi, dan setelah `paid` **jurnal terbentuk** — inilah
  regresi yang 10.65 keluhkan. Test lama yang menyuplai angka tangan diselaraskan.

### 6. Sisa yang dicatat (bukan diklaim selesai)

- `total_amount` `index: true` → kolom turunan `_total_amount` **tetap NULL**
  karena computed tidak dipersist; `?sort=total_amount` (kolom "Total" di tabel
  POS, `sortable: true`) karena itu mengurutkan NULL. → item `⏸️` sendiri.

## Urutan & dependensi

1 → 2 → 3 → 5 → 6 (verifikasi dev server) → changelog.

## Referensi

- `docs/spec/backend/05-field-types.md` §2.1 (aritmetika money),
  §2.2 (kolom turunan money)
- `docs/spec/backend/02-core-extended.md` §1.1 (denormalisasi finansial/snapshot)
- kafe 10.65 (`examples/kafe/gaps_found/TODO.md`), changelog `2026-10-03-002`

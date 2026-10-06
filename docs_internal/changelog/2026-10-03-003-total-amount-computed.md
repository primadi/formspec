# 2026-10-03-003 — `total_amount` diturunkan sebagai `computed`; jurnal QR hidup kembali (kafe 10.65)

**Apa yang diubah.** Rantai nilai pesanan kini **diturunkan**, bukan menunggu
pemanggil yang (tidak pernah ada):

- `order.branch_id` mendapat **`snapshot:`** untuk `tax_percent`,
  `service_charge_percent`, `apply_service_charge` (denormalisasi finansial,
  02-core-extended.md §1.1), dan ketiganya dideklarasikan sebagai field pesanan.
- `service_charge_amount`, `tax_amount`, `total_amount` kini **`computed`**:
  `subtotal → service charge → pajak → total (dikurangi diskon opsional)`.

**Kenapa snapshot, bukan join.** `computed` **tidak bisa membaca entity lain**:
`hydrateAndCompute` (`renderers/jsonb-persist/crud.go`) memanggil
`evaluateComputed` **sebelum** `resolveRelations`, dan env-nya hanya berisi field
record itu sendiri + `backdate_limit_days`. Jadi `branch.tax_percent` mustahil
dijangkau formula — persentase harus **ada di record**, dan mekanisme resminya
adalah `relation.snapshot`. Efek samping yang memang diinginkan: pesanan lama
tidak berubah pajaknya bila tarif cabang diubah kemudian.

**Dua celah engine yang ikut ditutup (keduanya penyebab senyap).**

1. `evaluateComputed` (`renderers/jsonb-persist/crud.go`) kini menyuntikkan
   `resource`/`data` sebagai **`FieldMap`** ke env formula — persis yang sudah
   dilakukan `EvaluateGuard` (`internal/starlark/guard.go`). Tanpa ini, formula
   yang menyebut field **opsional** (`discount_amount`, `manual_discount_amount`,
   `points_value`) mustahil ditulis dengan aman: env memuat sebuah key HANYA bila
   field-nya terisi, jadi field yang absen adalah **identifier tak terdefinisi =
   error compile**, dan `evaluateComputed` menelannya (`continue`) sehingga field
   hasilnya **absen tanpa sinyal apa pun**. Itulah akar 10.65 — terlihat di test
   kalibrasi: `subtotal - discount_amount` menghasilkan *absent*, bukan error.
   `FieldMap` menjawab `None` untuk field yang tidak ada, sehingga
   `resource.discount_amount if resource.discount_amount else …` bisa ditulis.
2. Builtin **`money_zero(x)`** (`internal/starlark/money.go`) — money bernilai 0
   bermata uang milik operand. Aritmetika money menolak campuran `money ± number`
   (05-field-types.md §2.1 "tidak ada koersi diam-diam"), jadi cabang "tidak ada
   diskon"/"service charge tidak berlaku" tidak boleh jatuh ke literal `0`.
   Mata uang diambil dari operand (bukan `IDR` hardcode), mengikuti aturan
   `ResolveMoneyCurrency`.

**Perubahan angka yang perlu diketahui pemilik.** Basis pajak mengikuti komentar
field di entity: `tax_percent × (subtotal + service charge)`. Verifikasi lama
(`o2c_e2e_test.go`, catatan 9.4 skenario 8) memakai 143750 = 125000 + **12500** +
6250, yaitu pajak atas **subtotal** saja. Angka lama itu **disuplai tangan oleh
test** (test menulis `total_amount`/`tax_amount` sendiri) dan event-nya
di-enqueue langsung, jadi ia tidak pernah menguji derivasi dan **tidak berubah**
oleh perbaikan ini — nilai yang disuplai menang atas hasil `computed` pada
payload. Konsekuensinya: repo kini memuat **dua basis pajak** (test vs manifest)
dan itu harus diputuskan pemilik. Bila maksudnya pajak atas subtotal, satu baris
di formula `tax_amount` mengubahnya.

**Bukti.**

- Unit `internal/starlark/money_zero_test.go` — mata uang diambil dari operand
  (IDR/SGD), ikut aritmetika money saat operand opsional **absen**, menolak
  non-money, dan `FieldMap` mengembalikan `None` (bukan error) untuk field yang
  tidak ada. Termasuk penegasan **bahwa identifier telanjang tetap error** —
  itulah alasan `resource.` wajib.
- Unit `renderers/jsonb-persist/computed_optional_test.go` — melalui store nyata:
  formula dengan operand opsional **absen** → `total_amount = 131250`, dan
  **hadir** → `106250`; varian identifier telanjang tetap **absen** (mode gagal
  senyap dipin supaya alasannya terdokumentasi).
- e2e `resource/kafe_order_total_computed_e2e_test.go` — **bentuk payload form QR
  yang sebenarnya** (tanpa satu pun angka uang): `subtotal 45000`, `service 2250`
  (5%), `tax 4725` (10% dari 45000+2250), `total 51975`; lalu `paid` →
  **jurnal `posted`, debit = kredit = 51975**. Cabang tanpa service charge →
  `service 0`, `tax 4500`, `total 49500` (menguji cabang `money_zero`). Ditambah
  guard sumber yang menolak operand opsional ditulis sebagai identifier telanjang
  (dengan kalibrasi anti-vakum).
- **Guard terkalibrasi:** blok `computed` `total_amount` dihapus → test gagal
  `total_amount is absent — the exact 10.65 failure`; dipulihkan dari backup
  `/tmp` (bukan `git checkout`).
- **Verifikasi dev server** (binary + manifest di-restart): pesanan QR
  `ORD-2026-00004` → `subtotal 25000, service 1250, tax 2625, TOTAL 28875`;
  `paid` → outbox `on_paid` **completed** (0 retry) → jurnal
  `JRN-2026-000002` **posted** untuk `ORD-2026-00004`, 4 baris
  (Omzet 25000 · Pajak 2625 · Service 1250 · Kas 28875). Sebelumnya event yang
  sama berakhir `failed` setelah 6 retry dan **nol jurnal**.
- `go test ./...` hijau · `gofmt` bersih · `formspec validate` 89 manifest 0
  problem · `formspec check` 0 error 0 warning.

**Sisa yang dicatat (bukan diklaim selesai).**

- **Baris lama tidak di-backfill.** Dua pesanan yang dibuat SEBELUM field
  snapshot ada (`ORD-2026-00001`, `ORD-2026-00003`) tetap tanpa `tax_amount` dan
  `total_amount`, karena `tax_percent` tidak ada di record-nya. Perilaku ini
  mengikuti preseden 10.42 (field baru tidak di-backfill; record menyembuhkan diri
  saat ditulis ulang) dan saya **tidak** membuat formula menebak pajak `0` —
  angka salah yang senyap lebih buruk daripada nilai yang jelas-jelas kosong.
  → **10.69 ⏸️**.
- **`computed` dievaluasi saat baca, tidak dipersist.** Karena `total_amount`
  `index: true`, kolom turunan `_total_amount` tetap **NULL**, sehingga
  `?sort=total_amount` (kolom "Total" di tabel POS, `sortable: true`)
  mengurutkan NULL. → **10.68 ⏸️**.
- **Dua basis pajak** (test vs manifest) perlu keputusan pemilik — lihat di atas.

**Dampak.** `renderers/jsonb-persist/crud.go` · `internal/starlark/money.go` ·
`examples/kafe/spec/modules/cafe-order/transaction/order/entity.yaml` · test baru
`internal/starlark/money_zero_test.go`,
`renderers/jsonb-persist/computed_optional_test.go`,
`resource/kafe_order_total_computed_e2e_test.go` · plan
`docs_internal/plan/total-amount-computed.md` · ledger kafe (10.65 ✅, 10.68/10.69
⏸️).

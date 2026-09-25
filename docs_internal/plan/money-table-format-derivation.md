# Plan — kolom `money` di Table turunan tampil sebagai JSON

Sumber: laporan runtime pengguna,
`/kafe/app/pos/cafe-order/cash-movements` — kolom **Jumlah** menampilkan
`{"amount":"50000","currency":"IDR"}`.

## Masalah

Field `money` **tidak pernah** mendapat `format: currency` dari derivasi kolom
tabel, sehingga sel-nya jatuh ke cabang terakhir `renderCellValue`
(`JSON.stringify` untuk nilai objek). Kontraknya sudah ada: `docs/kind/ui/Table.md`
§contoh memakai `- { field: total, format: currency }`, dan tiga jalur lain
(`ReportRenderer`, `DashboardRenderer`, `DetailPage`) sudah memformat `money`
dengan benar. Yang bolong hanya jalur **kolom turunan**.

Bukti (dari bundle App `kafe-pos`): `tables: []` — `cafe-order.cash-movement`
tidak punya Table tertulis, jadi `TableRenderer` memakai `deriveTable(entity)`.
Di `engine/derive.ts`:

```
function tableFormat(field) {
  datetime → relative · date → date · percent → percent
  decimal + rules[min] → currency      // heuristik
  money → (tidak ada cabang)           // ← akar
}
```

`cellHintsForField()` di `lib/renderCell.tsx` **sudah** memetakan
`money → currency`; `derive.tableFormat()` tidak. Dua tabel kosakata yang sama
berbeda isi — itulah kenapa child-grid benar tetapi kolom tabel utama salah.

## Perbaikan

1. **`engine/derive.ts`** — tambah cabang eksplisit
   `if (field.type === "money") return "currency"` **sebelum** cabang `decimal`,
   agar jenis `money` tidak bergantung pada heuristik.
2. **Buang heuristik `decimal + rules[min] → currency`.** Ia menebak mata uang
   dari aturan yang tidak ada hubungannya (`rules: [min: 0]` juga dipakai field
   non-uang: `gl-balance.opening_balance`, `clinic.setting.tax_percent`,
   `visit.total`). `05-field-types.md` §2 melarang tebakan seperti ini
   ("komponen/renderer/backend dilarang menyimpulkan mata uang dari heuristik").
   Setelah (1) landing, cabang `decimal` hanya membawa beban salah.
3. **Test drift** — `derive.test.ts`: `money → format: currency`, `decimal`
   (termasuk yang ber-`min` rule) → tanpa format. Test ini gagal sebelum
   perbaikan, jadi ia mengunci akarnya.
4. **`MoneyInput` readonly** — jangan pakai `fmt.money` yang selalu
   `settings.currency` untuk field yang meng-override mata uang. Normalisasi
   `{amount, currency}` atas nama field diterapkan ke `currency`.

## File

- `renderers/react-shadcn/src/engine/derive.ts` (akar).
- `renderers/react-shadcn/src/engine/derive.test.ts` (test drift).
- `renderers/react-shadcn/src/widgets/MoneyInput.tsx` (readonly override).

## Sisa yang diketahui (jadi item todo)

- **`TableColumn.align` tidak dikonsumsi renderer.** `order-table-pos.yaml`
  menulis `align: right` pada kolom Total; `align` dihormati di `SectionBlock`
  tetapi **tidak** di `TableColumn`/`ListingColumn` — jadi niat manifest hilang
  diam-diam. Bukan bagian perbaikan ini (blast radius: setiap Table && Listing).

## Verifikasi

- `vitest` derive + format hijau (test baru gagal sebelum patch).
- `tsc -p tsconfig.app.json --noEmit` bersih.
- Browser: `/kafe/app/pos/cafe-order/cash-movements` kolom Jumlah → `Rp50.000`
  (bukan JSON), dengan `settings.currency {IDR, Rp, 0}`.

## Estimasi: **small**

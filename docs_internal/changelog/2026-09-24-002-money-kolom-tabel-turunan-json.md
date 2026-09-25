# 2026-09-24-002 — Kolom `money` di Table turunan tampil sebagai JSON

**Plan**: `docs_internal/plan/money-table-format-derivation.md`
**Todo**: kafe **10.25 ⏸️** (dibuka & ditutup di jalur yang sama), **10.26 ⏸️**
(ditemukan; masih terbuka).

## Konteks

Terukur di `/kafe/app/pos/cafe-order/cash-movements`: kolom **Jumlah**
menampilkan `{"amount":"50000","currency":"IDR"}` alih-alih `Rp50.000`. Bundle
App `kafe-pos` membawa `tables: []` — `cafe-order.cash-movement` tidak punya
Table tertulis, sehingga `TableRenderer` memakai `deriveTable(entity)`.

Akar: `engine/derive.ts` `tableFormat()` hanya memetakan
`datetime`/`date`/`percent` (plus satu heuristik `decimal + rules[min]`), jadi
field `money` **tidak pernah** mendapat `format`, dan `renderCellValue` jatuh ke
`JSON.stringify` untuk nilai objek. `cellHintsForField()` di
`lib/renderCell.tsx` **sudah** memetakan `money → currency` — dua kosakata yang
sama, satu sudah benar (child-grid) dan satu bolong (kolom tabel).

## Yang diubah

- `engine/derive.ts`: `tableFormat()` memetakan `money → currency`. Heuristik
  `decimal + rules[min] → currency` **dibuang** — ia menebak mata uang dari
  aturan batas bawah yang juga dipakai field non-uang (`gl-balance.
opening_balance`, `setting.tax_percent`, `visit.total`), dan
  `05-field-types.md` §2 melarang tebakan itu. Setelah cabang `money` ada,
  heuristik hanya menyisakan salah label.
- `widgets/MoneyInput.tsx`: pratinjau memakai mata uang **nilai**-nya sebagai
  fallback bila field tidak mendeklarasikan `currency`, sehingga
  `{amount, currency: USD}` tidak lagi dicetak dengan simbol `Rp` dari
  `settings.currency`.

Verifikasi runtime (SPA di-rebuild, sesi `kasir` app-scoped `kafe-pos`):
`cash-movements` → Jumlah `Rp50.000`/`Rp100.000`/`Rp25.000`/`Rp12.500` (tanpa
`{"amount"` di DOM); detail page AMOUNT `Rp50.000`; `payments`, `shifts`,
`menu-item-prices` ikut benar (`Rp0`, `Rp500.000`, `Rp45.000`).

Test: 3 case baru di `derive.test.ts` (2 gagal sebelum patch: `money` tanpa
format + `decimal` ber-`min` yang salah jadi `currency`) dan 1 case di
`money-time-input.test.tsx` (gagal sebelum patch). Suite frontend **324 lulus**
(dari 320), `tsc -p tsconfig.app.json --noEmit` bersih, `go build ./...` ok.

## Sisa

- `TableColumn.align` tidak dikonsumsi renderer mana pun (`order-table-pos.yaml`
  menulis `align: right` pada kolom Total — hilang diam-diam) → **10.26 ⏸️**.

Referensi: laporan pengguna (kolom Jumlah) · `docs/kind/ui/Table.md` §contoh
(`format: currency`) · `docs/spec/backend/05-field-types.md` §2.

# 2026-09-24-004 — Kolom relasi menampilkan label; `format: number` untuk angka

**Plan**: `docs_internal/plan/relation-display-and-number-format.md`
**Todo**: kafe **10.28** (✅ ditutup), **10.29 ⏸️** + **10.30 ⏸️** (dibuka —
ditemukan di jalur yang sama).

## Konteks

Dua laporan runtime dari `/kafe/app/pos/cafe-stock/stock-levels`:

1. "mengapa cabang dan bahan masih uuid?"
2. "mengapa saldo tidak ada pemisah ribuan?"

**Akar #1.** `stock-level-table.yaml` mendeklarasikan `field: branch_id` /
`field: ingredient_id`. API mengembalikan **dua-duanya** — skalar FK **dan**
objek relasi yang sudah di-resolve:

```json
"branch_id": "01a0bf3d-…", "branch": { "id": "…", "name": "Kafe Senayan" }
```

Tapi hanya dua dari tiga jalur render yang membaca alias itu: `derive.ts`
menulis ulang kolom **turunan** jadi `branch.name` (dot-path), dan
`DetailPage.tsx` punya resolver sendiri. `TableRenderer`/`ListingRenderer`
hanya `String(value)` — jadi tabel **tertulis** yang menyebut FK-nya langsung
mencetak UUID, padahal namanya ada di baris yang sama. Terdampak juga
`order-table-pos.yaml`, `order-table-customer.yaml` (`dining_table_id`) dan
`visit/tables/list.yaml` (`polyclinic_id`).

`ListingRenderer` bahkan punya bug kedua: ia membaca `row["branch.name"]`,
padahal kuncinya bersarang — jadi dot-path pun `undefined` di Listing.

**Akar #2.** `quantity_on_hand` bertipe `decimal, scale: 3` tanpa `format`.
Kosakata `renderCellValue` hanya `currency`/`date`/`relative`/`percent` — tidak
ada cabang angka — sehingga jatuh ke `String(value)` → `20000`.
`formatter.number()` sudah ada tetapi hanya dipakai Dashboard dan `NumberInput`;
kolom tabel tidak punya cara memintanya. Jadi ini bukan formatter yang salah,
melainkan **kosakata yang kurang**.

## Yang diubah

- **`src/lib/relation.ts`** (baru) — satu resolver bersama: alias mengikuti
  aturan server (`patient_id` → `patient`, selain itu nama resource), label
  dari `label_field` entity tujuan → fallback `name`/`title`/`code`. Menerima
  **kedua** ejaan (`branch_id` dan `branch.name`), karena keduanya sah dan
  keduanya dipakai manifest. Tanpa objek terkait → tampilkan kunci mentah
  (lebih baik daripada kosong).
- **`src/lib/renderCell.tsx`** — `resolveColumnCell()` (dipakai **kedua**
  renderer, supaya perbaikannya tidak jadi salinan kedua — persis pola yang
  melahirkan 10.26) + cabang `format === "number"`.
- **`src/lib/format.ts`** — `number(value, scale?)`. **Skala field menang atas
  `settings.decimal_scale`**: field `scale: 3` yang dicetak dengan skala global
  2 akan membulatkan digit tersimpan — salah menyatakan data, bukan preferensi.
- **`TableRenderer`/`ListingRenderer`** — memakai resolver; `getNestedValue`
  lokal (dead setelah ini) dihapus.
- **`stock-level-table.yaml`** — Saldo dapat `format: number`.
- Docs: `docs/spec/frontend/06-page-kinds.md` §3.1.2 (normatif) + gotcha
  `docs/kind/ui/Table.md`.

Terukur sesudah (browser, sesi `manajer` app-scoped `kafe-pos`): `stock-levels`
→ Cabang `Kafe Senayan`, Bahan `Beras Putih`, Saldo `20.000`, **tanpa UUID di
DOM**; `menu-item-prices` → Cabang `Kafe Senayan`; `orders` → `Meja` `-`
(datanya memang `dining_table_id: null` — takeaway, diverifikasi lewat API).

Test: `src/lib/relation.test.tsx` (10 case, termasuk **DOM** yang dirender dari
payload API nyata — dibuktikan gagal tanpa perbaikan `ListingRenderer`) +
2 case `number(scale)` di `format.test.ts`. Suite frontend **346 lulus** (dari 334) · `tsc` bersih · `go build ./...` ok · kafe `validate` 85 manifest 0
problem.

## Sisa

- **Sortir kolom relasi menyortir UUID.** `sortable: true` pada `branch_id`
  mengirim `sort=branch_id`; `sort=branch.name` → **422 unknown field**
  (`internal/api/handler.go` hanya menerima nama field entity), dan `sort=branch`
  juga 422. Jadi "urutkan menurut Cabang" mengurutkan UUID, bukan nama — dan
  `sortable: true` pada kolom relasi **menjanjikan** urutan yang tidak
  diberikan. → **10.29 ⏸️**.
- **`TableColumn.format` belum himpunan tertutup** (beda dari
  `ReportColumn.format` yang enum sejak S16): salah ketik lolos validasi dan
  mencetak nilai mentah. Sudah ditandai **Open** di spec §3.1.2 →
  **10.30 ⏸️**.

Referensi: `docs_internal/plan/relation-display-and-number-format.md`.

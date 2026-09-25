# Plan — `TableColumn.align`/`width` diabaikan renderer (kafe 10.26)

Sumber: kafe TODO **10.26 ⏸️** (ditemukan saat memperbaiki 10.25).

## Masalah

`TableColumn` punya `sortable`, `width`, `align`, `link`
(`pkg/spec/frontend.go:444`), dan `docs/spec/frontend/06-page-kinds.md` §3
menyatakan keempatnya **didukung** `TableColumn` (tabel paritas vs
`ReportColumn`). Kenyataannya renderer tidak membaca dua di antaranya:

```
$ grep -rn "col\.align\|col\.width" renderers/react-shadcn/src   → 0 hasil
```

`TableRenderer` meng-hardcode `text-left` pada `<th>` dan tanpa alignment pada
`<td>`; `ListingRenderer` sama. Akibatnya manifest yang menyatakan niat —
`order-table-pos.yaml` (`align: right` pada Total),
`stock-level-table.yaml` (3 kolom `align: right`),
`visit/tables/list.yaml` (`align: center`/`right` + 6 `width`),
`journal-table.yaml` (`width` 2 kolom) — **diam-diam diabaikan**. Inilah kelas
yang sama dengan 10.25: kontrak tertulis, renderer tidak menepatinya, tanpa
peringatan apa pun.

## Ruang lingkup

| Atribut | Keputusan                                                                                                                                                                                                     |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `align` | **Perbaiki.** `left`/`center`/`right` → text-align pada `<th>` **dan** `<td>` (text-align tidak diwarisi dari th ke td — keduanya sibling).                                                                   |
| `width` | **Perbaiki.** Panjang CSS → `width` + `minWidth` pada `<th>` kolom itu; browser memakai th sebagai batas kolom.                                                                                               |
| `link`  | **Tidak** di sini → **10.27 ⏸️**. Semantiknya ("Page name to navigate to") belum menetapkan bagaimana `:param` route diisi dari record, dan **0 manifest** memakainya. Menebak lebih buruk daripada mencatat. |

`ReportColumn` **tidak** ikut: spec §3 menyatakan `align`/`width` `—` untuk
laporan, jadi tidak ada kontrak yang dilanggar.

## Konstruk

Satu helper bersama, supaya vocab-nya tidak drift lagi (pelajaran 10.25:
`money → currency` benar di `cellHintsForField`, bolong di `derive.tableFormat`):

`src/lib/tableColumn.ts` (baru):

- `columnAlignClass(align?): string` → `""` | `"text-center"` | `"text-right"`
  (nilai tak dikenal → `""`, bukan tebakan).
- `columnWidthStyle(width?): CSSProperties | undefined` → `{ width, minWidth }`.

Dipakai **kedua** renderer lewat peta `id → TableColumn` (id kolom TanStack =
`col.field`, termasuk dot-path seperti `patient.name`).

## File

- `renderers/react-shadcn/src/lib/tableColumn.ts` (baru).
- `renderers/react-shadcn/src/kinds/table/TableRenderer.tsx` — `<th>` + `<td>`.
- `renderers/react-shadcn/src/kinds/listing/ListingRenderer.tsx` — `<th>` + `<td>`.
- `renderers/react-shadcn/src/lib/tableColumn.test.tsx` (baru) — unit + DOM.
- `pkg/spec/frontend.go` — `align` jadi **enum tertutup** (`@schema {enum}`),
  supaya `align: rigth` tidak lolos diam-diam (precedent: `SectionBlock.align`
  sudah enum; `ReportColumn.format` jadi enum karena alasan yang sama).
- `docs/spec/frontend/06-page-kinds.md` §3 — definisi normatif `align`/`width`.
- Schema + kind docs diregenerasi.

## Verifikasi

- Unit: helper (termasuk nilai tak dikenal → tidak ada kelas).
- **Guard paritas** (pola `catalog.test.tsx`): kedua renderer wajib merujuk
  helper bersama — mencegah satu renderer drift lagi.
- DOM: render `ListingRenderer` dengan `align: right`/`width` → `<td>`
  ber-`text-right` dan `<th>` ber-`width`.
- Browser (terukur, sesi `kasir` app-scoped): `stock-level-table` (align) +
  `journal-table` (width).
- `vitest` penuh · `tsc` bersih · `go build ./...`.

## Estimasi: **small–medium**

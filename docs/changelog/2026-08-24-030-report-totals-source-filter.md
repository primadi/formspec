# 2026-08-24-030 — Report Totals/Subtotal + source.filter (5.13.1, 5.13.1a)

## Apa yang diubah

**Report totals row bug fix (5.13.1):** baris total sebelumnya menghitung nilai tapi
merender `<td>` kosong. Kini `TotalsRow` menempatkan nilai agregat di kolom yang cocok
(label di kolom pertama, nilai di kolom yang punya `totals` def, diformat seperti kolom
data). Ditambah **subtotal per group** (`computeTotals` shared untuk overall + per-group).

Export sebagai async job → download tray masih deferred (butuh backend job infra; saat ini
tetap CSV Blob client-side).

**Report `source.filter` (5.13.1a):** `ReportSource` (`{ entity, filter }`) ditambahkan ke
`ReportSpec` — filter parameterized deklaratif dengan `":param"` placeholder yang di-resolve
dari `parameters[]` saat eksekusi; literal pass-through. `ReportRenderer.fetchReport`
menggabungkan `source.filter` ke list params.

## File terdampak

- `pkg/spec/frontend.go` (`ReportSource`)
- `renderers/react-shadcn/src/types/manifest.ts` (`ReportSource`)
- `renderers/react-shadcn/src/kinds/report/ReportRenderer.tsx` (TotalsRow, subtotal,
  source.filter resolve)

## Referensi

- Plan: `docs/plan/fase5-completion.md` (WS-G)
- Todo: `docs/plan/todo.md` §5.13.1, §5.13.1a

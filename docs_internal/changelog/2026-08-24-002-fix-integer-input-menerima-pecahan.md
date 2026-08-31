# 2026-08-24-002 — Fix: Integer Input Menerima Pecahan (ChildTable + NumberInput)

## Apa yang diubah

User melaporkan: child field `quantity` bertipe `integer` tapi input-nya bisa
diisi pecahan (mis. "1.5"). Akar masalah: input `type="number"` di browser
mengizinkan mengetik karakter `.` (dan `e` untuk notasi ilmiah) meskipun nilai
akhir di-`parseInt` — user bisa mengetik pecahan (walaupun akhirnya ter-snap).

### Frontend (`renderers/react-shadcn`)

- `widgets/ChildTable.tsx` — case `integer`/`decimal` pada `ChildCell`:
  - `step={1}` untuk integer (spinner naik per 1, validasi native flag
    stepMismatch).
  - `onKeyDown` memblokir `.` / `,` / `e` / `E` untuk integer — pecahan tidak
    bisa diketik sama sekali.
  - `onChange` untuk integer men-strip bagian desimal (mis. paste "1.5" → 1)
    via `v.replace(/[.,].*$/, "")`.
- `widgets/NumberInput.tsx` — pola sama untuk field integer top-level
  (`step === 1`): blokir `.`/`,`/`e`/`E` di `onKeyDown`.

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass.
- Browser: ketik "1.5" di Qty → `.` diblokir, hasil "15"; paste "1.5" →
  tersanitasi jadi "1"; `step` kini "1".

## Referensi

- Plan: `docs/plan/cafe-order-child-autofill-readonly-dropdown.md`
- Lanjutan: `docs/changelog/2026-08-24-001-cafe-order-child-autofill-readonly-dropdown.md`

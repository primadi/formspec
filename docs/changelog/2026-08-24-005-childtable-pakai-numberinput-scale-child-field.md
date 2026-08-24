# 2026-08-24-005 — ChildTable pakai NumberInput (scale berlaku di child field)

## Apa yang diubah

User bertanya: field `quantity` (child field di `items`) tidak pakai
`NumberInput` untuk editing? Benar — sebelumnya `ChildTable.ChildCell`
menangani `integer`/`decimal` dengan `Input` mentah (duplikasi logika), sehingga
`scale` (05-field-types.md §1.2) tidak berlaku di child field. Setelah user
mengubah `quantity` menjadi `type: decimal, scale: 2`, input tetap `step="1"`.

### Frontend (`renderers/react-shadcn`)

- `widgets/ChildTable.tsx` — case `integer`/`decimal` di `ChildCell` kini
  memakai **`NumberInput`** (bukan `Input` mentah), mengirim
  `integer={field.type === "integer"}` dan `scale={field.scale}`. Logika
  duplikat (blokir desimal untuk integer, strip saat paste) dihapus — reuse
  satu implementasi dengan field top-level.

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass.
- Browser (child `quantity` = `decimal, scale: 2`): `step="0.01"`; fill
  "1.234" → "1.23" (di-round ke 2 desimal).

## Referensi

- Lanjutan: `docs/changelog/2026-08-24-004-decimal-scale-membatasi-input-pecahan.md`

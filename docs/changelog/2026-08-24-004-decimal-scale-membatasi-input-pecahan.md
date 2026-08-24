# 2026-08-24-004 — Decimal `scale` Membatasi Input Pecahan (NumberInput + spec)

## Apa yang diubah

User bertanya: apakah `precision` bisa membatasi input, mis. max 2 digit
desimal? Jawaban: ya — dan istilah yang tepat di spec adalah **`scale`**
(digit di belakang koma; `precision` = total digit signifikan, `05-field-types.md`
§1.2). Sebelumnya `precision`/`scale` hanya didokumentasikan di spec, belum ada
di struct Go, dan `NumberInput` tidak membatasi digit desimal sama sekali.

### Framework

- `pkg/spec/entity.go` — `Field` ditambah `Precision *int` dan `Scale *int`
  (05-field-types.md §1.2). `scale` = digit di belakang koma.
- `schemas/` — regenerasi via `make generate-schema`.
- `types/manifest.ts` — `Field.precision?: number`, `Field.scale?: number`.
- `widgets/NumberInput.tsx` — prop `precision` **di-rename ke `scale`**
  (selaras spec). `scale` kini **membatasi input**:
  - `onChange`: nilai di-round ke `scale` desimal (`n.toFixed(scale)`) —
    mencakup paste (mis. "1.234" → 1.23 dengan scale=2).
  - `onKeyDown`: digit ke-`scale+1` di belakang koma diblokir saat mengetik di
    akhir nilai.
  - `step` default = `1/10^scale` (mis. scale=2 → 0.01).
- `kinds/form/FormRenderer.tsx` — `case "number"` mengirim
  `scale={entityField.scale}`.

## Verifikasi

- `tsc -b` bersih; `vitest` 103 pass; `go test ./...` 736 pass.
- Browser (field `type: decimal, scale: 2`): `step="0.01"`; fill/paste
  "1.234" → "1.23" (di-round ke 2 desimal).

## Referensi

- Plan: `docs/plan/cafe-order-child-autofill-readonly-dropdown.md`
- Lanjutan: `docs/changelog/2026-08-24-003-numberinput-bedakan-integer-vs-decimal.md`
- Catatan: `precision` (total digit) belum dipakai untuk validasi input —
  hanya `scale` yang membatasi digit desimal; `precision` untuk validasi
  server-side (deferred).

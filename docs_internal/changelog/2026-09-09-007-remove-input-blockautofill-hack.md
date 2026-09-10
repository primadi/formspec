# 007 — Hapus hack anti-autofill readOnly di komponen Input

Plan: `docs_internal/plan/` (sesi login-autofill, lihat juga
`/memories/session/plan.md`). Laporan: di `/cafe/login`, saat memilih saran
autofill username dari dropdown Chrome, field password tidak ikut terisi —
user harus memilih autofill password terpisah.

## Apa

- `renderers/react-shadcn/src/components/ui/input.tsx`: hapus mekanisme
  `blockAutofill` (input dirender `readOnly` sampai focus pertama + default
  `autoComplete="nope"`). Komponen kini passthrough murni — `readOnly` dan
  `autoComplete` sepenuhnya mengikuti props caller.
- `renderers/react-shadcn/src/widgets/RelationPicker.tsx`: update komentar
  auto-focus yang merujuk mekanisme yang sudah dihapus (perilaku auto-focus
  dipertahankan).

## Kenapa

Hack `readOnly`-until-focus membuat Chrome tidak bisa pair-fill
username+password: saat user memilih saved credential dari dropdown username,
field password masih `readOnly` (tidak fillable) sehingga Chrome melewatinya.
Login bawaan (`LoginScreen`) sudah memakai `autoComplete="username"` +
`current-password` yang benar; dengan hack dihapus, autofill berpasangan
bekerja seperti app lain. Hack ini awalnya untuk menekan dropdown
address/payment Chrome pada form generik — dihapus karena mengganggu UX login
dan `autoComplete` eksplisit per-field sudah cukup mengendalikan perilaku.

## File terdampak

- `renderers/react-shadcn/src/components/ui/input.tsx`
- `renderers/react-shadcn/src/widgets/RelationPicker.tsx` (komentar saja)

## Verifikasi

- `vitest run`: 166 test / 8 file PASS.
- `tsc --noEmit`: 0 error.
- Manual: `/cafe/login` → pilih saran username dari dropdown autofill →
  username dan password terisi bersamaan.

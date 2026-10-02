# Plan — Wizard: commit `PATCH` + peluncur dari tombol transisi (B+C)

**Status:** ✅ landed 2026-10-02 (changelog `2026-10-02-002`, todo 5.25.4/5.25.5;
sisa 5.25.6) · **Tanggal:** 2026-10-02
**Todo:** 5.25.4 (launcher) + 5.25.5 (commit)
**Referensi:** `docs_internal/plan/registered-views.md`,
`pkg/spec/entity.go` §`TransitionPermission`/`ActionSources`,
`internal/api/handler.go` §PATCH (gate transisi), kafe 10.48
(`has_route` → klien tidak menebak).

## Masalah

`kind: Wizard` di kafe (`close-shift-wizard`) mengikat dirinya lewat
`entity: cafe-order.shift` + `action: close-shift`, di mana `close-shift`
adalah **transisi** (`via`) tanpa `impl`. Dua akibat terukur (2026-10-02):

1. **Tidak ada peluncur.** `DetailPage` merender tombol transisi dari state
   machine, tetapi kliknya mengambil jalur `PATCH status` (`has_route: false`)
   — **melompati wizard**, sehingga `counted_cash`/`note`/`supervisor_id` tak
   pernah dikumpulkan dan `difference` dihitung dari field kosong.
   Tidak ada jalur UI mana pun yang menuju `/wizard/<name>`.
2. **Wizard tidak bisa commit.** `WizardRenderer.handleSubmit` mem-POST
   `entry.spec.action` **mentah** ke client ber-prefix `/{ws}/_ui/entity` →
   `POST …/_ui/entity/close-shift`. Route seperti itu tidak ada (butuh
   `{module}/{entity}/{id}/{action}` dan action ber-`impl`), dan wizard juga
   tidak pernah membawa id record.

## Keputusan

- **B — commit mode `PATCH`.** Wizard memilih cara commit dengan **aturan yang
  sama** yang sudah dipakai `DetailPage` (kafe 10.48): lihat
  `ActionSummary.has_route` dari bundle. `has_route` → `POST
/{module}/{entity}/{id}/{action}`; selain itu, bila `action` cocok dengan
  sebuah transisi (`via`), → `PATCH /{module}/{entity}/{id}` dengan
  `{state_field: to, …field terkumpul}`. Id record dibaca dari `?id=`.
  Sekaligus memperbaiki path POST yang selama ini salah.
- **C — peluncur.** `DetailPage`, saat transisi cocok dengan Wizard
  (`spec.entity` menunjuk entity ini **dan** `spec.action == via`), **menavigasi
  ke wizard** (`?id=<record-id>`) alih-alih menjalankan PATCH. Deteksi ini
  berjalan sebelum dialog input/konfirmasi, karena wizard sendirilah yang
  mengumpulkan input.
- **Landing default surface-aware.** `on_complete` default sekarang
  `adminPath()` (`/{ws}/_admin`) — salah untuk App surface (kasir kafe-pos
  bahkan tidak punya `_admin.access`). Default baru: daftar entity wizard
  **di surface saat ini**; `adminPath()` hanya untuk wizard tanpa entity.

## Perubahan

| File                                  | Isi                                                                       |
| ------------------------------------- | ------------------------------------------------------------------------- |
| `src/engine/wizardCommit.ts` (baru)   | Logika murni: `resolveWizardCommit(...)` + `findWizardForTransition(...)` |
| `src/kinds/wizard/WizardRenderer.tsx` | Pakai helper; baca `?id=`; landing default surface-aware                  |
| `src/kinds/page/DetailPage.tsx`       | Deteksi wizard → navigasi (C)                                             |

## Verifikasi

- `npx tsc -b` bersih; `npx vitest run` (unit baru untuk kedua helper).
- `formspec check -f examples/kafe/spec` tetap 0/0 (tak ada perubahan manifest).
- Manual (setelah `npm run build`): di `/cafe-order/shifts/<id>` klik "Tutup
  Shift" → masuk wizard; selesaikan → `PATCH status=closed` **dengan**
  `counted_cash`/`note`/`supervisor_id`.

## Bukan bagian dari ini

- Kafe `close-shift` tetap transisi tanpa `impl` (tidak perlu script baru).
- Sisa 5.25.2 (picker→404) dan 5.25.3 (lint pre-existing) tak tersentuh.

# 2026-08-29-006 — Fix Nav Active-State: Beranda Selalu Bold

## Apa yang diubah

`renderers/react-shadcn/src/shell/NoNavShell.tsx` — logika active-state nav
link. Route `/` (Beranda) kini hanya aktif pada exact match terhadap base
path; route lain pakai prefix match ber-boundary `/` (`startsWith(href + "/")`)
agar `/listing-x` tidak mengaktifkan `/listing`.

## Kenapa

Untuk Beranda, `href` = `{base}/` dan `pathname.startsWith(href)` selalu true
di semua halaman (`/default/listing/...`.startsWith(`/default/`)) → Beranda
tetap bold meski halaman lain aktif.

## File terkena dampak

- `renderers/react-shadcn/src/shell/NoNavShell.tsx`

## Verifikasi

- Halaman `/default/listing/module-catalog`: Module bold, Beranda muted.
- Halaman `/default`: Beranda bold, Module muted.
- `tsc --noEmit` bersih.

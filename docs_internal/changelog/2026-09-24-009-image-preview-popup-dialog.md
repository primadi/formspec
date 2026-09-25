# 2026-09-24-009 — Foto menu dibuka di popup dialog, bukan tab baru

**Trigger:** laporan pengguna pada halaman detail menu kafe — "di FOTO MENU di
klik, tampilkan di jendela lain, ubah menjadi tampilkan di popup dialog".

Nilai `file` bergambar memang sudah dirender `<img>` (kafe 2.5 / #4), tetapi
pembungkusnya anchor `target="_blank"`: klik membuka JPEG telanjang di tab baru.
Di permukaan kasir App, sidebar dan record yang sedang dibuka ikut hilang — dan
tab nyasar mudah terlupakan. Kini gambar besar muncul di `Dialog` di dalam App,
halaman tetap di tempatnya (terukur: `openTabs` 2 → **1**, URL record tidak
berubah, sesudah Close dialog 0).

**Satu komponen, bukan tiga perbaikan** — cacat yang sama ada di tiga situs
(`DetailPage` field file; `FileInput` pratinjau readonly; `FileInput` thumbnail
mode edit), jadi perbaikannya diletakkan di
`components/ui/image-lightbox.tsx` (thumbnail + Dialog + judul nama file).
Tautan unduh berkas **non-gambar** (PDF/CSV) sengaja dibiarkan `target="_blank"`
— tab memang tempat unduhan.

**Satu jebakan pengukuran ikut ketemu.** Ukuran panel tidak bisa dibiarkan pada
default dialog (`sm:max-w-sm` = 384px < foto 960px → pratinjau seukuran
thumbnail; terukur 423px), tetapi `w-auto` **lebih buruk**: elemen `fixed` pada
`left: 50%` shrink-to-fit ke `viewport − 50%`, terukur 462px pada viewport 923px
— separuh layar sia-sia. `w-full sm:max-w-[92vw]` → 831px, foto 655×439 dengan
rasio asli, tercentang.

**Bukti:** diukur di browser (viewport 923×560) pada record kafe nyata; `vitest`
**383 lulus** (+6), `tsc -b` bersih, `make web-build` sukses. Test mengunci DOM
dialog **dan** ketiga situs render, dan **dibuktikan gagal** saat `DetailPage`
dikembalikan ke anchor `target="_blank"` (1 failed / 5 passed) — lalu hijau lagi
setelah dipulihkan.

Plan: `docs_internal/plan/image-preview-dialog.md`. Sisa: sel tabel
ber-`widget: image` (`renderCell.tsx`) dan kartu katalog (`PickerPanel`)
**belum** memakai dialog — pada sel, thumbnail adalah hit target navigasi baris,
jadi butuh keputusan kontrak tersendiri → todo **5.21.1 ⏸️**.

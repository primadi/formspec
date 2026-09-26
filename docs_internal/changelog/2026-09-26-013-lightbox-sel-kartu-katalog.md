# Pratinjau gambar di sel tabel & kartu katalog (todo 5.21.2)

## Apa yang diubah

Dua situs terakhir yang masih membuka gambar lewat tab — sel tabel
(`lib/renderCell.tsx`, dipakai Table/Listing/Report/ChildTable sekaligus) dan
kartu katalog (`kinds/form/PickerPanel.tsx`) — kini memakai dialog yang sama
dengan dua situs lain (`DetailPage`, `FileInput`).

**Konflik yang item ini sebut belum diputuskan, dan keputusannya:** pada kedua
situs thumbnail adalah bagian dari **hit target induknya** — sel berada di dalam
`<tr onClick>` (membuka record) dan kartu katalog **adalah** `<button>` yang
menambahkan item ke keranjang (terukur: klik foto Kopi Tubruk → total Rp18.000).
Jadi "klik gambar untuk memperbesar" tidak bisa memakai pola situs lain (satu
`<button>` membungkus thumbnail): ia akan **menelan** aksi induknya, dan pada
kasus katalog, menambah-ke-keranjang menjadi tidak bisa dibedakan dari
memperbesar.

Keputusan: **kontrol terpisah di sudut**, bukan memperbesar lewat thumbnail.
`ImageLightbox` mendapat prop `renderTrigger(open)` (pemanggil menyediakan
pemicunya sendiri, jadi tidak ada `<button>` bersarang — HTML tidak sah di dalam
`<button>`), dan komponen baru `ImageLightboxTrigger` merender thumbnail + tombol
`⤢` kecil di pojok yang `stopPropagation()` **dan** `preventDefault()` sebelum
membuka dialog. Aksi induk (buka record / tambah ke keranjang) tetap utuh.

## Kenapa

Item 5.21.2 mencatat dua situs tersisa dan satu konflik yang menghalangi, dengan
bukti pengukuran yang jelas. Yang membuatnya layak dikerjakan: begitu konfliknya
diputuskan, sisanya adalah memakai komponen yang sudah ada — bukan desain baru.

## File terdampak

- `renderers/react-shadcn/src/components/ui/image-lightbox.tsx` — prop
  `renderTrigger` + komponen `ImageLightboxTrigger`
- `renderers/react-shadcn/src/lib/renderCell.tsx` — sel `widget: image`
- `renderers/react-shadcn/src/kinds/form/PickerPanel.tsx` — kartu katalog
- `renderers/react-shadcn/src/components/ui/image-lightbox.test.tsx` — +4 test
  (2 site-scan, 2 komponen)

## Bukti

- `npx tsc -b` bersih; `npx vitest run` **515 lulus** / 37 file (baseline sesi ini
  511; +4 dari file ini).
- **Dibuktikan gagal:** `stopPropagation()`/`preventDefault()` dihapus dari
  pemicu → test `opens the preview and stops the click from reaching the parent`
  gagal (induk menerima klik), hijau sesudah dikembalikan. Itu tepat kegagalan
  yang item ini khawatirkan: klik memperbesar ikut menavigasi/menambah.
- Test site-scan menegaskan elemen `ImageLightboxTrigger` **tidak** membawa
  `target="_blank"`/`<a>`, dan secara eksplisit **mempertahankan** tautan unduh
  non-gambar yang `target="_blank"` (perilaku yang memang benar) — supaya
  perbaikan ini tidak menghapusnya.

## Catatan (sisa yang tidak ditutup)

**Belum diverifikasi di browser nyata.** Test memakai `fireEvent.click` di jsdom,
bukan klik sungguhan pada `<tr>`/`<button>` nyata; pada khususnya
**penempatan sudut tombol `⤢` belum diukur** (apakah ia cukup besar untuk
disentuh di POS dan tidak menutupi bagian penting foto). Kelas yang sama dengan
todo 17.7. Dicatat sebagai item baru di bawah.

## Rujukan

Todo **5.21.2** (tertutup) · **5.21.1** (`docs_internal/changelog/2026-09-24-009`,
dua situs pertama) · keputusan tema: `docs/renderers/shadcn-shell/`.

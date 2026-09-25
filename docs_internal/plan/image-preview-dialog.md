# Plan — Pratinjau gambar di popup dialog, bukan jendela/tab baru

**Trigger (pengguna, 2026-09-24):** "di FOTO MENU di klik, tampilkan di jendela
lain, ubah menjadi tampilkan di popup dialog" — pada halaman detail menu-item
kafe (`/kafe/app/pos/cafe-master/menu-items/{id}`).

## Masalah

Nilai `file` bergambar dirender `<img>` (#4 / kafe 2.5), tetapi **pembungkusnya
adalah anchor `target="_blank"`**: klik membuka JPEG telanjang di tab peramban
baru. Untuk permukaan kasir/POS ini mahal — App, sidebar, dan record yang sedang
dibuka semuanya hilang, dan tab nyasar mudah terlupakan (POS sering
`no-nav`/full-screen). Kontrak yang diinginkan pengguna: gambar besar muncul di
**dialog di dalam App**, halaman tetap di tempatnya.

Tiga situs menampilkan gambar dan **semuanya** punya cacat yang sama:

| Situs                       | Bentuk lama                                           |
| --------------------------- | ----------------------------------------------------- |
| `kinds/page/DetailPage.tsx` | `<a target="_blank">` membungkus `<img>` (field file) |
| `widgets/FileInput.tsx`     | sama, untuk pratinjau readonly                        |
| `widgets/FileInput.tsx`     | `<img>` polos lagi, untuk thumbnail mode edit         |

Cetakan berulang ini (satu kosakata, tiga tempat) adalah alasan perbaikannya
diletakkan di **satu komponen bersama**, bukan di tiga call-site.

## Konstruk & aturan

| Hal                  | Keputusan                                                                  |
| -------------------- | -------------------------------------------------------------------------- |
| Komponen             | `components/ui/image-lightbox.tsx` — thumbnail + `Dialog` (base-ui)        |
| Pemicu               | `<button>` dengan `aria-label="View {nama file}"`, `cursor-zoom-in`        |
| Judul dialog         | Nama file = segmen terakhir object key (nilainya key, bukan nama tampilan) |
| Ukuran gambar        | `max-h-[80vh] max-w-full object-contain` — tidak memotong, tidak meluber   |
| Ukuran panel         | `w-full sm:max-w-[92vw]` — **bukan** default dialog, **bukan** `w-auto`    |
| Non-gambar (PDF/CSV) | tetap tautan unduh `target="_blank"` — tab memang tempat unduhan           |
| Cakupan              | 3 situs di atas; `renderCell.tsx` sengaja tidak masuk (sel tabel)          |

**Mengapa `w-full sm:max-w-[92vw]` (diukur, bukan dipilih):**

1. Default dialog `sm:max-w-sm` = 384px, sedangkan foto sumber 960px → pratinjau
   membesar ke ukuran thumbnail. Terukur pada percobaan pertama: 423px.
2. `w-auto` (percobaan kedua) **lebih buruk**: elemen `fixed` pada `left: 50%`
   shrink-to-fit ke `viewport − 50%` → 462px pada viewport 923px, jadi separuh
   layar sia-sia.
3. `w-full` + cap lebar → 831px di viewport 923px, foto 655×439 (rasio asli
   dipertahankan, `scaleVsNatural` 0.68), muat di viewport, tercentang.

## File

- `src/components/ui/image-lightbox.tsx` (baru) — komponen.
- `src/kinds/page/DetailPage.tsx` — cabang `field.type === "file"`.
- `src/widgets/FileInput.tsx` — pratinjau readonly + thumbnail mode edit.
- `src/components/ui/image-lightbox.test.tsx` (baru) — 6 test.

## Bukti

**Browser** (`/kafe/app/pos/cafe-master/menu-items/01a0c83e-…`, viewport 923×560):

| Ukuran                 | Sebelum                        | Sesudah                          |
| ---------------------- | ------------------------------ | -------------------------------- |
| Yang terjadi saat klik | tab baru, halaman ditinggalkan | dialog di App, URL tidak berubah |
| Foto yang terlihat     | JPEG telanjang (tab)           | 655×439 di dalam dialog          |
| Panel                  | —                              | 831×489, tercentang, muat        |
| Tab terbuka            | 2                              | **1** (`openTabs`)               |
| Sesudah Close          | (kembali ke tab asal)          | dialog 0, URL record utuh        |

**Test:** `vitest` **383 lulus** (+6), `tsc -b` bersih. Test mengunci kontrak
DOM **dan** ketiga situs render — dan **dibuktikan gagal** bila `ImageLightbox`
di `DetailPage` dikembalikan ke anchor `target="_blank"` (1 failed / 5 passed),
lalu hijau kembali setelah dipulihkan.

## Sisa (dicatat sebagai item bernomor)

Sel tabel ber-`widget: image` (`lib/renderCell.tsx`, dipakai Table/Listing/Report
/ChildTable) dan `PickerPanel` (kartu katalog) **belum** memakai dialog — pada
sel, thumbnail adalah hit target navigasi baris, jadi klik-untuk-memperbesar
bertabrakan dengannya dan butuh keputusan kontrak tersendiri. Dicatat sebagai
sisa bernomor, bukan prosa di dalam item `[x]`.

# Katalog Kind Renderer

**Updated:** 2026-09-20 · Status: Draft

> Draft: isi di bawah kondisi kode `renderers/react-shadcn/src/kinds/` hari ini — tingkat
> kelengkapan bervariasi jauh antar kind, ditandai eksplisit per kind.

## 1. Registry

Wiring kind→component **bukan** lewat registry dinamis — `engine/registry.tsx`
sudah **dihapus**, isi `src/engine/` kini `derive.ts`, `entityRef.ts`,
`lifecycle.ts`, `permissions.ts` (lihat [`01-architecture.md`](01-architecture.md)
§5) — melainkan `lazy()` map hardcoded di `shell/router.tsx`.

## 2. Tier App

Chrome App dirakit oleh **satu** shell — `src/shell/RegionShell.tsx` — dari
peta **region** yang di-resolve backend (`bundle.app.chrome.regions`,
[`../../spec/frontend/05-app-kinds.md`](../../spec/frontend/05-app-kinds.md)
§5): `topbar`, `sidebar`, `rightbar`, `bottombar`, `footer` (+ `content`
implicit dari `Outlet`). Setiap region bernilai `none`, `auto` (isi
archetype), atau `<component-ref>` (dirender lewat `src/shell/AssetRenderer.tsx`).

**Archetype = preset**, bukan komponen terpisah: `sidebar-nav` = `no-nav` +
`sidebar: auto`, `topnav` = `no-nav` + `topbar: auto`. Registry `APP_SHELLS`
(`src/App.tsx`) memetakan ketiganya ke `RegionShell`; perilaku warna lama
dipertahankan per region (sidebar statis di desktop / overlay di mobile via
`useMediaQuery("(max-width: 767px)")`, isi topbar berbeda untuk sidebar-nav vs
topnav vs brand-bar). Preset/derivasi gula berada di `src/shell/regions.ts`
(paritas dengan `chromeRegionPreset` di `internal/ui/meta.go`).

`OverlayHost` kini dipasang untuk **semua** komposisi (dulu absen di
`no-nav`, sehingga form modal/drawer diam-diam mati di sana).

`no-nav`: tanpa bar **default** — tapi chrome tetap ada dan boleh diisi
(`regions.topbar: auto` memberi brand bar). Ini yang memberi App publik seperti
`kafe-qr` tempat menaruh kontrol sesi. Kontrol auth anonim dikendalikan
`chrome.auth`; user menu (→ Sign out) **selalu** dirender bila ada token
(`src/shell/AuthArea.tsx`), jadi tak ada lagi App yang mengurung pengguna tanpa
jalan keluar. Komponen bersama `src/shell/AuthArea.tsx`; blok `section:` pada
`kind: Page` dirender `src/components/sections/SectionBlocks.tsx`. Contoh:
`examples/storefront/` (`no-nav` + `public`), `registry/`
(`no-nav` + `chrome: {nav: menu, auth: links}`), `examples/kafe/`
(`kafe-qr` publik + brand bar; `kafe-kds` kiosk + tombol auth). **Lengkap.**

## 3. Tier Page

Kelengkapan per kind (semua di `kinds/`, kecuali `menu` — dihapus, lihat
[`01-architecture.md`](01-architecture.md) §2):

| Kind                 | Status                            | Catatan                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| -------------------- | --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Table`              | Fungsional                        | TanStack Table headless, pagination/sort/search server-side, inline editing (5.4.2), konfirmasi delete lewat `ConfirmDialog` shadcn (bukan `window.confirm()`). Navigasi memakai path permukaan aktif (`useSurface().surfacePath`), bukan prefiks `/_admin`. `Form.render` **dihormati**: `modal`/`drawer` membuka form di `OverlayHost`; mode lain (termasuk tanpa mode) pergi ke route halaman penuh.                                      |
| `Form`               | Fungsional                        | react-hook-form + zod (schema dibangun manual per tipe/rule field, bukan dari FormSpecExpr), auto-save debounced 2 detik untuk `two_step_autosave`, CAS `version` terkirim sebagai `If-Match`. **Gap:** tidak ada percabangan khusus 409 — conflict jatuh ke toast error generik, bukan alur refetch.                                                                                                                                        |
| `Page`               | Fungsional (blocks & tabs)        | Permission-gated per tab/blok, delegasi ke Table/Form. **Gap:** blok `component:` (custom component) murni placeholder teks — lihat [`04-theming-assets.md`](04-theming-assets.md) §2.                                                                                                                                                                                                                                                       |
| `Dashboard`/`Widget` | Fungsional                        | Empat tipe widget benar-benar dirender: `metric`, `chart`, `list`, `table`. Metric menghitung agregat atas baris ter-filter (`lib/aggregate.ts`, money-aware — agregat yang salah deklarasi menghasilkan `"--"`, bukan `0` yang menyesatkan); chart digambar tanpa library charting (satu seri per `config.group_by`). `Widget.spec.query` menerjemahkan subset FormSpecExpr (`field = today()`, `in [...]`, `=`/`==`/`!=`, digabung `and`). |
| `Wizard`             | Fungsional, melebihi rencana awal | Step state di `?step=N`, autosave per-instance ke `localStorage` (`?instance=<uuid>`), hook `on_enter`/`on_next`/`on_prev` best-effort, step type `search_select` dan form, `on_complete` (restart/redirect/banner) lengkap. **Gap:** step type `component:` custom masih placeholder.                                                                                                                                                       |
| `Kanban`             | Fungsional                        | Drag & drop antar kolom (transisi state, optimistic + 409 snap-back), drag-to-reorder dalam kolom (`position_field`), row action permission-gated, realtime refetch, filter server-side (`select`/`date`/`text`, seed `default`, `today()`), `fixed_filters` immutable, search client-side, `max_cards_per_column`.                                                                                                                          |
| `Timeline`           | Fungsional                        | Infinite scroll berbasis `IntersectionObserver`, grouping date/month/year/none, field tampilan terkonfigurasi, realtime (`realtime: true` → reset cursor + refetch).                                                                                                                                                                                                                                                                         |
| `Report`             | Fungsional                        | Form parameter → fetch list ter-filter (`per_page: 1000`, tanpa agregasi server sungguhan), grouping+totals dihitung client-side, export CSV via Blob, dan baris totals **dirender**: subtotal per grup (5.13.1) + baris Total keseluruhan (`TotalsRow`). Parameter `select` pada field berakhiran `_id` diperlakukan sebagai relasi (konvensi — `ReportParam` tidak membawa target eksplisit).                                              |
| `Print`              | Fungsional                        | `format: html` dirender klien (`window.print()` + CSS `@page`); `pdf` dan `thermal` dirender **server-side** (`internal/api/print.go` — `renderPrintPDF` dan `renderPrintThermal` ESC/POS 58mm). `dotmatrix` belum ada dan ditolak `501` oleh endpoint.                                                                                                                                                                                      |
| `Theme`              | Lengkap                           | Lihat [`04-theming-assets.md`](04-theming-assets.md) §1.                                                                                                                                                                                                                                                                                                                                                                                     |

## 4. Tier Component

`widget:` adalah **himpunan tertutup**, tercermin di tiga tempat yang dijaga
sinkron oleh test paritas (`src/widgets/catalog.test.tsx`):

| Sumber                   | Isi                                                         |
| ------------------------ | ----------------------------------------------------------- |
| `src/widgets/catalog.ts` | katalog runtime — dipakai dispatch renderer                 |
| `pkg/spec/widget.go`     | sumber schema: `$defs/FormWidget` / `$defs/TableCellWidget` |
| `src/widgets/*.tsx`      | komponen yang benar-benar dirender                          |

- **Form widget (25)** — `input`, `textarea`, `richtext`, `number`,
  `decimalinput`, `select`, `switch`, `radio-group`, `combobox`, `password`,
  `slider`, `tags`, `select-multi-tag`, `uuid`, `json`, `fileinput`,
  `relation-picker`, `datepicker`, `datetimeinput`, `child-grid`,
  `grants-editor`, `hidden`, `qrcode`, `moneyinput`, `timeinput`.
- **Table-cell widget (4)** — `badge`, `boolean`, `image`, `qrcode`. Dua
  kosakata ini sengaja terpisah: widget form pada kolom tabel tidak berarti
  apa-apa bagi sel (dan dulu diabaikan senyap lalu dicetak sebagai teks).

Alias **nama tipe field** (`string`, `text`, `integer`, `decimal`, `boolean`,
`enum`, `date`, `datetime`, `relation`, `child`, `file`) tetap di-routing ke
komponen kanoniknya agar spec lama tidak berhenti dirender, tetapi bukan bagian
katalog resmi: menulisnya di `widget:` ditolak validator dengan petunjuk nama
kanoniknya. Nama di luar himpunan dirender sebagai `UnknownWidget` yang terlihat
(`role="alert"`), bukan jatuh senyap ke input teks polos.

**Widget diturunkan dari Entity, bukan ditulis di Form.** `engine/derive.ts`
`deriveFormWidget(field)` adalah satu-satunya sumber pemetaan field → widget:
baik Form turunan maupun router `FormRenderer` (untuk `kind: Form` yang ditulis)
memanggilnya, sehingga kedua jalur tidak bisa berbeda. Deklarasi Entity yang
menentukan:

| Entity                                      | Widget                 |
| ------------------------------------------- | ---------------------- |
| `options` + `multiple: true`                | `select-multi-tag`     |
| `options` + `multiple: false` (atau skalar) | `select` (ber-caption) |
| `enum_values`                               | `select`               |
| `json`/`string` tanpa `options`             | `json` / `input`       |

Sebelum ini router Form yang ditulis hanya mengenal `money`/`time` sebagai
pemetaan, jadi field `json` ber-`options` jatuh ke editor JSON mentah kecuali
spec menulis `widget: select-multi-tag` — keputusan single/multi dengan demikian
hidup di Form. Permukaan baca mengikuti deklarasi yang sama: sel tabel/halaman
detail menampilkan label opsi (nilai tunggal sebagai badge ber-caption), dan
filter `select` membaca `options` lalu `enum_values` lewat resolver yang sama.

**Gambar → dialog, bukan tab.** `ImageLightbox`
(`src/components/ui/image-lightbox.tsx`) adalah satu-satunya jalur "lihat gambar
lebih besar": thumbnail ber-`aria-label`, lalu `Dialog` berisi gambar
`max-h-[80vh] object-contain` dan judul nama file. Dipakai DetailPage (field
`file`) dan `FileInput` (pratinjau readonly + thumbnail mode edit). Panelnya
`w-full sm:max-w-[92vw]` — default dialog (`sm:max-w-sm`, 384px) lebih sempit
dari foto kafe (960px) sehingga pratinjau turun ke ukuran thumbnail, sementara
`w-auto` lebih buruk: elemen `fixed` pada `left: 50%` shrink-to-fit ke
`viewport − 50%`. Berkas non-gambar tetap tautan unduh bertab. Sel ber-`widget:
image` dan kartu katalog `PickerPanel` **belum** memakai komponen ini (klik di
sana adalah navigasi baris / tambah ke keranjang) — sisa todo 5.21.2 ⏸️.

`kind: Page` dapat menunjuk `asset` (halaman full-code): `PageRenderer`
merender `AssetRenderer` untuk asset yang dideklarasikan — termasuk asset auth
bawaan (`formspec.core/auth/*`). Blok `component:` di dalam komposisi
blocks/tabs masih placeholder (todo 5.9.1).

## 5. Keputusan UX per Renderer

Keputusan yang bukan bagian kontrak (boleh berbeda di renderer lain):

- Konfirmasi delete/void memakai `ConfirmDialog` (shadcn) lewat `UiHost` dan
  renderer, dengan prioritas form override → default App → tanpa dialog;
  bukan `window.confirm()` browser-native.
- Toast notifikasi (`sonner`) dipanggil langsung tiap kind renderer, belum
  lewat satu layanan `formspec.ui` terpusat — lihat
  [`04-theming-assets.md`](04-theming-assets.md) §3.
- Ikon dikunci ke nama pustaka `lucide` (bukan bebas nama seperti disarankan
  spec) — cocok dengan catatan Open Question rencana desain awal, belum
  jadi keputusan final yang tercatat di spec manapun.

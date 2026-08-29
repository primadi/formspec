# Katalog Kind Renderer

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode `renderers/react-shadcn/src/kinds/` hari ini — tingkat
> kelengkapan bervariasi jauh antar kind, ditandai eksplisit per kind.

## 1. Registry

Wiring kind→component **bukan** lewat `engine/registry.tsx` (kode itu ada,
tapi tidak dipanggil dari mana pun — lihat
[`01-architecture.md`](01-architecture.md) §5) — melainkan `lazy()` map
hardcoded di `shell/router.tsx`.

## 2. Tier App

Ketiga archetype App renderer diimplementasikan di shell ini
([`../../spec/frontend/05-app-kinds.md`](../../spec/frontend/05-app-kinds.md)
§1). Pemilihan shell per surface dilakukan di `src/App.tsx` dari
`bundle.app.app_renderer` lewat registry `APP_SHELLS`
(`sidebar-nav` → `AppShell`, `topnav` → `TopNavShell`, `no-nav` →
`NoNavShell`). Auth (`access`) adalah sumbu terpisah — boot anonim saat
`access: public`, boot session saat `private`.

`sidebar-nav` (default): sidebar statis di desktop, overlay slide-in dengan
backdrop di mobile (`useMediaQuery("(max-width: 767px)")`, tertutup otomatis
saat route berganti) — **lengkap**.

`topnav`: chrome penuh dengan navigasi atas — brand + nav horizontal
(item level-1; group → dropdown) + breadcrumb + theme switcher + avatar,
tanpa sidebar kiri. Mobile → hamburger membuka drawer berisi tree yang sama.
Menu di-resolve lewat hook bersama `useResolvedMenu` (sama dengan Sidebar).
Contoh: `examples/arisan/` (`app_renderer: topnav`). **Lengkap.**

`no-nav`: chrome minimal **tanpa navigasi sama sekali** — brand bar + footer

- `Outlet`, tanpa sidebar/breadcrumb, tanpa nav link, tanpa auth controls
  secara default. Komposisi dikontrol `bundle.app.chrome` (frontend/
  05-app-kinds.md §5, di-resolve backend): App opt-in nav link via
  `chrome.nav: menu` dan auth controls via `chrome.auth: links|button`
  (komponen bersama `src/shell/AuthArea.tsx`). Dipakai untuk App `access:
public` (marketing/landing) maupun `private` (kiosk/full-screen — tetap
  di-guard surface boot). Blok `section:` pada `kind: Page` dirender oleh
  `src/components/sections/SectionBlocks.tsx` (hero, feature_grid, card,
  carousel, cta). Contoh: `examples/storefront/` (`no-nav` + `public`),
  `registry/` (`no-nav` + `chrome: {nav: menu, auth: links}`). **Lengkap.**

## 3. Tier Page

Kelengkapan per kind (semua di `kinds/`, kecuali `menu` — dihapus, lihat
[`01-architecture.md`](01-architecture.md) §2):

| Kind                 | Status                               | Catatan                                                                                                                                                                                                                                                                                                             |
| -------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Table`              | Fungsional                           | TanStack Table headless, pagination/sort/search server-side, row action ber-`window.confirm()`. **Gap:** navigasi New/row action hardcode prefiks `/_admin` (lihat [`01-architecture.md`](01-architecture.md) §5); `Form.render` diabaikan sepenuhnya — selalu ke route halaman penuh, tidak pernah `OverlayHost`.  |
| `Form`               | Fungsional                           | react-hook-form + zod (schema dibangun manual per tipe/rule field, bukan dari FormSpecExpr), auto-save debounced 2 detik untuk `two_step_autosave`, CAS `version` terkirim sebagai `If-Match`. **Gap:** tidak ada percabangan khusus 409 — conflict jatuh ke toast error generik, bukan alur refetch.               |
| `Page`               | Fungsional (blocks & tabs)           | Permission-gated per tab/blok, delegasi ke Table/Form. **Gap:** blok `component:` (custom component) murni placeholder teks — lihat [`04-theming-assets.md`](04-theming-assets.md) §2.                                                                                                                              |
| `Dashboard`/`Widget` | Placeholder                          | Layout + filter permission jalan, tapi `MetricWidget` merender `"--"` literal (tidak fetch data sama sekali); chart/list widget semua placeholder teks. Tidak ada library chart terpasang meski tipe chart ada di schema.                                                                                           |
| `Wizard`             | Fungsional, melebihi rencana awal    | Step state di `?step=N`, autosave per-instance ke `localStorage` (`?instance=<uuid>`), hook `on_enter`/`on_next`/`on_prev` best-effort, step type `search_select` dan form, `on_complete` (restart/redirect/banner) lengkap. **Gap:** step type `component:` custom masih placeholder.                              |
| `Kanban`             | Fungsional                           | Drag & drop antar kolom (transisi state, optimistic + 409 snap-back), drag-to-reorder dalam kolom (`position_field`), row action permission-gated, realtime refetch, filter server-side (`select`/`date`/`text`, seed `default`, `today()`), `fixed_filters` immutable, search client-side, `max_cards_per_column`. |
| `Timeline`           | Fungsional                           | Infinite scroll berbasis `IntersectionObserver`, grouping date/month/year/none, field tampilan terkonfigurasi.                                                                                                                                                                                                      |
| `Report`             | Fungsional dengan bug                | Form parameter → fetch list ter-filter (`per_page: 1000`, tanpa agregasi server sungguhan), grouping+totals dihitung client-side, export CSV via Blob. **Bug:** baris totals dihitung tapi baris `<tr>` yang seharusnya menampilkannya kosong — nilainya efektif dibuang, tidak pernah dirender.                    |
| `Print`              | Fungsional untuk `format: html` saja | `window.print()` + CSS `@page`, komposisi header/body/footer dari manifest. `pdf`/`thermal`/`dotmatrix` belum ada kode sama sekali (butuh pipeline server, di luar cakupan renderer ini).                                                                                                                           |
| `Theme`              | Lengkap                              | Lihat [`04-theming-assets.md`](04-theming-assets.md) §1.                                                                                                                                                                                                                                                            |

## 4. Tier Component

Widget field yang **ada**: `TextInput`, `NumberInput`, `Select` (enum),
`Switch` (boolean), `Badge`, `RelationPicker`. Widget yang derivation engine
_tunjuk_ (`formWidget()` di `engine/derive.ts`) tapi **tidak ada
komponennya**: `datepicker` (field `date`/`datetime`), `json`, `child-grid`
(field `child`) — ketiganya jatuh diam-diam ke `TextInput` polos di
`FormRenderer`. Ini kesenjangan nyata antara apa yang derivation engine
_bilang_ harus dipakai vs yang benar-benar dirender — bukan sekadar
"belum ditulis", karena user melihat input teks polos untuk field tanggal,
bukan pesan error atau placeholder yang jujur.

`widget` slot-filling ([`../../spec/frontend/07-component-kinds.md`](../../spec/frontend/07-component-kinds.md)
§2) belum punya implementasi konkret — mengikuti status Dashboard/Widget
placeholder di §3.

`asset` — lihat [`04-theming-assets.md`](04-theming-assets.md) §2 (belum
diimplementasikan).

## 5. Keputusan UX per Renderer

Keputusan yang bukan bagian kontrak (boleh berbeda di renderer lain):

- Konfirmasi delete/void pakai `window.confirm()` browser-native, bukan
  dialog custom shadcn.
- Toast notifikasi (`sonner`) dipanggil langsung tiap kind renderer, belum
  lewat satu layanan `formspec.ui` terpusat — lihat
  [`04-theming-assets.md`](04-theming-assets.md) §3.
- Ikon dikunci ke nama pustaka `lucide` (bukan bebas nama seperti disarankan
  spec) — cocok dengan catatan Open Question rencana desain awal, belum
  jadi keputusan final yang tercatat di spec manapun.

# Katalog Kind Renderer

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode `web/src/kinds/` hari ini — tingkat
> kelengkapan bervariasi jauh antar kind, ditandai eksplisit per kind.

## 1. Registry
Wiring kind→component **bukan** lewat `engine/registry.tsx` (kode itu ada,
tapi tidak dipanggil dari mana pun — lihat
[`01-architecture.md`](01-architecture.md) §5) — melainkan `lazy()` map
hardcoded di `shell/router.tsx`.

## 2. Tier App
`sidebar-nav` (satu-satunya App renderer yang diimplementasikan hari ini):
sidebar statis di desktop, overlay slide-in dengan backdrop di mobile
(`useMediaQuery("(max-width: 767px)")`, tertutup otomatis saat route
berganti) — fitur ini **sudah lengkap**. `topnav`/`landing-page`
([`../../spec/frontend/05-app-kinds.md`](../../spec/frontend/05-app-kinds.md)
§3–§4) belum punya renderer terpisah di kode.

## 3. Tier Page
Kelengkapan per kind (semua di `kinds/`, kecuali `menu` — dihapus, lihat
[`01-architecture.md`](01-architecture.md) §2):

| Kind | Status | Catatan |
|---|---|---|
| `Table` | Fungsional | TanStack Table headless, pagination/sort/search server-side, row action ber-`window.confirm()`. **Gap:** navigasi New/row action hardcode prefiks `/_admin` (lihat [`01-architecture.md`](01-architecture.md) §5); `Form.render` diabaikan sepenuhnya — selalu ke route halaman penuh, tidak pernah `OverlayHost`. |
| `Form` | Fungsional | react-hook-form + zod (schema dibangun manual per tipe/rule field, bukan dari FormaExpr), auto-save debounced 2 detik untuk `two_step_autosave`, CAS `version` terkirim sebagai `If-Match`. **Gap:** tidak ada percabangan khusus 409 — conflict jatuh ke toast error generik, bukan alur refetch. |
| `Page` | Fungsional (blocks & tabs) | Permission-gated per tab/blok, delegasi ke Table/Form. **Gap:** blok `component:` (custom component) murni placeholder teks — lihat [`04-theming-assets.md`](04-theming-assets.md) §2. |
| `Dashboard`/`Widget` | Placeholder | Layout + filter permission jalan, tapi `MetricWidget` merender `"--"` literal (tidak fetch data sama sekali); chart/list widget semua placeholder teks. Tidak ada library chart terpasang meski tipe chart ada di schema. |
| `Wizard` | Fungsional, melebihi rencana awal | Step state di `?step=N`, autosave per-instance ke `localStorage` (`?instance=<uuid>`), hook `on_enter`/`on_next`/`on_prev` best-effort, step type `search_select` dan form, `on_complete` (restart/redirect/banner) lengkap. **Gap:** step type `component:` custom masih placeholder. |
| `Kanban` | **Drag-and-drop TIDAK diimplementasikan** meski komentar header file mengklaim sebaliknya. Tidak ada handler drag, tidak ada import `@dnd-kit` (dependency terpasang di `package.json` tapi **nol pemakaian** di seluruh `web/src`). Kartu statis, tidak bisa di-drag. Fitur nyata yang jalan: layout kolom dari manifest, filter pencarian client-side, pemotongan per-kolom `max_cards_per_column`. |
| `Timeline` | Fungsional | Infinite scroll berbasis `IntersectionObserver`, grouping date/month/year/none, field tampilan terkonfigurasi. |
| `Report` | Fungsional dengan bug | Form parameter → fetch list ter-filter (`per_page: 1000`, tanpa agregasi server sungguhan), grouping+totals dihitung client-side, export CSV via Blob. **Bug:** baris totals dihitung tapi baris `<tr>` yang seharusnya menampilkannya kosong — nilainya efektif dibuang, tidak pernah dirender. |
| `Print` | Fungsional untuk `format: html` saja | `window.print()` + CSS `@page`, komposisi header/body/footer dari manifest. `pdf`/`thermal`/`dotmatrix` belum ada kode sama sekali (butuh pipeline server, di luar cakupan renderer ini). |
| `Theme` | Lengkap | Lihat [`04-theming-assets.md`](04-theming-assets.md) §1. |

## 4. Tier Component
Widget field yang **ada**: `TextInput`, `NumberInput`, `Select` (enum),
`Switch` (boolean), `Badge`, `RelationPicker`. Widget yang derivation engine
*tunjuk* (`formWidget()` di `engine/derive.ts`) tapi **tidak ada
komponennya**: `datepicker` (field `date`/`datetime`), `json`, `child-grid`
(field `child`) — ketiganya jatuh diam-diam ke `TextInput` polos di
`FormRenderer`. Ini kesenjangan nyata antara apa yang derivation engine
*bilang* harus dipakai vs yang benar-benar dirender — bukan sekadar
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
  lewat satu layanan `forma.ui` terpusat — lihat
  [`04-theming-assets.md`](04-theming-assets.md) §3.
- Ikon dikunci ke nama pustaka `lucide` (bukan bebas nama seperti disarankan
  spec) — cocok dengan catatan Open Question rencana desain awal, belum
  jadi keputusan final yang tercatat di spec manapun.

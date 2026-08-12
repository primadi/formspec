# Theming & Assets

**Updated:** 2026-07-16 · Status: Draft

> Draft: isi di bawah kondisi kode hari ini.

## 1. Theme
Implementasi kind `Theme` ([`../../spec/frontend/05-app-kinds.md`](../../spec/frontend/05-app-kinds.md)
§5) sudah lengkap, melebihi cakupan minimal kontraknya:

- `ThemeRenderer.tsx` menyuntik `spec.tokens` sebagai blok `<style>`
  `:root { ... }` (sengaja bukan inline style — inline style akan merusak
  cascade class `.dark`), plus `stylesheet` mentah sebagai `<style>` kedua.
  Pemetaan token → CSS var: `color.*` → `--*`, `radius.*` → `--radius`,
  `font.*` → `--sans`/`--heading`/`--mono` atau fallback `--font-*`.
- `hooks/useTheme.ts` + `stores/prefs.ts` mengimplementasikan toggle
  light/dark/system **independen dari** Theme manifest (listener
  `prefers-color-scheme`) — kalau Theme manifest aktif, preset warna user
  di-skip (Theme manifest menguasai warna) tapi toggle mode tetap jalan.
- `components/ThemeSwitcher.tsx` — dropdown: baris mode (light/dark/system),
  daftar Theme dari `bundle.themes` (preview warna dari
  `tokens["color.primary"]`), plus 6 preset warna (ditampilkan hanya kalau
  tidak ada Theme manifest aktif). Preference persisten ke localStorage
  (`formspec-prefs`).
- Token dasar (`index.css`) adalah token OKLCH shadcn/Tailwind v4 standar
  (`--background`, `--primary`, `--sidebar-*`, `--chart-1..5`) dengan blok
  override `.dark` — bukan custom.

**Ini fitur yang benar-benar jalan**, bukan stub — light/dark/preset
switching bahkan tidak disebut di rencana desain awal.

## 2. Asset Pipeline
**Belum diimplementasikan sama sekali** — bukan sekadar tertinggal, tapi
benar-benar tidak ada kode: tidak ada kontrak `mount(el, props, formspec)`,
tidak ada mekanisme pemuatan script/asset dinamis. Satu-satunya jejak di
kode adalah field tipe `BlockRef.asset` dan tiga teks placeholder literal
("Custom component: {name} ... supported in Fase 4.F6") di `PageRenderer.tsx`
(varian blocks dan tabs) dan `WizardRenderer.tsx` (step custom). Kontrak yang
harus dipenuhi kalau/ketika ini dibangun ada di
[`../../spec/frontend/07-component-kinds.md`](../../spec/frontend/07-component-kinds.md)
§4.

## 3. Layanan `formspec.ui`
**Belum ada** objek `formspec` yang di-inject ke component manapun (karena
component contract sendiri belum ada, §2). Toast (`sonner`) dipakai
langsung di dalam tiap kind renderer lewat `import {toast} from "sonner"` —
bukan lewat layanan `formspec.ui.toast()` yang diekspos ke component eksternal
seperti didesain di
[`../../spec/frontend/07-component-kinds.md`](../../spec/frontend/07-component-kinds.md)
§4. `dialog`/`confirm`/`drawer` juga belum ada bentuk terpusatnya —
konfirmasi destruktif hari ini pakai `window.confirm()` browser-native
(lihat [`03-kind-renderers.md`](03-kind-renderers.md) §5).

## 4. Status Implementasi Hari Ini
Ringkasan: Theme (§1) matang dan siap dipakai; Asset pipeline dan
`formspec.ui` (§2–§3) adalah pekerjaan yang belum dimulai, bukan yang
"hampir selesai" — siapa pun yang merencanakan fitur berbasis custom
component perlu tahu ini benar-benar dari nol, termasuk keputusan desain
seperti bagaimana asset dimuat (bundled saat build web/, atau fetch dinamis
saat runtime dari module) yang belum diputuskan di kontrak
([`../../spec/frontend/07-component-kinds.md`](../../spec/frontend/07-component-kinds.md)
tidak menspesifikasikan mekanisme pemuatan, hanya kontrak `mount()`-nya).

# Plan: Chrome Composition Spec — `App.spec.chrome`

**Status:** In Progress · **Tanggal:** 2026-08-29
**Referensi spec:** `docs/spec/frontend/05-app-kinds.md` §1, §4

## Latar Belakang

`no-nav` shell saat ini hardcode tiga hal di `NoNavShell.tsx`: brand bar, nav
link derived dari menu, dan auth controls (Sign in/Sign up saat anonim,
LogoutButton saat signed-in) + footer. Akibatnya `no-nav` tidak lagi berarti
"tanpa navigasi sama sekali" — App landing/marketing publik ikut menampilkan
Sign in/Sign up yang tidak diminta manifestnya.

## Tujuan

1. `no-nav` = konten murni: tanpa nav, tanpa auth controls secara default.
2. Komposisi chrome jadi deklaratif & general via sub-spec `App.spec.chrome`
   yang **ortogonal** terhadap `app_renderer` (archetype layout) dan `access`
   (sumbu auth).
3. Default di-resolve di backend meta (`internal/ui`) — single source of truth
   untuk semua `stack_family`; frontend membaca nilai efektif, tidak menebak.

## Desain Spec

```yaml
spec:
  app_renderer: no-nav # sidebar-nav | topnav | no-nav (default sidebar-nav)
  access: public # private | public (default private)
  chrome: # opsional — semua field default "auto"
    brand: auto # auto | show | hide
    nav: auto # auto | menu | none
    auth: auto # auto | links | button | none
    footer: auto # auto | show | hide
    breadcrumbs: auto # auto | show | hide
    theme_switcher: auto # auto | show | hide
```

Semantik:

- `auto` = default per archetype (matriks di bawah); nilai eksplisit override.
- `nav: menu` = render link dari menu resolved (leaf ber-route; entri
  `/login`|`/register` tetap dikecualikan); `nav: none` = tanpa nav.
- `auth`:
  - `links` — anonim: link "Sign in" + tombol "Sign up"; signed-in: logout.
  - `button` — anonim: satu tombol "Sign in"; signed-in: logout.
  - `none` — tidak pernah render auth UI. App `private` tetap di-guard
    surface boot (redirect ke login saat anonim); App `public` login hanya
    via URL langsung atau CTA page block.

Matriks default (`auto`) per archetype:

| Chrome elemen  | sidebar-nav | topnav | no-nav |
| -------------- | ----------- | ------ | ------ |
| brand          | show        | show   | show   |
| nav            | menu        | menu   | none   |
| auth           | links       | links  | none   |
| breadcrumbs    | show        | show   | hide   |
| theme_switcher | show        | show   | hide   |
| footer         | hide        | hide   | show   |

Skenario yang tercakup:

| Skenario                          | Spec                                                     |
| --------------------------------- | -------------------------------------------------------- |
| Landing/marketing publik          | `no-nav` + `public` + default (konten murni)             |
| Katalog publik + login (Registry) | `no-nav` + `public` + `chrome: {nav: menu, auth: links}` |
| Kiosk/POS private full-screen     | `no-nav` + `private` + default                           |
| Kiosk dengan logout               | `no-nav` + `private` + `chrome: {auth: button}`          |
| Admin standar                     | `sidebar-nav` + `private` + default                      |
| Internal tool top-nav             | `topnav` + `private` + default                           |

Escape hatch tetap: chrome ekstrem via `asset` custom component; archetype
benar-benar baru via `VisualSpecKind tier: app` (05-app-kinds.md §7).

## File yang Diubah

| File                                                | Perubahan                                                                               |
| --------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `pkg/spec/resources.go`                             | Struct `AppChrome` + field `Chrome` di `AppSpec` + konstanta nilai + validasi enum      |
| `internal/ui/meta.go`                               | `ChromeConfig` (resolved) di `AppSummary`; `resolveChrome()` menerapkan matriks default |
| `internal/api/meta.go`                              | Pass `Spec.Chrome` ke `AppContext`                                                      |
| `renderers/react-shadcn/src/types/manifest.ts`      | Tipe `ChromeConfig` + `AppSummary.chrome`                                               |
| `renderers/react-shadcn/src/shell/AuthArea.tsx`     | Komponen auth bersama (links/button/none × anonim/signed-in)                            |
| `renderers/react-shadcn/src/shell/NoNavShell.tsx`   | Baca chrome; hapus hardcode auth/nav/footer                                             |
| `renderers/react-shadcn/src/shell/SideNavShell.tsx` | `AuthArea` + override breadcrumbs/theme_switcher                                        |
| `renderers/react-shadcn/src/shell/TopNavShell.tsx`  | idem                                                                                    |
| `registry/spec/apps/registry.yaml`                  | `chrome: {nav: menu, auth: links}` — perilaku portal tidak berubah                      |
| `schemas/dist/latest/`                              | Regenerate via `make generate-schema`                                                   |
| `docs/spec/frontend/05-app-kinds.md`                | §4 no-nav + section Chrome Composition                                                  |
| `docs/renderers/shadcn-shell/03-kind-renderers.md`  | Perilaku shell terhadap chrome                                                          |
| `docs/reference/glossary.md`                        | Entri `chrome`                                                                          |

## Dependensi & Urutan

1. Spec Go (`pkg/spec`) → 2. resolusi meta (`internal/ui`, `internal/api`) →
2. tipe frontend → 4. shells → 5. registry.yaml + docs → 6. schemas + tests.

Level of effort: **medium** (spec+backend kecil, frontend sedang, docs).

## Keputusan

- Default `no-nav` benar-benar kosong (keputusan user 2026-08-29).
- Mekanisme: sub-spec `chrome:`, bukan archetype baru / asset-only.
- Resolusi default di backend meta, bukan frontend.
- Nilai chrome tidak dikenal fallback ke default archetype (lenient) —
  validasi ketat ditangani JSON Schema + `formspec validate`.

## Sisa yang belum diputuskan (2026-09-24)

Chrome menjadi **satu-satunya** tempat kontrol auth bisa hidup, dan itu membuat
"hak akses menu" **mandatory-by-omission**: kalau manifest tidak menyatakannya,
pengguna tidak punya jalan masuk/keluar — dan manifest-nya tetap lolos validasi.

Rantai bukti (kafe, diukur 2026-09-24):

1. `AuthArea` hanya dirender di dalam header chrome ketiga shell
   (`NoNavShell.tsx:104`, `SideNavShell.tsx:147`, `TopNavShell.tsx:188`).
2. `AuthArea` → `if (mode !== "links" && mode !== "button") return null`, jadi
   `auth: none` = **nol** kontrol (bukan "default").
3. Kontrol itu ada di dalam `{chrome?.brand !== "hide" && …}` — sehingga
   `chrome: {brand: hide, auth: links}` juga menghapus semua kontrol auth.
4. `auth_action` (satu-satunya deklarasi auth non-chrome) = closed set
   `login, register, change_password, forgot_password, reset_password` —
   **tanpa `logout`**; Page kind tidak punya blok/CTA auth (`SectionCTA` =
   `{label, href, variant}`).
5. `useAutoLogout` tidak dipasang pada permukaan publik (`App.tsx:383`), jadi
   sesinya juga tidak kedaluwarsa sendiri.
6. App `private` + `no-nav` + `chrome: {auth: none}` → `formspec validate`
   **0 problem** (diuji pada salinan spec kafe).

Tiga pertanyaan yang menunggu keputusan pemilik proyek:

1. **Apakah `chrome.auth: none` tetap sah pada App `private`?** Kalau tidak,
   validator harus menolaknya (invarian: "App privat, kecuali yang seluruh
   halamannya `public: true`, wajib punya jalan masuk").
2. **Apakah Page/Form perlu bisa menyatakan aksi auth (khususnya `logout`)**
   lewat `auth_action` dan/atau blok CTA — supaya kontrol auth tidak lagi
   eksklusif milik chrome, dan menu benar-benar bisa jadi dekorasi?
3. **Apa perilaku aman `brand: hide`** — apakah kontrol auth harus keluar dari
   blok brand (mis. selalu render bila `auth != none`), atau `brand: hide` +
   auth aktif ditolak saat validasi?

Terkait (dicatat di kafe TODO, bukan di sini): **10.22 ⏸️** — App privat berisi
0 entity tidak bergerbang (200 + bundle kosong), padahal `_admin` sudah punya
preseden benar (403 `missing permission: _admin.access`).

## Opsi: region + attach (usulan pemilik proyek, 2026-09-24)

Rumusan yang diusulkan: **`no-nav` bukan "tanpa chrome", melainkan archetype
tanpa bar _default_** — topbar/bottombar/sidebar tetap mungkin ada, hanya saja
tidak ada yang dipasang otomatis. `sidebar-nav`/`topnav` = chrome yang sudah
_predefined_. Developer yang ingin topbar kustom **plus** sidebar-menu memakai
`no-nav` + memasang sidebar; tidak perlu archetype baru.

**Kenapa ini lebih baik dari matriks sekarang.** Matriks `chrome: {brand, nav,
auth, footer, breadcrumbs, theme_switcher}` menjawab pertanyaan _"apakah region
predefined ditampilkan?"_ — boolean per region, tertutup. Ia tidak punya bahasa
untuk _"region ini diisi komponen saya"_, sehingga kombinasi "topbar kustom +
sidebar-menu" tidak bisa dinyatakan; satu-satunya jalan adalah `asset` yang
menggantikan seluruh shell. Model region+attach memisahkan **dua sumbu** yang
sekarang tercampur: (1) region mana yang aktif, (2) apa yang mengisinya.

**Yang sudah ada (bukan usulan di atas kertas):**

- `Sidebar` sudah komponen mandiri — props `{collapsed, onToggle, mobile,
mobileOpen, onMobileClose}`, menu dibaca sendiri via `useResolvedMenu()`
  (`hooks/useResolvedMenu.ts`). TopNavShell sudah memakai hook yang sama, jadi
  "menu resolved" memang bukan milik satu shell.
- Preseden **attach komponen ke chrome** sudah ada dan berjalan:
  `App.spec.auth.chrome_auth` (component ref yang menggantikan area auth).
  Model region adalah generalisasi pola itu dari satu region ke N.
- Sistem **slot** sudah dideklarasikan di spec (`accepts_slots` tier
  `page`/`app`, `implements_slot` tier `component`,
  `02-visual-spec-kind.md` §4) — bentuk formal dari "lubang + isi".

**Batas jujur hari ini:**

- Slot itu **belum ada implementasinya di renderer**: `grep implements_slot`
  di `renderers/react-shadcn/src` → **0 hit**; satu-satunya pemakaian adalah
  validasi tier di `internal/manifest/renderer.go:163-168`. Jadi jalur slot
  masih spec-only.
- `kind: App` **bukan** `VisualSpecKind` (punya `AppSpec` sendiri), sehingga App
  belum berpartisipasi di mesin slot.
- Ketiga shell (973 baris) masih hard-coded: `SideNavShell` menaruh brand +
  sidebar-toggle + breadcrumbs + `AuthArea` di dalam **satu** `<header>`, jadi
  memisahkan header itu menjadi region-region mandiri adalah bagian pekerjaan —
  kalau tidak, "attach sidebar ke no-nav" akan menggandakan chrome.

**Dua jalur implementasi:**

| Jalur                                       | Isi                                                                                                                              | Biaya                                                       | Catatan                                                                               |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| **A — region override via component asset** | Generalisasi `chrome_auth` → mis. `chrome.regions: {topbar: assets/…, sidebar: auto\|none\|<asset>}`, `auto` = default archetype | **medium** — infrastruktur asset sudah ada, tanpa kind baru | Menyelesaikan "no-nav + attach sidebar" sekarang; pola yang sama dengan `chrome_auth` |
| **B — slot resmi (`tier: app`)**            | `kind: App` ber-`accepts_slots: [topbar, sidebar, …]`; komponen `implements_slot`                                                | **large** — membangun mesin slot di renderer dari nol       | Bentuk jangka panjang sesuai spec §4; jangan dipilih kalau hanya butuh A              |

**Rekomendasi:** ambil **A** sebagai langkah nyata (memberi kemampuan yang
diminta tanpa membangun mesin baru), dan simpan **B** sebagai formalisasi
setelah ada ≥2 konsumen nyata. Yang perlu diputuskan sebelum A: apakah
`chrome.*` boolean tetap berdampingan (region `auto` = "ikut matriks") atau
digantikan seluruhnya oleh `chrome.regions`.

**Catatan penting soal pertanyaan #2 di atas:** model region membuat kontrol auth
menjadi _region yang bisa diganti/dipasang_ — perbaikan besar dibanding
chrome-only — tetapi ia **tetap hidup di shell, bukan di Page**. Jadi model ini
belum dengan sendirinya menutup keluhan "menu itu dekorasi page, sedangkan page
tidak punya login/logout": untuk itu Page/Form tetap perlu bisa menyatakan aksi
auth (pertanyaan #2), atau auth harus diperlakukan sebagai region yang _selalu_
ada betapa pun `no-nav`-nya.

# Katalog Kind — Tier App

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Semantik Tier App

App renderer (`App.spec.app_renderer`) menentukan **bentuk chrome/navigasi**
subtree — siapa yang menguasai bootstrap
([`01-visual-hierarchy.md`](01-visual-hierarchy.md) §1): chrome penuh
(menu persisten, header) vs minimal (tanpa nav standar). **Ortogonal dengan
auth** (`App.spec.access`): `sidebar-nav`/`topnav`/`no-nav` bisa `public`
(tanpa login) ATAU `private` (perlu login). Keduanya tidak boleh dicampur —
"landing/marketing" adalah _satu kombinasi_ (`no-nav` + `public`), bukan nama
renderer.

| Sumbu              | Field `App.spec`  | Nilai                                                           |
| ------------------ | ----------------- | --------------------------------------------------------------- |
| Chrome/navigasi    | `app_renderer`    | `sidebar-nav` \| `topnav` \| `no-nav` (default `sidebar-nav`)   |
| Auth               | `access`          | `private` \| `public` (default `private` — secure by default)   |
| Shell implementasi | `stack_family`    | `react-shadcn` (default) — see `03-renderer-kind.md`            |
| Backend persist    | `persist_backend` | `jsonb-persist` (default) — see `backend/04-persist-backend.md` |

`access: public` memicu: bundle anonim (`alwaysVisible`) dan data seam
anonim (list/find/create di `/_ui/entity/`). `root_url` kini bebas di dalam
workspace (`/`, `/barbershop`, `/app/kafe`, …) — server me-mount SPA shell
dinamis di setiap `root_url`; `access` tidak lagi membatasi pilihan prefix.
`app_renderer` hanya memilih chrome — tidak menyiratkan public/private.

Navigasi App sendiri (`App.spec.menu`/`Module.spec.menu`, bentuk `MenuItem`,
batas nesting 3 level) adalah kontrak `kind: App`/`kind: Module` — didokumentasikan
di [`../platform/02-workspace-app-module.md`](../platform/02-workspace-app-module.md)
§4, bukan bagian VisualSpecKind tier app. App kind di sini
cuma mengonsumsi menu yang sudah resolved lewat Spec Resolution API
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §2).

## 2. `sidebar-nav`

Chrome penuh dengan navigasi samping — binding ke menu App yang sudah
resolved, responsive (collapse ke overlay di breakpoint mobile).

## 3. `topnav`

Chrome penuh dengan navigasi atas — pola bootstrap yang sama dengan
`sidebar-nav`, beda penempatan chrome saja.

## 4. `no-nav`

Chrome minimal **tanpa navigasi sama sekali**: brand bar + konten + footer,
tanpa sidebar/top-nav, **tanpa nav link, dan tanpa auth controls** secara
default. Ini bukan soal "publik" — auth ditentukan oleh `access`. App
`no-nav` + `private` (kiosk/POS/full-screen) tetap di-guard surface boot
(redirect ke login saat anonim); App `no-nav` + `public`
(marketing/landing/pendaftaran publik) tampil sebagai konten murni — CTA
login (bila perlu) adalah blok `section:` pada `kind: Page`, bukan chrome.
Halaman berisi blok presentasi `section:` ([`06-page-kinds.md`](06-page-kinds.md)
§1) + blok lain. Pasangan alami Page kind `listing`
([`06-page-kinds.md`](06-page-kinds.md) §10) untuk katalog publik.

Skenario yang butuh sebagian chrome (katalog publik dengan login, kiosk
dengan tombol logout) memakai **Chrome Composition** (§5) — bukan default
shell.

```yaml
apiVersion: formspec.dev/v1
kind: App
metadata: { name: storefront, module: core }
spec:
  root_url: /
  app_renderer: no-nav
  access: public
  modules: [catalog]
```

## 5. Chrome Composition

Komposisi elemen chrome dikontrol deklaratif lewat `App.spec.chrome`
(**ortogonal** terhadap `app_renderer` dan `access`). Setiap elemen default
`auto` — artinya default archetype-nya sendiri (matriks di bawah); nilai
eksplisit meng-override. Engine me-resolve nilai efektif di meta API
(`bundle.app.chrome`) — renderer membaca nilai final, tidak menebak.

```yaml
spec:
  app_renderer: no-nav
  access: public
  chrome: # opsional — semua field default "auto"
    brand: auto # auto | show | hide — brand bar (title + logo)
    nav: auto # auto | menu | none — nav link dari menu resolved
    auth: auto # auto | links | button | none
    footer: auto # auto | show | hide
    breadcrumbs: auto # auto | show | hide (sidebar-nav/topnav)
    theme_switcher: auto # auto | show | hide (sidebar-nav/topnav)
```

Semantik `auth`:

| Nilai    | Anonim                                                                                                                | Signed-in |
| -------- | --------------------------------------------------------------------------------------------------------------------- | --------- |
| `links`  | link "Sign in" + tombol "Sign up"                                                                                     | logout    |
| `button` | satu tombol "Sign in"                                                                                                 | logout    |
| `none`   | tanpa auth UI — App `private` tetap di-guard surface boot; App `public` login hanya via URL langsung / CTA page block | —         |

Matriks default (`auto`) per archetype:

| Chrome elemen  | `sidebar-nav` | `topnav` | `no-nav` |
| -------------- | ------------- | -------- | -------- |
| brand          | show          | show     | show     |
| nav            | menu          | menu     | **none** |
| auth           | links         | links    | **none** |
| breadcrumbs    | show          | show     | hide     |
| theme_switcher | show          | show     | hide     |
| footer         | hide          | hide     | show     |

Contoh skenario:

| Skenario                      | Spec                                                     |
| ----------------------------- | -------------------------------------------------------- |
| Landing/marketing publik      | `no-nav` + `public` + default (konten murni)             |
| Katalog publik + login        | `no-nav` + `public` + `chrome: {nav: menu, auth: links}` |
| Kiosk/POS private full-screen | `no-nav` + `private` + default                           |
| Kiosk dengan logout           | `no-nav` + `private` + `chrome: {auth: button}`          |
| Admin standar                 | `sidebar-nav` + `private` + default                      |

Escape hatch chrome ekstrem: custom component via `asset`
([`07-component-kinds.md`](07-component-kinds.md)); archetype benar-benar
baru lewat §7.

## 6. Theme Binding

`kind: Theme` — tampilan sebagai artifact marketplace, di-resolve **per App**
lewat `theme_ref` di `App.spec`
([`../platform/02-workspace-app-module.md`](../platform/02-workspace-app-module.md) §3):

```yaml
apiVersion: formspec.dev/v1
kind: Theme
metadata:
  name: batik-dark
  module: acme-themes
spec:
  tokens:
    color.primary: "#B8860B"
    radius.md: 10px
  stylesheet: assets/batik-dark.css
  widgets: # override skin opsional per widget dasar
    badge: assets/widgets/badge.js
```

Theme dikirim di dalam module → versioned, signed, bisa dijual di
marketplace ([`../platform/07-marketplace.md`](../platform/07-marketplace.md)).
**Theme dipilih per App** (`theme_ref` di manifest App) — beda App bisa beda
Shell dan beda kebutuhan brand; workspace boleh menetapkan default sebagai
fallback untuk App tanpa `theme_ref`. Token bisa di-override lewat `Config`
key di bawah `theme.*`. Theme merestyle
**pustaka component dasar** ([`07-component-kinds.md`](07-component-kinds.md)
§1) — Theme **tidak pernah** mengubah semantik layout atau melewati
visibilitas berbasis permission
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4).

**Tidak ada kosakata styling per-kind (normatif).** Instance
`Page`/`Form`/`Table`/VisualSpecKind lain membawa **nol** field styling di
manifest-nya — tanpa CSS-in-YAML inline, tanpa override warna/spacing
per-instance. Seluruh styling visual hidup **eksklusif** di `kind: Theme` (token
level workspace/App di §6 ini) plus CSS component yang scoped
([`07-component-kinds.md`](07-component-kinds.md) §4). Ini batasan sengaja: ia
mencegah manifest sprawl dan menjaga Theme sebagai satu-satunya seam styling.

## 7. Menambah App Kind Baru

Lewat `VisualSpecKind` `tier: app`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)) — mengikuti kebijakan
Shell baru di [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §4 kalau
yang ditambah adalah asumsi bootstrap yang benar-benar baru (bukan sekadar
variasi chrome dari App renderer yang sudah ada).

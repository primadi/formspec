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

`access: public` memicu: bundle anonim (`alwaysVisible`), data seam anonim
(list/find/create di `/_ui/entity/`), dan App boleh memegang root workspace
(`root_url: /`). `app_renderer` hanya memilih chrome — tidak menyiratkan
public/private.

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

Chrome minimal **tanpa navigasi standar** (brand bar + konten, tanpa
sidebar/top-nav persisten). Ini bukan soal "publik" — auth ditentukan oleh
`access`. Kombinasi umum: `no-nav` + `access: public` untuk halaman
marketing/landing/pendaftaran publik; `no-nav` + `access: private` untuk
kiosk/POS/full-screen app yang tetap butuh login. Halaman berisi blok
presentasi `section:` ([`06-page-kinds.md`](06-page-kinds.md) §1) + blok
lain. Pasangan alami Page kind `listing`
([`06-page-kinds.md`](06-page-kinds.md) §10) untuk katalog publik.

````yaml
apiVersion: formspec.dev/v1
kind: App
metadata: { name: storefront, module: core }
spec:
  root_url: /
  app_renderer: no-nav
  access: public
  modules: [catalog]


## 5. Theme Binding
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
  widgets:                      # override skin opsional per widget dasar
    badge: assets/widgets/badge.js
````

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
level workspace/App di §5 ini) plus CSS component yang scoped
([`07-component-kinds.md`](07-component-kinds.md) §4). Ini batasan sengaja: ia
mencegah manifest sprawl dan menjaga Theme sebagai satu-satunya seam styling.

## 6. Menambah App Kind Baru

Lewat `VisualSpecKind` `tier: app`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)) — mengikuti kebijakan
Shell baru di [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §4 kalau
yang ditambah adalah asumsi bootstrap yang benar-benar baru (bukan sekadar
variasi chrome dari App renderer yang sudah ada).

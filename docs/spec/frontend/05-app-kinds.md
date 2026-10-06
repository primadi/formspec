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

### 1.1 Grant anonim — diturunkan dari permukaan App

`access: public` **tidak** berarti "seluruh module terbuka". App publik
mengekspos tepat apa yang **view-nya benar-benar panggil** — dan itu
**diturunkan**, bukan dideklarasikan ulang:

> **permukaan view** (target menu ∪ `registered_views`) **× `public` per-view**
> → grant anonim (entity + aksi + scope baris).

Tidak ada field `public_entities` di manifest. Sebelumnya ada, dan ia bermasalah
karena menduplikasi fakta yang sudah tersirat di view: Page sudah menyatakan
`public: true`, field child dengan `picker` sudah menyebut entity sumber, Table
sudah memberi `param` route yang menjadi scope baris. Daftar manual kedua kalinya
hanya bisa **drift** — entri yang ditulis tetapi tidak pernah di-fetch klien, atau
view baru yang lupa didaftarkan sehingga jalur yang seharusnya jalan malah 401.

**Aturan turunannya adalah graf FETCH klien, bukan graf referensi spec.** Hanya
konstruk yang membuat renderer mengirim request ke entity API yang memberi aksi:

| Konstruk                                                              | Di-fetch klien    | Aksi                               |
| --------------------------------------------------------------------- | ----------------- | ---------------------------------- |
| Table block / Table / Kanban / Report / Listing / Timeline / Calendar | `list`            | `list`                             |
| Form `mode: create`                                                   | `create`          | `create`                           |
| Form `mode: edit`                                                     | `update` + `find` | `update`, `find`                   |
| Form `mode: view` / kosong                                            | `find`            | `find`                             |
| `context: {source: entity}`                                           | `find` by id      | `find`                             |
| field child `picker.entity`                                           | `list`            | `list`                             |
| `picker.display.price_entity`                                         | `list`            | `list`                             |
| field `relation` yang ter-render (`RelationPicker`)                   | `list` + `find`   | `list`, `find`                     |
| `registered_views: [{entity: X}]`                                     | derived CRUD      | `list`, `find`, `create`, `update` |

Yang **tidak** memberi aksi — dan di sinilah presisinya:

- **`picker.display.category_field`** — chip kategori dibaca dari baris yang sudah
  dimuat, tidak ada request.
- **kolom table ber-relasi** dan **`context`** yang di-inline server (alias
  relasi) — label sudah ada di record.
- **snapshot dan field `computed`** — nilai statis, atau dihitung server saat baca.
- **field `widget: hidden`** — disimpan di state form, tidak di-mount, jadi tidak
  ada kontrol yang fetch.

`delete` **tidak pernah** diberikan implisit: ia operasi administratif yang
tempatnya di App privat.

**`scope` adalah otorisasi per baris untuk pembacaan anonim**, dan ia pun
diturunkan: `param` pada Table block yang nilainya placeholder route
(`{guest_token: ":guest_token"}`) menjadi filter baris yang ditegakkan server —
nilainya datang dari route, jadi klien tidak bisa melebarkannya.

- Parameter yang tidak ada **menolak permintaan** (403). Tidak pernah berarti
  "tanpa filter".
- Nilai scope **menimpa** filter klien pada field yang sama.
- **`scope` tidak bisa digabung dengan `find`.** `find` me-resolve lewat id dan
  scope tidak bisa menjaganya, jadi kombinasi itu akan terlihat terfilter padahal
  mengembalikan record apa pun yang id-nya diketahui — turunan membuang `find`
  pada grant ber-scope.

**Grant adalah _floor_, bukan bypass permission.** Pada route yang sama,
permintaan **sudah terautentikasi** yang memegang permission entity
(`{module}.{plural}.{action}`) tunduk pada permission itu (dan `row_scope`
entity) — scope grant **tidak** diterapkan padanya. Ini penting karena
`/_ui/entity` dipakai bersama: surface POS kasir yang tidak membawa token tamu
tidak boleh ikut terfilter.

Pemanggil yang **tidak** memegang permission itu — anonim, atau sudah
terautentikasi tanpa permission tersebut — jatuh ke grant dan boleh melakukan
persis apa yang boleh dilakukan tamu, tidak lebih; **scope grant ikut berlaku
padanya**. Efektivitasnya `max(permission miliknya, grant publik)`.

Fallback ini bukan kelonggaran: grant publik sudah publik (diturunkan dari view
yang publik, dan dikirim apa adanya ke pengunjung anonim lewat bundle), dan jalur
fallback tetap dibatasi `scope`. Tanpanya, pemanggil yang login justru **lebih
buruk** daripada tamu di permukaan App itu sendiri — halaman katalog yang
sebelumnya terbuka menjadi 404 begitu pengunjung menekan "Sign up" atau login
lewat `chrome.auth`, dan `create` anonim (mis. pesan QR) berhenti bekerja untuk
pelanggan yang sudah mendaftar.

**Dua konsekuensi yang perlu disadari operator.** (1) Menaruh view staf di dalam
App publik ikut membuka data view itu — itulah sebabnya view yang bukan untuk
anonim harus `public: false`. (2) Grant di-union lintas App publik se-workspace
saat registrasi route, karena `/_ui/entity` berskope workspace/module, bukan
per-App.

### 1.2 `registered_views` — permukaan App

`spec.modules` mengatakan modul mana yang di-mount, tetapi **bukan** view mana
yang boleh dibuka. `registered_views` menutup celah itu: **permukaan App yang
dapat diakses = setiap target leaf menu ∪ `registered_views`.** Route di luar
himpunan itu tidak pernah didaftarkan, jadi URL langsungnya menjawab 404 —
walaupun pemanggil memegang permission entity-nya. Menu adalah deklarasi utama
(target `view:` maupun `route:` otomatis terdaftar); `registered_views` adalah
tambahan untuk view yang tidak dijangkau navigasi — terutama App tanpa menu
(no-nav/kiosk) atau view yang dibuka dari halaman lain.

```yaml
spec:
  modules: [cafe-master, cafe-order]
  # menu: []  # opsional — target menu otomatis terdaftar
  registered_views:
    - { view: cafe-order/menu-catalog } # satu view terdaftar (Page/Form/Table/
      # Dashboard/Report/Wizard/Kanban/Timeline/
      # Calendar/Listing/Print/ApprovalInbox/
      # NotificationCenter)
    - { entity: cafe-master/menu-item } # seluruh derived view entity (list/detail/create/edit)
```

Tepat satu dari `entity`/`view` per entri. `entity` menerima `module/entity`
atau `module.entity`; modul **wajib** anggota `spec.modules`. Bentuknya salah →
`validate`/`apply` menolak; view/entity yang tidak ada → ditolak saat resolve
(boot). Tiga state seperti `public_entities` dulu:

| Deklarasi                 | Efek                                                                  |
| ------------------------- | --------------------------------------------------------------------- |
| absen                     | permukaan = target menu saja (perilaku baru; App tanpa menu → kosong) |
| `registered_views: []`    | sama dengan absen untuk gating, tetapi eksplisit                      |
| `registered_views: [...]` | permukaan = target menu ∪ entri-entri ini                             |

**Ini kurasi permukaan (least privilege), BUKAN gerbang otorisasi data.** Route
data (`/_ui/entity/...`) berskope workspace/module, bukan per-App; RBAC dan
grant turunan (`§1.1`) tetap penjaga datanya. Konsekuensinya: entity yang tidak
terdaftar **tetap dikirim** di bundle (agar relasi/picker tetap resolve) dengan
penanda `routable: false` — klien tidak mendaftarkan route-nya, tetapi memilih
entity itu tetap mengembalikan datanya (sesuai permission). Entity non-routable
yang di-navigasi dari picker akan mendarat di 404.

**Satu pengecualian: wizard yang mengikat transisi mewarisi reachability.**
Sebuah `kind: Wizard` yang `spec.entity` + `spec.action`-nya menunjuk sebuah
transisi (`via`) pada entity yang routable **ikut reachable tanpa didaftarkan**
— ia UI untuk transisi itu, bukan tujuan mandiri, dan kalau tidak diwariskan
tombol transisi akan diam-diam jatuh ke penulisan state mentah, melewati input
yang seharusnya dikumpulkan wizard. Pendaftaran eksplisit di
`registered_views` tetap berlaku dan tetap diperlukan untuk wizard yang tidak
mengikat transisi (mis. `entity` + commit create).

Editor grants (`?grants=true`) **tidak** digerbang `registered_views`: ia harus
melihat semua view agar admin bisa memberi grant untuk view yang belum
terdaftar. (Surface `_admin` (`?admin=true`) yang dulu menyertainya **sudah
dipensiunkan** — plan `app-scoped-login.md` D4.)

`formspec check` memberi **warning** (bukan error) bila App memount modul
tetapi tidak punya menu maupun `registered_views` — tidak ada yang dapat
diakses; legal (App yang di-stage), tetapi mudah terjadi tanpa sengaja.

Navigasi App sendiri (`App.spec.menu`/`Module.spec.menu`, bentuk `MenuItem`,
batas nesting 3 level) adalah kontrak `kind: App`/`kind: Module` — didokumentasikan
di [`../platform/02-workspace-app-module.md`](../platform/02-workspace-app-module.md)
§4, bukan bagian VisualSpecKind tier app. App kind di sini
cuma mengonsumsi menu yang sudah resolved lewat Spec Resolution API
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §2).

## 2. `sidebar-nav`

Chrome penuh dengan navigasi samping — binding ke menu App yang sudah
resolved, responsive (collapse ke overlay di breakpoint mobile).

Tree menu yang lebih tinggi dari viewport harus tetap bisa dijangkau seluruhnya:
nav dibungkus area scroll yang tingginya dibatasi sisa tinggi sidebar (di
bawah header brand), sehingga vertical scroll bar muncul saat item tidak
seluruhnya terlihat. Item yang melimpah **tidak boleh** hanya meluber keluar
sidebar tanpa cara menjangkaunya.

## 3. `topnav`

Chrome penuh dengan navigasi atas — pola bootstrap yang sama dengan
`sidebar-nav`, beda penempatan chrome saja.

## 4. `no-nav`

Archetype **tanpa bar default**: brand bar minimal + konten + footer, tanpa
sidebar/top-nav, **tanpa nav link, dan tanpa auth controls** secara default.

`no-nav` **bukan** "tanpa chrome" — chrome tetap ada, hanya tidak ada region
yang diisi otomatis. Region tetap mungkin ditambahkan (§5): developer boleh
mengisi topbar/sidebar/rightbar/bottombar sendiri. `sidebar-nav` sendiri
adalah `no-nav` + sidebar yang diisi menu; `topnav` adalah `no-nav` + topbar
yang diisi menu. Ketiganya hanyalah **preset** di atas satu peta region yang
sama (§5).

Ini bukan soal "publik" — auth ditentukan oleh `access`. App
`no-nav` + `private` (kiosk/POS/full-screen) tetap di-guard surface boot
(redirect ke login saat anonim); App `no-nav` + `public`
(marketing/landing/pendaftaran publik) tampil sebagai konten murni — CTA
login (bila perlu) adalah blok `section:` pada `kind: Page`, bukan chrome.
Halaman berisi blok presentasi `section:` ([`06-page-kinds.md`](06-page-kinds.md)
§1) + blok lain. Pasangan alami Page kind `listing`
([`06-page-kinds.md`](06-page-kinds.md) §10) untuk katalog publik.

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

Chrome adalah **himpunan region tetap**, bukan shell hardcode. Setiap App
memiliki region berikut:

| Region      | Posisi                    | Default archetype                           |
| ----------- | ------------------------- | ------------------------------------------- |
| `topbar`    | atas, full-width          | `auto` di ketiganya                         |
| `sidebar`   | kiri, di bawah topbar     | `auto` hanya `sidebar-nav`                  |
| `rightbar`  | kanan, di bawah topbar    | `none`                                      |
| `bottombar` | bawah, di atas footer     | `none`                                      |
| `footer`    | paling bawah, full-width  | `auto` hanya `no-nav`                       |
| `content`   | tengah (implicit, Outlet) | selalu ada, **tidak bisa** diganti manifest |

Isi setiap region salah satu dari:

- `none` — region tidak dirender;
- `auto` — isi predefined archetype (brand/menu/breadcrumb/auth);
- `<component-ref>` (`module/name`) — komponen `tier: component` yang
  mendeklarasikan `implements_slot: <region>` ([`02-visual-spec-kind.md`](02-visual-spec-kind.md) §4).

**Archetype = preset.** `sidebar-nav` adalah `no-nav` + `sidebar: auto`;
`topnav` adalah `no-nav` + `topbar: auto`. Ketiganya mengisi region yang sama
lewat peta yang sama, sehingga "topbar kustom **plus** sidebar-menu bawaan"
bisa dinyatakan tanpa archetype baru:

```yaml
spec:
  app_renderer: no-nav # mulai dari krom kosong
  chrome:
    regions:
      topbar: storefront/components/topbar # komponen sendiri
      sidebar: auto # sidebar-menu bawaan
```

Override eksplisit menang atas preset; `none` adalah cara **menghapus** region
predefined (mis. `sidebar: none` pada `sidebar-nav`). Flag boolean
`brand`/`nav`/`auth`/`footer`/`breadcrumbs`/`theme_switcher` tetap berlaku
sebagai **gula** yang menyetel **isi** region `auto`; `regions.footer`
mencerminkan `footer` dan tidak pernah berbeda.

```yaml
spec:
  app_renderer: no-nav
  access: public
  chrome:
    regions: # opsional — semua region default ke preset archetype
      topbar: auto # none | auto | <component-ref>
      sidebar: none
      rightbar: none
      bottombar: none
      footer: auto
    # gula (opsional): menyetel ISI region auto
    brand: auto # auto | show | hide — brand bar (title + logo)
    nav: auto # auto | menu | none — nav link dari menu resolved
    auth: auto # auto | links | button | none — kontrol auth ANONIM
    breadcrumbs: auto # auto | show | hide
    theme_switcher: auto # auto | show | hide
```

Engine me-resolve nilai efektif di meta API (`bundle.app.chrome.regions`) —
renderer membaca peta final, tidak menebak.

**Kontrol sesi tidak digerbangi `chrome`.** `chrome.auth` mengatur **titik masuk
anonim** saja (Sign in/Sign up). Bila ada sesi (token), user menu (→ Sign out)
**selalu** dirender, terlepas dari `chrome.auth` — sehingga sebuah App tidak
pernah bisa mengurung pengguna tanpa jalan keluar. Ini menutup kafe **10.18**.
Region yang menjadi rumah kontrol itu tetap ditentukan `regions` (mis. `no-nav`

- `regions.topbar: auto` memberi brand bar yang memuat user menu).

Semantik `auth` (anonim):

| Nilai    | Anonim                            |
| -------- | --------------------------------- |
| `links`  | link "Sign in" + tombol "Sign up" |
| `button` | satu tombol "Sign in"             |
| `none`   | tanpa auth UI anonim              |

Matriks default (`auto`) per archetype:

| Region         | `sidebar-nav` | `topnav` | `no-nav` |
| -------------- | ------------- | -------- | -------- |
| topbar         | `auto`        | `auto`   | `auto`   |
| sidebar        | `auto`        | `none`   | `none`   |
| rightbar       | `none`        | `none`   | `none`   |
| bottombar      | `none`        | `none`   | `none`   |
| footer         | `none`        | `none`   | `auto`   |
| nav link       | menu          | menu     | none     |
| auth (anonim)  | links         | links    | none     |
| breadcrumbs    | show          | show     | hide     |
| theme_switcher | show          | show     | hide     |

Contoh skenario:

| Skenario                       | Spec                                                                  |
| ------------------------------ | --------------------------------------------------------------------- |
| Landing/marketing publik       | `no-nav` + `public` + default (brand bar saja)                        |
| Katalog publik + login         | `no-nav` + `public` + `chrome: {nav: menu, auth: links}`              |
| Kafe QR (brand bar + sesi)     | `no-nav` + `public` + `regions: {topbar: auto, footer: auto}`         |
| Kiosk/POS private full-screen  | `no-nav` + `private` + default                                        |
| Kiosk dengan logout            | `no-nav` + `private` + `chrome: {auth: button}`                       |
| Topbar kustom + sidebar bawaan | `no-nav` + `regions: {topbar: acme/components/topbar, sidebar: auto}` |
| Admin standar                  | `sidebar-nav` + `private` + default                                   |

Escape hatch chrome ekstrem: custom component via `asset`
([`07-component-kinds.md`](07-component-kinds.md)) mengisi satu region;
archetype benar-benar baru lewat §7.

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

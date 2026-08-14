# Katalog Kind — Tier Component

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Setiap kind di sini adalah instance
> VisualSpecKind `tier: component`
> ([`02-visual-spec-kind.md`](02-visual-spec-kind.md)).

## 1. Base Component Library
Shell resmi wajib menyediakan pustaka component dasar yang **closed, themeable**.
Registry widget dasar adalah **himpunan tertutup yang dienumerasi eksplisit** —
bukan daftar terbuka yang boleh tumbuh informal. Widget input form yang wajib
disediakan (kontrak value/validation/permission per field mengikuti field spec
Entity, [`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1):

`textinput`, `numberinput`, `decimalinput`, `dateinput`, `datetimeinput`,
`select`, `relation-picker`, `child-grid`, `fileinput` (§1.1), `toggle`,
`textarea`, `json-editor`, `richtext` (§1.2).

Di luar widget input, pustaka dasar juga menyediakan tabs, badge, card,
empty-state, breadcrumb, skeleton/loading, dan pagination. **Himpunan dasar ini
tidak tumbuh secara informal**: widget baru ditambahkan dengan mendaftarkan
`VisualSpecKind` baru ber-`tier: component`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md) §2, §6) — bukan dengan
memperluas daftar di atas secara ad-hoc. Component custom
([`../frontend/07-component-kinds.md`](07-component-kinds.md) §4 di bawah) boleh
menyusun ulang lewat `formspec.components` — bukan menulis ulang widget dasar dari
nol. Restyle tampilan pustaka ini adalah urusan `kind: Theme`
([`05-app-kinds.md`](05-app-kinds.md) §5) — Theme tidak pernah mengubah semantik
layout atau melewati visibilitas berbasis permission (§ Spec Resolution API —
[`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4).

### 1.1 `fileinput` — Upload / Attachment
Widget untuk field bertipe `file`/`attachment` (single atau multi — ditentukan
field spec Entity, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§1). Upload mengalir ke primitive **`storage`** (`ctx.storage`, dilayani
Datastore ber-`serves: [storage]` — s3/minio/fs,
[`../platform/06-datastore.md`](../platform/06-datastore.md) §2); file
tenant-isolated seperti semua data.

- **Preview** per tipe umum: gambar inline (thumbnail), PDF viewer embed,
  lainnya jadi tombol download.
- **Batas ukuran & tipe** yang diizinkan dibaca dari field rules — ditegakkan
  client untuk UX, server tetap otoritas
  ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §3).
- Tray upload/download disediakan renderer (`formspec.files`, §4) — `fileinput`
  adalah widget dasar, bukan kind tersendiri.

### 1.2 `richtext` — Rich Text
Widget untuk field bertipe `richtext`. Disimpan sebagai **HTML tersanitasi**:
sanitisasi server-side bersifat **normatif** — backend melucuti
script/markup berbahaya saat tulis, terlepas dari klien mana yang mengirim
(payload dari klien tak jujur tetap tersanitasi server,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §3).

- Toolbar dasar: bold/italic, list (ordered/unordered), link, heading. **Bukan
  page builder** — tanpa layout multi-kolom, embed, atau blok kompleks.
- HTML yang dirender ke pembaca sudah tersanitasi server; klien tak pernah
  mempercayai HTML mentah.

## 2. `widget` — Component Pengisi Slot
Component yang mengisi slot `widget` milik Page tier (mis. Dashboard —
[`06-page-kinds.md`](06-page-kinds.md) § Dashboard), dideklarasikan sebagai
`VisualSpecKind` `tier: component` dengan `implements_slot: widget`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md) §4):

```yaml
apiVersion: formspec.dev/v1
kind: Widget
metadata:
  name: gl-cashflow-chart
  module: gl
spec:
  size: { w: 2, h: 1 }
  chart: { type: line, entity: gl-cashflow-summary, x: date, y: net, range: 30d }
```

Widget bisa dikontribusikan module manapun (bukan cuma module pemilik
Dashboard yang memasangnya) — konsisten dengan prinsip "write once, siapa
saja bisa menyediakan implementasi". **Visibilitas di katalog widget
diturunkan otomatis**: user melihat sebuah widget di katalog hanya kalau ia
punya permission `list`/`view` atas entity/action yang mendasarinya — sama
seperti aturan visibilitas Spec Resolution API
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4), bukan flag
visibilitas terpisah yang ditulis manual.

Widget bawaan (`stat`, `chart`) membaca **summary entity atau `list` action
saja** — agregasi custom jadi summary entity yang diisi event durable
([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §6),
bukan query ad-hoc dari widget.

## 3. Slot Filling di Instance
Instance Page mereferensikan Widget ke posisi slot lewat `layout`/`widgets`
milik Page tersebut (lihat [`06-page-kinds.md`](06-page-kinds.md) § Dashboard
untuk kontrak lengkap Dashboard sebagai penerima slot, dan
[`02-visual-spec-kind.md`](02-visual-spec-kind.md) §4 untuk kontrak slot
system-nya):

```yaml
kind: Dashboard
spec:
  widgets:
    - stat:  { title: "Today's Revenue", entity: sales-daily-summary, field: total }
    - chart: { type: line, entity: sales-daily-summary, x: date, y: total, range: 30d }
    - component: { asset: billing/assets/heatmap.js }   # §4 — full-custom widget
```

**Dashboard customizable:** kalau `spec.customizable: true`, layout user
(tambah/hapus/urutkan dari katalog widget) tersimpan sebagai *runtime
preference* di `formspec.core` — **manifest mendefinisikan apa yang mungkin;
preference mencatat apa yang dipilih.** Tidak pernah ditulis balik ke YAML.

## 4. `asset` — Escape Hatch Component
Untuk ~20% UI yang tidak berpola. Component adalah **ES module** di
`assets/`, kontrak mount framework-agnostic:

```js
// modules/billing/assets/payment-timeline.js
export default {
  mount(el, props, formspec) { /* render ke el */ },
  unmount(el) { }
}
```

`formspec` adalah client yang di-inject: **`formspec.api`** (generated, typed —
berjalan sebagai user yang login, seluruh keamanan tetap server-side),
`formspec.subscribe(entity, cb)` (realtime,
[`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5),
`formspec.navigate(page, params)`, `formspec.theme` (token), **`formspec.ui`**
(`toast`, `dialog`, `confirm`, `drawer`), dan `formspec.files` (upload/download
tray — infrastruktur renderer, bukan kind tersendiri).

**`needs:`— `uses`-nya frontend.** Component itu opaque bagi derivasi
footprint, jadi component yang memanggil `formspec.api` **wajib** mendeklarasikan
apa yang ia sentuh di tempat ia dipasang:

```yaml
- component:
    asset: billing/assets/checkout-wizard.js
    needs:
      actions: [order.create, order.checkout, customer.find]
      subscribe: [billing.order]
```

> **Open — `needs:` belum didukung skema `BlockRef`.** Deklarasi `needs`
> belum ada di `pkg/spec` (field `BlockRef` saat ini: `ref`/`asset`/`id`/`mode`/
> `param`/`props`) — ditracking di `docs/plan/todo.md`. Enforcement footprint
> `uses` di sisi backend sudah berjalan; deklarasi `needs` di manifest adalah
> target kontrak berikutnya.

Panggilan `formspec.api` di luar `needs` gagal client-side (dan memang tidak
pernah diotorisasi server-side juga). `formspec validate` memperingatkan
deklarasi yang tidak dipakai.

**Batas sandbox CSP (normatif, bukan anjuran).** Component custom yang dimuat
lewat escape hatch `asset` dikungkung Content-Security-Policy: `connect-src`
dibatasi **hanya ke origin App itu sendiri**. Konsekuensi yang mengikat:
component **dilarang** membaca state global `window`/`document` di luar
container-nya sendiri, dan **dilarang** melakukan fetch/connect ke endpoint
apa pun selain lewat client `formspec.api` yang di-inject. Ini batas keamanan atas
escape hatch, bukan pedoman opsional — jalur data satu-satunya keluar dari
component adalah `formspec.*`.

**CSS bundle scoped ke container.** Bundle CSS sebuah component custom
di-inject **scoped ke container-nya sendiri** (mis. CSS Modules, Shadow DOM,
atau mekanisme scoping setara) — CSS component **tidak pernah** boleh bocor ke
chrome Page/App di sekitarnya atau ke component lain. Ini konsisten dengan
prinsip styling terpusat di `kind: Theme`
([`05-app-kinds.md`](05-app-kinds.md) §5).

**Headless Form Engine.** `formspec.form(entity, { mode, id? })` mengembalikan
instance form **headless**: field state, dirty tracking, validasi client dari
field rules, evaluasi FormSpecExpr
([`08-formaexpr.md`](08-formaexpr.md)), dan `submit()` yang sudah terhubung ke
action yang tepat (create/update, dengan CAS `version`). Tanpa layout, tanpa
widget — developer menguasai 100% markup. Tangga kontrol penuh: Form
terkelola → custom widget → component → full-custom Page → headless → raw
`formspec.api`.

**Unmanaged client** (Flutter, native, SPA lain) adalah **konsumen API
kelas satu hari ini**: HTTP, realtime WebSocket, permission server-enforced,
typed client tergenerate (target codegen resmi: TypeScript dan Dart). Tidak
ada satupun di dokumen ini yang wajib dipenuhi client semacam itu.

## 5. Menambah Component Kind Baru
Lewat `VisualSpecKind` `tier: component`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)); `formspec apply` menolak
`implements_slot` dari tier selain `component`. Distribusi lewat marketplace
([`../platform/07-marketplace.md`](../platform/07-marketplace.md)), sama
seperti menambah VisualSpecKind tier lain.

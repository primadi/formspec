# Katalog Kind — Tier Page

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Setiap kind di sini adalah instance
> VisualSpecKind `tier: page` — skema shell-agnostic, satu definisi untuk
> semua shell.

## 1. `kind: Page` — Routing & Komposisi
Layar ber-route yang menyusun blok. Ini kind dasar tier page — Form/Table
(§2, §3) sendiri **tidak** independen routable, cuma tampil sebagai blok di
dalam sebuah Page atau lewat route CRUD per-entity turunan framework.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Page
metadata:
  name: order-detail
  module: billing
spec:
  route: /orders/:id
  title: "Order {order.number}"
  blocks:
    - form:  { ref: order-edit, entity: order, id: ":id", mode: view }
    - table: { ref: order-payments, param: { order_id: ":id" } }
    - component:
        asset: billing/assets/payment-timeline.js
        props: { order_id: ":id" }
  layout: { columns: 2 }
```

**Aturan:** route unik per App; `:params` satu-satunya sintaks route dinamis;
blok mereferensikan Form/Table lewat nama atau meng-inline component
([`07-component-kinds.md`](07-component-kinds.md) §4). **Full-custom page** =
satu entry `component:` tanpa blocks/tabs.

**Varian tabs** — beberapa sub-layar dalam satu route:
```yaml
spec:
  route: /settings
  tabs:
    - { label: General,  form:  { ref: settings-general } }
    - { label: Tax,      form:  { ref: settings-tax } }
    - { label: Products, table: { ref: product-list } }
```
`blocks` dan `tabs` mutually exclusive. Renderer memperlakukan tiap tab
sebagai resource yang di-permission-check independen.

**Pola Tabbed Resources:** untuk app dengan banyak master-data kecil (jenis
kelamin, status pernikahan, spesialisasi), memberi tiap satu entry menu
sendiri bikin sidebar berantakan. Kelompokkan resource kecil terkait di bawah
satu `kind: Page` ber-`tabs` — satu entry menu, satu route, sub-layar
terorganisir. Ini keputusan pengelompokan design-time.

**Pola Configuration Page:** untuk setting sistem (parameter key-value yang
strukturnya dikunci developer, nilainya diubah admin) — `kind: Page`
ber-`tabs`, tiap tab mereferensikan `kind: Form` `mode: edit` atas Entity
`characteristic: reference` dengan `id` sentinel (misal `"0"`). Renderer
**tidak boleh** merender tombol "New Item"/"Delete" untuk Entity reference —
hanya action Update yang disurfacekan.

Backend otomatis mendukung **find-or-create** untuk pola ini: ketika
`GET /{entity}/{id}` gagal karena record belum ada, framework mencari record
yang sudah ada untuk workspace tersebut. Jika tidak ada, framework auto-create
record baru dengan nilai default dari entity spec. Lihat
[`backend/01-core-basic.md`](../backend/01-core-basic.md) §1.1 untuk detail
implementasi.

### 1.1 Master-detail (split view)
Pola dua blok bersisian di mana seleksi baris pada blok list menggerakkan blok
detail — tanpa navigasi route. Deklaratif lewat `binds` pada blok detail; bukan
kind baru, cuma pola komposisi Page:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: order-workbench, module: billing }
spec:
  route: /orders/workbench
  layout: { mode: split }               # master kiri (sempit), detail kanan (lebar)
  blocks:
    - table: { ref: order-list, entity: order }          # master — sumber seleksi
    - form:
        ref: order-detail
        entity: order
        binds: { source: order-list, param: id }         # detail mengikuti seleksi
```

**`binds`:**
- `source` — nama blok list (Table/listing) yang jadi sumber seleksi.
- `param` — field record terpilih yang diinjeksikan ke blok detail sebagai
  konteksnya (biasanya `id`, menggantikan peran `:id` route).
- Blok detail refetch saat seleksi berubah; tanpa seleksi ia menampilkan
  empty-state.

`layout.mode: split` membagi master + detail bersisian; tanpa mode ini blok
tersusun vertikal biasa (default). Common case: order + order lines, journal +
entries, master data + panel detail. Enforcement permission tetap **per-blok**
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4) — split cuma
menautkan seleksi, tidak melonggarkan gating.

## 2. `data-entry` (`kind: Form`)
Layout + perilaku input satu Entity, menggantikan form hasil derivasi:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Form
metadata:
  name: order-edit
  module: billing
spec:
  entity: order
  mode: edit                    # create | edit | view
  render: separate_page         # modal | drawer | separate_page (default: modal)
  layout:
    sections:
      - title: Customer
        columns: 2
        fields:
          - { field: customer_id, widget: relation-picker }
          - { field: member_tier, readonly: true }
      - title: Totals
        visible_when: "fields.items != null and len(fields.items) > 0"
        fields:
          - { field: total, readonly: true,
              compute: "sum([i.quantity * i.price for i in fields.items])" }
  actions:
    - { action: checkout, label: "Checkout", style: primary }
```

**`render` — keputusan container design-time**, bukan runtime:

| `render` | Perilaku | Kapan dipakai |
|---|---|---|
| `modal` (default) | Dialog overlay di atas Page saat ini; route tak berubah, state di baliknya tetap ada | Entity ringan (≤5 field), create/edit cepat tanpa kehilangan konteks list |
| `drawer` | Panel slide-in dari kanan, sama sifatnya dengan modal tapi lebih lebar | Form medium (5–12 field), khususnya `columns: 2` |
| `separate_page` | Route sendiri, breadcrumb + URL sendiri | Entity padat (12+ field, child table, validasi kompleks), butuh deep-link |

Entity yang sama boleh punya banyak Form dengan `render` berbeda (mis. modal
quick-create + separate_page full-edit) — keputusan per-Form, ditegakkan
renderer (tidak ada switch runtime; butuh mode lain → deklarasikan Form
kedua).

**Aturan:** tiap `field` wajib ada di Entity; tiap `action` wajib ada dan
permission-gated otomatis
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4). Vocabulary
perilaku client tertutup: `visible_when`, `readonly_when`, `required_when`,
`compute` (FormaExpr, [`08-formaexpr.md`](08-formaexpr.md)) — begitu butuh
efek imperatif, field itu jadi custom widget
([`07-component-kinds.md`](07-component-kinds.md) §4). `rules` field dari
Entity manifest ditegakkan client-side untuk UX; **validasi server-side
tetap otoritas — cek client bukan pernah keamanan.**

### 2.1 Pola UI: Lifecycle vs Plain CRUD
Renderer memilih pola UI berdasar apakah reserved action `submit` aktif di
Entity ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1.2)
— **bukan** berdasar `characteristic: transaction` (dua flag itu independen:
`characteristic` murni soal periode akuntansi, pola UI murni soal apakah
lifecycle draft→submit bermakna secara bisnis).

```
submit dinonaktifkan eksplisit
  → Plain CRUD: satu tombol "Save", tanpa tombol Submit,
    tanpa konsep draft ditampilkan (doc_status null)

submit AKTIF (default)
  → Pilih satu dari tiga pola, lewat hint manifest `ui:`.
    Default kalau tidak dideklarasikan: 2-step + auto-save.
```

| Pola | Kapan dipakai | UI |
|---|---|---|
| **2-step + auto-save** (default) | Entity kompleks, butuh review (Invoice, Order, Contract) | Auto-save senyap saat draft (debounced `update`), satu tombol "Submit" eksplisit |
| **2-step manual** | Draft sengaja dipisah untuk direview orang lain dulu | Tombol "Save Draft" + "Submit" terpisah |
| **1-step (`create-submit`)** | Entry cepat volume tinggi (POS, antrean klinik) | Satu tombol, pakai reserved action `create-submit` ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1.2) — tanpa konsep draft di UI, atomik |

Dua tombol standar (Save Draft/auto-save, Submit) selalu otomatis tersedia
dari model tanpa perlu dideklarasikan; `create-submit` menambah jalur cepat
opsional, bukan mengganti keduanya.

## 3. `table-list` (`kind: Table`)
Daftar ber-filter/sort/paginasi; kolom terderivasi dari entity:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Table
metadata: { name: order-list, module: billing }
spec:
  entity: order
  columns:
    - { field: number, link: order-detail }
    - { field: customer.name }
    - { field: total, format: currency }
    - { field: status, widget: badge }
  filters: [status, created_at]
  default_sort: { field: created_at, direction: desc }
  search: true
  realtime: true
  row_actions: [mark-paid, void]
  bulk_actions: [export]
```

`realtime: true` = auto-subscribe + patch baris di tempat
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5). `row_actions`/
`bulk_actions` permission-gated otomatis, sama seperti action Form.

### 3.1 Prioritas & Overflow Kolom (derivasi — normatif)
Table tanpa `columns:` eksplisit menderivasi kolomnya dari entity. Derivasi
**tidak boleh** membuang field secara diam-diam. Default normatif:

- Renderer menampilkan **N kolom prioritas pertama** (N menyesuaikan lebar
  viewport). Urutan prioritas: natural key → field title-ish
  (`label_field`: `name`/`title`/`number`,
  [`04-spec-resolution-api.md`](04-spec-resolution-api.md) §2) → field status/
  state machine → `transaction_date` → sisanya sesuai urutan deklarasi field di
  Document.
- Field sisa yang tak muat **tetap terjangkau** lewat row detail/expand (klik
  baris membuka Page detail derived, atau baris di-expand) — tak pernah hilang.
- `columns:` eksplisit **menang penuh**: developer memilih persis kolom dan
  urutannya, tanpa overflow otomatis di atasnya — daftar 15 kolom dirender 15
  (horizontal scroll bila perlu), keputusan sadar.

Renderer **dilarang** memotong keras daftar kolom derived (mis. berhenti di 8
kolom) sehingga field lain tak pernah bisa dilihat — pemotongan tanpa jalan
akses balik adalah data-loss, bukan layout.

### 3.2 Inline & Batch Editing
Opsional, opt-in per Table:

```yaml
kind: Table
spec:
  entity: product
  inline_edit: true
  batch_edit: [price, category_id]
```

**`inline_edit: true`** — sel bisa disunting in-place. Kolom yang editable
dibatasi field yang rules-nya mengizinkan (**derived**: field
`readonly`/`compute`/`immutable`, atau di luar permission `update`, tidak
editable). Commit sel = action `update` biasa per baris, membawa `version` (CAS,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5) — mismatch →
`409 CONFLICT`, baris ditandai stale, tak pernah menimpa senyap. Guard lifecycle
tetap berlaku: baris `submitted` menolak inline-edit (update ditolak server,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1.2).

**`batch_edit: [field, ...]`** — pilih beberapa baris → set nilai satu/lebih
field → framework mengeksekusi action `update` **per baris**, tiap baris
divalidasi server-side independen (rules, guard, CAS). **Partial failure
dilaporkan per baris**: baris yang gagal ditampilkan dengan alasannya, baris
sukses tetap commit — tak pernah all-or-nothing diam-diam, tak pernah menelan
error. Permission = permission `update` entity; baris yang caller tak berhak
tak masuk seleksi editable.

## 4. `kanban`
Papan kolom drag-drop — instance VisualSpecKind `tier: page`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)), dan contoh unggulan
kontrak itu. Operasional: tiap kartu satu record entity, tiap kolom satu nilai
status, drag antar kolom = transisi state.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Kanban
metadata: { name: support-board, module: helpdesk }
spec:
  entity: ticket
  realtime: true
```

**Zero-config:** dengan hanya `entity`, renderer menderivasi kolom dari state
machine bisnis entity ([`../backend/02-core-extended.md`](../backend/02-core-extended.md)
§1) — satu kolom per nilai `state_machine.field`, urut sesuai urutan transisi;
kartu menampilkan field prioritas sama seperti derivasi kolom Table (§3.1).
Board langsung jalan tanpa manifest tambahan.

**Derivasi kolom:**
- Default: kolom = nilai field state machine (§ core-extended §1). Kalau entity
  tak punya state machine tapi punya field enum, `group_by: <field>` memakai
  nilai enum sebagai kolom.
- `columns:` eksplisit **menang penuh** (nilai, urutan, dan `wip_limit`
  per-kolom).

**Derivasi kartu:** field kartu diderivasi seperti prioritas kolom Table (§3.1)
— natural key/title-ish, `transaction_date`; field status implisit (sudah jadi
kolom, tak diulang di kartu). Override lewat `card_fields:`.

**Drag-drop = state transition:**
- Menjatuhkan kartu ke kolom lain memanggil action `via` transisi yang cocok
  (`from` = kolom asal, `to` = kolom tujuan). **Guard state machine dievaluasi
  server-side — otoritas.** Permission drag = permission action transisi itu
  ([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4); caller tanpa
  permission itu tak bisa men-drag kartu ke kolom tersebut.
- Transisi yang tak dideklarasikan → tak ada drop target; kalaupun dipaksa,
  server menolaknya (`STATE_TRANSITION_ERROR`, § core-extended §1).
- `drag_guard` (FormaExpr opsional, [`08-formaexpr.md`](08-formaexpr.md)) —
  pre-check UX sebelum drop, mencegah drop yang pasti ditolak. Konteks evaluasi
  = field record kartu (§ [`08-formaexpr.md`](08-formaexpr.md) §3). UX-only;
  validasi server tetap otoritas (§ [`08-formaexpr.md`](08-formaexpr.md) §4).

**WIP limit:** `wip_limit` per kolom (opsional) — batas jumlah kartu; kolom
penuh menolak drop di UI (soft, pre-check UX), server tak menegakkan batas ini.

**Realtime:** `realtime: true` = subscribe event `updated`/`created`, kartu
pindah kolom di tempat saat status berubah dari klien lain
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5).

**Empty/overflow:** kolom kosong tetap terlihat sebagai drop target. Kolom
dengan banyak kartu paginasi cursor-based — renderer **tak boleh** diam-diam
memotong kartu tanpa indikator "muat lebih" (prinsip no-silent-drop yang sama
dengan Table §3.1).

Override penuh + WIP + guard:
```yaml
spec:
  entity: ticket
  group_by: priority                 # atau derivasi state machine (default)
  columns:
    - { value: low }
    - { value: normal, wip_limit: 20 }
    - { value: urgent, wip_limit: 5 }
  card_fields: [number, subject, assignee.name, created_at]
  drag_guard: "fields.assignee_id != null"
  realtime: true
```

**Kapan pakai Kanban vs Table:** Kanban kalau status adalah dimensi kerja utama
dan pemindahan status adalah aksi utama (support queue, order fulfillment,
triage board). Table kalau operasi utama adalah sort/filter/edit banyak kolom.

## 5. `calendar`
View kalender atas entity yang punya field tanggal/waktu — instance
VisualSpecKind `tier: page`. Untuk penjadwalan (appointment, delivery
planning).

```yaml
apiVersion: forma.dev/v1alpha1
kind: Calendar
metadata: { name: appointment-calendar, module: clinic }
spec:
  entity: appointment
  date_field: scheduled_at
```

**Zero-config:** dengan `entity` + `date_field`, renderer merender view `month`
(default), menempatkan event pada tanggalnya, judul dari `label_field` entity
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §2). Klik event → Page
detail/Form entity itu.

**View:** `views: [month, week, day, resource]`, default `month`. View
`resource` = satu lajur per nilai `resource_field` (mis. dokter/ruangan) untuk
resource scheduling.

**Field:**
- `date_field` (wajib) — tanggal/datetime awal event.
- `end_field` (opsional) — event rentang (start–end); tanpa ini event
  titik-waktu.
- `title_field` (opsional) — override `label_field` derived.
- `resource_field` (opsional) — mengaktifkan view `resource`, satu lajur per
  nilai (biasanya field relation, mis. `doctor_id`).
- `color_field` (opsional) — pewarnaan kategori.

**Interaksi:**
- Klik event → detail/form (sama seperti `link` kolom Table).
- Klik slot kosong → Form create dengan `date_field` ter-prefill.
- **Drag reschedule** → memanggil action `update` yang mengubah `date_field`
  (dan `end_field` bergeser proporsional untuk rentang); validasi server-side
  otoritas (guard lifecycle + rules,
  [`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5). Permission =
  permission `update` entity. Record `submitted` immutable tak bisa di-drag
  (§ core-basic §1.2) — renderer menonaktifkan drag untuknya.
- `realtime: true` = event muncul/pindah in-place
  ([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5).

**Recurrence (normatif).** Field recurrence pada entity **wajib** berformat
**RRULE (RFC 5545)** — standar yang sama dipakai iCalendar/Google
Calendar/Outlook — bukan grammar bikinan sendiri, supaya interop
(export/import `.ics`) gratis dan tooling expansion yang sudah matang bisa
langsung dipakai:

```yaml
- { name: recurrence, type: string }   # nilai: "FREQ=WEEKLY;BYDAY=MO;INTERVAL=2"
```

**Expansion terjadi saat baca/render**, bukan materialized rows di
PersistBackend — Calendar meng-expand RRULE jadi instance konkret untuk
rentang tanggal yang sedang di-view (bulan/minggu/hari), lewat pustaka
expansion RRULE standar di sisi renderer. Ini murni komputasi tampilan;
tidak butuh dukungan Query Builder backend ([`../backend/02-core-extended.md`](../backend/02-core-extended.md)
§16) untuk kasus umum tanpa exception.

**Di luar cakupan v1 (Open — exception per-instance).** Mengubah/membatalkan
**satu** occurrence tanpa mengubah pattern-nya (mis. "pertemuan tanggal 5
dipindah ke jam 15:00, sisanya tetap") butuh model data exception
tersendiri (row terpisah yang mereferensikan tanggal asli + override) —
belum dispesifikasikan, ditunda ke iterasi berikutnya. Calendar v1
menampilkan seluruh occurrence hasil expansion RRULE tanpa exception; drag
reschedule (di atas) berlaku ke **field tanggal Entity itu sendiri**,
bukan ke satu occurrence dari pattern berulang.

**Bukan pengganti recurring job.** Recurrence di sini murni untuk
menampilkan/mengedit pola tanggal yang dilihat manusia di Calendar —
**bukan** mekanisme untuk menjalankan action terjadwal berkala (mis. tutup
buku bulanan, generate invoice periodik). Kebutuhan itu domain modul resmi
`forma/scheduler` di atas primitive yang ada
([`../platform/06-datastore.md`](../platform/06-datastore.md) §2 "Set
primitive tertutup"), bukan Calendar.

## 6. `wizard`
Proses bisnis sekuensial multi-step lintas entity; framework mengurus
navigasi stepper, validasi per-step, dependency antar-field, autosave
per-instance, dan perilaku completion:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Wizard
metadata:
  name: patient-registration
  module: clinic
spec:
  title: "Patient Registration — {step.title}"
  entity: visit             # tanpa `action`: step akhir create biasa di entity ini
  on_complete:
    restart: true            # reset stepData/currentStep ke 0, bukan navigasi keluar
    banner:
      - { label: "Queue Number", field: response.queue_number }
  steps:
    - title: "Find Patient"
      layout: search_select
      entity: patient
      search_fields: [nik, name, phone]
      allow_create: true                 # tombol "New Patient" kalau tidak ketemu
    - title: "Select Poly & Doctor"
      required: [polyclinic_id, doctor_id]
      fields:
        - { field: polyclinic_id, entity: polyclinic, type: dropdown, required: true }
        - { field: doctor_id, entity: doctor, type: dropdown, required: true,
            depends_on: polyclinic_id }
      on_prev: discard-poly-selection
    - title: "Confirm & Submit"
      on_enter: prefill-visit-defaults
      summary:
        - { label: "Patient", field: patient.name }
```

**Aturan:**
- `action` (level wizard) opsional. Kalau diisi: action server-side yang
  atomik menulis seluruh data step saat submit final, wajib ada di minimal
  satu entity yang terlibat. Kalau tidak diisi: step akhir melakukan `create`
  biasa di `entity` pakai data step terakumulasi — tiap field yang dibutuhkan
  entity itu wajib sudah resolved dari step sebelumnya.
- `on_complete.restart: true` mengosongkan `stepData`, kembali ke step 0 —
  untuk alur gaya front-desk (daftar satu pasien, lanjut ke berikutnya).
  `redirect` navigasi ke path lain, diabaikan kalau `restart: true`. `banner`
  merender info dari submission yang baru selesai, di-resolve terhadap
  `response.*` (response API submit final) — bukan `stepData` (sudah
  dikosongkan saat restart).
- `required: [field, ...]` di level step menggerbang tombol Next.
- Hook step: `on_enter` (saat step jadi aktif, termasuk lewat Back), `on_next`
  (sebelum maju), `on_prev` (saat keluar lewat Previous) — ketiganya opsional,
  best-effort (gagal tidak memblokir navigasi).
- `depends_on` = filter chain client-side; UX-only, validasi server tetap
  otoritas.
- Step sekuensial — renderer menegakkan penyelesaian step N sebelum N+1
  bisa diakses; Back selalu diizinkan (data step N-1 tetap tersimpan).
- Wizard punya route sendiri (`/wizard/:name`); state step di URL
  (`?step=2`) untuk deep-link; tiap instance wizard yang terbuka diidentifikasi
  `?instance=<id>` (auto-generate) — `stepData` autosave ke `localStorage`
  kunci `wizard:{name}:{instance}`, sehingga multi-tab dan refresh tidak
  saling menimpa. Tidak ada draft row sisi server.
- UI custom di dalam step lewat `component:` — component menerima props
  `{ wizard, step, data, forma }`.

**Relasi ke kind lain:** Wizard adalah komposisi stateful dari step
mirip-Form dengan shell stepper. Kalau prosesnya cuma section form linear
tanpa penegakan sekuensial, pakai `kind: Form` dengan `layout.sections`
biasa (§2) — bukan Wizard.

## 7. `dashboard`
Grid slot `widget` ([`02-visual-spec-kind.md`](02-visual-spec-kind.md) §4,
[`07-component-kinds.md`](07-component-kinds.md) §2–§3 untuk kontrak Widget
dan slot filling-nya):

```yaml
apiVersion: forma.dev/v1alpha1
kind: Dashboard
metadata: { name: sales-today, module: billing }
spec:
  customizable: true                   # user boleh tambah/hapus/urutkan dari katalog widget
  defaults: [sales-today-stat, gl-cashflow-chart]
  refresh: 60                          # atau realtime: true
  widgets:
    - stat:  { title: "Today's Revenue", entity: sales-daily-summary, field: total }
    - chart: { type: line, entity: sales-daily-summary, x: date, y: total, range: 30d }
```

Detail kontrak Widget (`kind: Widget` tier component, visibilitas katalog
derived dari permission, mekanisme customizable) — lihat
[`07-component-kinds.md`](07-component-kinds.md) §2–§3.

## 8. `report` dan `print`

### `kind: Report`
Output tabular terparameterisasi:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Report
metadata: { name: sales-by-category, module: billing }
spec:
  required_permission: reports.sales-by-category
  params:
    - { field: date_from, type: date, required: true }
    - { field: date_to,   type: date, required: true }
  source: { entity: order, filter: { status: paid,
            paid_at: { between: [":date_from", ":date_to"] } } }
  columns: [number, customer.name, category, total]
  group_by: [category]
  totals: [total]
  exports: [xlsx, csv, { print: receipt-style }]
```

`source` selalu query entity, permission-checked — Report tidak pernah
meng-embed SQL (kontrak "gabungkan sources by join_key" adalah urusan
PersistBackend, [`../backend/02-core-extended.md`](../backend/02-core-extended.md)
§6). Export berjalan sebagai **async job**
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5 `call:
async`); file mendarat di download tray.

### `kind: Print`
Dokumen cetak untuk satu entity, multi-target output:

```yaml
apiVersion: forma.dev/v1alpha1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: order
  output:
    format: pdf                   # pdf | thermal | dotmatrix | html
    paper: { size: A5, margin: 12mm }
  header: { logo: true, title: "Receipt {order.number}" }
  body:
    - fields: [number, paid_at, customer.name]
    - child_table: { field: items, columns: [product_id, quantity, price] }
    - totals: { field: total, format: currency }
  footer: { text: "Thank you — {tenant.name}" }
```

| Format | Pipeline | Ukuran kertas | Kegunaan |
|---|---|---|---|
| `pdf` (default) | Generate PDF server-side | `A4`, `A5`, `Letter`, `Legal`, `custom` | Invoice, surat jalan, laporan |
| `thermal` | Server-side ESC/POS byte stream → printer mentah | `thermal_58mm`, `thermal_80mm` | Struk POS, slip apotek, tiket antrean |
| `dotmatrix` | Teks polos + escape code server-side, printer continuous-feed | `dotmatrix_80col`, `dotmatrix_136col` | Pick list gudang, print akuntansi legacy |
| `html` | `window.print()` client-side + CSS `@media print` — tanpa render server | Ukuran apa saja lewat CSS `@page` | Print browser-native, preview-sebelum-print |

**Aturan:** `output.paper.size` divalidasi terhadap format terpilih
(`thermal_58mm` cuma valid dengan `format: thermal`); semua format kecuali
`html` render server-side, hasil ke download tray; `html` render di browser,
stylesheet `@media print` disuntik renderer (menyembunyikan navigasi global);
kertas custom (`custom: { width, height, unit }`) divalidasi saat `forma
validate`. Print programatik: `ctx.print(entity_id, "receipt")` — pemilihan
format per-manifest Print, bukan per-panggilan.

## 9. `timeline` / `timeseries`
Feed kronologis vertikal, dikelompokkan per tanggal — untuk audit trail
append-only, activity log, rekam medis (ditulis sekali, tidak pernah diubah):

```yaml
apiVersion: forma.dev/v1alpha1
kind: Timeline
metadata:
  name: patient-medical-history
  module: clinic
spec:
  entity: medical_record
  bind_param: patient_id                  # konteks filter — dari route/parent page/nilai tetap
  bind_value: ":patient_id"
  display:
    title_field: visit_date
    subtitle_field: doctor.name
    content_field: diagnosis_and_notes
    icon_field: visit_type
  group_by: date                          # date | month | year | none
  sort: desc
  page_size: 20
```

**Aturan:** renderer **tidak boleh** menampilkan tombol create/edit/delete
untuk entity Timeline — entity itu SEBAIKNYA menonaktifkan action
`update`/`delete` (`disabled: true`,
[`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5), hanya
menyisakan `create` sisi server; kalaupun masih ada, renderer mengabaikannya
untuk Timeline — kind ini sendiri yang jadi guard. Infinite scroll
cursor-based pakai `created_at`. Realtime: subscribe event `created`,
card baru masuk di atas tanpa mengganggu posisi scroll. Custom card lewat
`display.component`.

**Kapan pakai Timeline vs Table:** Timeline kalau urutan waktu adalah
narasi utama (rekam medis, audit log, activity feed) — pada dasarnya
*cerita read-only*. Table kalau user perlu sort/filter/operate baris —
*permukaan operasional*.

## 10. `listing`
Katalog publik (e-commerce, movie search) — pasangan alami App kind
`landing-page` ([`05-app-kinds.md`](05-app-kinds.md) §4). Secara struktural
mirip `table-list` (§3) tapi tanpa asumsi Auth-wrap dari App renderer-nya, dan
tanpa `row_actions`/`bulk_actions` yang menyiratkan operasi tulis
terautentikasi.

## 11. `approval-inbox`
Task-queue "persetujuan saya" — daftar step approval Workflow
([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §2) yang
menunggu tindakan caller. Instance VisualSpecKind `tier: page`. Tipis: mesin
approval hidup di backend, kind ini hanya permukaan standarnya.

```yaml
apiVersion: forma.dev/v1alpha1
kind: ApprovalInbox
metadata: { name: my-approvals, module: core }
spec:
  realtime: true
```

**Zero-config:** sumber adalah step Workflow pending yang eligible untuk caller
(keanggotaan role per step, § core-extended §2) — lintas semua entity/module
dalam App, permission-filtered otomatis. Caller hanya melihat approval yang
boleh ia tindak; "pemohon tak pernah menyetujui permintaannya sendiri"
ditegakkan backend, bukan disembunyikan di UI. Tiap baris menampilkan entity
terkait + ringkasan + langkah saat ini; klik baris → detail entity (konteks
penuh sebelum memutuskan).

**Action inline** `approve`/`reject` per baris = pencatatan approval
bertanda tangan di Workflow (§ core-extended §2); `reject` mengikuti
`on_reject`. Transisi yang di-intercept baru eksekusi setelah quorum seluruh
step tercapai — di luar tanggung jawab kind ini. **Badge count** = jumlah
pending caller, `realtime: true` default (subscribe perubahan Workflow,
[`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5). `filters`/`search`
opsional seperti Table.

## 12. `notification-center`
Permukaan in-app notifikasi yang terkirim ke user saat ini — sumbernya module
resmi `forma/notify` (bridge delivery `notification` dari Subscription,
[`../backend/02-core-extended.md`](../backend/02-core-extended.md) §3). Instance
VisualSpecKind `tier: page`.

```yaml
apiVersion: forma.dev/v1alpha1
kind: NotificationCenter
metadata: { name: notifications, module: core }
spec:
  realtime: true
```

**Zero-config:** daftar notifikasi caller (terurut terbaru), badge unread, aksi
`mark-read` (per item + mark-all). Notifikasi per-user dan tenant/workspace-
scoped seperti semua data. `realtime: true` **default** — item baru masuk
in-place dan badge unread naik ([`04-spec-resolution-api.md`](04-spec-resolution-api.md)
§5). Klik notifikasi → navigasi ke deep-link entity/Page yang dirujuknya (bila
ada). Template pesan & channel provider (email/push/in-app) hidup di
`forma/notify`, bukan di kontrak ini (§ core-extended §3) — kind ini hanya
permukaan in-app-nya.

## 13. Custom Page — Escape Hatch Expert
Untuk layar yang tak berpola sama sekali: `kind: Page` dengan `mode: custom`
menyerahkan **seluruh** rendering ke kode programmer, sambil tetap men-declare
footprint backend yang ia konsumsi.

```yaml
apiVersion: forma.dev/v1alpha1
kind: Page
metadata: { name: dispatch-console, module: logistics }
spec:
  route: /dispatch
  mode: custom
  asset: logistics/assets/dispatch-console.js
  binds:
    entities: [shipment, vehicle, driver]
    actions: [shipment.assign, shipment.dispatch]
    subscribe: [logistics.shipment]
```

`binds` adalah **footprint consent/permission** Page ini — peran yang sama
dengan `needs:` milik component ([`07-component-kinds.md`](07-component-kinds.md)
§4): entity/service/action yang boleh ia sentuh. Panggilan di luar `binds`
gagal client-side, dan tak pernah diotorisasi server-side juga (enforcement
selalu di resource, [`../backend/01-core-basic.md`](../backend/01-core-basic.md)
§5).

Yang **diinjeksikan** ke asset (kontrak mount
[`07-component-kinds.md`](07-component-kinds.md) §4): client typed per entity
yang di-bind, `forma.api`, `forma.subscribe`, `forma.navigate`, `forma.ui`, dan
token `forma.theme`. Programmer menguasai 100% markup — tapi **wajib mengikuti
Shell tempat ia hidup**: di shadcn shell berarti React + shadcn
(`stack_family`, [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §3); kode
custom tak lintas Shell.

**Beda dari full-custom via satu `component:`** (§1): keduanya menyerahkan
render ke asset, tapi `mode: custom` men-declare footprint backend di **level
Page** (bukan per-blok `needs:`) dan tak punya `blocks`/`tabs` sama sekali —
Page itu sepenuhnya milik programmer. Ini anak tangga teratas kontrol frontend:
Form terkelola → custom widget → component → custom Page → headless form engine
→ raw `forma.api` ([`07-component-kinds.md`](07-component-kinds.md) §4).

## 14. Derivasi Otomatis (Layer 0)
Tiap Entity otomatis menghasilkan, tanpa manifest UI sama sekali: **Table**
list, **Form** create/edit, **Page** detail, dan entry navigasi turunan di
menu App (dikelompokkan per module Entity, untuk module yang belum
tercakup entry `App.spec.menu`/`Module.spec.menu` yang ditulis eksplisit).
Kind di tier ini ada untuk *override* default itu — tim bisa mengirim tool
internal lengkap dengan nol manifest frontend.

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
apiVersion: formspec.dev/v1
kind: Page
metadata:
  name: order-detail
  module: billing
spec:
  route: /orders/:id
  title: "Order {order.number}"
  blocks:
    - form: { ref: order-edit, id: ":id", mode: view }
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

**Blok `section:` — presentasi deklaratif.** Page bisa memuat blok `section:`
(closed set: `hero`, `feature_grid`, `card`, `carousel`, `cta`) — region
presentasi penuh-lebar tanpa data binding dan tanpa auth. Generik dan
reusable di App mana pun (public `no-nav`, sidebar-nav, topnav, ...), bukan
milik satu archetype. Murni deklaratif — nol field styling; seluruh token
visual hidup di `kind: Theme` ([`05-app-kinds.md`](05-app-kinds.md) §5),
tidak pernah inline.

```yaml
spec:
  route: /
  title: "Home"
  blocks:
    - section:
        type: hero
        title: "Toko Kami"
        subtitle: "Belanja mudah, aman, dan cepat."
        cta: { label: "Lihat Katalog", href: /listing/product-catalog }
    - section:
        type: feature_grid
        title: "Kenapa Kami"
        items:
          - { icon: zap, title: "Cepat", text: "24 jam." }
    - section:
        type: carousel
        autoplay: true
        items: [{ title: "A", text: "..." }, { title: "B", text: "..." }]
```

**Varian tabs** — beberapa sub-layar dalam satu route:

```yaml
spec:
  route: /settings
  tabs:
    - { label: General, form: { ref: settings-general } }
    - { label: Tax, form: { ref: settings-tax } }
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
apiVersion: formspec.dev/v1
kind: Page
metadata: { name: order-workbench, module: billing }
spec:
  route: /orders/workbench
  layout: { mode: split } # master kiri (sempit), detail kanan (lebar)
  blocks:
    - table: { ref: order-list } # master — sumber seleksi
    - form:
        ref: order-detail
        binds: { source: order-list, param: id } # detail mengikuti seleksi
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

> **Open — `binds`/`layout.mode: split`.** Master-detail belum didukung skema
> `PageSpec`/`BlockRef` maupun renderer — ditracking di `docs/plan/todo.md`.
> Saat ini master-detail dilakukan via `param` + route (`:id`) biasa.

## 2. `data-entry` (`kind: Form`)

Layout + perilaku input satu Entity, menggantikan form hasil derivasi:

```yaml
apiVersion: formspec.dev/v1
kind: Form
metadata:
  name: order-edit
  module: billing
spec:
  entity: order
  mode: edit # create | edit | view
  render: { mode: separate_page } # modal | drawer | separate_page (default: modal)
  sections:
    - title: Customer
      columns: 2
      fields:
        - { field: customer_id, widget: relation-picker }
        - { field: member_tier, read_only: true }
    - title: Totals
      visible_when: "fields.items != null and len(fields.items) > 0"
      fields:
        - {
            field: total,
            read_only: true,
            compute: "sum([i.quantity * i.price for i in fields.items])",
          }
  actions:
    - { action: checkout, label: "Checkout", style: primary }
```

**`render` — keputusan container design-time**, bukan runtime:

| `render`          | Perilaku                                                                             | Kapan dipakai                                                             |
| ----------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| `modal` (default) | Dialog overlay di atas Page saat ini; route tak berubah, state di baliknya tetap ada | Entity ringan (≤5 field), create/edit cepat tanpa kehilangan konteks list |
| `drawer`          | Panel slide-in dari kanan, sama sifatnya dengan modal tapi lebih lebar               | Form medium (5–12 field), khususnya `columns: 2`                          |
| `separate_page`   | Route sendiri, breadcrumb + URL sendiri                                              | Entity padat (12+ field, child table, validasi kompleks), butuh deep-link |

Entity yang sama boleh punya banyak Form dengan `render` berbeda (mis. modal
quick-create + separate_page full-edit) — keputusan per-Form, ditegakkan
renderer (tidak ada switch runtime; butuh mode lain → deklarasikan Form
kedua).

**Aturan:** tiap `field` wajib ada di Entity; tiap `action` wajib ada dan
permission-gated otomatis
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4). Vocabulary
perilaku client tertutup: `visible_when`, `readonly_when`, `required_when`,
`compute` (FormSpecExpr, [`08-formspec-expr.md`](08-formspec-expr.md)) — begitu butuh
efek imperatif, field itu jadi custom widget
([`07-component-kinds.md`](07-component-kinds.md) §4). `rules` field dari
Entity manifest ditegakkan client-side untuk UX; **validasi server-side
tetap otoritas — cek client bukan pernah keamanan.**

`render` menerima bentuk objek `{ mode: separate_page }` (bentuk kanonik,
sesuai skema dan renderer) maupun shorthand skalar `render: separate_page` —
keduanya disetarakan saat parse.

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

| Pola                             | Kapan dipakai                                            | UI                                                                                                                                                        |
| -------------------------------- | -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **2-step + auto-save** (default) | Entity kompleks, butuh review (Invoice, Order, Contract) | Auto-save senyap saat draft (debounced `update`), satu tombol "Submit" eksplisit                                                                          |
| **2-step manual**                | Draft sengaja dipisah untuk direview orang lain dulu     | Tombol "Save Draft" + "Submit" terpisah                                                                                                                   |
| **1-step (`create-submit`)**     | Entry cepat volume tinggi (POS, antrean klinik)          | Satu tombol, pakai reserved action `create-submit` ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §1.2) — tanpa konsep draft di UI, atomik |

Dua tombol standar (Save Draft/auto-save, Submit) selalu otomatis tersedia
dari model tanpa perlu dideklarasikan; `create-submit` menambah jalur cepat
opsional, bukan mengganti keduanya.

## 3. `table-list` (`kind: Table`)

Daftar ber-filter/sort/paginasi; kolom terderivasi dari entity:

```yaml
apiVersion: formspec.dev/v1
kind: Table
metadata: { name: order-list, module: billing }
spec:
  entity: order
  columns:
    - { field: number, link: order-detail }
    - { field: customer.name }
    - { field: total, format: currency }
    - { field: status, widget: badge }
  filters:
    - { field: status, label: Status, type: select }
    - { field: created_at, label: "Created", type: date_range }
  default_sort: -created_at # "field" = asc, "-field" = desc
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

### 3.3 Kontrak Filter (dipakai bersama Table & Kanban)

Model filter data kind generik, dipakai identik oleh `Table` dan `Kanban`
(serta kind lain yang me-list record). Setiap filter dideklarasikan sebagai
objek `FilterSpec` dan memegang salah satu dari dua peran:

```yaml
spec:
  filters:
    - { field: transaction_date, label: "Tanggal", type: date, default: today }
    - { field: polyclinic_id, label: "Poliklinik", type: select }
  fixed_filters:
    - { field: tenant_id, default: tenant-1 }
```

- **`filters`** — kontrol yang bisa diubah user di UI. Bila `default` diisi,
  kontrol ter-seed nilai itu saat dibuka (user tetap bisa mengganti/​mengosongkan).
- **`fixed_filters`** — filter **immutable, server-side**: selalu digabung ke
  request list, **tidak dirender sebagai kontrol**, dan tidak bisa dihapus
  user. Dipakai untuk scope yang tak boleh diganggu (mis. pin satu tanggal,
  satu tenant, satu konteks halaman).

`FilterSpec`:

| Field       | Wajib | Deskripsi                                                                  |
| ----------- | ----- | -------------------------------------------------------------------------- |
| `field`     | ya    | Nama field entity (boleh dot-path relation, mis. `patient.name`)           |
| `label`     | tidak | Label kontrol; fallback `field`                                            |
| `type`      | tidak | `select` (default) · `text` · `date` · `date_range`                        |
| `op`        | tidak | Operator filter API — default `eq` (set operator backend §6)               |
| `default`   | tidak | Nilai seed. Mendukung `today` / `today()` (resolver = tanggal server, UTC) |
| `show_all`  | tidak | Hanya tipe `select`: tampilkan opsi "All" (clear). Default `true`          |
| `all_label` | tidak | Hanya tipe `select`: caption opsi "All" (clear). Default `"(ALL)"`         |

Nilai terkirim ke API sebagai `field[op]=value` (mis. `transaction_date[eq]=2026-08-07`),
sehingga DB mem-filter **sebelum** baris dikirim. `fixed_filters` selalu menang
atas pilihan user bila field-nya sama. `today()` meniadakan perbedaan zona
waktu: memakai tanggal server (RFC3339 UTC), bukan tanggal lokal browser —
sama dengan konvensi widget query.

## 4. `kanban`

Papan kolom drag-drop — instance VisualSpecKind `tier: page`
([`02-visual-spec-kind.md`](02-visual-spec-kind.md)), dan contoh unggulan
kontrak itu. Operasional: tiap kartu satu record entity, tiap kolom satu nilai
status, drag antar kolom = transisi state.

```yaml
apiVersion: formspec.dev/v1
kind: Kanban
metadata: { name: support-board, module: helpdesk }
spec:
  entity: ticket
  status_field: status # wajib — field state machine/enum yang jadi kolom
  realtime: true
```

**Kontrak saat ini:** `status_field` **wajib** — menunjuk field yang nilainya
jadi kolom (field state machine bisnis entity
[`../backend/02-core-extended.md`](../backend/02-core-extended.md) §1, atau
field enum biasa). `columns` eksplisit berisi nilai status yang ditampilkan
sebagai kolom.

> **Open — zero-config derivasi kolom.** Derivas kolom otomatis dari state
> machine/`group_by` (menghilangkan kewajiban `status_field`) belum
> diimplementasikan — ditracking di `docs/plan/kanban-full-implementation.md`.

**Derivasi kolom:**

- `columns:` eksplisit **menang penuh** — setiap entry `{ status, label, color }`
  (nilai, urutan).
- Tanpa `columns:` eksplisit, renderer menderivasi kolom dari nilai unik
  `status_field` — urut sesuai urutan transisi (state machine) atau urutan
  deklarasi `enum_values`.

**Derivasi kartu:** field kartu diderivasi seperti prioritas kolom Table (§3.1)
— natural key/title-ish, `transaction_date`; field status implisit (sudah jadi
kolom, tak diulang di kartu). Override lewat `card_template`
(`{ title, subtitle, badge, assignee, fields, component }`).

**Drag-drop = state transition:**

- Menjatuhkan kartu ke kolom lain memanggil action `via` transisi yang cocok
  (`from` = kolom asal, `to` = kolom tujuan). **Guard state machine dievaluasi
  server-side — otoritas.** Permission drag = permission action transisi itu
  ([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §4); caller tanpa
  permission itu tak bisa men-drag kartu ke kolom tersebut.
- Transisi yang tak dideklarasikan → tak ada drop target; kalaupun dipaksa,
  server menolaknya (`STATE_TRANSITION_ERROR`, § core-extended §1).

> **Open — `drag_guard`.** Pre-check UX sebelum drop (FormSpecExpr,
> [`08-formspec-expr.md`](08-formspec-expr.md)) belum diimplementasikan — ditracking di
> `docs/plan/kanban-full-implementation.md`. Validasi server (guard state
> machine) tetap otoritas dan sudah berjalan.

> **Open — WIP limit.** `wip_limit` per kolom (batas jumlah kartu, soft
> pre-check UX) belum ada di skema `columns` — ditracking di
> `docs/plan/kanban-full-implementation.md`. Pengganti saat ini:
> `max_cards_per_column` (integer, level board) di `KanbanSpec`.

**Realtime:** `realtime: true` = subscribe event `updated`/`created`, kartu
pindah kolom di tempat saat status berubah dari klien lain
([`04-spec-resolution-api.md`](04-spec-resolution-api.md) §5).

**Empty/overflow:** kolom kosong tetap terlihat sebagai drop target. Kolom
dengan banyak kartu paginasi cursor-based — renderer **tak boleh** diam-diam
memotong kartu tanpa indikator "muat lebih" (prinsip no-silent-drop yang sama
dengan Table §3.1).

Override penuh:

```yaml
spec:
  entity: ticket
  status_field: status
  columns:
    - { status: low, label: Low }
    - { status: normal, label: Normal, color: blue }
    - { status: urgent, label: Urgent, color: red }
  card_template:
    title: number
    subtitle: subject
    fields: [assignee.name, created_at]
  realtime: true
```

**Filter & scope tanggal.** Kanban memakai kontrak filter yang sama dengan
Table (§3.3). Filter `type: date` dengan `default: today` membuat board
terbuka ter-scope ke satu tanggal (mis. antrean hari ini) dan tetap bisa
diganti user via date picker; `fixed_filters` mem-pin scope yang tak bisa
diganti user. Nilai filter dikirim server-side (`field[op]=value`).

**Within-column ordering.** Renderer dapat mengurutkan kartu dalam satu kolom
dan mengizinkan operator mengubah urutan via drag-to-reorder dalam kolom:

- `sortable: true` — mengaktifkan drag-to-reorder dalam kolom. Renderer
  otomatis mengirim `?sort=<position_field>` ke API, sehingga kartu tampil
  sesuai urutan posisi.
- `position_field: "nama_field"` — field entity yang menyimpan nilai posisi
  (biasanya integer). Renderer mengupdate field ini via PATCH saat kartu
  di-drag ke posisi baru.
- `sortable: true` tanpa `position_field` adalah konfigurasi tidak valid
  — manifest validation wajib menolaknya.

Saat kartu dipindah antar kolom, renderer juga mengisi `position_field`
dengan `max(posisi_kolom_tujuan) + 1` sehingga kartu baru muncul di urutan
terakhir kolom tujuan.

**Kapan pakai Kanban vs Table:** Kanban kalau status adalah dimensi kerja utama
dan pemindahan status adalah aksi utama (support queue, order fulfillment,
triage board). Table kalau operasi utama adalah sort/filter/edit banyak kolom.

## 5. `calendar`

View kalender atas entity yang punya field tanggal/waktu — instance
VisualSpecKind `tier: page`. Untuk penjadwalan (appointment, delivery
planning).

```yaml
apiVersion: formspec.dev/v1
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
- { name: recurrence, type: string } # nilai: "FREQ=WEEKLY;BYDAY=MO;INTERVAL=2"
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
`formspec/scheduler` di atas primitive yang ada
([`../platform/06-datastore.md`](../platform/06-datastore.md) §2 "Set
primitive tertutup"), bukan Calendar.

## 6. `wizard`

Proses bisnis sekuensial multi-step lintas entity; framework mengurus
navigasi stepper, validasi per-step, dependency antar-field, autosave
per-instance, dan perilaku completion:

```yaml
apiVersion: formspec.dev/v1
kind: Wizard
metadata:
  name: patient-registration
  module: clinic
spec:
  title: "Patient Registration — {step.title}"
  entity: visit # tanpa `action`: step akhir create biasa di entity ini
  on_complete:
    restart: true # reset stepData/currentStep ke 0, bukan navigasi keluar
    banner:
      - { label: "Queue Number", field: response.queue_number }
  steps:
    - title: "Find Patient"
      layout: search_select
      entity: patient
      search_fields: [nik, name, phone]
      allow_create: true # tombol "New Patient" kalau tidak ketemu
    - title: "Select Poly & Doctor"
      required: [polyclinic_id, doctor_id]
      fields:
        - {
            field: polyclinic_id,
            entity: polyclinic,
            type: dropdown,
            required: true,
          }
        - {
            field: doctor_id,
            entity: doctor,
            type: dropdown,
            required: true,
            depends_on: polyclinic_id,
          }
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
  `{ wizard, step, data, formspec }`.

**Relasi ke kind lain:** Wizard adalah komposisi stateful dari step
mirip-Form dengan shell stepper. Kalau prosesnya cuma section form linear
tanpa penegakan sekuensial, pakai `kind: Form` dengan `sections` biasa (§2)
— bukan Wizard.

## 7. `dashboard`

Grid slot `widget` ([`02-visual-spec-kind.md`](02-visual-spec-kind.md) §4,
[`07-component-kinds.md`](07-component-kinds.md) §2–§3 untuk kontrak Widget
dan slot filling-nya). Dashboard **mereferensikan** widget by name — widget
didefinisikan terpisah sebagai `kind: Widget`:

```yaml
apiVersion: formspec.dev/v1
kind: Dashboard
metadata: { name: sales-today, module: billing }
spec:
  customizable: true # user boleh tambah/hapus/urutkan dari katalog widget
  defaults: [sales-today-stat, gl-cashflow-chart]
  refresh: 60 # atau realtime: true
  widgets:
    - ref: sales-today-stat
      layout: { x: 0, y: 0, w: 4, h: 2 }
    - ref: gl-cashflow-chart
      layout: { x: 4, y: 0, w: 8, h: 4 }
      config: { range: 30d }
```

```yaml
apiVersion: formspec.dev/v1
kind: Widget
metadata: { name: sales-today-stat, module: billing }
spec:
  title: "Today's Revenue"
  type: metric # metric | chart | table | list
  entity: sales-daily-summary
  config: { field: total } # specifik per type
```

`DashboardWidget` = `{ ref, layout: {x,y,w,h}, config }`; `WidgetSpec` =
`{ title, type, entity?, query?, refresh_secs?, size?, config? }`. Visibilitas
katalog widget derived dari permission, mekanisme customizable — lihat
[`07-component-kinds.md`](07-component-kinds.md) §2–§3.

> **Open — rendering widget.** Renderer widget (`stat`/`chart`/`table`/`list`)
> dan kanvas dashboard belum sepenuhnya diimplementasikan — skema kontrak di
> atas sudah final; eksekusi ditracking di `docs/plan/todo.md` §5.7.

## 8. `report` dan `print`

### `kind: Report`

Output tabular terparameterisasi:

```yaml
apiVersion: formspec.dev/v1
kind: Report
metadata: { name: sales-by-category, module: billing }
spec:
  title: "Sales by Category"
  entity: order
  required_permission: reports.sales-by-category
  parameters:
    - { field: date_from, label: "Dari", type: date, required: true }
    - { field: date_to, label: "Sampai", type: date, required: true }
  columns:
    - { field: number, label: "No." }
    - { field: customer.name, label: "Customer" }
    - { field: category, label: "Kategori" }
    - { field: total, label: "Total", aggregate: sum, format: currency }
  groups:
    - { field: category, label: "Kategori" }
  totals:
    - { label: "Total", field: total, fn: sum }
  export: [xlsx, csv]
```

`entity` selalu query entity, permission-checked — Report tidak pernah
meng-embed SQL (kontrak "gabungkan sources by join_key" adalah urusan
PersistBackend, [`../backend/02-core-extended.md`](../backend/02-core-extended.md)
§6). Export berjalan sebagai **async job**
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5 `call:
async`); file mendarat di download tray.

> **Open — `source.filter`.** Filter parameterized deklaratif (`source:
{ entity, filter }` dengan `":param"` placeholder) belum didukung skema —
> parameter saat ini dikirim sebagai filter query `?<field>=<value>` per
> `parameters[]` saat eksekusi report.

### `kind: Print`

Dokumen cetak untuk satu entity, multi-target output:

```yaml
apiVersion: formspec.dev/v1
kind: Print
metadata: { name: receipt, module: billing }
spec:
  entity: order
  output:
    format: pdf # pdf | thermal | dotmatrix | html
    paper: { size: A5, margin: 12mm }
  header: { logo: true, title: "Receipt {order.number}" }
  body:
    - fields: [number, paid_at, customer.name]
    - child_table: { field: items, columns: [product_id, quantity, price] }
    - totals: { field: total, format: currency }
  footer: { text: "Thank you — {tenant.name}" }
```

| Format          | Pipeline                                                                | Ukuran kertas                           | Kegunaan                                    |
| --------------- | ----------------------------------------------------------------------- | --------------------------------------- | ------------------------------------------- |
| `pdf` (default) | Generate PDF server-side                                                | `A4`, `A5`, `Letter`, `Legal`, `custom` | Invoice, surat jalan, laporan               |
| `thermal`       | Server-side ESC/POS byte stream → printer mentah                        | `thermal_58mm`, `thermal_80mm`          | Struk POS, slip apotek, tiket antrean       |
| `dotmatrix`     | Teks polos + escape code server-side, printer continuous-feed           | `dotmatrix_80col`, `dotmatrix_136col`   | Pick list gudang, print akuntansi legacy    |
| `html`          | `window.print()` client-side + CSS `@media print` — tanpa render server | Ukuran apa saja lewat CSS `@page`       | Print browser-native, preview-sebelum-print |

**Aturan:** `output.paper.size` divalidasi terhadap format terpilih
(`thermal_58mm` cuma valid dengan `format: thermal`); semua format kecuali
`html` render server-side, hasil ke download tray; `html` render di browser,
stylesheet `@media print` disuntik renderer (menyembunyikan navigasi global);
kertas custom (`custom: { width, height, unit }`) divalidasi saat `formspec
validate`. Print programatik: `ctx.print(entity_id, "receipt")` — pemilihan
format per-manifest Print, bukan per-panggilan.

## 9. `timeline` / `timeseries`

Feed kronologis vertikal, dikelompokkan per tanggal — untuk audit trail
append-only, activity log, rekam medis (ditulis sekali, tidak pernah diubah):

```yaml
apiVersion: formspec.dev/v1
kind: Timeline
metadata:
  name: patient-medical-history
  module: clinic
spec:
  entity: medical_record
  bind_param: patient_id # konteks filter — dari route/parent page/nilai tetap
  bind_value: ":patient_id"
  display:
    title_field: visit_date
    subtitle_field: doctor.name
    content_field: diagnosis_and_notes
    icon_field: visit_type
  group_by: date # date | month | year | none
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
_cerita read-only_. Table kalau user perlu sort/filter/operate baris —
_permukaan operasional_.

## 10. `listing`

Katalog publik (e-commerce, movie search) — pasangan alami App `access:
public` (biasanya `app_renderer: no-nav`,
[`05-app-kinds.md`](05-app-kinds.md) §4). Secara struktural
mirip `table-list` (§3) tapi tanpa asumsi Auth-wrap dari App renderer-nya, dan
tanpa `row_actions`/`bulk_actions` yang menyiratkan operasi tulis
terautentikasi.

## 11. `approval-inbox`

Task-queue "persetujuan saya" — daftar step approval Workflow
([`../backend/02-core-extended.md`](../backend/02-core-extended.md) §2) yang
menunggu tindakan caller. Instance VisualSpecKind `tier: page`. Tipis: mesin
approval hidup di backend, kind ini hanya permukaan standarnya.

```yaml
apiVersion: formspec.dev/v1
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
resmi `formspec/notify` (bridge delivery `notification` dari Subscription,
[`../backend/02-core-extended.md`](../backend/02-core-extended.md) §3). Instance
VisualSpecKind `tier: page`.

```yaml
apiVersion: formspec.dev/v1
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
`formspec/notify`, bukan di kontrak ini (§ core-extended §3) — kind ini hanya
permukaan in-app-nya.

## 13. Custom Page — Escape Hatch Expert

Untuk layar yang tak berpola sama sekali: `kind: Page` dengan `mode: custom`
menyerahkan **seluruh** rendering ke kode programmer, sambil tetap men-declare
footprint backend yang ia konsumsi.

> **Open — `mode: custom`/`binds`.** Custom Page belum didukung skema
> `PageSpec` maupun renderer (Page saat ini hanya `blocks`/`tabs`) — ditracking
> di `docs/plan/todo.md`. Kontrak di bawah adalah target desain.

```yaml
apiVersion: formspec.dev/v1
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
yang di-bind, `formspec.api`, `formspec.subscribe`, `formspec.navigate`, `formspec.ui`, dan
token `formspec.theme`. Programmer menguasai 100% markup — tapi **wajib mengikuti
Shell tempat ia hidup**: di shadcn shell berarti React + shadcn
(`stack_family`, [`01-visual-hierarchy.md`](01-visual-hierarchy.md) §3); kode
custom tak lintas Shell.

**Beda dari full-custom via satu `component:`** (§1): keduanya menyerahkan
render ke asset, tapi `mode: custom` men-declare footprint backend di **level
Page** (bukan per-blok `needs:`) dan tak punya `blocks`/`tabs` sama sekali —
Page itu sepenuhnya milik programmer. Ini anak tangga teratas kontrol frontend:
Form terkelola → custom widget → component → custom Page → headless form engine
→ raw `formspec.api` ([`07-component-kinds.md`](07-component-kinds.md) §4).

## 14. Derivasi Otomatis & UI 3-Layer Wrapping

FormSpec UI mengikuti model **3-layer wrapping** yang ketat. Memahami model ini
adalah kunci untuk tahu kapan perlu mendeklarasikan UI kind vs membiarkan
engine men-derive semuanya secara otomatis.

```
┌─────────────────────────────────────────────┐
│ PAGE  (route + composition)                 │
│  /app/klinik/invoice/create                 │
│                                             │
│  ┌───────────────────────────────────────┐  │
│  │ FORM / TABLE  (layout override)       │  │
│  │  visible_when, readonly_when, ...     │  │
│  │                                       │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │ ENTITY  (data model)            │  │  │
│  │  │  fields, state_machine,         │  │  │
│  │  │  permissions, actions           │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

### Layer 0 — Entity (selalu ada)

Tiap Entity otomatis menghasilkan, **tanpa manifest UI sama sekali**:

- REST API endpoint (UI surface: `/_ui/entity/` — lihat
  [`backend/01-core-basic.md`](../backend/01-core-basic.md) §8.1)
- **Table** — list/browse view dengan kolom terderivasi (§3.1)
- **Form create** — form input data baru
- **Form edit** — form ubah data existing
- **Page detail** — halaman detail satu record (read-only)
- Entry navigasi turunan di menu App (dikelompokkan per module Entity)

Ini mencakup **80-95%** kebutuhan UI aplikasi bisnis. Developer tidak perlu
menulis satu pun UI kind untuk mayoritas entity.

### Aturan Wrapping — Kapan Override Diperlukan

| Kamu deklarasi                   | Engine auto-derive                                       | Kapan override?                                                                               |
| -------------------------------- | -------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `Entity` saja                    | Default Table + Form(create) + Form(edit) + Page(detail) | Field order/layout khusus, hide field, group field, validasi custom, multi-entity composition |
| `Form` (`public: true`)          | Auto-wrapped dalam Page, route `/<module>/form/<name>`   | Form ini perlu Page kustom (multi-tab, side panel, master-detail)                             |
| `Table` (`public: true`)         | Auto-wrapped dalam Page, route `/<module>/table/<name>`  | Table ini perlu Page kustom                                                                   |
| `Page`                           | Route langsung — tidak ada wrapping tambahan             | — (Page selalu eksplisit)                                                                     |
| `Form`/`Table` (`public: false`) | Tidak punya route; hanya bisa di-embed di Page lain      | —                                                                                             |

### Decision Flow

```
Apakah auto-derived UI dari Entity cukup?
  ├── YA → Done. Tidak perlu deklarasi UI kind apapun.
  └── TIDAK → Apa yang perlu diubah?
       ├── Urutan/label/hide field → deklarasi kind: Form
       ├── Kolom/sort/filter → deklarasi kind: Table
       ├── Komposisi multi-entity → deklarasi kind: Page (blocks/tabs)
       ├── Dashboard/report/wizard → deklarasi UI kind sesuai
       └── Custom component → deklarasi kind: Page dengan asset block
```

### `public` — Kontrol Route Auto-Generated

Setiap visual kind punya field `public` (default `true`):

| `public`         | Perilaku                                                                                                                                  |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `true` (default) | Engine auto-generate Page wrapper + route `/<module>/<kind-lowercase>/<name>`. Kind bisa di-navigate langsung atau di-embed di Page lain. |
| `false`          | Tidak ada route. Kind hanya bisa tampil sebagai blok di dalam Page yang authored secara eksplisit.                                        |

```yaml
kind: Form
metadata:
  name: quick-create-invoice
  module: billing
spec:
  public: false # embed-only — tidak punya route mandiri
  entity: billing.invoice
  mode: create
```

### Prinsip Kunci

- **Entity dulu, override belakangan.** Tulis semua Entity, jalankan
  `formspec dev`, lihat hasil auto-derived-nya, baru putuskan mana yang butuh
  override.
- **Jangan over-engineer.** Mayoritas entity tidak butuh UI kind sama sekali.
  Kalau menulis `kind: Form` untuk setiap entity, itu anti-pattern.
- **Override minimal.** Kalau cuma perlu mengubah 1-2 field, tulis Form dengan
  hanya field yang berbeda — sisanya tetap auto-derived.
- **Page = komposisi.** Page dipakai saat satu layar butuh banyak entity
  (master-detail, tabs, multi-block). Bukan untuk sekadar mengubah tampilan
  satu entity — itu domain Form/Table.

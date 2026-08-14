# Core Basic

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku. Seluruh kontrak di dokumen ini
> storage-agnostic; contoh SQL konkret hidup di dokumentasi renderer
> jsonb-persist.

## Daftar Isi

1. [Entity](#1-entity-entity) — karakteristik, lifecycle doc_status, field reserved, child vs relation, auth, spec reference, lifecycle vs state_machine
2. [Primary Key & Natural Key](#2-primary-key--natural-key)
3. [Persistence Sebagai Kontrak](#3-persistence-sebagai-kontrak)
4. [Migration = Structural Diff](#4-migration--structural-diff)
5. [Action](#5-action) — impl types, UI hints, permission model, idempotensi, concurrency
6. [Query & Filter Operator](#6-query--filter-operator)
7. [Event & Outbox](#7-event--outbox)
8. [Dua Permukaan API: UI vs External](#8-dua-permukaan-api-ui-vs-external)
9. [Error Model](#9-error-model)
10. [Config & Global Settings](#10-config--global-settings)

## 1. Entity (Entity)

### 1.1 Taksonomi Resource
Dua tipe resource: `type: document` (persisted, sumber kebenaran data bisnis)
dan `type: service` (stateless, komputasi murni — tidak punya `characteristic`,
`doc_status`, atau lifecycle guard).

Entity punya `characteristic`, tepat satu nilai (mutually exclusive; `formspec
apply` menolak lebih dari satu):

| Characteristic | Arti | Wajib |
|---|---|---|
| `master` | Data referensi stabil (Customer, Product) | Boleh punya lifecycle (kalau `submit` aktif) atau tidak |
| `transaction` | Append-heavy, time-partitioned (Invoice, Journal Entry) | Wajib field `transaction_date` |
| `reference` | Seed data read-only, dimiliki App Owner (Provinsi, Tarif Pajak). Backend mendukung **find-or-create**: jika record belum ada saat diakses via GET, framework auto-create dengan field defaults. | — |
| `summary` | Projeksi terkelola sistem (GL Balance) | `create`/`update`/`delete` permanen nonaktif via API |

`summary` bukan tipe resource keempat — ia nilai `characteristic` yang sama
kelasnya dengan `master`/`transaction`/`reference`.

> **Find-or-create untuk reference.** Ketika `GET /{id}` tidak menemukan record pada entity
> `characteristic: reference`, framework tidak langsung mengembalikan 404 —
> melainkan mencari record yang sudah ada untuk workspace tersebut. Jika tidak
> ada, framework auto-create record baru dengan nilai default dari field
> definition (`spec.fields[].default`). Ini memungkinkan pola Configuration
> Page (Page tabs dengan sentinel `id: "0"`) bekerja tanpa seeding manual.
> Lihat implementasi `findOrCreateReference()` di renderer persist.

### 1.2 Field Reserved & Lifecycle (`doc_status`)
Field berikut **reserved** — tidak boleh dipakai ulang sebagai nama field
custom, otomatis ada di semua `type: document`, framework-managed: `owner`,
`created_at`, `modified`, `doc_status`, `amends`, `amended_by`, `version`.
`transaction_date` **wajib** dideklarasikan eksplisit untuk `characteristic:
transaction` — `formspec apply` menolak kalau tidak ada.

Setiap Entity punya lifecycle bawaan lewat `doc_status` (`draft |
submitted | cancelled`, closed set — kebutuhan proses bisnis granular pakai
field terpisah, lihat [`02-core-extended.md`](02-core-extended.md) §1),
ditegakkan lewat delapan reserved action:

| Action | Guard dasar | Post-condition |
|---|---|---|
| `create` | — | `doc_status = draft` |
| `update` | `doc_status == draft` | — |
| `submit` | `doc_status == draft` | `doc_status = submitted` |
| `cancel` | `doc_status == submitted AND no_pending_references` | `doc_status = cancelled` |
| `delete` | `doc_status == draft AND no_referencing_documents` | row dihapus (guard absolut, **tanpa** `override_permission`) |
| `amend` | `doc_status == submitted OR cancelled` | atomik: cancel original + set `amended_by` + buat Entity baru linked (`amends`) sebagai `draft` |
| `create-submit` | gabungan `create`+`submit` | derivasi otomatis kalau keduanya aktif |
| `amend-submit` | gabungan `amend`+`submit` | derivasi otomatis kalau keduanya aktif |

Developer boleh menambah `conditions` di atas guard dasar, **tidak boleh**
melemahkannya. `create-submit`/`amend-submit` otomatis tersedia (tidak perlu
dideklarasikan) begitu kedua action penyusunnya aktif; `formspec apply` menolak
deklarasi eksplisit `create-submit` kalau `submit` di-`disabled: true`.

**Gating transitif:** `submit` nonaktif → `cancel` dan `amend` implisit
nonaktif; `cancel` nonaktif → `amend` implisit nonaktif. Kalau ketiganya
nonaktif (eksplisit atau transitif), Entity itu **lifecycle-free**:
`doc_status` selalu `null`, guard lifecycle di `update`/`delete` di-bypass,
berperilaku plain CRUD — ini bukan kategori resource keempat, cuma Entity
dengan lifecycle nonaktif, zero-cost.

**`delete` vs `cancel`:** `delete` menghapus row — guard-nya absolut (setara
`ON DELETE RESTRICT`), berlaku dari tipe field `relation` yang menunjuk ke
sini, terlepas dari `doc_status`. `cancel` tidak menghapus row, cuma mengubah
status — guard-nya bisa dibuka lewat handler yang membongkar dependency dulu.
**`update` setelah `submit` selalu ditolak, tanpa pengecualian** — inilah yang
membuat Entity "immutable" setelah submit; perubahan field spesifik pasca-
submit tetap mungkin lewat custom action bernama (tercatat di audit log
sebagai nama action, bukan "document updated").

**Referenceability:** hanya Entity `doc_status = null` (lifecycle-free) atau
`'submitted'` yang boleh jadi target field `relation` — `draft`/`cancelled`
ditolak sebagai target relation saat runtime.

### 1.3 `child` vs `relation`
Garis pembeda adalah **kepemilikan lifecycle**, bukan bentuk penyimpanan:

| Aspek | `child` | `relation` |
|---|---|---|
| Lifecycle | Ikut parent — submit/cancel parent otomatis diteruskan | Independen — `doc_status` sendiri |
| Identitas | `storage: jsonb` → tanpa UUID, embedded; `storage: table` → UUID v7 sendiri | UUID v7 sendiri, independen |
| Eksistensi | Tidak bisa ada tanpa parent | Bisa berdiri sendiri |

Uji keputusan: "apakah punya makna di luar parent?" — line item invoice tanpa
Invoice tidak bermakna (`child`); Order mereferensi Customer tapi keduanya
berdiri sendiri (`relation`). Bahkan child ber-`storage: table` dengan UUID
sendiri tetap ikut ter-submit/cancel bersama parent — lifecycle-nya tidak
pernah independen. Detail layout storage (kolom generated, tabel child) adalah
implementasi backend — lihat
[`../../renderers/jsonb-persist/02-schema-strategies.md`](../../renderers/jsonb-persist/02-schema-strategies.md).

**`sequence_field` — line-ordering eksplisit.** Sebuah `child` array boleh
menetapkan `sequence_field: <field-name>` yang menunjuk field child mana yang
membawa nomor urut baris eksplisit (mis. `line_number`):

```yaml
- name: items
  type: child
  child:
    resource: invoice_item
    storage: table
    sequence_field: line_number
```

Framework **memelihara dan memvalidasi urutan monotonik** field ini pada
insert/reorder — nilai duplikat atau non-monotonik di antara sibling ditolak
`VALIDATION_ERROR` (422). Framework **tidak** merenumber ulang sibling yang sudah
ada secara otomatis kecuali diminta eksplisit (mis. operasi reorder yang memang
menugaskan ulang seluruh nomor) — menyisip di tengah tidak diam-diam menggeser
nomor baris lain. Ini berbeda dari urutan penyimpanan implisit: `sequence_field`
membuat urutan menjadi **data yang bermakna dan stabil**, bukan artefak insertion
order. Katalog field-nya sendiri ada di
[`05-field-types.md`](05-field-types.md) §1.4.

`relation.on_delete`: `restrict` (default, absolut — sama dengan guard
`delete` §1.2) | `cascade` (ikut terhapus, hanya kalau referencing document
`draft`/lifecycle-free) | `set_null` (hanya valid kalau field tidak
`required`).

### 1.4 `spec.auth` — Persyaratan Autentikasi
Entity (dan Service, `pkg/spec/resources.go`) boleh mendeklarasikan
persyaratan autentikasi lewat `spec.auth`:

```yaml
spec:
  auth:
    required: true              # operasi Entity wajib terautentikasi
    strategies: [sso, passkey]  # strategy autentikasi yang diterima
```

- `required` — `bool`. Menandai operasi Entity ini wajib dijalankan oleh
  caller terautentikasi.
- `strategies` — daftar nama strategy autentikasi yang diterima. Set strategy
  **terbuka untuk ditambah** (bukan closed enum): `basic-auth`, `sso`
  (OIDC/SAML), `social-sso` (Google, Facebook, GitHub, dst), `passwordless`
  (magic link/OTP), `passkey` (WebAuthn), dst — strategy baru didaftarkan
  sebagai artifact, mengikuti trust tier yang sama dengan artifact lain
  ([`../../spec/platform/02-workspace-app-module.md`](../../spec/platform/02-workspace-app-module.md) §3).

Field ini **deklaratif** — kontrak konsumsi untuk tooling, App renderer, dan
audit. Enforcement nyata tetap di lapisan autentikasi (§8): otorisasi
berbasis permission `{module}.{entity}.{action}` dijalankan server-side
selalu, tanpa pengecualian.

### 1.5 Entity Spec Reference — Atribut Lengkap

Berikut adalah seluruh atribut yang bisa dideklarasikan di `spec` Entity.
Atribut wajib ditandai **\[wajib\]**.

| Atribut | Tipe | Wajib | Default | Keterangan |
|---|---|---|---|---|
| `version` | string | **Ya** | — | Versi skema Entity. Selalu `v1` untuk Entity baru. |
| `characteristic` | enum | **Ya** | — | `master` / `transaction` / `reference` / `summary`. Mutually exclusive. |
| `plural` | string | — | auto (nama + `s`) | Nama jamak untuk URL collection. |
| `display_field` | string | — | `name` / field pertama | Field yang dipakai sebagai label di UI (dropdown, breadcrumb). |
| `lifecycle` | string | — | `two_step_autosave` | `two_step_autosave` / `two_step_manual` / `plain_crud`. String enum, bukan map. |
| `soft_deactivate` | object | — | disabled | `{ enabled: true }` — tambah action `deactivate` + `reactivate`. |
| `fields` | array | — | `[]` | Daftar field custom. Lihat [`05-field-types.md`](05-field-types.md). |
| `state_machine` | object | — | — | State machine untuk business states di luar `doc_status`. Field: `field`, `initial`, `states[]`, `transitions[]`. Lihat §1.6. |
| `actions` | array | — | reserved actions | Custom action bernama di luar reserved. Field: `name`, `description`, `required_permission`, `uses`, `audit`, `ui`, `idempotent`. |
| `expose` | array | — | `[]` (UI only) | `[{ type: rest, actions: [...] }]` — kontrol permukaan external API (§8.4). |
| `persist` | object | — | — | `{ soft_delete, category, indexes[] }` — kontrak persistensi (§3). |
| `auth` | object | — | `{ required: true }` | `{ required, strategies[] }` — persyaratan autentikasi (§1.4). |
| `events` | array | — | reserved events | Event custom di luar `before_*`/`on_*` reserved. (§7). |
| `indexes` | array | — | `[]` | Indeks tambahan: `[{ fields: [...], unique: true/false }]`. |

### 1.6 `doc_status` vs `state_machine` — Dua Lapis State

Entity punya **dua lapis state** yang independen namun bisa berinteraksi:

| Lapis | Dikelola oleh | Scope | Kustomisasi |
|---|---|---|---|
| `doc_status` | Framework (built-in) | Lifecycle dokumen: `draft → submitted → cancelled` | Disable lewat `lifecycle: plain_crud` atau disable `submit`/`cancel`/`amend` |
| Custom `state_machine` | Developer (deklaratif) | State bisnis: `draft → in_progress → completed` | Bebas mendefinisikan states, transitions, dan actions via `state_machine` block |

Keduanya berjalan **bersamaan**: Entity bisa punya `doc_status: submitted`
(immutable secara data) DAN `state_machine.field: in_progress` (state bisnis).
Transition custom `via: complete` bisa di-guard dengan `conditions` yang
memeriksa `doc_status` — misalnya, hanya izinkan transisi bisnis kalau
dokumen sudah `submitted`.

```yaml
spec:
  version: v1
  characteristic: transaction
  lifecycle: two_step_autosave     # doc_status aktif
  fields:
    - name: status
      type: enum
      enum_values: [draft, in_progress, completed, cancelled]
      default: draft
      index: true
  state_machine:                   # state bisnis — paralel dengan doc_status
    field: status
    initial: draft
    states:
      - { name: draft, label: "Draft" }
      - { name: in_progress, label: "In Progress" }
      - { name: completed, label: "Completed" }
      - { name: cancelled, label: "Cancelled" }
    transitions:
      - { from: draft, to: in_progress, via: start-work }
      - { from: in_progress, to: completed, via: complete }
      - { from: "*", to: cancelled, via: cancel }
  actions:
    - name: submit
      disabled: false
    - name: start-work
      description: "Mulai pengerjaan"
      required_permission: billing.invoice.start-work
      audit: true
```

Untuk Workflow (approval multi-level yang meng-intercept transition
state_machine), lihat [`02-core-extended.md`](02-core-extended.md) §2.

## 2. Primary Key & Natural Key
Primary key: **UUID v7** (time-ordered) untuk semua Entity — ini kontrak,
bukan pilihan per backend. Natural key adalah **unique constraint per
tenant**, bukan pernah jadi PK:

```yaml
- name: number
  type: string
  natural_key: true
  immutable: true
  unique: true
  natural_key_rule:
    strategy: sequence            # sequence | custom
    format: "{prefix}-{year}-{seq:06d}"
    prefix: { config: billing.invoice_prefix, default: "INV" }
    reset: yearly                 # never | yearly | monthly | daily
    scope_field: branch_id        # opsional — sequence terpisah per nilai field ini
```

Jaminan generasi (gap-free, atomik, duplicate-free) adalah kontrak yang wajib
dipenuhi tiap PersistBackend lewat `ctx.next_key` — lihat
[`04-persist-backend.md`](04-persist-backend.md) §2 untuk mekanismenya.

## 3. Persistence Sebagai Kontrak

**Transaksi adalah kewajiban, bukan opsi.** FormSpec adalah framework aplikasi
bisnis: setiap mutasi yang secara logis satu unit (mutasi entity + guard
lifecycle + counter natural key + penulisan outbox) **wajib** atomik dalam
satu transaksi PersistBackend — commit semua atau tidak sama sekali
([`04-persist-backend.md`](04-persist-backend.md) §2, §3). **Integritas data
ditegakkan di backend, selalu** — validasi, rules, guard lifecycle, dan
constraint referensial dievaluasi server-side pada setiap jalur masuk
(HTTP, script, event); frontend menegakkan hal yang sama untuk UX tapi tidak
pernah menjadi satu-satunya penjaga — payload yang melewati frontend (atau
dikirim langsung oleh klien yang tidak jujur) tetap tertahan di backend.

**Unknown field = rejection (normatif).** Setiap field di payload
`create`/`update` **wajib** ada di Entity spec (`spec.fields[]`), child
relation (`spec.fields[].type: child`), atau reserved field name (framework-
managed, tidak bisa dikirim user). Field yang tidak dikenal **harus ditolak**
dengan `VALIDATION_ERROR` (422) — framework tidak boleh diam-diam menerima
atau mengabaikan field arbitrer. Ini berlaku untuk semua jalur masuk: HTTP
(UI surface + external API), Starlark script, dan event handler. Script
yang membaca/menulis field tidak dikenal adalah error validasi, bukan
silent no-op.

**Multi-datastore per workspace (normatif).** Satu workspace **boleh**
punya lebih dari satu Datastore transactional — binding-nya di level
**Module**, bukan pilihan bebas per kode
([`../platform/06-datastore.md`](../platform/06-datastore.md) §1.1). Setiap
Module resolve `ctx.db()` tanpa argumen ke **satu** Datastore yang di-bind
ke Module itu (default `'default'`); tidak ada jalan bagi kode untuk
"lupa menyebut" datastore lain, karena tidak ada datastore lain yang bisa
dijangkau dari `ctx.db()` polos. Konsekuensinya: mutasi yang melibatkan dua
Module dengan Datastore berbeda **tidak pernah** atomik dalam satu transaksi
— interaksi lintas-Module-lintas-Datastore **wajib** lewat event-subscribe/
outbox (§7 di bawah), **tidak ada** escape hatch `ctx.db` lintas-Datastore
sekalipun dengan `uses` consent
([`../platform/02-workspace-app-module.md`](../platform/02-workspace-app-module.md)
§7). Ini beda datastore = beda deployment boundary = wajib async, bukan
sekadar butuh izin lebih tinggi.

`spec.persist` mendeklarasikan apa yang dijanjikan framework ke Entity itu
— bukan bagaimana backend memenuhinya:

```yaml
persist:
  soft_delete: true
  category: operational   # operational | financial | compliance | analytics | master | archive
  indexes:
    - { field: status, type: btree }
```

`category` adalah pengelompokan data yang framework jamin **tidak boleh
di-join lintas kategori** (isolasi, bukan sekadar performa) — cara sebuah
PersistBackend mewujudkan batas ini (schema Postgres terpisah, database
terpisah, dll.) adalah detail implementasinya, lihat
[`../../renderers/jsonb-persist/02-schema-strategies.md`](../../renderers/jsonb-persist/02-schema-strategies.md).
`indexes` memenuhi kontrak `04-persist-backend.md` §2 "Index generation".

## 4. Migration = Structural Diff
Framework menghasilkan structural diff dari perubahan spec; PersistBackend
menerima diff dan menerjemahkannya ke storage-nya sendiri. Tidak ada asumsi
"framework generate SQL" — lihat [`04-persist-backend.md`](04-persist-backend.md)
§2 untuk kontrak lengkapnya (aturan `renamed_from`, dua tahap untuk field
removal, dll).

Tiga jenis migrasi: **structural** (otomatis penuh dari diff Entity, tidak
pernah ditulis tangan), **custom DDL** (`kind: Migration` — index, function,
trigger, extension, materialized view; DML ditolak saat runtime), **data
migration** (script ber-versi, run/rollback manual — backfill masuk sini,
bukan structural diff).

## 5. Action

`impl.native` / `impl.script` / `impl.script_ref` / `impl.compiled` /
`impl.sidecar`; context yang tersedia; hooks (before/after/on_error).

### 5.1 UI Hints

Setiap action bisa mendeklarasikan `ui` — petunjuk rendering untuk frontend,
baik di tabel/kanban (row action) maupun di detail page (transition button).
Field ini opsional dan tidak memengaruhi perilaku backend.

```yaml
actions:
  - name: start-consultation
    ui:
      button_label: "Mulai Konsultasi"   # label tombol (default: action name)
      icon: play                         # ikon lucide-react
      style: primary                     # primary | secondary | danger
      confirm: "Panggil pasien ini ke ruang konsultasi?"  # pesan konfirmasi
```

| Field | Tipe | Default | Keterangan |
|---|---|---|---|
| `button_label` | string | action name | Label tombol di UI |
| `icon` | string | — | Nama ikon lucide-react (kebab-case, mis. `play`, `x`, `check`) |
| `style` | string | `secondary` | `primary` (tombol solid), `secondary` (outline), `danger` (merah) |
| `confirm` | string | — | Jika diisi, munculkan ConfirmDialog sebelum eksekusi; nilai = pesan |
| `show_when` | string | — | FormSpecExpr; tombol hanya ditampilkan jika expression `true` |

`confirm` adalah satu-satunya mekanisme konfirmasi untuk action — definisikan
di entity, bukan di table/kanban. Table/kanban tetap bisa override via
`confirm_msg` di row action-nya, tapi sumber kebenaran ada di entity.

**Model permission (normatif untuk kelima jenis impl).** Setiap action
mendeklarasikan dua hal secara eksplisit:
- `required_permission` — guard bagi si pemanggil: siapa yang boleh memanggil.
- `uses` — akses kode action itu sendiri: tier database, resource lain,
  primitives (`ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, dst.).

**Grant tidak pernah diturunkan dari pemakaian aktual di kode** — deklarasi
adalah satu-satunya sumber kebenaran; implementasi wajib menegakkan permission
lewat identity proxy untuk kelima jenis impl secara seragam, bukan hanya untuk
`native`. `uses` yang undeclared harus ditolak saat resolusi, bukan silently
diizinkan. Auto-scan kode (mis. `formspec validate`) hanya berperan sebagai
verifikator kejujuran deklarasi terhadap kode — bukan sumber grant, dan tidak
pernah memberi grant sendiri. Footprint modul (agregat seluruh
`required_permission` + `uses` miliknya) adalah dasar consent yang wajib
ditampilkan ke pemilik workspace saat instalasi.

**Scope `ctx.db` default = module sendiri.** `ctx.db()` tanpa argumen
resolve ke Datastore yang di-bind ke Module pemanggil (§3, biasanya sama
untuk seluruh workspace, tapi boleh berbeda per Module —
[`../platform/06-datastore.md`](../platform/06-datastore.md) §1.1). Akses
`ctx.db` lintas-module **wajib** dideklarasikan eksplisit di `uses` dan
muncul di consent footprint; **tulis lintas-module** disajikan sebagai
consent risiko-tinggi, presentasinya berbeda dari akses biasa. Akses
`ctx.db` yang tidak dideklarasikan — bahkan sekadar mengecek keberadaan
data — diblokir saat runtime, memicu alert, dan **men-suspend module
secara otomatis** disertai insiden audit
(`USES_VIOLATION`, [`../platform/05-plane-protocol.md`](../platform/05-plane-protocol.md) §4.4).
**Kalau Module pemanggil dan Module target di-bind ke Datastore yang
berbeda, tidak ada bentuk consent yang membuka akses `ctx.db` langsung** —
satu-satunya jalur adalah event-subscribe/outbox (§3, §7).

**Idempotensi.** `idempotent: true` mensyaratkan sumber `idempotency_key`
(`header` | `param` | `server`). Untuk sumber `server`, framework menyediakan
alur **prepare dua-langkah**: klien meminta kunci lebih dulu lewat
`POST /{resource}/{action}/prepare`, menerima sebuah key, lalu mengirim ulang
panggilan action sebenarnya dengan key itu terlampir. Ini melindungi dari
double-submit browser pada action `create` yang tidak punya kunci idempotency
alami dari sisi klien. Framework menjaga idempotency store
`(tenant, action, key) → pending|completed + response tersimpan`. Duplikat
setelah completed → replay response asli; duplikat saat masih pending →
tunggu/409. Entry kedaluwarsa lewat retention (default 24 jam, dibaca dari
`core.idempotency_retention`), tidak pernah dihapus saat commit.

**Optimistic concurrency lewat `version`** — default aktif di semua Entity:
update wajib membawa `version` yang dibaca client; mismatch → `409 CONFLICT`
dengan version terkini. `modified` (timestamp) murni metadata audit, bukan
mekanisme konkurensi.

## 6. Query & Filter Operator

Konvensi HTTP query yang wajib diimplementasikan setiap PersistBackend secara
identik (bukan sekadar direkomendasikan):

```
?page&per_page&sort&direction&fields&filter[field][op]=value&search&include
```

Filter operator yang wajib didukung: `eq neq gt gte lt lte between in nin like
ilike null notnull`. `per_page` default **20**, maksimum **100** — nilai di
atas maksimum di-clamp (bukan ditolak); nilai non-numerik atau negatif adalah
`VALIDATION_ERROR` (422). Implementasi BOLEH menurunkan batas maksimum per
dokumen tapi TIDAK BOLEH menaikkannya di atas 100.

**Type-aware sort & filter.** Saat `sort` atau `filter` mengenai field yang
disimpan di JSONB (`data`), PersistBackend mengekstrak nilainya sebagai text
lalu meng-cast ke tipe native field yang dideklarasikan di Entity spec. Ini
memastikan `?sort=queue_position` mengurutkan secara numerik (1, 2, 10) bukan
lexicographic (1, 10, 2). Field tanpa tipe yang perlu cast (`string`,
`richtext`, `enum`, `uuid`, `json`, `file`, `relation`, `money`, `child`)
dibiarkan sebagai text. Developer tidak perlu `index: true` semata-mata untuk
sort/filter yang benar — index tetap berguna untuk performa pada data besar.

## 7. Event & Outbox

**Konvensi penamaan** mengunci tipe event lewat prefix: `before_*` (mis.
`before_cancel`) **selalu sync** — gate yang harus selesai sebelum state
berubah. `on_*` (mis. `on_submit`) **selalu async** — notifikasi setelah
commit. Event custom di luar pola ini **wajib** mendeklarasikan `type`
eksplisit. `formspec apply` menolak event yang `type`-nya kontradiktif dengan
prefix-nya. Aturan ini otomatis berlaku untuk kedelapan reserved action (§1.2)
— tiap reserved action punya `before_{action}` (sync) dan `on_{action}`
(async) berpasangan tanpa perlu dideklarasikan manual.

**Prioritas handler** (event sync): urutan `priority` (kecil dijalankan
duluan) — kelipatan 10 supaya handler baru bisa disisipkan tanpa
renumbering. Tier: Critical (1–9, gate yang harus dicek pertama), Normal
(10–89, default **10**, mayoritas business logic), Low (90–99, side-effect
non-kritis tapi tetap sync).

**Kontrak durabilitas.** `publish.durable: true` → event ditulis ke outbox
sebelum action return; untuk Entity, transaksi yang sama dengan perubahan
data (atomik). Reliabilitas mensyaratkan **kedua sisi**: publisher durable +
subscriber durable = reliable. Publisher non-durable + subscriber durable =
error validasi.

**Outbox (normatif).** PersistBackend wajib menyediakan tabel outbox dan
worker: poll pending → cek idempotency → **sync call** ke target action →
delivered, atau backoff retry → dead-letter. `retry.initial_delay_ms` menyetel
jeda sebelum percobaan retry **pertama**; retry berikutnya mengikuti strategi
`backoff` yang dideklarasikan mulai dari jeda itu.

**`kind: Subscription`** — module lain bereaksi terhadap event resource lain
tanpa mengubah publisher (lihat [`02-core-extended.md`](02-core-extended.md)
§3 untuk mode streaming/durable-nya). Kontrak durabilitas dua-sisi berlaku
sama; Subscription masuk consent footprint module konsumen.

## 8. Dua Permukaan API: UI vs External

FormSpec menyediakan **dua permukaan (surface) API** untuk operasi data, dengan
auth, gating, dan visibility yang berbeda — mengikuti pola route Laravel
(`web.php` untuk UI, `api.php` untuk external service).

### 8.1 Permukaan UI — `_ui/entity`

Permukaan ini **selalu tersedia** untuk setiap Entity, tanpa memerlukan
`spec.expose`. Digunakan oleh seluruh UI kind (Form, Table, Kanban, Timeline,
Calendar) untuk operasi CRUD dan custom action.

```
GET    /{ws}/_ui/entity/{module}/{entity}             → list
GET    /{ws}/_ui/entity/{module}/{entity}/{id}         → find
POST   /{ws}/_ui/entity/{module}/{entity}              → create
PATCH  /{ws}/_ui/entity/{module}/{entity}/{id}         → update
DELETE /{ws}/_ui/entity/{module}/{entity}/{id}         → delete
POST   /{ws}/_ui/entity/{module}/{entity}/{id}/{action} → custom action
```

| Aspek | Ketentuan |
|---|---|
| Auth | Session cookie / OAuth (user agent). **Tidak menerima** API key. |
| Gating | Permission `list`/`view`/`create`/`update`/`delete` + `required_permission` action-level ([§5](#5-action)) — **tidak** digerbangi `spec.expose` |
| Field visibility | Semua field kecuali yang di-strip oleh `required_permission` field-level ([`05-field-types.md`](05-field-types.md) §5.3) |
| Rate limit | Per user session |
| Audit attribution | `user_id`, `session_id` |
| Ketersediaan | Selalu ada untuk setiap Entity yang user punya permission-nya |

### 8.2 Permukaan External — `api/v1`

Permukaan ini **hanya tersedia kalau Entity opt-in** lewat `spec.expose:
[{type: rest} | {type: grpc} | {type: ws}]`. Digunakan oleh third-party
service, SDK (`formspec generate`), dan integrasi eksternal.

```
GET    /{ws}/api/v1/{module}/{plural}             → list
GET    /{ws}/api/v1/{module}/{plural}/{id}         → find
POST   /{ws}/api/v1/{module}/{plural}              → create
PATCH  /{ws}/api/v1/{module}/{plural}/{id}         → update
DELETE /{ws}/api/v1/{module}/{plural}/{id}         → delete
POST   /{ws}/api/v1/{module}/{plural}/{id}/{action} → custom action
```

| Aspek | Ketentuan |
|---|---|
| Auth | API key header (`X-FormSpec-Key`). **Tidak menerima** session cookie. |
| Gating | `spec.expose` (deny-by-default) + `required_permission` action-level ([§5](#5-action)) |
| Field visibility | Field dengan `exclude: [public_api]` disembunyikan dari respons ([`05-field-types.md`](05-field-types.md) §5.3) |
| Rate limit | Per API key + plan tier |
| Audit attribution | `api_key_id`, `service_label` |
| Ketersediaan | Hanya untuk Entity dengan `spec.expose` — tanpa expose, endpoint 404 |

`formspec generate` hanya menghasilkan typed client untuk permukaan external ini.
Kalau tidak ada entity yang exposed, `formspec generate` menolak berjalan — tidak
ada yang bisa digenerate.

### 8.3 Router & Middleware

Router **wajib** memakai radix-tree (atau setara) dengan lookup O(jumlah segmen
path) — jumlah route tidak boleh mendegradasi performa secara linear. Dua
permukaan didaftarkan sebagai **dua route group** dengan middleware auth yang
berbeda di level router:

| Route group | Middleware auth |
|---|---|
| `/_ui/` | Session cookie / OAuth |
| `/api/v1/` | API key header (`X-FormSpec-Key`) |

**Satu logic path internal.** Engine (validasi, permission enforcement, guard
lifecycle, state machine, natural key generation, event publishing) adalah **satu
code path** yang dipanggil dari dua entry point di atas — tidak ada duplikasi
logika bisnis. Yang berbeda hanya: auth method, rate limiting, field visibility
per surface, dan audit attribution.

Pemanggil internal (same-process service, script Starlark, event) **melewati
jaringan sepenuhnya** — dispatch fungsi langsung tanpa overhead serialisasi, dan
tidak terikat aturan `spec.expose` atau auth method manapun. Permukaan gRPC dan
WebSocket mengikuti kontrak yang sama dengan REST external ([§8.2](#82-permukaan-external--apiv1)).

### 8.4 `spec.expose` — Definisi Ulang

`spec.expose` **hanya** mengontrol ketersediaan di permukaan external
([§8.2](#82-permukaan-external--apiv1)). Ia **tidak** memengaruhi:

- Permukaan UI (`/_ui/entity/`) — selalu tersedia, gated permission
- Pemanggil internal (same-process, Starlark script, event) — selalu tersedia
- `/_meta/` endpoint — selalu tersedia, gated permission

```yaml
# Entity tanpa expose: UI tetap bisa CRUD, tapi third-party tidak bisa akses API
spec:
  expose: []   # atau tidak dideklarasikan sama sekali → UI jalan, external 404

# Entity dengan expose: UI + external service keduanya bisa
spec:
  expose:
    - type: rest
      actions: [list, find]          # read-only external API (UI tetap full CRUD)
```

`kind: Api` ([`02-core-extended.md`](02-core-extended.md) §12) hanya
meng-override bagaimana permukaan external dipublikasikan — `base_path`,
`version`, `disable` — dan tidak berlaku untuk permukaan UI.

### 8.5 Shared Contract

Kedua permukaan **wajib** mematuhi kontrak yang sama untuk:

| Kontrak | Referensi |
|---|---|
| Workspace prefix (`/{workspace_slug}`) | §8.2 — router wajib jatuh ke UUID kalau slug tidak diset |
| Response envelope (`data`, `meta`, `error`) | `list: { data, meta: {page, per_page, total, total_pages}, links }`, `single: { data, meta: {request_id, timestamp} }`, `error: { error: {code, message, details}, meta }` |
| Kode error standar | `VALIDATION_ERROR` (422), `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409), `STATE_TRANSITION_ERROR` (422), `INTERNAL_ERROR` (500) — daftar lengkap: [`error-glossary.yaml`](error-glossary.yaml) |
| Query & filter ([§6](#6-query--filter-operator)) | `?page&per_page&sort&direction&fields&filter[...]&search&include` |
| Optimistic concurrency (`version`) | Update wajib membawa `version`; mismatch → `409 CONFLICT` |

## 9. Error Model
Kode kanonik berformat `FORMSPEC.{DOMAIN}.{REASON}` (mis.
`FORMSPEC.DOC.UPDATE_NOT_DRAFT`, `FORMSPEC.PERIOD.CLOSED`) — satu file
`error-glossary.yaml`, versioned bersama Core Basic, dipakai ganda sebagai
matcher programatik **dan** kunci lookup i18n (tidak ada field `key` terpisah).
`code` **tidak pernah** diubah atau dipakai ulang setelah rilis — integrasi
pihak ketiga yang `switch(error.code)` tidak boleh diam-diam rusak; situasi
baru menambah entri baru. Error dari mekanisme framework (guard reserved
action, dll.) wajib pakai kode kanonik ini, bukan pesan inline hard-coded.
`conditions:` custom milik developer bebas pakai pesan sendiri, SEBAIKNYA
tetap format `code` + `params` dengan namespace App sendiri (bukan `FORMSPEC.*`).

## 10. Config & Global Settings

Config adalah manifest, bukan dotenv:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: app
  module: core
spec:
  keys:
    invoice_due_days: { type: int, default: 30 }
    smtp_host:        { type: string, secret: true }
```

Nilai di-resolve per environment. Secret dan definisi environment digovern
Control Plane ([`../platform/04-control-plane.md`](../platform/04-control-plane.md)
§2). Script membaca lewat `ctx.config.get("key")` — tidak pernah env var
mentah.

**Global settings — jangan pernah menebak.** Setting yang memengaruhi
interpretasi data atau tampilan lintas-komponen (currency, locale, timezone,
format tanggal/angka, awal tahun fiskal, dst.) hidup di **satu tempat: level
global** (workspace/App Config di bawah namespace `settings.*`) — bukan
ditebak per komponen:

- Spec **wajib** menetapkan nilai default standar yang bisa diterima umum
  untuk setiap setting global (mis. format tanggal ISO-8601), sehingga
  perilaku konsisten di seluruh komponen walau tidak diset — dan bisa diubah
  di satu tempat kalau tidak sesuai.
- Komponen/renderer **dilarang menebak** (mis. menyimpulkan "ini currency"
  dari heuristik rule numerik) — ia membaca setting global atau deklarasi
  eksplisit di manifest. Setting yang dibutuhkan tapi tidak tersedia dan
  tidak punya default standar adalah **error**, bukan tebakan diam-diam —
  menebak membuat default tiap komponen berbeda dan perilaku tidak konsisten.

Contoh konkret namespace `settings.*` di workspace Config:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: workspace
  module: formspec.core
spec:
  keys:
    settings.default_currency:  { type: string, default: "USD" }
    settings.locale:            { type: string, default: "en-US" }
    settings.timezone:          { type: string, default: "UTC" }
    settings.date_format:       { type: string, default: "YYYY-MM-DD" }
    settings.fiscal_year_start: { type: string, default: "01-01" }
```

Module membaca lewat `ctx.config.get("settings.default_currency")` —
namespace `settings.*` memastikan tidak bentrok dengan key config milik
Module sendiri.

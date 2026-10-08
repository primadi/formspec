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

| Characteristic | Arti                                                                                                                                                                                            | Wajib                                                   |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `master`       | Data referensi stabil (Customer, Product)                                                                                                                                                       | Boleh punya lifecycle (kalau `submit` aktif) atau tidak |
| `transaction`  | Append-heavy, time-partitioned (Invoice, Journal Entry)                                                                                                                                         | Wajib field `transaction_date`                          |
| `reference`    | Seed data read-only, dimiliki App Owner (Provinsi, Tarif Pajak). Backend mendukung **find-or-create**: jika record belum ada saat diakses via GET, framework auto-create dengan field defaults. | —                                                       |
| `summary`      | Projeksi terkelola sistem (GL Balance)                                                                                                                                                          | `create`/`update`/`delete` permanen nonaktif via API    |

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

| Action          | Guard dasar                                         | Post-condition                                                                                  |
| --------------- | --------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `create`        | —                                                   | `doc_status = draft`                                                                            |
| `update`        | `doc_status == draft`                               | —                                                                                               |
| `submit`        | `doc_status == draft`                               | `doc_status = submitted`                                                                        |
| `cancel`        | `doc_status == submitted AND no_pending_references` | `doc_status = cancelled`                                                                        |
| `delete`        | `doc_status == draft AND no_referencing_documents`  | row dihapus (guard absolut, **tanpa** `override_permission`)                                    |
| `amend`         | `doc_status == submitted OR cancelled`              | atomik: cancel original + set `amended_by` + buat Entity baru linked (`amends`) sebagai `draft` |
| `create-submit` | gabungan `create`+`submit`                          | derivasi otomatis kalau keduanya aktif                                                          |
| `amend-submit`  | gabungan `amend`+`submit`                           | derivasi otomatis kalau keduanya aktif                                                          |

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

**`lifecycle:` adalah hint UI, bukan penentu (D2).** Nilai `two_step_autosave` /
`two_step_manual` / `plain_crud` **tidak** dibaca untuk memutuskan apakah sebuah
Entity punya lifecycle. Yang menentukan adalah **aksi mana yang aktif**: kalau
`submit`, `cancel`, dan `amend` semuanya nonaktif (eksplisit atau lewat gating
transitif di atas), Entity itu lifecycle-free — `doc_status` selalu `null` dan
barisnya langsung bisa direferensikan. Konsekuensinya: mendeklarasikan
`lifecycle: plain_crud` dan tidak mendeklarasikan `lifecycle` sama sekali
memberi hasil yang sama; `lifecycle:` hanya menyetel perilaku form di klien.

### 1.3 `child` vs `relation`

Garis pembeda adalah **kepemilikan lifecycle**, bukan bentuk penyimpanan:

| Aspek      | `child`                                                                     | `relation`                        |
| ---------- | --------------------------------------------------------------------------- | --------------------------------- |
| Lifecycle  | Ikut parent — submit/cancel parent otomatis diteruskan                      | Independen — `doc_status` sendiri |
| Identitas  | `storage: jsonb` → tanpa UUID, embedded; `storage: table` → UUID v7 sendiri | UUID v7 sendiri, independen       |
| Eksistensi | Tidak bisa ada tanpa parent                                                 | Bisa berdiri sendiri              |

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

**`picker` — mengisi baris dengan memilih, bukan mengetik.** Sebuah `child`
boleh menetapkan `picker:`, yang membuat barisnya diisi dengan **memilih record
dari entity lain** alih-alih mengetik satu per satu:

```yaml
- name: lines
  type: child
  child:
    storage: jsonb
    sequence_field: line_no
    picker:
      entity: cafe-master.menu-item # sumber baris (relatif ke module, atau "module.entity")
      filter: { is_available: "true" } # pre-filter; nilainya boleh template {token}
      lookup: # OPSIONAL — baca SATU nilai per baris dari entity terkait
        entity: cafe-master.menu-item-price # dari mana nilainya dibaca
        key: menu_item_id # field di `entity` yang dicocokkan ke record sumber
        field: price # field di `entity` yang nilainya dipakai
        filter: { is_active: "true" } # narrowing KLIEN (template {token})
        scope: # narrowing yang DITEGAKKAN SERVER (bentuk `row_scope`)
          - {
              field: branch_id,
              from: route,
              param: session_id,
              via: cafe-order.table-session,
              via_field: branch_id,
            }
      display: # apa yang dilihat user (tile); murni presentasi
        name_field: name
        image_field: photo
        description_field: description
        category_field: menu_category_id # chip filter
        columns: 3
        search: true
        empty_text: "Menu belum tersedia"
      map: # apa yang DITULIS ke baris
        ref_field: menu_item_id # WAJIB — relasi kembali ke record sumber
        name_field: name_snapshot # snapshot
        lookup_field: unit_price_snapshot # menerima `lookup.field` (turunan)
        quantity_field: quantity
        note_field: note
        max_quantity: 20
    fields:
      - {
          name: menu_item_id,
          type: relation,
          relation: { resource: cafe-master.menu-item },
        }
      - { name: name_snapshot, type: string }
      - { name: unit_price_snapshot, type: money }
      - { name: quantity, type: integer }
      - { name: note, type: string }
```

**`lookup` — nilai per baris dari entity terkait.** Bentuknya generik dan itu
memang tujuannya: "baca satu nilai per baris dari tabel yang di-key oleh baris
itu" adalah satu pola, bukan fitur satu domain. Ganti kata bendanya dan ia
menjadi harga per outlet, tarif pajak per region, stok per gudang, tarif per
klinik, atau kuota per tenant. Entity picker memasok **baris**; `lookup`
memasok **nilai** yang menyertai tiap baris.

Tiga pasangan yang harus dibedakan, karena keduanya terlihat serupa:

| Deklarasi      | Sifat                                                                                     |
| -------------- | ----------------------------------------------------------------------------------------- |
| `filter`       | Narrowing **klien** (di-merge browser). Bukan batas — klien mana pun bisa menghilangkan   |
| `lookup.scope` | **Ditegakkan server**; `filter` di atas boleh dilebarkan, `scope` tidak                   |
| `lookup_field` | Nilai **turunan**: server membacanya dari `lookup.entity`, jadi kiriman klien **diganti** |

Alasan deklarasinya ada **di field child**, bukan di kind/halaman:

- **Berlaku di mana saja.** Form apa pun (dan langkah Wizard) yang mengedit
  entity itu mendapatkannya — susunan pesanan, pesanan pembelian, hitung stok,
  perpindahan stok, jurnal, resep, checklist: semuanya pola "pilih baris dari
  sumber + bawa jumlah + snapshot".
- **Jalur tulisnya tetap milik Form.** Baris yang dipilih adalah baris child
  biasa di state form, jadi submit memakai jalur yang sudah ada — validasi
  `rules`, permission, idempotency, `action:` lifecycle, redirect, dan event.
  Tidak ada jalur tulis kedua yang perlu dipelihara.

Aturan normatif:

| Aturan                                                          | Konsekuensi                                                                                                                                  |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `map.ref_field` wajib                                           | Tanpa relasi kembali ke sumber, baris tidak bisa dipetakan ulang                                                                             |
| Field di `map` wajib ada di `child.fields`                      | Snapshot yang menunjuk field tak ada akan hilang diam-diam — ditolak saat `formspec apply`                                                   |
| `quantity_field` wajib disertai `map.max_quantity`              | Batas per baris harus **dipilih**, tidak boleh tak terbatas (salah ketik di perangkat bersama = jumlah tak sengaja)                          |
| Tanpa `quantity_field` → satu baris per pilih                   | Kasus daftar (mis. baris jurnal memilih akun; checklist memilih item)                                                                        |
| `lookup` wajib disertai `entity` + `key` + `field`              | Sumber nilai dinyatakan, tidak ditebak                                                                                                       |
| `map.lookup_field` tanpa `lookup` ditolak                       | Field penerima tanpa pengisi tidak akan pernah berisi — dan karena server yang mengisinya, submit akan ditolak                               |
| Baris tanpa nilai lookup → tampil, **tidak bisa dipilih**       | Tidak ada nilai = tidak ada yang diambil; lebih baik tidak bisa dipilih daripada terkirim kosong                                             |
| Tidak ada baris lookup / lebih dari satu → **422**              | Fail closed: nilai tidak boleh jatuh ke kiriman klien, dan katalog harus tidak ambigu                                                        |
| `map.lookup_field` selalu **digantikan** server saat baris baru | Nilai turunan bukan milik pemanggil (aturan yang sama dengan `computed`)                                                                     |
| Baris yang sudah ada mempertahankan nilai tersimpan             | Beku, bukan dihitung ulang: mengedit jumlah tidak boleh mengubah nilai yang sudah disetujui                                                  |
| Snapshot (`name_field`/`lookup_field`)                          | Denormalisasi finansial (D2): record lama tetap terbaca setelah sumber berubah                                                               |
| Nilai `filter`/`lookup.filter` boleh template                   | `{dotted.path}` dari render context, plus `{now}`/`{today}`; token yang tak terselesaikan dibiarkan verbatim                                 |
| `lookup.scope` dengan `from: route` + `via` memakai RUJUKAN     | Nilainya dibaca dari record yang ditunjuk param, bukan dari param itu sendiri — klien menyebut SESI-nya, bukan dimensi yang ingin dilihatnya |

Pickernya **tidak** menghitung atau menyimpan total apa pun: total tetap urusan
`computed` field entity ([`05-field-types.md`](05-field-types.md) §2.1). Tampilan
berjalannya (subtotal pilihan) murni UI.

Sisi renderer: Form menempatkannya lewat `render.picker_panel` — `inline`
(default: tile di atas child grid, grid tetap editor barisnya) atau `aside`
(tile **dan** editor baris di kolom sendiri; child grid untuk field itu tidak
dirender dua kali) — lihat [`../frontend/06-page-kinds.md`](../frontend/06-page-kinds.md).

### 1.4 `spec.auth` — Persyaratan Autentikasi

Entity (dan Service, `pkg/spec/resources.go`) boleh mendeklarasikan
persyaratan autentikasi lewat `spec.auth`:

```yaml
spec:
  auth:
    required: true # operasi Entity wajib terautentikasi
    strategies: [sso, passkey] # strategy autentikasi yang diterima
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

| Atribut           | Tipe   | Wajib  | Default                | Keterangan                                                                                                                        |
| ----------------- | ------ | ------ | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `version`         | string | **Ya** | —                      | Versi skema Entity. Selalu `v1` untuk Entity baru.                                                                                |
| `characteristic`  | enum   | **Ya** | —                      | `master` / `transaction` / `reference` / `summary`. Mutually exclusive.                                                           |
| `plural`          | string | —      | auto (nama + `s`)      | Nama jamak untuk URL collection.                                                                                                  |
| `display_field`   | string | —      | `name` / field pertama | Field yang dipakai sebagai label di UI (dropdown, breadcrumb).                                                                    |
| `lifecycle`       | string | —      | `two_step_autosave`    | `two_step_autosave` / `two_step_manual` / `plain_crud`. String enum, bukan map.                                                   |
| `soft_deactivate` | object | —      | disabled               | `{ enabled: true }` — tambah action `deactivate` + `reactivate`.                                                                  |
| `fields`          | array  | —      | `[]`                   | Daftar field custom. Lihat [`05-field-types.md`](05-field-types.md).                                                              |
| `state_machine`   | object | —      | —                      | State machine untuk business states di luar `doc_status`. Field: `field`, `initial`, `states[]`, `transitions[]`. Lihat §1.6.     |
| `actions`         | array  | —      | reserved actions       | Custom action bernama di luar reserved. Field: `name`, `description`, `required_permission`, `uses`, `audit`, `ui`, `idempotent`. |
| `expose`          | array  | —      | `[]` (UI only)         | `[{ type: rest, actions: [...] }]` — kontrol permukaan external API (§8.4).                                                       |
| `persist`         | object | —      | —                      | `{ soft_delete, category, indexes[] }` — kontrak persistensi (§3).                                                                |
| `auth`            | object | —      | `{ required: true }`   | `{ required, strategies[] }` — persyaratan autentikasi (§1.4).                                                                    |
| `events`          | array  | —      | reserved events        | Event custom di luar `before_*`/`on_*` reserved. (§7).                                                                            |
| `indexes`         | array  | —      | `[]`                   | Indeks tambahan: `[{ fields: [...], unique: true/false }]`.                                                                       |
| `scope`           | object | —      | —                      | `{ dimension, field, required }` — Entity ini dipartisi per dimensi (§1.7).                                                       |
| `row_scope`       | array  | —      | `[]`                   | Filter yang **ditegakkan server** pada setiap baca: `[{ field, op, value` \| `from: session\|route, attr\|param }]` (§1.7).       |
| `assignments`     | array  | —      | `[]`                   | `[{ dimension, field, principal_field }]` — Entity ini memetakan principal ke nilai dimensi (§1.7).                               |

### 1.6 `doc_status` vs `state_machine` — Dua Lapis State

Entity punya **dua lapis state** yang independen namun bisa berinteraksi:

| Lapis                  | Dikelola oleh          | Scope                                              | Kustomisasi                                                                     |
| ---------------------- | ---------------------- | -------------------------------------------------- | ------------------------------------------------------------------------------- |
| `doc_status`           | Framework (built-in)   | Lifecycle dokumen: `draft → submitted → cancelled` | Disable lewat `lifecycle: plain_crud` atau disable `submit`/`cancel`/`amend`    |
| Custom `state_machine` | Developer (deklaratif) | State bisnis: `draft → in_progress → completed`    | Bebas mendefinisikan states, transitions, dan actions via `state_machine` block |

Keduanya berjalan **bersamaan**: Entity bisa punya `doc_status: submitted`
(immutable secara data) DAN `state_machine.field: in_progress` (state bisnis).
Transition custom `via: complete` bisa di-guard dengan `conditions` yang
memeriksa `doc_status` — misalnya, hanya izinkan transisi bisnis kalau
dokumen sudah `submitted`.

**Record lahir di `initial` (normatif).** `create` hanya boleh menghasilkan
record pada state `state_machine.initial`; mengirim state lain di payload
`create` **ditolak** `VALIDATION_ERROR` (422). Alasannya bukan kerapian
semata: state yang dicapai lewat transisi membawa **gerbang permission** dan
**`emit:`** milik transisi itu, dan keduanya hanya berjalan pada jalur
perubahan state (`update`). Record yang lahir di tengah siklus karena itu
melewati keduanya **tanpa gejala apa pun** — ia tampak sah, tetapi integrasi
yang seharusnya dipicu transisi (jurnal, proyeksi, notifikasi) tidak pernah
berjalan. Penulis **sistem** (seed, restore, migrasi) dikecualikan: ia
mereproduksi baris tersimpan apa adanya, bukan membuat record baru.

#### `via` adalah Action

`state_machine.transitions[].via` **mendeklarasikan action**. Nama `via` muncul
di `/{workspace}/api/v1/{module}/{plural}` sebagai action yang dapat dipanggil,
appear di meta bundle (`schema.actions`), dapat di-grant per-role, dan — bila
transisi mendeklarasikan `impl` — mendapat route sendiri di
`POST /_ui/entity/{module}/{entity}/{id}/{via}`.

Konsekuensinya, **satu nama tidak ditulis dua kali**. Blok `actions:` **tidak
perlu** mengulang `via`:

```yaml
spec:
  version: v1
  characteristic: transaction
  lifecycle: two_step_autosave # doc_status aktif
  fields:
    - name: status
      type: enum
      enum_values: [draft, in_progress, completed, cancelled]
      default: draft
      index: true
  state_machine: # state bisnis — paralel dengan doc_status
    field: status
    initial: draft
    states:
      - { name: draft, label: "Draft" }
      - { name: in_progress, label: "In Progress" }
      - { name: completed, label: "Completed" }
      - { name: cancelled, label: "Cancelled" }
    transitions:
      - { from: draft, to: in_progress, via: start-work }
      # Gate per-transisi: `PATCH` diperiksa `{module}.{plural}.update`, jadi
      # tanpa ini setiap pemegang `update` bisa menjalankan transisi apa pun.
      - {
          from: in_progress,
          to: completed,
          via: complete,
          description: "Selesaikan pekerjaan",
          require_permission: billing.invoices.complete,
        }
      - { from: "*", to: cancelled, via: cancel }
  # `actions:` hanya untuk action yang BUKAN `via` dari transisi mana pun.
```

Transisi boleh membawa **seluruh atribut action**, sehingga `via` tidak pernah
menjadi warga kelas dua:

| Field transisi                  | Arti                                                                                                          |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `via`                           | Nama action (wajib untuk bisa dipanggil; boleh kosong untuk transisi yang hanya berlaku lewat `PATCH`/script) |
| `description`                   | Deskripsi action; dipakai sebagai label tombol bila `ui.button_label` tidak ada                               |
| `impl`                          | Membuat action punya route sendiri (`POST /{id}/{via}`)                                                       |
| `require_permission`            | Gate yang diperiksa pada jalur `PATCH`/route; **inilah** tempat menulis permission, bukan di `actions:`       |
| `audit`, `emits`, `emit`        | Jejak audit dan tautan event                                                                                  |
| `conditions`, `ui`              | Aturan bisnis dan petunjuk tombol                                                                             |
| `idempotent`, `idempotency_key` | Semantik two-step (`/prepare`)                                                                                |
| `uses`, `expose`, `rate_limit`  | Footprint consent, protokol, limit                                                                            |
| `params`                        | Kontrak input — `validate` (rule) **dan** `inputs` (yang dirender dan diisi pemanggil); lihat di bawah        |

#### `params` — kontrak input yang ditegakkan, bukan sekadar ditolak

`params` punya dua paruh yang menjawab pertanyaan berbeda:

| Kunci         | Menjawab                                            | Bentuk                                                                                                                                       |
| ------------- | --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `validate`    | "body yang dikirim pemanggil sah?"                  | daftar rule per nama field (`{field, rules}`)                                                                                                |
| `inputs`      | "nilai apa yang harus **dikumpulkan**?"             | `ParamInput` — `name`, `type` (ad-hoc), `label`, `widget`, `required`/`required_when`, `visible_when`, `readonly_when`, `compute`, `persist` |
| `inputs_from` | "set input mana yang dipakai ulang?"                | nama di `Entity.spec.input_sets`                                                                                                             |
| `render`      | "container apa?" (`modal`/`drawer`/`separate_page`) | keputusan design-time, sejajar `Form.render`                                                                                                 |

Aturan yang mengikat keduanya:

- **Input yang namanya sama dengan sebuah field Entity MERUJUK field itu** —
  tipe, `enum_values`/`options`, dan `multiple` diwarisi, dan nilai yang
  dikumpulkan **disimpan** ke field tersebut. Menuliskan `type` pada input yang
  merujuk ditolak: itu dua tipe untuk satu nilai, dan hanya milik field yang
  ditegakkan.
- **Input ad-hoc** (tidak ada field-nya) wajib mendeklarasikan `type`, dan
  nilainya diteruskan sebagai `params` ke handler — bukan disimpan. `persist:
true` padanya ditolak karena tidak ada tujuan tulis.
- **`params.inputs` ditegakkan server, bukan hanya dirender.** Paruh render dan
  paruh validasi digabung menjadi satu daftar rule (`EffectiveParamValidation`),
  dipakai di ketiga jalur tulis: `PATCH`, `POST /{id}/{action}`, dan action
  Service. Deklarasi yang hanya tampil di UI tetapi diterima server adalah bug
  kontrak, bukan fitur.
- **`conditions` transisi dievaluasi di jalur `PATCH`.** Jalur itu dulu hanya
  mengevaluasi `guard`, sehingga transisi tanpa `impl` — yang hanya bisa lewat
  `PATCH` — tidak punya satu pun jalur yang menjalankan `conditions`-nya.
- Karena aturan di atas dibaca lewat **`ActionSources()`** (declared ∪ `via`) dan
  `EffectiveActionSpec`, kontrak boleh ditulis di transisi **atau** di entri
  `actions:` dengan `via` yang sama — keduanya berperilaku identik.

#### Nama permission sebuah action

Permission action diturunkan dengan aturan tunggal (`internal/api/generator.go`):

```
perm := action.required_permission
if perm == "" { perm = "{module}.{plural}.{action_name}" }
```

`required_permission` boleh ditulis singkat (`invoices.complete`) atau penuh
(`billing.invoices.complete`); `internal/permission.AutoPrefixPermission`
melengkapi prefix module. Untuk action yang hanya lahir dari transisi,
permissionnya adalah `{module}.{plural}.{via}` kecuali transisi menulis
`require_permission`.

#### Kapan `via` boleh diulang di `actions:`

Dua bentuk saja — keduanya karena entri itu **benar-benar mengikat**, bukan
sekadar mengulang:

```yaml
# (1) Nama reserved dengan route lifecycle generik.
#     `cancel` sudah punya route `/{id}/cancel` dengan permission
#     `{module}.{plural}.cancel`. Entri ini adalah satu-satunya cara
#     mempersempit/mengalihkannya; tanpa entri ini setiap pemegang `update`
#     bisa membatalkan dokumen.
actions:
  - { name: cancel, required_permission: billing.invoice.cancel }

# (2) Entri yang memberi `impl` (jadi dialah handler route) atau
#     `required_permission` eksplisit yang berbeda dari transisi.
```

Di luar dua bentuk itu, `spec.ValidateActionTransitionDuplication` menolak
manifest: entri semacam itu tidak menambahkan apa pun dan membuat satu nama punya
dua rumah.

#### Aturan yang berlaku untuk semua action

- **Satu nama, satu action.** Bila beberapa transisi memakai `via` yang sama
  (mis. `available→occupied` dan `reserved→occupied` sama-sama `via: occupy`),
  hasilnya **satu** action, bukan satu per transisi.
- **Nama action harus cocok dengan nama yang di-grant.** Grant menyebut **nama
  action**, dan nama itu harus sama persis dengan `via`/`actions[].name`.
  Grant untuk action yang tidak ada dibuang tanpa suara — grant editor
  menampilkan footprint nyata per entity, jadi pilih nama dari sana.

Untuk approval multi-level (gate pada transisi `state_machine`), lihat
[`02-core-extended.md`](02-core-extended.md) §2 — field `approval` pada transisi,
bukan kind terpisah.

### 1.7 `scope` / `row_scope` / `create_scope` / `assignments` — Isolasi Baris (Normatif)

Empat konstruk terpisah menjawab empat pertanyaan berbeda. Keempatnya **tidak
bisa digantikan** satu sama lain, dan mencampurnya adalah sumber kebocoran
multi-outlet:

| Konstruk       | Menjawab                         | Sifat                                     |
| -------------- | -------------------------------- | ----------------------------------------- |
| `scope`        | "Entity ini dipartisi oleh apa?" | Fakta model data — **tidak** memfilter    |
| `row_scope`    | "Apa yang ditegakkan saat BACA?" | Ditegakkan server-side, fail closed       |
| `create_scope` | "Apa yang ditegakkan saat BUAT?" | Baris belum ada → dijepit dari rujukannya |
| `assignments`  | "Dari mana nilai itu berasal?"   | Sumber nilai untuk `from: session`        |

```yaml
# Pada entity yang dipartisi (mis. order)
spec:
  scope: { dimension: branch, field: branch_id, required: true }
  row_scope:
    - { field: branch_id, op: eq, from: session }
  create_scope:
    - { field: branch_id, from: record, ref_field: table_session_id,
        via: cafe-order.table-session, via_field: branch_id }

# Pada entity yang mencatat penugasan (mis. employee)
spec:
  assignments:
    - { dimension: branch, field: branch_id, principal_field: username }
```

**`scope` — deklarasi, bukan filter.** `dimension` menamai dimensi partisi
(`^[a-z][a-z0-9_]*$`), `field` menunjuk field pembawa nilainya, dan
`required: true` menyatakan setiap baris wajib punya nilai dimensi —
`required: true` ditolak kalau field-nya sendiri tidak `required: true`, supaya
deklarasinya tidak menjanjikan lebih dari yang dipaksakan bentuk datanya.
`scope` juga memberi `natural_key_rule.scope_field` acuan generik (§2) dan
dipakai untuk menurunkan penyaringan otomatis di renderer/permukaan.

**`scope.enforced` — siapa yang menjaga pembaca tetap di dalam dimensinya.**
Deklarasi dimensi tanpa penegakan tidak bisa dibedakan dari dimensi yang memang
lintas cabang — dan itulah cara isolasi multi-outlet bocor diam-diam. Karena itu
`scope` menyatakan pemilik penegakannya:

| `enforced`          | Arti                                                               | Validator menuntut                                       |
| ------------------- | ------------------------------------------------------------------ | -------------------------------------------------------- |
| `session` (default) | atribut sesi membawanya (`row_scope` `from: session`)              | ada `row_scope` pada `field` dari `session`              |
| `route`             | parameter permintaan membawanya                                    | ada `row_scope` pada `field` dari `route`                |
| `none`              | dimensi **memang** lintas cabang di entity ini (mis. promo global) | tidak ada `row_scope` pada `field` (kontradiksi ditolak) |
| `external`          | ditegakkan di luar entity — grant publik atau `create_scope`       | pengecualian eksplisit                                   |

Nilai ini **deklarasi, bukan saklar runtime**: penegakan tetap apa pun yang
dikatakan `row_scope`/grant. Yang berubah hanya apakah manifest bisa menjawab
"siapa penegaknya" — pertanyaan audit yang sebelumnya hanya bisa dijawab dengan
membaca kode. `session`/`route` yang tidak menemukan `row_scope` yang cocok
**ditolak** `formspec validate`, dan menentukan field yang berbeda (atau literal)
juga ditolak: konstanta tidak bisa membawa dimensi milik pemanggil.

**`row_scope` — otorisasi, bukan kenyamanan UI.** Berbeda dari `fixed_filters`
pada Table/Kanban — yang di-merge di browser dan bisa dihilangkan klien mana
pun — `row_scope` dibaca server dan **tidak bisa dilebarkan lewat query string**:

| Sumber nilai | Cara menulis                                                         | Kalau tak terselesaikan |
| ------------ | -------------------------------------------------------------------- | ----------------------- |
| literal      | `value: "paid,in_kitchen"` (`from` kosong)                           | ditolak validasi        |
| `session`    | `from: session` — atribut identitas: `attr`, atau field dari `scope` | **403** (fail closed)   |
| `route`      | `from: route` — parameter query (`param`, default nama field)        | **403** (fail closed)   |

Nilai `from: session` **menimpa** filter klien pada field yang sama, dan predikat
literal **selalu diterapkan** — klien tidak bisa menambah, mengganti, atau
menghilangkannya. Aturan mutlaknya: nilai yang tidak bisa diselesaikan **tidak
boleh** berubah menjadi "tanpa filter" — permintaan gagal, bukan melebar.

**`value` (literal) vs `default`.** Keduanya string dan keduanya terlihat seperti
"nilai tetap", tetapi hanya satu yang merupakan batas:

- `default` hanya **pre-seed kontrol filter yang bisa diubah pengguna** di UI;
  klien boleh mengubah atau mengosongkannya, jadi ia bukan pembatas apa pun.
- `value` adalah **konstanta manifest** yang dipaksakan server. Untuk `op: in` /
  `nin` nilainya daftar berkoma (`"paid,in_kitchen,ready"`); untuk `op: between`
  tepat dua batas berkoma. Bentuk yang tidak memenuhi syarat operatornya
  **ditolak** (fail closed), bukan dibiarkan menjadi predikat yang tidak
  menyaring.

**`create_scope` — batas yang tidak bisa dinyatakan `row_scope`.** Setiap scope
lain membatasi **bacaan**, dan bacaan bisa dibatasi karena barisnya sudah ada.
Pada `create` belum ada baris untuk difilter — penyimpanan **tidak punya**
predikat di jalur tulis — sehingga tanpa konstruk ini pemanggil cukup mengirim
nilai dimensi pilihannya sendiri. Terukur pada kafe: tamu anonim dari QR bisa
mem-POST pesanan (dan sesi meja) dengan `branch_id` cabang lain, karena
`branch_id` tidak mendeklarasikan `required_permission` sehingga penjaga tulis
per-field pun tidak melihatnya.

```yaml
create_scope:
  # Cabang pesanan baru = cabang sesi meja yang dirujuknya.
  - {
      field: branch_id,
      from: record,
      ref_field: table_session_id,
      via: cafe-order.table-session,
      via_field: branch_id,
    }
```

Aturannya:

- **Kondisional pada rujukannya.** Diperiksa hanya bila payload membawa
  `ref_field`. Pesanan kasir (walk-in/takeaway) tidak punya sesi meja, jadi tidak
  ada record untuk menurunkan cabangnya — dan deklarasinya tidak boleh berubah
  menjadi "setiap create wajib punya sesi meja".
- **Ketidakcocokan DITOLAK, tidak ditimpa.** Menimpa diam-diam membuat permintaan
  dan baris tersimpan tidak sepakat, tanpa cara bagi pemanggil untuk
  mengetahuinya. Statusnya **403** — pemanggil minta menulis di luar dimensinya.
- **Rujukan yang tidak bisa diselesaikan adalah input buruk → 422**
  (`VALIDATION_ERROR`), bukan 403. Pemanggil boleh, payload-nya salah; ini kelas
  yang sama dengan relasi menggantung. Menggabungkan kedua kelas ini pernah
  menjadi regresi nyata (relasi rusak mulai dijawab 403).
- **Fail closed.** Rujukan yang tidak ada, atau record rujukan yang tidak membawa
  nilai dimensi, **menolak** permintaan — bukan berarti "tanpa aturan".
- `field` tidak boleh dideklarasikan dua kali: dua pemeriksaan bisa saling
  bertentangan, dan mana yang menang bergantung urutan iterasi.

**`assignments` — dari mana nilai atribut berasal.** Token boleh membawa atribut
langsung di klaim `attrs`. Kalau tidak, server menyelesaikannya dari entity yang
mendeklarasikan `assignments`: baris yang `principal_field`-nya sama dengan
username pemanggil menentukan nilainya. Inilah sebabnya `row_scope: {from:
session}` cukup ditulis **tanpa** `attr` pada entity yang punya `scope` — nama
atributnya = `scope.field`. Hasilnya di-memo sesaat (atribut yang berubah saat
sesi berjalan berlaku tanpa login ulang).

**Batas per PERAN: `row_scope` pada grant.** `row_scope` di atas bersifat
**per-entity**: ia menjawab "batas siapa", berlaku untuk setiap pemanggil, dan
karena itu tidak bisa menyatakan aturan yang berlaku untuk **satu peran saja**.
Menaruh filter `status` di sana — untuk aturan bisnis "hanya pesanan lunas yang
masuk dapur" — justru akan membutakan **kasir** terhadap draft yang sedang ia
susun, dan mengunci **dapur** keluar dari pekerjaannya sendiri.

Karena itu sebuah action di dalam grant boleh membawa `row_scope`-nya sendiri.
Ia action-scoped (bukan page-scoped), karena sebuah halaman hampir selalu
mencakup beberapa aksi dengan kebutuhan berbeda: `list`/`view`/`update`
mencocokkan baris, sedangkan `create` tidak.

```yaml
# roles.yaml — peran dapur hanya melihat pesanan yang sudah lunas, di cabangnya
- page: "order-page"
  actions:
    - name: list
      row_scope:
        - { field: status, op: in, value: "paid,in_kitchen,ready,served" }
        - { field: branch_id, op: eq, from: session }
```

Aturannya:

- **Digabung dengan AND**, tidak pernah menggantikan. Predikat grant di-AND
  dengan `row_scope` entity dan dengan filter klien. Dua batasan pada field yang
  sama karena itu **menyempitkan**; tidak ada yang saling menghapus.
- **Sebuah batasan tidak pernah bisa dilebarkan atau dihapus klien.** Nilainya
  dari manifest atau dari sesi; `from: session` gagal-closed (403) bila atributnya
  tak bisa diselesaikan — sama seperti `row_scope` entity.
- **Baris di luar batas dibaca sebagai TIDAK ADA (404), bukan 403.** Batasnya
  tidak boleh menjadi oracle keberadaan: `list` menyembunyikan baris itu, jadi
  `find`/`update`/`delete` menjawab sama. 403 disimpan untuk batas yang **tidak
  bisa diselesaikan** (salah konfigurasi) — itu keadaan yang harus berisik.
- **Grant yang tidak bisa diresolusi dilewati per-grant, dan dilaporkan.** Nama
  page/action yang salah ketik tidak boleh mematikan seluruh role (itu pernah
  terjadi — semua request jadi 404 tanpa error), dan tidak boleh pula menghapus
  batasan baris tanpa jejak: resolver menuliskan setiap grant yang gagal ke log.
- **Bentuk `grants` diperiksa pada dua titik, dengan SATU aturan.** `grants` adalah
  JSON bebas pada entity `role`, dan pembacaan bertipe bersifat **lossy**:
  `json.Unmarshal` membuang kunci yang tidak dikenalnya. Jadi `row_scopes:`
  (alih-alih `row_scope:`) tetap menghasilkan permission-nya sambil
  **menghilangkan batasnya** — fail-open, dan tak terlihat. Karena itu bentuk
  mentahnya dibaca `auth.ValidateGrantListShape`, dipakai oleh **gerbang deploy**
  (`formspec check` dan `formspec validate`) **dan** loader role saat runtime.
  Aturannya: `page` wajib; tiap grup punya `actions` atau `tabs`; tiap action
  punya `name`; dan untuk `row_scope` — field wajib, `op` di closed set, dan
  **tepat satu** sumber nilai, kecuali operator tanpa nilai (`null`/`notnull`/
  `root`). Field yang ada di _entity_ target serta resolusi page/action diperiksa
  Materializer, memakai validator `spec.ValidateRowScopeFilters` yang sama.

- **Batas baris yang tidak bisa diterapkan adalah PENOLAKAN, bukan catatan log.**
  Dua kelas cacat berperilaku berlawanan dan tidak boleh disamakan: grant yang
  page/action-nya tidak resolve hanya menghilangkan permission (sudah
  fail-closed dengan sendirinya), sedangkan grant yang batasnya tidak terbaca
  atau tidak bisa diterapkan **mempertahankan permission-nya dan menghilangkan
  batasnya**. `resource.find`/`resource.fetch` dari script memakai batas yang
  sama dengan jalur HTTP (permission `view`), sehingga script tidak bisa melihat
  baris yang HTTP sembunyikan dari pemanggil yang sama. Pemanggil **sistem**
  (subscription, worker, job terjadwal) tidak disaring — itu diputuskan lapisan
  dispatch, tidak pernah disimpulkan dari "tidak ada identitas", karena pemanggil
  anonim juga tanpa identitas.

  Sebelum aturan ini, satu-satunya penegak aturan #1 kafe ("hanya pesanan LUNAS
  yang masuk dapur") adalah **kolom Kanban**: `list`/`find` ke API tetap memuat
  pesanan `draft`, lengkap dengan `guest_token` milik tamu.

**Gerbang validasi (anti "hijau tapi tak pernah jalan").** `formspec validate`
menolak `row_scope` `from: session` tanpa `attr` yang atributnya tidak punya
sumber: bukan atribut identitas bawaan, bukan `scope.field`, dan tidak ada
`assignments` untuk dimensinya. Tanpa gerbang ini, bentuk tersebut selalu 403
untuk semua orang tanpa gejala apa pun di manifest — kelas kegagalan yang paling
mahal karena terlihat benar. Penulis yang tahu nilainya datang dari token di
luar spec tree menuliskan `attr:` eksplisit, dan tidak diusik.

**Pengecualian: `{module}.{plural}.read_all`.** Sebagian pemanggil **tidak punya
dimensi sama sekali** — pemilik workspace, auditor lintas cabang, atau
super-admin di dev. Untuk mereka, `row_scope` akan fail closed di **setiap**
pembacaan, sehingga aplikasi mati justru bagi orang yang memang seharusnya
melihat semuanya. Karena itu "boleh melihat semua baris" dinyatakan sebagai
**permission eksplisit**, bukan aturan implisit:

```yaml
# role pemilik — grant-nya terlihat di daftar permission, bisa diaudit
permissions:
  - cafe-order.orders.read_all
  - cafe-order.shifts.read_all
```

Aturannya:

- Nama mengikuti bentuk kanonik `{module}.{plural}.read_all` (§8.6), plural dari
  `spec.plural` entity (fallback `<name>s`).
- Pemegangnya **dilewati dari `row_scope` entity itu** — tidak ada filter yang
  dipasang, dan atribut yang tak bisa diselesaikan bukan lagi error (bagi
  pemanggil ini, "tidak punya cabang" adalah keadaan normal).
- Permission ini **tidak punya route sendiri**: ia kebijakan, bukan aksi. Ia
  tetap **didaftarkan** bersama permission standar supaya bisa diberikan dan
  terlihat di audit — "siapa yang boleh membaca lintas cabang" harus bisa
  dijawab dari daftar grant, bukan disimpulkan dari wildcard.
- `*` juga memenuhi pemeriksaan ini (super-wildcard), sehingga identitas dev
  tetap berfungsi; kasir yang hanya memegang `{module}.{plural}.list` **tetap
  ter-scope**.
- Ruang lingkupnya **per entity**: `read_all` pada `order` tidak memberi akses
  lintas cabang pada `shift`. Pemberiannya harus disengaja per entity.

## 2. Primary Key & Natural Key

Primary key: **UUID v7** (time-ordered) untuk semua Entity — ini kontrak,
bukan pilihan per backend. Natural key adalah **unique constraint per tenant**, bukan pernah jadi PK.
Menulis `natural_key: true` **sudah berarti unik**: key itu dipakai sebagai
lookup alternatif (`GetByID`) dan identifier route, jadi key yang duplikat tidak
bisa memenuhi perannya. Flag `unique` di-set validator, sehingga tidak perlu
ditulis ulang — dan `unique: false` eksplisit **ditolak** sebagai kontradiksi.

`natural_key_entry` menyatakan **siapa** yang memasok nilainya — pertanyaan yang
berbeda dari "apakah unik", dan yang menentukan UI:

| Mode                      | Arti                      | Perilaku engine                                   |
| ------------------------- | ------------------------- | ------------------------------------------------- |
| `auto_generated`          | Hanya engine              | Nilai dari caller **diabaikan**; key bukan input  |
| `user_entry`              | User/seed/hook            | Engine tidak pernah membuat nilai                 |
| `auto_generated_if_empty` | Engine, boleh di-override | Nilai caller dipakai; engine mengisi kalau kosong |

Mode tidak ditulis pun tetap sah dan diresolusi dengan konvensi (lihat
`ResolveNaturalKeyEntry`): tanpa rule → `user_entry`; `strategy: custom` →
`user_entry`; `strategy: sequence` → `auto_generated_if_empty`. Konvensi inilah
yang membuat deklarasi lama tetap bermakna — termasuk `order.number` dan
`purchase-order.number`, yang memang mengizinkan kasir memakai nomor kertas.

`required` adalah keputusan author, bukan implikasi engine: `account.code` diisi
seed, `journal-entry.number` dibuat engine, dan kode yang diisi user boleh kosong
sampai dibutuhkan. Pada key yang **opsional**, nilai kosong berarti "belum
diisi", sehingga index uniknya melewatkan baris kosong (dua record tanpa kode
bukan duplikat).

```yaml
- name: number
  type: string
  natural_key: true
  natural_key_entry: auto_generated_if_empty # opsional — konvensi sudah memberi ini
  immutable: true
  natural_key_rule:
    strategy: sequence # sequence | custom
    format: "{prefix}-{year}-{seq:06d}"
    prefix: { config: billing.invoice_prefix, default: "INV" }
    reset: yearly # never | yearly | monthly | daily
    scope_field: branch_id # opsional — sequence terpisah per nilai field ini
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
  category: operational # operational | financial | compliance | analytics | master | archive
  indexes:
    - { fields: [status], unique: false }
```

`indexes` boleh juga ditulis di tingkat Entity (bentuk yang dipakai contoh),
dan boleh menyebut **field relasi** — baik sebagai kolom yang diindeks maupun di
predikat. Index bisa **parsial** lewat `where:`, yang menutup keunikan
bersyarat seperti "satu shift terbuka per kasir":

```yaml
indexes:
  - fields: [branch_id, menu_item_id]
    unique: true
  - fields: [branch_id, cashier_id]
    unique: true
    where: "status = 'open'" # index parsial
```

Predikat memakai **nama field**, bukan SQL bebas — grammar tertutup
(`<field> <op> <literal>` dan `<field> IS [NOT] NULL`, digabung `AND`) supaya
salah ketik ditolak saat validate, bukan berakhir sebagai DDL. Index parsial
didukung SQLite maupun PostgreSQL, jadi aturannya portabel; DDL yang benar-benar
di luar bahasa tempatnya di `persist.raw_ddl` (§4.3).

**Keunikan adalah urusan database, bukan script.** Aturan "satu baris per kunci"
dinyatakan sebagai `unique: true` (dengan `where:` bila bersyarat) — **bukan**
sebagai guard script yang melakukan SELECT-lalu-INSERT. Alasannya: index berlaku
untuk **semua** jalur tulis (API, script, seed, operator) dan atomik di level
database, sedangkan guard hanya berjalan pada jalur yang melewatinya dan rentan
race (dua permintaan bersamaan sama-sama lolos SELECT). Guard script tetap boleh
ada sebagai **lapis kedua** untuk memberi pesan yang lebih ramah, tetapi ia
tidak boleh menjadi satu-satunya penegak. `resource.find()` (§9.3 platform)
adalah cara guard mengecek tanpa SQL mentah.

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
§2 untuk kontrak lengkapnya (aturan `renamed_from`, dll).

Migrasi **tidak pernah ditulis tangan** dan tidak punya manifest sendiri: tidak
ada `kind: Migration`. Sebuah perubahan skema dinyatakan dengan mengubah Entity —
menambah field, menandai field `removed`, menyatakan `persist.raw_ddl` — dan
engine menerapkannya otomatis saat spec dimuat (`formspec dev`, `formspec
migrate apply`). Yang ditulis tangan hanyalah **izinnya**, bukan SQL-nya.

### 4.1 Klasifikasi perubahan

Setiap perbedaan antara spec dan schema ter-apply dinilai sebelum dieksekusi:

| Tingkat     | Contoh                                                                                                                         | Perlakuan                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------- |
| **aditif**  | tabel, kolom turunan, atau index baru                                                                                          | otomatis, senyap                   |
| **derived** | kolom turunan dibangun ulang; rename via `renamed_from`; index dihapus (termasuk unique)                                       | otomatis, diumumkan                |
| **lossy**   | nilai field dihapus dari payload; type change pada kolom turunan yang nilainya gagal cast; unique index sementara duplikat ada | **ditolak** kecuali dideklarasikan |
| **never**   | tabel dihapus (Entity hilang dari manifest)                                                                                    | selalu ditolak                     |

Pembeda `derived` dan `lossy` bukan besar-kecilnya perubahan, melainkan **bisa
tidaknya nilai dipulihkan**. Kolom turunan dan index dihitung ulang dari payload
`data`, jadi menyatakan ulang spec memulihkannya. Field yang dihapus dan nilai
yang gagal di-cast tidak bisa dihitung ulang dari apa pun — hanya penyimpanan
yang menahannya, dan menghapusnya berarti hilang.

Alasan tidak ada lagi kind untuk migrasi custom: diff otomatis yang hanya bisa
**menambah** — dan diam untuk sisanya — adalah bentuk yang paling berbahaya.
Field yang dihapus tidak dimigrasikan, tidak dilaporkan, tetapi tetap merusak
penulisan record lama karena key-nya masih ada di `data`. Kondisi itu sekarang
**gagal keras** dengan jumlah barisnya, bukan lewat senyap.

### 4.2 Deklarasi perubahan destruktif

Deklarasi ditulis pada field yang bersangkutan, dan `reason` wajib — flag tanpa
alasan hanya mencatat apa yang terjadi, bukan apakah itu memang dikehendaki:

```yaml
fields:
  - name: old_branch_code
    type: string
    removed: true # tombstone: nilainya dibuang dari data
    reason: "digantikan branch_id"
  - name: amount_cents
    type: integer
    accept_data_loss: true # type change: baris yang gagal cast kehilangan nilainya
    reason: "dipindah dari text bebas ke integer pada 2026-09"
```

| Aturan                                                                 | Alasan                                                                            |
| ---------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| namanya `removed`, bukan `deleted`                                     | `deleted_at`/`soft_delete` sudah berarti soft delete di spec ini                  |
| tombstone tetap mendeklarasikan `name` + `type`                        | keduanya wajib di Field; tombstone mendeskripsikan apa yang sedang dibuang        |
| tombstone hidup **satu kali apply**; barisnya boleh dihapus sesudahnya | setelah diterapkan, field itu tidak ada lagi di baseline — tidak ada yang tersisa |
| baris field dihapus **tanpa** tombstone → error                        | membedakan penghapusan yang disengaja dari salah ketik                            |
| `removed` dan `renamed_from` saling eksklusif                          | rename menyimpan datanya, removal membuangnya                                     |
| tabel **tidak** punya tombstone                                        | blast radius-nya seluruh entity; jalur resminya `formspec backup` → drop manual   |

`accept_data_loss` hanya relevan untuk field yang punya kolom turunan (`index`,
`unique`, `natural_key`, atau disebut sebuah `indexes:`): nilai field biasa
disimpan apa adanya di dalam `data`, sehingga mengubah tipenya tidak menjatuhkan
nilai apa pun dan tidak memerlukan izin.

### 4.3 `persist.raw_ddl` — DDL di luar bahasa spec

Trigger, function, materialized view, atau index atas ekspresi JSONB tidak
terekspresi oleh field. Itu tempatnya `persist.raw_ddl`, **di dalam** Entity —
bukan manifest kind terpisah — supaya ia berjalan di jalur sync yang sama:

```yaml
kind: Entity
spec:
  persist:
    raw_ddl:
      - name: menu-price-unique
        reason: "satu harga per menu per cabang"
        ddl_by:
          sqlite: "CREATE UNIQUE INDEX idx_menu_price ON menu_prices (json_extract(data, '$.menu_item_id'))"
          postgres: "CREATE UNIQUE INDEX idx_menu_price ON menu_prices ((data->>'menu_item_id'))"
```

| Aturan                                                                  | Alasan                                                                       |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `ddl` (portabel) **atau** `ddl_by` (per driver) — bukan keduanya        | maksud yang ambigu lebih buruk daripada pilihan yang tegas                   |
| dialek himpunan tertutup (`sqlite`, `postgres`)                         | salah ketik tidak boleh berarti "varian itu dilewati"                        |
| driver tanpa varian dilewati **dengan peringatan**                      | menjalankan SQL driver lain adalah kegagalan yang ingin dihindari            |
| `reason` wajib                                                          | perubahan di lapisan storage harus bisa diaudit: _kenapa_, bukan hanya _apa_ |
| hanya DDL — data statement dan `DROP` ditolak saat validate             | data dan penghapusan storage bukan urusan manifest                           |
| **forward-only**: menghapus deklarasi tidak menjatuhkan apa yang dibuat | menebak invers dari DDL bebas lebih buruk daripada meninggalkannya           |
| checksum per pernyataan dicatat                                         | pernyataan yang tidak berubah tidak dijalankan ulang                         |

### 4.4 Perbaikan data & backfill — di luar spec, dijalankan sekali

Sebuah constraint hanya bisa ditambahkan setelah datanya memenuhi syarat, dan
duplikat muncul justru **karena** constraint-nya belum ada. Spec tidak
menyediakan tempat untuk DML: kalau masih ada duplikat, migrasi **ditolak dengan
hitungannya** ("3 grup duplikat"), operator merapikan datanya sekali lewat
`formspec repl --no-sync -f repair.star`, lalu apply dijalankan lagi. Backfill besar
berjalan lewat jalur yang sama.

`--no-sync` bukan kenyamanan: penolakan migrasi berlaku juga pada boot console
(`formspec repl` menyelaraskan schema seperti boot lain), sehingga tanpa flag itu
perintah yang ditunjuk pesan penolakan gagal dengan pesan penolakan yang sama.
Flag itu **hanya** membuka datastore tanpa menyentuh schema — ia tidak melewati
gerbang untuk boot normal.

Pemisahan ini disengaja: spec menyatakan **bentuk storage**, sedangkan perbaikan
data adalah tindakan operasional yang jejaknya ada di riwayat shell/ops, bukan di
manifest. Yang tidak boleh terjadi — dan karena itu ditolak — adalah apply
setengah jalan yang gagal setelah sebagian perubahan ter-commit.

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
      button_label: "Mulai Konsultasi" # label tombol (default: action name)
      icon: play # ikon lucide-react
      style: primary # primary | secondary | danger
      confirm: "Panggil pasien ini ke ruang konsultasi?" # pesan konfirmasi
```

| Field          | Tipe   | Default     | Keterangan                                                          |
| -------------- | ------ | ----------- | ------------------------------------------------------------------- |
| `button_label` | string | action name | Label tombol di UI                                                  |
| `icon`         | string | —           | Nama ikon lucide-react (kebab-case, mis. `play`, `x`, `check`)      |
| `style`        | string | `secondary` | `primary` (tombol solid), `secondary` (outline), `danger` (merah)   |
| `confirm`      | string | —           | Jika diisi, munculkan ConfirmDialog sebelum eksekusi; nilai = pesan |
| `show_when`    | string | —           | FormSpecExpr; tombol hanya ditampilkan jika expression `true`       |

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

**Nama state polos bukan konvensi (D4).** `order.paid` — nama event yang sama
dengan nama state — **tidak** punya arti khusus dan **tidak** diperlakukan
sebagai konvensi yang ditegakkan: tanpa prefix `before_*`/`on_*` ia tetap wajib
mendeklarasikan `type`. Dokumen vertical yang menulis `billing.order.paid`
mengikuti kebiasaan penamaan, bukan kontrak.

**Transisi memancarkan event lewat `emit:` (S13).** Event terikat pada
**aksi/hook** dan pada **transisi**. `TransitionDecl` punya
`from`/`to`/`via`/`guard`/`emit`: mengubah state lewat `via` mentransisikan
state machine-nya, dan bila transisi itu mendeklarasikan `emit`, event yang
disebut dipancarkan. Keterkaitannya **eksplisit** — bukan asosiasi "nama event =
nama state" — sehingga `formspec validate` bisa memastikan `emit` menunjuk event
yang benar-benar dideklarasikan, dan integrasi lintas-resource tidak perlu
menebak.

```yaml
state_machine:
  field: status
  transitions:
    - { from: awaiting_payment, to: paid, via: confirm-payment, emit: on_paid }
    - {
        from: [paid, in_kitchen, ready, served],
        to: cancelled,
        via: void-order,
        emit: on_cancel,
      }
```

Transisi tanpa `emit` tidak memancarkan apa pun (default). Konvensi penamaan
event (`on_*` = async, `before_*` = sync) tetap berlaku dan **terpisah** dari
keterkaitan ini — keduanya kini terverifikasi sendiri-sendiri.

**`emit` berlaku juga saat transisi dijalankan lewat approval.** Event
dipancarkan bersamaan dengan penulisan state — satu transaksi, satu
`store.Update` — jadi tidak ada jendela di mana state sudah berubah tetapi
konsumennya belum diberi tahu. Ini eksplisit karena sempat tidak begitu: jalur
approval `return` sebelum resolusi emisi di jalur `PATCH` dan tidak
meresolusinya sendiri, sehingga transisi ber-approval **tidak memancarkan apa
pun**. Terukur pada kafe `void-order` (`emit: on_cancel`): order menjadi
`cancelled` sementara mejanya tetap `occupied` — jembatan meja-lah yang
mendengarkan `on_cancel` — sehingga tamu berikutnya tidak bisa check-in.
Menjalankan transisi yang sama **tanpa** gate approval memancarkan event-nya, dan
itulah yang membuat asimetri ini tidak terlihat.

Konsekuensinya untuk penulis spec: jangan mengandalkan approval sebagai alasan
menunda efek samping. Kalau sebuah transisi menyatakan `emit`, efeknya terjadi
saat transisi itu **selesai** — langsung atau setelah quorum, bukan hanya pada
jalur langsung.

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

**Cek idempotency memakai `deliver[].idempotency_key`.** Cara memanggil target
yang dijanjikan §12.2 memuat `deliver: [{channel: reliable_event, target: {...},
idempotency_key: "balance.{id}"}]` — jadi kunci itulah yang diperiksa worker
sebelum panggilan sinkron:

- Template `{dotted.path}` di-resolve dari **payload event** (bukan dari record:
  yang diterima target adalah proyeksi yang dijanjikan `payload.fields`).
- Key yang sudah **completed** → consequence **dilewati** (retry = replay, dan
  bagi sebuah konsekuensi replay berarti "tidak melakukan apa-apa"). Tanpa ini
  retry adalah tulis-ganda — terukur pada kafe: me-requeue event `journal-posted`
  yang sama mengakumulasi pergerakan saldo dua kali (`143750 → 287500`).
- Key **pending/failed** → dijalankan: percobaan yang gagal harus bisa di-retry
  (delivery at-least-once).
- Template yang **tidak bisa di-resolve** (payload tak punya path itu) →
  **gagal**, bukan jadi literal: literal `balance.{id}` membuat semua event yang
  kekurangan `id` memakai kunci yang SAMA, sehingga delivery kedua dilewati
  sebagai "sudah selesai" — konsekuensi yang hilang, lebih buruk daripada
  duplikat.
- Entry **tanpa** `idempotency_key` tetap seperti sebelumnya: idempotensi alami
  adalah kontrak target (`idempotent: true` diwajibkan untuk panggilan lintas
  boundary, §5 / 02-core-extended §5).

Kuncinya di-namespace `deliver:{resource}.{action}` di tabel yang sama dengan
kunci idempotency HTTP, sehingga satu delivery tidak bisa bertabrakan dengan
idempotency sebuah action. **Batas jujurnya:** kunci diklaim (`pending`)
**sebelum** target dipanggil dan ditandai `completed` sesudahnya, jadi crash
tepat di antara tulisan target dan penandaan menyisakan key `pending` → retry
berikutnya mengklaim ulang dan menjalankannya sekali lagi. Jendela itu hanya
bisa ditutup oleh idempotensi alami target (mis. kunci unik
`(source_id, account_id, period)`), bukan oleh store ini — yang ditutup store
ini adalah kasus terukur: retry **setelah** sukses.

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

| Aspek             | Ketentuan                                                                                                                                       |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Auth              | Session cookie / OAuth (user agent). **Tidak menerima** API key.                                                                                |
| Gating            | Permission `list`/`view`/`create`/`update`/`delete` + `required_permission` action-level ([§5](#5-action)) — **tidak** digerbangi `spec.expose` |
| Field visibility  | Semua field kecuali yang di-strip oleh `required_permission` field-level ([`05-field-types.md`](05-field-types.md) §5.3)                        |
| Rate limit        | Per user session                                                                                                                                |
| Audit attribution | `user_id`, `session_id`                                                                                                                         |
| Ketersediaan      | Selalu ada untuk setiap Entity yang user punya permission-nya                                                                                   |

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

| Aspek             | Ketentuan                                                                                                       |
| ----------------- | --------------------------------------------------------------------------------------------------------------- |
| Auth              | API key header (`X-FormSpec-Key`). **Tidak menerima** session cookie.                                           |
| Gating            | `spec.expose` (deny-by-default) + `required_permission` action-level ([§5](#5-action))                          |
| Field visibility  | Field dengan `exclude: [public_api]` disembunyikan dari respons ([`05-field-types.md`](05-field-types.md) §5.3) |
| Rate limit        | Per API key + plan tier                                                                                         |
| Audit attribution | `api_key_id`, `service_label`                                                                                   |
| Ketersediaan      | Hanya untuk Entity dengan `spec.expose` — tanpa expose, endpoint 404                                            |

`formspec generate` hanya menghasilkan typed client untuk permukaan external ini.
Kalau tidak ada entity yang exposed, `formspec generate` menolak berjalan — tidak
ada yang bisa digenerate.

### 8.3 Router & Middleware

Router **wajib** memakai radix-tree (atau setara) dengan lookup O(jumlah segmen
path) — jumlah route tidak boleh mendegradasi performa secara linear. Dua
permukaan didaftarkan sebagai **dua route group** dengan middleware auth yang
berbeda di level router:

| Route group | Middleware auth                   |
| ----------- | --------------------------------- |
| `/_ui/`     | Session cookie / OAuth            |
| `/api/v1/`  | API key header (`X-FormSpec-Key`) |

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

| Kontrak                                          | Referensi                                                                                                                                                                                                                     |
| ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Workspace prefix (`/{workspace_slug}`)           | §8.2 — router wajib jatuh ke UUID kalau slug tidak diset                                                                                                                                                                      |
| Response envelope (`data`, `meta`, `error`)      | `list: { data, meta: {page, per_page, total, total_pages}, links }`, `single: { data, meta: {request_id, timestamp} }`, `error: { error: {code, message, details}, meta }`                                                    |
| Kode error standar                               | `VALIDATION_ERROR` (422), `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409), `STATE_TRANSITION_ERROR` (422), `INTERNAL_ERROR` (500) — daftar lengkap: [`error-glossary.yaml`](error-glossary.yaml) |
| Query & filter ([§6](#6-query--filter-operator)) | `?page&per_page&sort&direction&fields&filter[...]&search&include`                                                                                                                                                             |
| Optimistic concurrency (`version`)               | Update wajib membawa `version`; mismatch → `409 CONFLICT`                                                                                                                                                                     |

### 8.6 Permission — `{module}.{plural}.{action}` (Normatif)

Permission adalah **resource + action**, tidak pernah nama role. Bentuk kanonik
satu-satunya adalah:

```
{module}.{plural}.{action}     # mis. cafe-master.menu-item-prices.update
```

`{plural}` adalah `spec.plural` Entity (§1.5), fallback `<name>s`. Bentuk
**singular** (`{module}.{entity}.{action}`) **bukan** varian yang setara dan
tidak pernah cocok — registry mendaftarkan plural, jadi permission berbentuk
singular hanya menghasilkan 403 yang sulit dilacak. Tooling yang menyusun nama
permission wajib menurunkan plural dari metadata.

**Setiap action punya permission sendiri (D6).** `submit` memakai
`{module}.{plural}.submit`, bukan `update`; berlaku juga untuk `cancel`,
`amend`, `delete`, dan setiap custom action. Jadi "siapa yang boleh
menyelesaikan record" adalah pertanyaan terpisah dari "siapa yang boleh
mengubahnya" — dan permission `update` **tidak** memberi hak `submit`.

**`read_all` — permission kebijakan, bukan aksi.** `{module}.{plural}.read_all`
mengecualikan pemegangnya dari `row_scope` entity itu (§1.7). Ia tidak punya
route sendiri karena tidak ada operasi yang dijalankannya; ia menjawab "siapa
boleh membaca lintas baris/cabang". Nama dan granularitasnya mengikuti bentuk
kanonik di atas supaya bisa digabungkan dengan grant lain tanpa aturan khusus —
dan supaya pertanyaan audit "siapa yang bisa melihat semua cabang" dijawab oleh
daftar grant, bukan oleh penafsiran wildcard.

### 8.7 Konteks sesi — (principal, role, cabang) (Normatif)

Sesi selalu **spesifik**: siapa, sebagai **role apa**, di **cabang mana**. Tidak
ada sesi yang memegang "semua role yang dimiliki principal" sekaligus.

**`assignments` hidup di principal.** `formspec.core.user.spec.assignments`
adalah daftar `{role, dimension, value}` — ketiganya wajib. `dimension` adalah
**nama field** yang dibandingkan `row_scope.field` (mis. `branch_id`), bukan
nama dimensi dekoratif `scope.dimension` (`branch`), karena
`row_scope: {from: session}` membaca nilai itu dengan kunci tersebut.

**Login memilih satu.** Aturan pemilihan, dan tidak ada varian lain:

| Keadaan                                     | Hasil                                                                                                                                                |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0 assignment                                | sesi **tanpa boundary** — perilaku principal tanpa konteks (pemilik/service account): permission = union role, bacaan lintas cabang lewat `read_all` |
| tepat 1                                     | dipakai otomatis                                                                                                                                     |
| >1 tanpa `assignment`                       | **409 `CONTEXT_REQUIRED`** + `choices[{id, role, dimension, value}]`; **tidak ada token**                                                            |
| `assignment` yang tidak ada / sudah dicabut | **409 `CONTEXT_REQUIRED`** juga — fail closed, minta pilih ulang                                                                                     |

Server **tidak pernah** memilih boundary atas nama pemanggil, dan tidak pernah
menurunkan "tidak tahu" menjadi "tanpa boundary".

**Token.** Sesi ber-konteks membawa klaim `role` (tunggal) + `attrs` (dimension →
value). Bila `role` ada, klaim `roles` **diabaikan** — daftar role yang basi atau
ditempel tidak bisa melebarkan sesi.

**Permission.** Materialisasi memakai grant **role yang dipilih saja** (+
permission langsung principal). Union dari seluruh role tidak pernah dipakai
untuk sesi ber-konteks; itulah yang membuat audit bisa menjawab "sebagai role
apa, di cabang mana" untuk setiap aksi.

**Refresh & pencabutan.** Sesi ber-konteks divalidasi ulang saat refresh:
assignment-nya harus masih ada **dan** role-nya masih terdaftar. Bila tidak,
refresh dijawab **409 `CONTEXT_REQUIRED`** — bukan token baru dengan boundary
lama, dan bukan 401 yang membuat klien mengulang selamanya.

**Pindah konteks.** `POST /_ui/auth/switch` (`refresh_token` + `assignment`)
menerbitkan pair baru untuk konteks yang diminta **dan me-revoke sesi lama**:
tidak pernah ada dua konteks hidup dari satu pemilihan. `assignment` wajib di
endpoint ini — pindah tanpa menyebut boundary bukan operasi yang sah.

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

## 10. Read-Through Cache (Find-by-ID)

Entity dapat opt-in ke cache framework untuk endpoint find-by-id:

```yaml
spec:
  cache:
    ttl: 300s # wajib; 1s–1h; absent = off (correctness by default)
```

Semantik (normatif):

- **Lingkup**: hanya `GET /{id}` (find). List/query **tidak pernah** di-cache.
- **Isi cache**: record **mentah** (pre-sanitize) — field security
  (`exclude: [ui]`) tetap dievaluasi per-request; hasil sanitize tidak pernah
  di-cache.
- **Key**: `{workspace}:{module}:{entity}:id:{id}` — tenant di dalam key;
  semantik cross-tenant 404 tetap terjaga.
- **Invalidasi**: create/update/delete/submit/transisi state menghapus key
  (in-process). Multi-instance: saat backend cache adalah Redis/Valkey,
  invalidasi di-broadcast via pub/sub sehingga semua instance konsisten.
- **Write path**: selalu ke DB — CAS version check tidak pernah melewati
  cache; cache tidak pernah sumber kebenaran untuk write.
- **Backend**: datastore module-bound yang `serves: [cache]` (mis. driver
  `valkey`), atau shared in-memory saat module tidak bound.

Implementasi: `internal/api/entitycache.go`; keputusan desain:
`docs_internal/plan/fase14-entity-cache.md`.

## 11. Config & Global Settings

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
    smtp_host: { type: string, secret: true }
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

**`settings` dibaca framework, bukan hanya renderer (D7).** `settings` hidup di
`kind: Config` level **App** — satu tempat, bukan per-komponen. Script membacanya
lewat `ctx.config.get("settings.…")`; framework membacanya untuk keputusan yang
tidak boleh ditebak. Yang paling terlihat: field `money` menyimpan
`{amount, currency}`, dan bila `currency` tidak dinyatakan di field, nilainya
diambil dari `settings.default_currency`. Bila tetap tidak bisa ditentukan,
runtime **menolak** penulisan itu — data uang tanpa mata uang bukan data
([`05-field-types.md`](05-field-types.md) §2).

Contoh konkret namespace `settings.*` di workspace Config:

```yaml
apiVersion: formspec.dev/v1
kind: Config
metadata:
  name: workspace
  module: formspec.core
spec:
  keys:
    settings.default_currency: { type: string, default: "USD" }
    settings.locale: { type: string, default: "en-US" }
    settings.timezone: { type: string, default: "UTC" }
    settings.date_format: { type: string, default: "YYYY-MM-DD" }
    settings.fiscal_year_start: { type: string, default: "01-01" }
```

Module membaca lewat `ctx.config.get("settings.default_currency")` —
namespace `settings.*` memastikan tidak bentrok dengan key config milik
Module sendiri.

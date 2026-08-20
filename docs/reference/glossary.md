# Glossary

**Status:** Draft

> Definisi di sini kanonik dan mengikat pemakaian di seluruh dokumentasi.

- **Workspace** — Unit kepemilikan tertinggi dalam model `Workspace → App →
Module → Resource`: dimiliki satu Data Owner (Workspace Owner), menampung
  banyak App dan Module yang berjalan atas identitas tenant yang sama. Data
  bisnis selalu menjadi milik Workspace tempat resource berjalan, bukan milik
  App/Module yang menghasilkannya.
- **App** — Root project manifest (`kind: App`) — unit deployment, trust
  boundary, dan publikasi interface. Bersifat kurasi (bukan pemilik objek):
  memilih subset entity/action dari Module yang di-`depends_on`/mount, punya
  App renderer dan menu sendiri; satu Workspace boleh berisi lebih dari satu
  App yang berjalan bersamaan, dibedakan `root_url`.
- **Module** — Package manifest (identitas, versi, dependency) yang benar-benar
  memiliki objek domain (Document, Service, seluruh instance VisualSpecKind
  seperti Page/Form/Table) — satu Module = satu bounded context bisnis utuh.
  Isi ditemukan lewat scanning file (bukan didaftar manual); Module yang sama
  boleh di-mount lebih dari satu App dalam Workspace yang sama.
- **Kind** — Satuan taksonomi manifest FormSpec: seluruh kind memakai format
  `apiVersion/kind/metadata/spec` yang sama, terbagi menurut concern (model
  domain, kurasi, konfigurasi, DDL, proses bisnis, visual, governance,
  infrastruktur) dan menurut plane tempat ia hidup (Control atau Resource).
- **Meta-kind** — Kind yang mendeklarasikan kind lain, membuat sistem kind
  extensible dalam tiga layer: built-in spec, kind yang didaftarkan module
  resmi lewat `KindDefinition`, dan kind namespaced dari module pihak ketiga
  yang tunduk Verified Badge. `VisualSpecKind`, `Renderer`, dan
  `PersistBackend` adalah meta-kind.

- **Document (Entity)** — Resource persisted (`type: document`, disebut juga
  Entity) yang menjadi sumber kebenaran data bisnis, dengan tepat satu
  `characteristic` (`master`, `transaction`, `reference`, atau `summary`) dan
  lifecycle bawaan lewat `doc_status` — berbeda dari `type: service` yang
  stateless dan tidak punya `characteristic`/lifecycle.
- **Action** — Unit eksekusi behavior atas Document/Service, diimplementasikan
  lewat salah satu dari lima jenis `impl` (`native`, `script`, `script_ref`,
  `compiled`, `sidecar`). Setiap Action mendeklarasikan `required_permission`
  (siapa boleh memanggil) dan `uses` (akses kode itu sendiri) secara eksplisit
  — grant tidak pernah diturunkan dari pemakaian aktual kode. Delapan reserved
  action (`create`, `update`, `submit`, `cancel`, `delete`, `amend`,
  `create-submit`, `amend-submit`) menegakkan lifecycle Document.
- **Lifecycle** — Model status Document, dua lapis dan independen: `doc_status`
  (bawaan, framework-enforced, closed set `draft | submitted | cancelled`,
  ditegakkan delapan reserved action) dan state machine bisnis (field terpisah
  yang didefinisikan developer, transisi lewat action bernama, approval
  berbasis role opsional lewat `kind: Workflow`).
- **Extension** — Document (`extend_storage`) yang ditulis module lain untuk
  menambah field/perilaku ke Document milik module lain tanpa fork dan tanpa
  merusak jalur upgrade-nya, wajib bisa di-uninstall bersih tanpa sisa. Field
  extension diakses lewat pemanggilan bernamespace
  (`invoice.ext("kastem1").project_code`), bukan asumsi nama kolom fisik.

- **Shell** — Tingkat tertinggi hirarki visual: stack teknologi plus kontrak
  bootstrap penuh (routing awal, derivasi otomatis Layer 0) yang menghosting
  App/Page/Component renderer — satu App selalu memakai satu Shell. Bukan
  `VisualSpecKind` dan tidak punya nilai `tier`; dipilih lewat `stack_family`
  renderer yang dipasang.
- **App renderer** — Tingkat kedua hirarki visual: menentukan bentuk
  chrome/navigasi subtree sebuah App — chrome penuh (menu persisten, header)
  versus minimal (tanpa nav standar). Contoh:
  `sidebar-nav`, `topnav`, `no-nav` — dipilih lewat field `app_renderer`
  di manifest App. Auth adalah sumbu terpisah (`access`: `private`/`public`).
- **Page renderer** — Tingkat ketiga hirarki visual: mengisi konten utama
  sebuah route di dalam App renderer, mis. `data-entry`, `wizard`, `kanban`,
  `table-list`, `report`, `listing`.
- **Component renderer** — Tingkat terkecil hirarki visual: elemen granular
  dan reusable yang bisa dipakai di Page manapun dalam Shell yang sama, mis.
  `textinput`, `dateinput`, `widget`.

- **VisualSpecKind** — Meta-kind untuk mendeklarasikan jenis view baru (mis.
  Kanban) tanpa mengubah framework inti: mendefinisikan skema instance
  shell-agnostic (satu definisi dipakai semua Shell) plus `renderer_contract`
  yang wajib dipenuhi Renderer manapun yang mengimplementasikannya. Wajib
  menyatakan `tier` (`app | page | component`).
- **Renderer** — Meta-kind implementasi konkret sebuah VisualSpecKind untuk
  kombinasi `(implements, stack_family)` tertentu — satu VisualSpecKind boleh
  punya banyak Renderer dengan filosofi UX/stack berbeda. Interpreter runtime
  yang mengonsumsi Spec Resolution API, bukan build step per kombinasi
  spec+renderer.
- **tier** — Field wajib pada `VisualSpecKind` bernilai `app | page |
component`, menentukan di mana sebuah kind boleh dipakai/dikomposisi — dasar
  validasi slot compatibility (`accepts_slots` hanya sah dari tier
  `page`/`app`; `implements_slot` hanya sah dari tier `component`).
- **slot** — Perluasan `VisualSpecKind` untuk pola Page yang menerima Component
  tertentu di posisi tertentu (mis. Dashboard menerima Widget) —
  dideklarasikan sebagai lubang dengan kontrak data-shape (`accepts_slots`/
  `implements_slot`), bukan referensi ke komponen bernama spesifik; rekursi
  dibatasi satu level.
- **stack_family** — Field wajib pada `Renderer` yang menyatakan kecocokan
  shell (mis. `react-shadcn`, `vue`, `flutter`) — App shell, Page
  shell-integrated, dan Component wajib berbagi `stack_family` yang sama
  supaya tetap satu render tree; mismatch adalah compile-time error saat
  `formspec apply`.
- **trust tier** — Klasifikasi kepercayaan (`official | verified | community`)
  yang seragam untuk Renderer, PersistBackend, dan Module Registry — tier
  `official` dipakai sebagai default resolusi, tier lain butuh proses
  verifikasi (Verified Badge) sebelum dipakai di environment yang governed.

- **Spec Resolution API** — Seam runtime antara engine dan Shell manapun:
  kontrak internal yang dipanggil interpreter Shell untuk mendapat
  representasi siap-render dari App/Page/Entity (lewat endpoint `_meta/ui`,
  `_meta/apps`, `_meta/me`, `_meta/entities`), sudah permission-filtered dan
  backend-agnostic. Rendering adalah interpretasi runtime, bukan code
  generation.
- **Layer 0/1** — Dua tingkat manifest frontend untuk App developer: Layer 0
  adalah derivasi otomatis penuh tanpa manifest UI sama sekali (Table list,
  Form create/edit, Page detail, entry menu digenerate otomatis dari
  Document); Layer 1 adalah manifest minim yang meng-override sebagian
  default itu — keduanya tanpa perlu dev environment lokal.
- **Tier 2/3 developer** — Persona yang menulis handler native/script kustom,
  frontend custom (`asset`), atau mengonsumsi codegen (`formspec generate`) —
  berbeda dari App developer Layer 0/1 yang cukup mengandalkan derivasi
  otomatis dari Document.

- **PersistBackend** — Meta-kind implementasi penyimpanan — seam setara Shell
  di sisi visual: satu implementasi resmi (jsonb-persist) dipakai default,
  tapi seluruh framework wajib bicara ke interface ini (structural diff
  apply, query resolution, `ctx.next_key`, index generation, uninstall
  extension bersih) tanpa shortcut langsung ke backend fisik.
- **Datastore** — `kind: Datastore`, resource Control Plane yang
  mendefinisikan backend infrastruktur bernama (Postgres, SQLite, Valkey,
  dst) beserta primitive `ctx.*` yang dilayaninya (`serves`), kredensial
  lewat `credential_ref`, dan access control (`filter`/`permission`).
  Resource Plane hanya menerima Datastore yang sudah diotorisasi lewat
  snapshot Plane Protocol, tidak bisa membuat/mengubah definisi backend
  sendiri.
- **structural diff** — Kontrak migrasi skema: framework menghasilkan diff
  dari perbandingan manifest Document versi lama vs baru (field
  ditambah/dihapus/`renamed_from`, index berubah), lalu tiap PersistBackend
  menerjemahkan diff itu ke storage-nya sendiri — bukan framework yang
  generate SQL. Salah satu dari tiga jenis migrasi, berbeda dari custom DDL
  (`kind: Migration`) dan data migration (backfill manual).
- **natural key** — Unique constraint per tenant pada Document (mis. nomor
  invoice) yang tidak pernah menjadi primary key — primary key selalu UUID
  v7. Generasinya (gap-free, atomik, duplicate-free) diatur lewat
  `natural_key_rule` (strategy, format, reset, `scope_field`) dan dipenuhi
  tiap PersistBackend lewat `ctx.next_key`.

- **Plane (control/resource)** — Dua proses/binary terpisah dalam arsitektur
  FormSpec: Control Plane (`formspec-control`) menguasai governance (Environment,
  Policy, kunci, kontrak, transparency log) tanpa pernah membaca data bisnis
  atau mengeksekusi handler bisnis; Resource Plane menjalankan Engine (CRUD,
  Action, State Machine, Event/Outbox), Spec Resolution API, dan
  PersistBackend, menarik desired-state dari Control Plane lewat pull policy
  tanpa write-back.
- **plane protocol** — Kontrak yang membuat implementasi Control Plane dan
  Resource Plane yang independen bisa saling beroperasi lewat dua channel
  asimetris: desired-state (Control→Resource, pull-only, snapshot bertanda
  tangan) dan evidence (Resource→Control, append-only, write-once) — Resource
  Plane tidak pernah bisa mengubah governance state, hanya menambah evidence.

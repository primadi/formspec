# Forma Technical Note: Hirarki Visual — Shell, App, Page, Component sebagai Kontrak vs Renderer

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec**
**Status: bahan eksplorasi arsitektur, belum committed ke spesifikasi resmi. Melanjutkan pembahasan Form kind (Layer 0/1/2/3) dari Core Extended Spec.**

---

## 0. Latar Belakang

Diskusi ini dimulai dari pertanyaan: apakah Forma bisa fleksibel menghasilkan tampilan di luar pola admin panel — misalnya pemesanan tiket bioskop atau pendaftaran pasien publik — tanpa mengunci page layout dan app layout ke satu bentuk tertentu.

Dari situ berkembang jadi kesadaran yang lebih besar: **kind untuk keperluan visual tidak flat**. Selama ini "Form" diperlakukan sebagai satu kind datar. Padahal secara alami ada hirarki (Shell → App → Page → Component), masing-masing punya banyak *jenis*, jenis baru bisa ditambahkan kapan saja, dan setiap jenis butuh *renderer* (implementasi konkret) yang bisa diganti atau ditambah tanpa mengubah kontraknya.

Prinsip inti yang disepakati sepanjang diskusi: **spec adalah kontrak, renderer adalah implementasi kontrak itu** — pola yang sama dengan `impl.native`/`impl.script`/`impl.script_ref` di action resource, hanya sekarang diterapkan ke lapisan visual dan, seperti terungkap di penutup diskusi, berpotensi berlaku juga ke lapisan penyimpanan (§8).

**Prinsip arsitektural yang mengikat seluruh dokumen ini:** kalau sebuah implementasi (Shell kedua, Persist Backend kedua) suatu saat mau bisa diganti, *seam*-nya harus dirancang sejak implementasi pertama dibangun — bukan ditambal belakangan. Retrofit seam setelah kode inti terlanjur mengasumsikan satu implementasi tertentu berarti membongkar bagian yang sudah berjalan, bukan sekadar menambah modul baru.

---

## 1. Empat Tingkat Hirarki Visual

| Tingkat | Definisi | Karakteristik |
|---|---|---|
| **Shell** | Stack teknologi + kontrak bootstrap penuh (routing awal, Layer 0 auto-generate dari kind system) | Satu App = satu Shell. Contoh: shadcn/React (resmi, sekarang). Dart/Flutter (kemungkinan roadmap, dibahas §6) |
| **App renderer** | Menentukan *asumsi bootstrap* — apakah subtree dibungkus Auth/Menu/Navigation sejak awal, atau tidak | Contoh: `sidebar-nav`, `topnav` (asumsi menu+header+logo+Auth-wrap ada), `landing-page` (publik, tanpa Auth-wrap, tanpa Menu persisten) |
| **Page renderer** | Isi konten utama sebuah route | Contoh: `data-entry` (≈ Form kind yang sudah ada), `wizard`, `kanban`, `timeseries`, `table-list`, `report`, `print`, `listing` (e-commerce/movie search) |
| **Component renderer** | Elemen granular, reusable di dalam Page renderer manapun (dalam Shell yang sama) | Contoh: `textinput`, `dateinput`, `widget` (untuk mengisi slot — lihat §4) |

**Koreksi penting selama diskusi:** App renderer awalnya disangka soal "punya nav atau tidak" (sidebar/topnav = punya nav, landing-page = beda kategori). Ini salah tingkat. App renderer yang benar adalah soal **siapa yang menguasai bootstrap** — sidebar-nav dan landing-page sama-sama App renderer, cuma beda asumsi bootstrap-nya (chrome penuh vs minimal). Konsekuensinya, konsep `route_mode: standalone` di level Page yang sempat diusulkan **dibatalkan** — keputusan "dibungkus shell atau tidak" sudah selesai di App renderer, tidak perlu flag tambahan di Page.

---

## 2. VisualSpecKind — Meta-Kind untuk Skema Kontrak

Supaya jenis baru bisa ditambahkan tanpa mengubah framework inti, dibutuhkan meta-kind yang mendeklarasikan *jenis* view baru dan skemanya — pola yang sudah ada presedennya di `MockupModule` kind.

```yaml
apiVersion: forma/v1
kind: VisualSpecKind
metadata:
  name: kanban
spec:
  tier: page                # WAJIB — app | page | component. Menentukan di mana kind ini boleh dipakai/dikomposisi
  schema: {...}              # field wajib di instance spec Kanban (columns, card_fields, dst.)
  renderer_contract: {...}   # interface yang WAJIB dipenuhi renderer manapun
```

**Field `tier` yang sebelumnya hilang dari contoh — ini gap yang perlu ditutup, bukan detail kosmetik.** Tanpa `tier` eksplisit, framework tidak tahu:

- Apakah `kind: Kanban` boleh dipasang langsung sebagai isi route App (perlu `tier: page`), atau cuma boleh mengisi slot milik Page lain (`tier: component`).
- Bagaimana memvalidasi slot compatibility di §4 — `accepts_slots` cuma masuk akal dideklarasikan oleh `VisualSpecKind` bertier `page` (atau `app`, kalau nanti App-level slot dibutuhkan), dan `implements_slot` cuma valid dari `tier: component`. Tanpa field ini, `forma apply` tidak punya cara memverifikasi "widget beneran component, bukan page nyasar."
- `Shell` sengaja **tidak** termasuk nilai `tier` di sini — Shell bukan sesuatu yang dideklarasikan lewat `VisualSpecKind` sama sekali, ia adalah wadah yang menghosting App/Page/Component renderer via `Renderer.stack_family` (§3). `tier` hanya berlaku untuk tiga tingkat di dalam Shell.

Instance spec yang ditulis developer aplikasi tetap seperti Form kind sekarang — cuma `kind: Kanban` alih-alih `kind: Form`. **Skema ini shell-agnostic** — satu definisi Kanban dipakai baik untuk Shell shadcn maupun (nantinya) Flutter, tanpa ditulis ulang. Ini argumen "write once" yang layak masuk dokumentasi produk: developer/AI menulis kontrak sekali, dapat web app dan mobile app dari spec yang sama.

---

## 3. Renderer — Implementasi Konkret, Bisa Banyak per VisualSpecKind

```yaml
apiVersion: forma/v1
kind: Renderer
metadata:
  name: kanban-vue-community
spec:
  implements: kanban
  stack_family: vue          # bukan shadcn-react (official)
  trust_tier: community
```

Siapa pun bisa: (a) bikin renderer baru untuk VisualSpecKind yang sudah ada (Kanban versi Vue dengan filosofi UX berbeda), atau (b) mendefinisikan VisualSpecKind sama sekali baru (mis. `seat-map-booking`) plus renderer resminya. Trust tier (official/verified/community) yang sudah ada di Module Registry berlaku sama di sini.

### 3.1 Batas Kompatibilitas Antar-Stack (`stack_family`)

Awalnya dipertimbangkan apakah satu App boleh mencampur Page React dengan Page Vue dalam App shell yang sama. **Diputuskan: tidak — terlalu jauh.** Alasannya bukan teknis semata, tapi biaya investasi jangka panjang: mendukung kombinasi stack di dalam satu shell berarti setiap tooling first-party (Studio, Agent Skill, Layer 0 generator) harus mempertimbangkan matriks kompatibilitas selamanya — bertentangan dengan prinsip "tetap sederhana."

**Aturan final:**

| Konteks | Stack | Governance |
|---|---|---|
| App shell + Page (shell-integrated) + Component | Satu stack resmi (default: React/shadcn), titik. Ekstensi hanya lewat renderer/component registry di dalam stack yang sama | Diatur Forma — karena berbagi render tree |
| Page yang benar-benar lepas dari App manapun (mis. dikonsumsi via API generik) | Bebas stack apa saja | Bukan urusan Forma — cukup konsumsi `forma/gen-openapi`/`forma/gen-typescript` yang sudah ada, tidak perlu Renderer kind sama sekali |

Validasi saat `forma apply`:
```
Jika Page dipasang di dalam App (shell-integrated):
  renderer.stack_family HARUS sama dengan App.stack_family
  → mismatch = compile-time error

Jika Page dikonsumsi independen dari App manapun:
  tidak ada compatibility check — karena tidak ada shared render tree
```

---

## 4. Slot System — Relasi Antar-Renderer (Dashboard + Widget)

Pola "Page tertentu berharap Component tertentu bisa mengisi posisi di dalamnya" (dashboard menerima widget) memerlukan perluasan `VisualSpecKind`: bukan cuma daftar jenis view, tapi juga tempat mendeklarasikan **slot** — lubang dengan kontrak data, bukan referensi ke komponen bernama spesifik.

```yaml
apiVersion: forma/v1
kind: VisualSpecKind
metadata:
  name: dashboard
spec:
  tier: page
  schema: {...}
  accepts_slots:
    - name: widget
      contract:
        required_props: [title, data_binding, size_unit]
        optional_props: [refresh_interval_sec]
```

```yaml
apiVersion: forma/v1
kind: VisualSpecKind
metadata:
  name: kpi-widget
spec:
  tier: component            # WAJIB component — forma apply menolak implements_slot dari tier lain
  schema: {...}
  implements_slot: widget
```

Instance spec aplikasi tinggal reference widget mana yang dipasang di posisi mana:

```yaml
kind: Dashboard
spec:
  layout:
    - slot: widget
      use: kpi-widget
      position: { row: 0, col: 0, w: 2, h: 1 }
      data_binding: sales-summary
```

**Batasan yang dikunci untuk v1:**
1. Slot filling hanya valid dalam satu Shell yang sama (mengisi slot = berbagi render tree, sama seperti aturan `stack_family` di §3.1).
2. Kontrak slot adalah data-shape, bukan visual — supaya komunitas tetap bebas berkreasi secara visual selama kontrak data dipenuhi.
3. Kedalaman rekursi dibatasi satu level untuk v1 (Page menerima Component; Component-di-dalam-Component didefer sampai ada use case mendesak).
4. `forma apply` memvalidasi `tier` sebelum menerima slot binding: `accepts_slots` hanya sah dideklarasikan `VisualSpecKind` bertier `page` (atau `app`, kalau nanti dibutuhkan), `implements_slot` hanya sah dari tier `component`. Kombinasi lain (mis. Page mencoba `implements_slot`, atau Component mendeklarasikan `accepts_slots`) ditolak saat apply, bukan dibiarkan lolos ke runtime.

---

## 5. Spec Resolution API — Seam Runtime, Bukan Code Generation

Draft awal diskusi ini sempat keliru membingkai hubungan Shell↔Spec sebagai "code generation" (mis. "generate kode React dari spec"). **Ini salah dan sudah dikoreksi.** Yang benar:

- `forma generate` dan turunannya (`forma/gen-openapi`, `forma/gen-typescript`, `forma/gen-dart`) menghasilkan **tipe data & endpoint untuk Tier 2/3 developer** yang menulis native handler atau custom frontend. Ini tidak ada hubungannya dengan bagaimana Shell resmi me-render Layer 0/1.
- Shell (React/shadcn) adalah **satu interpreter generik yang di-deploy sekali**, yang membaca spec **saat runtime** dan me-render secara dinamis untuk App apa pun — bukan build artifact per-app, bukan hasil compile spec-ke-kode.

**Seam yang sebenarnya perlu dirancang sejak awal bukan "generator kode," tapi Spec Resolution API** — kontrak internal yang dipanggil interpreter Shell untuk mendapat representasi siap-render dari suatu Page/Entity:

```
GET /_forma/view-spec/{app}/{page}
→ VisualSpecKind instance (resolved, sudah permission-filtered per user —
   field yang required_permission-nya tidak dipenuhi user tidak ikut terkirim)
→ metadata field (type, validation, relation target, dst.)
→ referensi slot yang sudah di-resolve (kalau ada, lihat §4)
```

Ini menegaskan ulang kenapa Layer 0 tidak butuh dev environment lokal (keputusan lama di Form kind spec) — itu hanya mungkin karena rendering memang runtime, bukan compile-time. Konsekuensi penting untuk desain seam:

1. **Spec Resolution API harus backend-agnostic.** Ia menyerahkan bentuk data (field/type/validation/permission) — bukan query result mentah dari PersistBackend tertentu. Selama API ini tidak membocorkan detail Postgres (nama kolom fisik, path JSONB), Shell mana pun bisa jadi konsumen tanpa peduli PersistBackend di baliknya (lihat audit lengkap di §8).
2. **Renderer komunitas (Kanban versi Vue, dsb.) juga interpreter runtime**, konsisten dengan pola yang sama — bukan build step terpisah per kombinasi spec+renderer.
3. Untuk Shell kedua nanti (§6): pekerjaan besarnya bukan "bikin code generator baru per platform," tapi **bikin interpreter baru yang mengonsumsi Spec Resolution API yang sama**. Investasinya tetap besar (perlu native widget/component mapping per Page/Component renderer jenis), tapi seam-nya sudah tersedia dari desain Shell pertama — asalkan Spec Resolution API memang dirancang agnostik sejak sekarang, bukan ditambal belakangan.

---

## 6. Shell Baru (mis. Flutter) — Struktur Sama, Investasi Beda Kelas

Ditanyakan: kalau nanti ada Shell Flutter, apakah hirarki Shell/App/Page/Component tetap berlaku sama? **Ya — pola arsitektural ini tidak terikat web.** Yang berubah hanya isi katalog renderer, bukan bentuk tingkatannya.

| Tingkat | Shadcn shell | Flutter shell (potensial) |
|---|---|---|
| App renderer | sidebar-nav, topnav, landing-page | bottom-tab, drawer-nav, onboarding-flow |
| Page renderer | data-entry, wizard, kanban, listing, report | jenis yang sama — satu spec, renderer beda |
| Component renderer | textinput, dateinput, widget | native TextField, DatePicker, widget |

**Perbedaan kelas dengan Renderer komunitas biasa:** Shell baru bukan sekadar konsumen API sederhana. Shell memegang Layer 0 auto-generate, Navigation, Menu, Auth wiring, permission-aware rendering — ia adalah interpreter penuh atas kontrak kind system (lewat Spec Resolution API di §5), bukan sekadar "panggil satu endpoint lalu render." Investasinya setara membangun ulang setengah framework per platform. Karena itu Shell baru **sebaiknya first-party Forma dulu** ("proven first, then offered to community" — pola yang sama dengan store/studio/workspace-admin), bukan dibuka ke komunitas dengan cara yang sama seperti Renderer standalone di §3.1.

**Catatan Navigation model tidak fully-portable:** Sidebar/topnav mengasumsikan URL-based routing (web). Flutter idiomatik pakai stack-based push/pop dan bottom-tab/drawer — paradigma navigasi yang beda secara mental, bukan cuma tampilan. Navigation kind (closed enum di App) kemungkinan perlu di-namespace per-shell nanti — dicatat sebagai constraint, belum keputusan final karena Flutter shell belum dibangun.

---

## 7. Menu Source — Dua Mode, Tanpa Layer Ketiga

Menu App punya dua mode: `module` (auto-ambil semua entity yang di-`depends_on` App) dan `custom` (pilih eksplisit). Sempat diusulkan mode `override` (ambil semua lalu kecualikan sebagian) sebagai jalan tengah — **ditolak**, karena redundan dengan `custom` dan cuma menambah kompleksitas tanpa menyelesaikan masalah baru.

```yaml
# app.yaml
spec:
  app_renderer: bottom-tab
  menu:
    mode: custom
    items:
      - entity: billing/invoice
      - entity: billing/customer
      # billing/expense-report tidak disebut = otomatis tidak muncul
```

**Keputusan detail:**
- Urutan item di list = urutan tampil. Tidak ada field `priority` terpisah.
- Trade-off yang disadari: App bermode `custom` lepas dari auto-sync — kalau Module menambah Entity baru, App itu tidak otomatis dapat menu barunya, harus ditambahkan manual. Ini harga wajar untuk kurasi eksplisit.
- **Alasan menolak tooling tambahan** (mis. `forma menu dump`): instinct pertama developer adalah buka file Module, cari definisi menu, copy-paste ke `app.yaml`. Solusi yang benar adalah membuat *schema* Module dan App identik bentuknya supaya copy-paste langsung berhasil — bukan menambah CLI command untuk menjembatani gap format. Konsisten dengan prinsip "kalau ada gesekan penggunaan, perbaiki schema dulu sebelum menambah tool."
- **Notasi qualifier ikut konvensi yang sudah ada di spec** (`module/resource`, dipakai konsisten di `sources.resource`, penamaan Named Scripts `{module}/{script-name}`) — bukan notasi titik. Referensi di Module sendiri tetap tanpa qualifier (`resource: invoice`, karena konteksnya sudah jelas satu module); qualifier baru dibutuhkan saat entity direferensikan dari App yang punya banyak dependency module, mirip package-qualified reference di Go (`Invoice` di dalam package sendiri vs `billing.Invoice` dari luar — analoginya pakai slash, bukan titik, di Forma).
- **Validasi gratis yang perlu ditambahkan** di `forma apply`: `menu.items[].entity` harus konsisten dengan `depends_on` App — App tidak boleh menaruh entity di menu dari Module yang tidak ia deklarasikan sebagai dependency.

---

## 8. PersistBackend — Seam Arsitektural yang Dikunci Sejak Sekarang

Pertanyaan penutup diskusi: kalau Entity spec adalah kontrak dan implementasi visualnya bisa diganti (Renderer), apakah implementasi *penyimpanan* entity juga bisa diganti — misalnya dari hybrid JSONB (default sekarang) ke fully-relational (tiap field jadi kolom nyata), atau ke backend SQLite untuk deployment kecil/edge?

**Keputusan: ya, secara arsitektural ini harus direncanakan sekarang — bukan didefer.** Prinsip yang sama dengan Shell (§6) berlaku: kalau PersistBackend kedua suatu saat ingin dimungkinkan, seam-nya harus ada sejak PersistBackend pertama (Postgres hybrid) dibangun. Menunda ini sampai "nanti kalau perlu" berarti retrofit besar — membongkar migration engine, query resolution, dan `ctx.next_key` sekaligus, bukan menambah modul baru.

**PersistBackend levelnya setara Shell** (bukan setara Page/Component renderer) — satu implementasi resmi (`forma-persist-postgres`, hybrid JSONB) dibangun dan dipakai untuk waktu lama, tapi *seluruh* framework wajib bicara ke interface PersistBackend, tidak boleh mengasumsikan Postgres langsung di kode inti.

### 8.1 Audit: Mana Kontrak, Mana Detail Implementasi

| Bagian spec | Status |
|---|---|
| Entity fields, relations, `persist.indexes`, `natural_key_rule`, primary key strategy (UUID v7/integer/natural key) | **Kontrak** — deklaratif, backend-agnostic sejak desain |
| Filter operator HTTP query convention (`eq`, `gt`, `between`, dst.) | **Kontrak** — sudah deklaratif; setiap PersistBackend WAJIB mengimplementasikan operator yang sama persis |
| Migration structural (auto-generated) | **Kontrak niatnya, perlu reformulasi**: bukan "framework generate SQL," tapi "PersistBackend menerima structural diff, backend menerjemahkan ke storage-nya sendiri" |
| SQL yang di-generate untuk Summary multi-source join (contoh konkret di spec sekarang) | **Bukan kontrak** — detail bagaimana renderer Postgres resmi menjawab kontrak "gabungkan sources by join_key." PersistBackend lain menjawab kontrak yang sama dengan cara berbeda |
| `ctx.db` (raw SQL escape hatch) | **Sengaja backend-coupled — harus didokumentasikan eksplisit sebagai forfeit portability.** Resource yang pakai `ctx.db` otomatis terkunci ke PersistBackend yang menyediakan dialek SQL itu; ini bukan bug, tapi konsekuensi yang harus disadari developer saat memilih memakainya |
| JSONB column-per-extension (uninstall via `ALTER TABLE DROP COLUMN`) | **Detail implementasi backend Postgres** — kontraknya adalah "extension harus bisa di-uninstall bersih tanpa sisa"; *cara* mencapainya adalah urusan masing-masing PersistBackend |

### 8.2 Seam yang Wajib Ada Sebelum M2 Selesai

1. Perkenalkan `PersistBackend` sebagai kind formal, setara `Shell` di lapisan visual.
2. Migration engine, query resolution, `ctx.next_key`, index generation — semua bicara ke interface PersistBackend, tidak ada shortcut ke Postgres langsung di kode inti `forma-server`.
3. `ctx.db` didokumentasikan ulang sebagai "escape hatch yang mengorbankan portabilitas backend," bukan escape hatch generik.
4. Spec Resolution API (§5) yang dikonsumsi Shell juga harus lewat lapisan resolusi yang seragam ini — tidak boleh menyerahkan bentuk data yang membocorkan detail Postgres, supaya Shell benar-benar tidak perlu tahu PersistBackend apa yang ada di baliknya.

Ini pekerjaan arsitektural, bukan pertanyaan terbuka untuk didiskusikan nanti — implementasinya (PersistBackend kedua yang benar-benar berjalan) tetap boleh ditunda sampai ada kebutuhan nyata, tapi *seam*-nya dikunci sekarang.

---

## 9. Ringkasan Keputusan

| # | Keputusan | Status |
|---|---|---|
| 1 | Kind visual tidak flat — ada hirarki Shell / App renderer / Page renderer / Component renderer | Final |
| 2 | `VisualSpecKind` sebagai meta-kind untuk mendeklarasikan skema+kontrak jenis view baru | Diusulkan |
| 2a | `VisualSpecKind.spec.tier` (app / page / component) wajib dideklarasikan eksplisit — menentukan di mana kind boleh dipakai dan divalidasi untuk slot compatibility. Shell tidak punya nilai tier karena bukan VisualSpecKind | Diusulkan |
| 3 | `Renderer` kind sebagai implementasi konkret suatu VisualSpecKind, dengan `stack_family` dan `trust_tier` | Diusulkan |
| 4 | App shell + Page (shell-integrated) + Component wajib satu `stack_family` yang sama; Page yang benar-benar lepas dari App bebas stack apa saja tanpa perlu Renderer kind | Final |
| 5 | `route_mode: standalone` di level Page dibatalkan — keputusan shell-or-not sudah ditentukan di App renderer | Final (revisi dari usulan sebelumnya) |
| 6 | Slot system (`accepts_slots`/`implements_slot`) untuk relasi Page↔Component seperti Dashboard↔Widget, dibatasi satu level rekursi untuk v1 | Diusulkan |
| 7 | Rendering Shell terjadi di runtime lewat Spec Resolution API — bukan code generation. `forma generate`/`gen-typescript`/`gen-dart` khusus untuk Tier 2/3 developer, tidak terkait rendering Layer 0/1 | Final (koreksi dari framing sebelumnya) |
| 8 | Shell baru (mis. Flutter) mengikuti hirarki yang sama dan mengonsumsi Spec Resolution API yang sama, tapi merupakan investasi first-party, bukan community Renderer biasa | Diusulkan |
| 9 | Navigation kind kemungkinan perlu di-namespace per-shell (belum final, menunggu Shell kedua benar-benar dibangun) | Open |
| 10 | Menu App: dua mode (`module`, `custom`), mode `override` ditolak karena redundan | Final |
| 11 | Qualifier entity di App multi-module pakai notasi `module/resource` (konsisten dengan konvensi existing), bukan titik | Final |
| 12 | Validasi `forma apply`: `menu.items[].entity` harus tercakup dalam `depends_on` App | Diusulkan |
| 13 | `PersistBackend` sebagai seam arsitektural setara Shell — dikunci sekarang (interface, audit kontrak-vs-implementasi), implementasi kedua boleh ditunda sampai ada kebutuhan nyata | Final (arsitektural) |
| 14 | `ctx.db` didokumentasikan ulang sebagai escape hatch yang eksplisit mengorbankan portabilitas PersistBackend | Diusulkan |

---

## 10. Pertanyaan Terbuka untuk Sesi Berikutnya

- Apakah `VisualSpecKind`/`Renderer` perlu masuk Core Basic atau cukup Core Extended?
- Bagaimana `forma lint`/`forma apply` memvalidasi bahwa sebuah Renderer benar-benar memenuhi `renderer_contract` yang dideklarasikan VisualSpecKind-nya — validasi statis dari skema, atau perlu test-suite konformansi seperti `Extended Conformance` yang sudah ada?
- Bentuk konkret interface `PersistBackend` (§8.2) — daftar method minimal yang wajib diimplementasikan setiap backend (structural diff apply, query resolution, next_key, dst.) perlu dirumuskan di sesi terpisah sebelum ditulis sebagai revisi resmi Core Basic Spec.
- Bagaimana nasib `ctx.next_key`, JSONB extension pattern, dan migration engine dirumuskan ulang sebagai kontrak generik (bukan spesifik Postgres) tanpa kehilangan garansi yang sudah ada (gap-free sequence, uninstall bersih, dst.)?

---

*Dokumen ini adalah rangkuman kerja dari sesi diskusi. Tujuannya menyimpan alur penalaran dan keputusan agar tidak hilang. Bukan keputusan final — perlu direview dan diformalkan sebagai revisi resmi spesifikasi Forma.*

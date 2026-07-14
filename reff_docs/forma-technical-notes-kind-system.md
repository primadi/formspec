# Forma Technical Note: Kind System — App, Module, Menu, Navigation, Theme, dan Akses Lintas-Module

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec.**
**Status: bahan eksplorasi desain, belum committed ke Forma Core Basic/Extended Spec. Beberapa poin di sini memperluas Unified Permission Declaration (Core Basic §11.4) dan mendahului Technical Note "Form Layout & List View Declaration" yang masih open item.**

---

## 0. Latar Belakang

Diskusi ini bermula dari kekhawatiran bahwa Forma bisa membuat aplikasi bisnis jadi monoton — semua app terlihat sama karena satu pola shell (sidebar) dipaksakan ke semua kasus. Dari situ diskusi berkembang ke pertanyaan yang lebih besar: **bagaimana Forma memisahkan tanggung jawab antara struktur data (Entity), tampilan (View), navigasi (App), dan identitas visual (Theme)** — tanpa jatuh ke dua ekstrem: dipisah kaku ala "backend module vs frontend module" (over-engineered, melanggar prinsip cohesion), atau dibiarkan bebas tanpa struktur (rawan inkonsistensi, melanggar prinsip *safety via structure*).

Kasus pemicu paling konkret: **aplikasi pendaftaran siswa baru (PPDB)** — punya dua populasi user yang sangat berbeda (staf sekolah vs orang tua calon siswa), dua model auth yang berbeda, dan dua pola navigasi yang berbeda, tapi mengakses data yang sama.

---

## 1. Dua Sumbu yang Berbeda Level: View Kind vs Navigation Kind

Sering tercampur dalam percakapan sehari-hari, tapi keduanya beroperasi di level yang berbeda:

| | View Kind | Navigation Kind |
|---|---|---|
| Level | Per-resource / per-layar | Per-App |
| Contoh nilai | `list`, `search_layout`, `kanban`, `timeline`, `wizard`, `form`, `detail` | `sidebar`, `topbar`, `command-first`, `master-detail`, `portal` |
| Siapa yang deklarasi | Module (di dalam ViewKind resource) | App |
| Sifat | Granular, bisa berbeda-beda per resource dalam satu Module | Satu App = satu Navigation Kind, berlaku untuk seluruh App |

**Relasi:** Navigation Kind adalah wadah, View Kind adalah isi. `wizard` misalnya tetap View Kind yang sama — tapi kalau dipasang dalam Navigation `sidebar`, dia muncul sebagai modal/panel di dalam shell yang tetap ada; kalau dipasang dalam Navigation `portal`, wizard itu jadi keseluruhan layar tanpa shell di sekitarnya.

---

## 2. Kasus Publik vs Internal — Dipaksa Jadi Dua App, Bukan Satu App dengan Dua Surface

Pertimbangan awal adalah menambahkan konsep "Surface" (public/internal) di dalam satu App. Ini **ditolak** — keputusan finalnya: **App Publik dan App Internal adalah dua App yang benar-benar terpisah**, bukan dua Surface dalam satu App.

**Alasan:**

1. **Model auth berbeda secara arsitektur, bukan cuma config** — App Internal pakai tenant-based RBAC (staf sekolah = identitas Workspace), App Publik pakai Forma ID Jalur B (orang tua = *consented external identity*, bisa anonim sampai OTP). Ini dua auth stack berbeda, bukan dua mode dari satu stack.
2. **Navigation Kind jadi properti App yang bersih** — satu App = satu Navigation Kind, tidak perlu mekanisme override per-Surface yang menambah kompleksitas config.
3. **Model multi-App per Workspace sudah ada dan terbukti berguna** — satu Workspace bisa menjalankan banyak App yang mengakses data yang sama; kasus PPDB justru membuktikan kegunaan model itu, bukan kasus khusus baru yang butuh konsep tambahan.
4. Menghapus kebutuhan konsep "Surface" sama sekali → sejalan dengan prinsip closed-set primitives (tidak menambah primitive baru kalau primitive yang ada sudah cukup).

**Kesimpulan struktural:**

```
App = (Navigation Kind + Auth Kind + Menu + Theme-ref) yang me-mount
      View/Action tertentu dari satu atau lebih Module
```

Contoh:

```
Workspace: Sekolah XYZ
├── App: ppdb-internal   (Navigation: sidebar, Auth: tenant_rbac)
│     └── mount Module ppdb → expose: list, detail, timeline, approve/reject
└── App: ppdb-publik      (Navigation: portal, Auth: forma_id_public)
      └── mount Module ppdb → expose: wizard, cek-status
```

---

## 3. Isi Kind App

```yaml
kind: App
spec:
  navigation: sidebar | topbar | command-first | portal   # enum, closed set — bukan kind terpisah
  menu:
    - label: "Verifikasi Pendaftaran"
      module: ppdb
      view: invoice-list
    - label: "Laporan"
      module: ppdb
      view: laporan-timeline
  auth: tenant_rbac | forma_id_public   # enum untuk jenis model; auth_config_ref opsional untuk reuse credential (lihat §3.3)
  theme_ref: forma/theme-shadcn          # REFERENSI ke kind Theme, bukan inline
```

### 3.1 Navigation — enum, bukan kind terpisah

Navigation tetap enum langsung di App (bukan Kind mandiri) karena sifatnya struktural dan closed-set — berbeda dengan Theme yang memang dirancang untuk dijual/dipakai ulang lintas App di Store/Registry.

### 3.2 Theme — kind terpisah, direferensikan, bukan inline

Theme **tidak** ditaruh inline di App, melainkan kind sendiri yang direferensikan lewat `theme_ref`. Alasan: satu Workspace sering punya beberapa App internal yang harus konsisten brand-nya — kalau Theme inline per App, token harus dicopy-paste ke tiap App. Ini juga sudah selaras dengan item yang sudah ada di Extended Spec (Admin Panel Extensions: `forma/theme-material`, `forma/theme-shadcn`) — Theme memang dirancang sebagai artefak pluggable/installable dari registry, bukan config App.

**Isi Theme** — bukan cuma warna & font, tapi satu paket token:
- Warna & font (dasar)
- **Density** — compact/comfortable/spacious (mengubah spacing & ukuran row)
- **Motion profile** — none/subtle/expressive untuk transisi & micro-interaction
- **Iconography set** — outline/filled/duotone
- **Radius & elevation profile** — sharp/flat vs rounded/soft-shadow
- **Empty-state & illustration set**
- **Tone-of-voice preset** untuk microcopy (formal vs santai) — terintegrasi dengan sistem TranslationAsset/i18n yang sudah ada
- **Data-viz palette** — skema warna chart, terpisah dari warna UI utama

Density dan Animation yang sebelumnya dianggap sejajar dengan Theme di level App, **dipindah ke dalam Theme** sebagai bagian dari token set — supaya isi kind App tetap ramping: Navigation, Menu, Auth, Theme(-ref).

### 3.3 Auth — open question

Auth punya karakter di antara Navigation (struktural, closed-set) dan Theme (butuh reuse). Belum diputuskan final: apakah cukup enum tertutup (`tenant_rbac | forma_id_public`), atau perlu tambahan `auth_config_ref` untuk kasus App yang butuh custom SSO/OAuth dengan credential yang reusable lintas App. **Deferred** — perlu dikonfirmasi sebelum masuk spec resmi.

### 3.4 Menu — milik App, independen dari Module

**Keputusan kunci:** Menu bukan milik Module, melainkan milik App. Ini bukan keputusan estetika — ini konsekuensi langsung dari keputusan §2: karena Views/Actions yang di-expose bisa berbeda per App-mount (App Publik cuma expose `wizard`+`cek-status`, App Internal expose `list`+`approve`), Menu — yang pada dasarnya adalah enumerasi "apa yang bisa dicapai lewat navigasi" — harus ikut ditentukan di level yang sama dengan keputusan visibility itu, yaitu App.

Analogi: **Module = katalog, App.menu = daftar belanja dari katalog itu.**

Supaya tidak membebani App Owner dengan wiring manual setiap menu item dari nol (mengingat kekhawatiran "setting App jadi rumit"), **Module tetap boleh menyediakan default Menu suggestion** yang bisa langsung diadopsi App tanpa konfigurasi tambahan — App tetap bisa override/restrict/rearrange kapan pun dibutuhkan. ini menjaga convention-over-configuration ala Laravel yang jadi acuan Forma, tanpa mengorbankan fleksibilitas struktural yang sudah diputuskan.

---

## 4. Isi Kind Module — Closed Set, Bukan Backend/Frontend Terpisah

**Keputusan:** Entity dan View (Form/List/Kanban/dll) **tetap dalam satu Module** — tidak dipecah jadi "backend module" (isi Entity) vs "frontend/App module" (isi menu dll).

**Alasan:**

1. **Precedent dari benchmark yang sudah dipakai Forma sendiri** — Frappe DocType membundel field/schema + form layout + list view + permission jadi satu definisi; Filament Resource membundel model + form schema + table columns + actions. Keduanya berubah bersamaan, jadi harus didefinisikan bersama.
2. **Menghindari versioning/deployment yang tidak sinkron** — kalau Entity dan View-nya ada di dua Module terpisah, perubahan field di satu sisi bisa lepas sinkron dengan sisi lain (classic distributed-monolith trap).
3. **Menghindari overhead deklarasi permission lintas-module yang tidak perlu** — Unified Permission Declaration (Core Basic §11.4) mengharuskan setiap akses lintas-resource dideklarasikan eksplisit; memisahkan Entity dan View jadi dua Module berarti View harus declare cross-module permission ke Entity miliknya sendiri, overhead yang tidak proporsional untuk sesuatu yang sebenarnya satu bounded context.
4. Ini juga persis kasus yang sudah dilarang sendiri sebagai **dual-layer terminology anti-pattern** — memecah sesuatu yang kohesif jadi dua vocabulary/boundary buatan.

**Struktur Module — closed set of Kind types yang diizinkan di dalamnya:**

```
Module (= unit deployment, versioning, ownership — dipegang Module Owner)
├── Entity           → invoice, customer
├── ViewKind         → invoice-list, invoice-wizard, invoice-detail, invoice-timeline
├── BusinessService  → tax-calculator
├── BusinessRule     → validasi custom
└── (permission declarations mengikat semuanya di atas)
```

Module boleh berisi kombinasi apa saja dari kind-kind di atas — **tapi hanya kind-kind itu**, bukan tipe bebas. Module tidak wajib punya semuanya: Module murni integrasi (mis. `forma/tax-calculator`) bisa cuma berisi BusinessService tanpa Entity/ViewKind sama sekali.

**Mental model untuk developer:** buka satu folder Module → lihat satu kapabilitas bisnis utuh (entity, service, semua kemungkinan tampilannya) dalam satu tempat. Baru di level App, developer memikirkan "App mana yang boleh pakai potongan mana dari Module ini, disusun jadi navigasi seperti apa."

**Catatan konvensi yang tetap berlaku:** semua Kind didefinisikan flat di file YAML; folder hanya pembagian logis, tidak membawa makna struktural tambahan.

---

## 5. Akses Lintas-Module dalam Satu Workspace

Module boleh mengakses Module lain, selama masih dalam satu Workspace yang sama. Karena Module Owner adalah identitas akuntabel terpisah, akses ini melintasi *ownership boundary*, bukan cuma technical boundary — perlu deklarasi yang lebih eksplisit dari yang sudah ada.

### 5.1 Tiga Jenis Interaksi — Level Coupling Berbeda

| Jenis | Contoh | Coupling |
|---|---|---|
| Entity read (query langsung) | `invoice` baca `customer` (`actions: [find]`) | Paling erat — A jadi tahu bentuk data B |
| Service/Action call | `ctx.call("billing", "invoice.send", ...)` | Longgar — A cukup tahu kontrak Action, tidak tahu skema internal |
| Event subscribe (pubsub) | subscribe `invoice.paid` | Paling longgar, async, tanpa dependency waktu-boot |

**Preferensi urutan:** Action call / Event dulu untuk apa pun yang menyangkut *behavior*; Entity read langsung dibatasi untuk kasus read-only sederhana (mis. cek data referensi). Framework tidak melarang entity read lintas-module (sudah ada contohnya di spec), tapi convention/dokumentasi mengarahkan ke pola yang lebih longgar untuk hal yang menyangkut logic.

### 5.2 Deklarasi Dependency di Level Module

Permission saat ini dideklarasikan per-resource, cukup untuk enforcement runtime tapi belum cukup untuk visibilitas dependency di level Module. Tambahan yang diusulkan:

```yaml
kind: Module
metadata:
  name: billing
spec:
  depends_on:
    - module: customer
      version: ">=1.0.0"
```

**Manfaat:**
- Store/Registry bisa menampilkan dependency graph antar Module secara eksplisit — relevan untuk automated gate trust tier.
- Versioning/breaking change di satu Module bisa ditelusuri dampaknya ke Module lain — analog ke dependency DAG yang sudah ada untuk Summary entities.
- Murah dideklarasikan sekarang, mahal ditelusuri manual nanti — sejalan prinsip *defer aggressively, lock structurally*.

### 5.3 Batas: Hanya dalam Satu Workspace

Mekanisme ini berlaku **dalam satu Workspace yang sama**. Lintas Workspace bukan lagi soal "Module akses Module" — itu domain Data Sovereignty yang sudah final (App Owner terstruktur tidak bisa baca data Workspace lain). Cross-module access = urusan Module Owner dalam satu Workspace; cross-workspace = mekanisme terpisah yang jauh lebih ketat. Keduanya tidak boleh dicampur jadi satu mekanisme.

---

## 6. Akses Lintas-Module dari Module Owner Berbeda — Consent

### 6.1 Dua Kasus Berbeda

- **Module dengan Module Owner yang sama** (mis. satu vendor publish beberapa Module sebagai satu suite) → akses langsung via `depends_on` + permission declaration (§5.2) sudah cukup. Tidak perlu lapisan consent tambahan — menambahkannya di sini adalah over-engineering untuk kasus solo developer/software house yang umum.

- **Module dari Module Owner berbeda** (mis. Workspace install `vendor-A/inventory` dan `vendor-B/billing` dari Store, keduanya perlu saling baca data) → **butuh consent eksplisit**. Vendor-A tidak boleh diam-diam terbaca oleh Module lain hanya karena Module lain men-declare permission sepihak di YAML-nya — terlalu implisit untuk ekosistem marketplace pihak ketiga.

### 6.2 Siapa yang Berhak Memberi Consent — Workspace Owner, Bukan Module Owner

**Keputusan:** Consent lintas Module-Owner-berbeda diberikan oleh **Workspace Owner**, bukan oleh Module Owner asal data.

**Alasan:** ini konsisten dengan prinsip yang sudah final — data adalah milik Workspace (Data Sovereignty), dan forma-control adalah billing/approval source of truth tanpa self-approval. Kalau Module Owner ikut punya hak veto atas consent akses datanya sendiri, itu kontradiksi langsung dengan prinsip data-ownership yang sudah dikunci. Module Owner menyediakan **Public Surface** (subset Entity/Action yang sengaja dia-expose untuk dikonsumsi Module lain — pola yang sama dengan Views/Actions per-App-mount di §2), tapi keputusan "boleh dikonsumsi Module lain atau tidak" tetap di tangan Workspace Owner.

Pola ini konsisten dengan Forma ID (Jalur B): akses ke data pihak lain harus lewat consent eksplisit yang terlihat, bukan implisit lewat deklarasi sepihak. Efek sampingnya menguntungkan positioning trust Forma — Workspace Owner punya daftar audit jelas soal "koneksi antar-Module pihak ketiga apa saja yang aktif di Workspace saya."

### 6.3 Open Questions

- Apakah consent ini kind artefak baru yang di-install, atau cukup flag pada `depends_on` yang sudah ada (mis. `depends_on: [{module: inventory, owner_consent: required}]`) tanpa artefak tambahan? **Condong ke opsi kedua** — reuse mekanisme approval yang sudah ada (Deployment Policy approval flow), bukan menambah primitive baru.
- Siapa/apa yang men-trigger consent — approval manual oleh Workspace Owner, atau otomatis muncul sebagai approval request begitu Module dengan dependency lintas-owner pertama kali di-deploy?

---

## 7. Ringkasan Keputusan

| Topik | Keputusan | Status |
|---|---|---|
| View Kind vs Navigation Kind | Dua sumbu berbeda level — View per-resource, Navigation per-App | Final |
| Publik vs Internal | Dua App terpisah, bukan Surface dalam satu App | Final |
| Isi kind App | Navigation (enum) + Menu + Auth + Theme(-ref) | Final |
| Theme | Kind terpisah, direferensikan via `theme_ref`, mencakup density/motion/icon/dll | Final |
| Auth | Enum jenis auth model; `auth_config_ref` untuk reuse credential | Diusulkan — perlu konfirmasi |
| Menu | Milik App, independen dari Module; Module sediakan default suggestion | Final |
| Isi kind Module | Closed set: Entity, ViewKind, BusinessService, BusinessRule — tidak dipisah backend/frontend | Final |
| Akses lintas-Module (owner sama) | Langsung via `depends_on` + permission declaration | Final |
| Akses lintas-Module (owner beda) | Butuh consent eksplisit dari Workspace Owner | Final (prinsip) — mekanisme detail diusulkan |
| Bentuk consent artefak | Flag pada `depends_on`, bukan Kind baru | Diusulkan — perlu konfirmasi |
# Forma Technical Note: Katalog Aplikasi Forma Cloud — Satu Rumah per Pemangku Kepentingan

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec**
**Status: bahan desain produk untuk wajah Forma Cloud, melengkapi Technical Note "Mengapa Forma Harus Ada" dan Investment Memorandum bagian 8.**

---

## 0. Latar Belakang

Technical Note sebelumnya memetakan enam pemangku kepentingan yang membutuhkan Forma. Dokumen ini menjawab pertanyaan lanjutannya: **kalau tiap pemangku kepentingan punya kebutuhan berbeda, apa mereka juga butuh "rumah" (aplikasi) sendiri-sendiri di ekosistem Forma, bukan dipaksa berbagi satu dashboard generik?**

Jawabannya ya — dan ini bukan sekadar preferensi desain, ini konsekuensi langsung dari four-layer actor model yang sudah dikunci: Workspace Owner, App Owner, Module Owner, Cloud Owner masing-masing punya *concern* yang berbeda secara fundamental (data bisnis vs kode aplikasi vs distribusi modul vs infrastruktur). Memaksa mereka ke satu UI yang sama akan mengaburkan batas yang justru sengaja dipertahankan tegas di seluruh Technical Note Kedaulatan Data.

Dokumen ini adalah rujukan katalog produk — dipakai untuk perencanaan tim produk, bahan pitch investor (bagian "produk apa saja yang sudah/akan dibangun"), dan bahan onboarding developer baru yang bergabung ke tim Forma.

---

## 1. Prinsip Desain Sebelum Daftar Aplikasi

Sebelum masuk daftar, satu kebingungan yang harus diluruskan dulu: **ada dua jenis "aplikasi" yang sama sekali berbeda pemiliknya**, dan keduanya sama-sama muncul di ekosistem Forma.

**(a) Aplikasi first-party milik Forma** — dibangun dan dioperasikan Forma sendiri, closed-source, jadi wajah dari Forma Cloud sebagai produk (forma/workspace-admin, forma/studio, forma/store, forma/module-admin, dst — daftar lengkap di bagian 3).

**(b) Admin panel yang otomatis dihasilkan untuk SETIAP aplikasi bisnis** yang dibangun App Owner di atas Forma (Core Basic Spec, Section 11.3) — ini **bukan** aplikasi Forma, ini bagian dari aplikasi milik App Owner/Workspace Owner itu sendiri. Kalau App Owner membangun aplikasi bengkel, admin panel yang dihasilkan otomatis dari resource definition itu adalah panel bengkel itu — Forma cuma menyediakan mesin generatornya, bukan memilikinya.

Kenapa ini penting dibedakan: **forma/workspace-admin bukan pengganti admin panel bisnis App Owner.** Workspace Owner (pemilik bengkel) akan pakai **dua** antarmuka berbeda — panel bisnis harian mereka (dihasilkan dari resource definition, milik App Owner mereka) untuk kerja sehari-hari, dan forma/workspace-admin (milik Forma) khusus untuk hal-hal yang levelnya "hosting/langganan," bukan "data bisnis" — mirip beda cPanel vs aplikasi WordPress yang di-hosting di atasnya.

---

## 2. Peta Lengkap: Siapa Punya Rumah Apa

| Pemangku Kepentingan | Rumah Utama | Bentuk | Sumber |
|---|---|---|---|
| Workspace Owner | `forma/workspace-admin` | Web app / PWA | Closed-source, first-party Forma |
| App Owner (developer) | `forma` CLI | CLI | Closed-source reference impl, gratis dipakai |
| App Owner (low-code/non-teknis) | `forma/studio` | Web app | Closed-source, first-party Forma (roadmap) |
| App Owner + Module Owner | `forma/store` | Web app | Closed-source, first-party Forma |
| Module Owner | `forma/module-admin` | Web app | Closed-source, first-party Forma |
| Cloud Owner (tim infra Forma) | `forma-ctl` CLI | CLI | Closed-source reference impl |
| Cloud Owner (tim infra Forma) | `forma/ops` | Web app internal | Closed-source, internal-only |
| App Owner + Cloud Owner | `forma/observe` | Web app / dashboard | Modul terpisah, installable |
| Pengguna individu (Forma ID) | `forma/id` | Mobile-first PWA | Closed-source, first-party Forma |
| *(bukan aplikasi Forma)* | Admin panel bisnis | Web / PWA | Milik App Owner, auto-generated |

---

## 3. Detail Per Aplikasi

### 3.1 `forma/workspace-admin` — Rumah Workspace Owner

**Audiens:** pemilik bisnis (bengkel, klinik, enterprise) — baik yang tidak punya tim teknis maupun yang punya.

**Masalah yang diselesaikan:** Workspace Owner butuh kontrol atas hal-hal levelnya "pemilik akun," terpisah dari aplikasi bisnis harian yang dibangun App Owner mereka — supaya App Owner tidak perlu (dan tidak harus diberi kesempatan) membangun ulang fitur-fitur ini sendiri di tiap aplikasi.

| Fitur | Deskripsi |
|---|---|
| Manajemen langganan & billing | Pilih tier Forma Cloud (Murah/Menengah/Mahal — Technical Note Tiering), lihat tagihan, riwayat pembayaran |
| Manajemen App Owner yang diberi akses | Lihat siapa (developer/software house) yang jadi App Owner untuk Workspace ini, cabut akses kapan saja |
| **Log consent & break-glass** | Lihat riwayat kapan App Owner (atau pegawai Forma) meminta akses ke data mereka, kapan disetujui/ditolak — ini antarmuka pelanggan untuk audit trail yang dijanjikan di Technical Note Kedaulatan Data bagian 7 |
| Manajemen modul terinstall | Lihat modul apa saja yang aktif di aplikasi mereka, status Verified Badge tiap modul |
| Kuota & penggunaan | Lihat penggunaan storage/compute mereka vs kuota tier (Technical Note Tiering bagian 7) |
| Ekspor/backup data mandiri | Self-service `forma backup create` tanpa perlu minta App Owner — memperkuat klaim kedaulatan data (data selalu bisa diambil pemiliknya sendiri) |
| Pengaturan data residency (tier Mahal) | Pilih region hosting untuk kebutuhan kepatuhan |
| Manajemen Forma ID terhubung (opsional) | Kalau ikut program Forma ID, kelola pengaturan consent-nya dari sisi bisnis |

**Catatan desain:** ini yang harus paling mudah dipakai non-teknis — audiens utamanya pemilik bengkel/klinik, bukan developer. PWA, tanpa install, konsisten dengan keputusan format frontend yang sudah dibahas.

---

### 3.2 Admin Panel Bisnis — Bukan Aplikasi Forma, Tapi Perlu Disebut di Sini

**Audiens:** staf operasional Workspace Owner (kasir bengkel, resepsionis klinik) dan Workspace Owner sendiri untuk kerja harian.

**Kenapa disebut di katalog ini meski bukan milik Forma:** karena inilah antarmuka yang **paling sering** dipakai harian oleh mayoritas pengguna akhir ekosistem Forma — jauh lebih sering dibanding forma/workspace-admin. Auto-generated dari resource definition (Core Basic Spec Section 11.3), mengikuti pola UI yang sudah dikunci di Technical Note Document Model (2-step + auto-save untuk transaksi, CRUD polos untuk master data, dst).

**Fitur yang diwariskan otomatis dari framework** (App Owner tidak perlu membangun manual):
- CRUD sesuai `characteristics` resource (master/transaction/summary)
- Live API Explorer untuk testing (Core Extended)
- Role-based access sesuai permission yang dideklarasikan
- Live ticker/notifikasi via WebSocket kalau resource pakai `ctx.pubsub`

---

### 3.3 `forma` CLI — Rumah Utama App Owner

**Audiens:** developer, dari solo developer sampai tim software house besar.

**Masalah yang diselesaikan:** satu titik masuk untuk seluruh siklus hidup aplikasi Forma — dari `forma dev` hari pertama sampai `forma backup`/`forma restore` produksi bertahun-tahun kemudian.

| Kategori | Perintah kunci |
|---|---|
| Development | `forma dev`, `forma generate`, `forma apply` |
| Data | `forma migrate`, `forma seed`, `forma backup create/inspect`, `forma restore` |
| Modul | `forma module install`, `forma contract-test` |
| Registry & signing | `forma sign`, `forma promote`, `forma registry publish/verify` |
| Observability lokal | `forma dev` dashboard (Mail UI, Storage, status service) |
| Operasional darurat | `forma freeze`, `forma rollback`, `forma lock tenant` |
| Arsip | `forma archive run/inspect` |
| Skala (mode standalone) | `forma scale --replicas N` |

**Catatan desain:** ini satu-satunya "aplikasi" di katalog ini yang sepenuhnya text/terminal — sengaja, karena audiensnya developer yang justru lebih produktif di CLI dibanding GUI untuk tugas-tugas ini.

---

### 3.4 `forma/studio` — Rumah App Owner Non-Teknis (Roadmap)

**Audiens:** developer yang lebih nyaman low-code, atau (visi jangka panjang) pemilik bisnis yang ingin coba kustomisasi sendiri tanpa developer.

**Masalah yang diselesaikan:** menurunkan barrier masuk lebih jauh dari yang bisa dicapai CLI — sesuai visi "AI-assisted resource definition" yang sudah tercatat sebagai roadmap Core Extended.

| Fitur (roadmap) | Deskripsi |
|---|---|
| Deskripsi natural language → draf resource | Developer/pengguna jelaskan kebutuhan bisnis, AI hasilkan draf `.resource.yaml` sesuai konvensi Forma |
| Commit ke git dari GUI | Menjaga provenance (semua perubahan tetap lewat git, sesuai prinsip "provenance requires git") meski penggunanya tidak terbiasa command line |
| Preview visual sebelum apply | Lihat bentuk admin panel yang akan dihasilkan sebelum benar-benar `forma apply` |
| Template vertikal siap pakai | Titik masuk untuk starter template (`forma/clinic`, `forma/barbershop`, dst dari diskusi GTM) |

**Catatan desain:** ini yang paling belum matang — masih "roadmap teridentifikasi," bukan spec resmi. Perlu keputusan eksplisit: apakah forma/studio ditujukan buat developer (mempercepat mereka) atau langsung buat pemilik bisnis (self-service penuh) — dua target audiens ini butuh UX yang sangat berbeda.

---

### 3.5 `forma/store` — Rumah Bersama App Owner & Module Owner

**Audiens:** App Owner yang mencari modul untuk dipasang, Module Owner yang mempublikasikan modul mereka.

**Masalah yang diselesaikan:** tempat pertemuan supply (Module Owner) dan demand (App Owner) — analog registry npm/Packagist tapi khusus modul bisnis Forma dengan lapisan kepercayaan (Verified Badge) yang tidak ada di registry generik.

| Fitur | Deskripsi |
|---|---|
| Pencarian & filter modul | Filter per kategori (locale, vertikal, integrasi), per trust tier (official/verified/community) |
| Halaman detail modul | Deskripsi, changelog, hasil contract-test, badge, rating dari App Owner lain |
| Instalasi satu klik ke CLI | Salin perintah `forma module install`, atau trigger langsung kalau terhubung ke project lokal |
| Perbandingan modul serupa | Kalau ada beberapa modul untuk kebutuhan sama (mis. beberapa payment gateway) |
| Halaman starter template vertikal | Titik masuk GTM — `forma/clinic`, `forma/barbershop`, dst dipromosikan di sini |
| Dashboard vendor (link ke module-admin) | Module Owner yang login diarahkan ke forma/module-admin untuk kelola listing mereka |

---

### 3.6 `forma/module-admin` — Rumah Module Owner

**Audiens:** vendor pihak ketiga (payment gateway, SMS/WhatsApp API, penyedia modul kepatuhan lokal), juga tim Forma sendiri untuk modul first-party.

**Masalah yang diselesaikan:** Module Owner butuh kelola siklus hidup modul mereka dan lihat performanya tanpa akses ke infrastruktur Forma secara umum.

| Fitur | Deskripsi |
|---|---|
| Publish & versioning modul | Upload versi baru, kelola changelog, tandai breaking change |
| Status Verified Badge | Ajukan verifikasi, lihat status review, perpanjangan tahunan |
| Hasil contract-test | Lihat hasil otomatis apakah mockup mereka akurat merepresentasikan implementasi asli |
| Analytics distribusi | Berapa banyak App Owner yang install, tren adopsi dari waktu ke waktu |
| Manajemen billing badge fee | Untuk vendor berbayar (verified tier) |
| Revocation & security advisory | Kalau modul mereka punya celah keamanan, alur untuk revoke versi tertentu dan notifikasi ke App Owner yang terpasang |

---

### 3.7 `forma-ctl` CLI — Rumah Cloud Owner/Tim Infra & Security

**Audiens:** tim infra/security — baik internal Forma (untuk Forma Control Cloud) maupun tim internal klien Enterprise (untuk Forma Control Enterprise self-hosted).

**Masalah yang diselesaikan:** kredensial dan operasi yang levelnya control plane, sengaja dipisah dari `forma` CLI biasa (App Owner) supaya tidak ada percampuran privilege.

| Kategori | Perintah kunci |
|---|---|
| Insiden keamanan | `forma-ctl freeze`, `forma-ctl revoke sessions --all` |
| Kunci | `forma-ctl key rotate --environment production` |
| Kebijakan deployment | Kelola `DeploymentPolicy` (approval rules, signing requirement) |
| Audit | Query audit store immutable/hash-chained |

---

### 3.8 `forma/ops` — Rumah Internal Cloud Owner (Tim Infra Forma)

**Audiens:** tim SRE/infra internal Forma yang menjalankan Forma Cloud sehari-hari.

**Masalah yang diselesaikan:** satu tempat memantau automasi operasional yang sudah dibahas di Technical Note Kedaulatan Data bagian 9 — memastikan "manusia hanya datang saat insiden" benar-benar berjalan, bukan cuma klaim.

| Fitur | Deskripsi |
|---|---|
| Dashboard status automasi harian | Status backup, rotasi sertifikat, scaling — mana yang berjalan normal, mana yang butuh perhatian |
| Antrian break-glass access | Approval workflow untuk akses darurat (Technical Note Kedaulatan Data bagian 7) |
| Kapasitas & hard ceiling | Monitor auto-provisioning server, alert mendekati batas (bagian 10) |
| Insiden aktif | Status insiden yang sedang ditangani, riwayat post-mortem |

**Catatan penting:** perlu diselesaikan tumpang tindih cakupan antara `forma/ops`, `forma/observe`, dan kebutuhan Job/Queue Monitoring yang sudah dicatat sebagai gap ekosistem sebelum desain paralel dimulai — supaya tidak ada dua tim membangun dashboard yang saling tumpang tindih.

---

### 3.9 `forma/observe` — Rumah Bersama App Owner & Cloud Owner

**Audiens:** App Owner yang ingin memonitor aplikasi mereka sendiri, dan Cloud Owner yang memonitor kesehatan platform secara umum.

**Masalah yang diselesaikan:** observability bawaan untuk aplikasi bisnis yang dibangun di atas Forma, tanpa App Owner harus mengintegrasikan tools observability sendiri dari nol.

| Fitur | Deskripsi |
|---|---|
| Dashboard application logs | Terstruktur, filterable per tenant/module/resource |
| Job monitor | Status `ctx.queue` — pending, running, failed, dead-letter |
| Event monitor | Reliable event delivery — durable event yang gagal, retry status |
| Slow query & slow service | Threshold dari `observe.slow_query_ms`/`observe.slow_service_ms` |
| Metric dashboard | Circuit breaker state, latency p50/p95/p99 per resource |

**Sudah tercatat di spec sebagai modul terpisah, installable** — bukan bagian wajib `forma.core`.

---

### 3.10 `forma/id` — Rumah Pengguna Individu

**Audiens:** pelanggan akhir bisnis-bisnis yang pakai Forma — pemegang identitas lintas-Workspace dari Technical Note Kedaulatan Data bagian 4.

**Masalah yang diselesaikan:** satu tempat individu mengelola identitas dan consent mereka sendiri lintas semua bisnis yang mereka datangi.

| Fitur | Deskripsi |
|---|---|
| Profil identitas tunggal | Nomor telepon/WA terverifikasi OTP sebagai kunci utama |
| Notifikasi permintaan consent | Push notification real-time saat ada Workspace minta akses data mereka dari Workspace lain |
| Riwayat consent yang diberikan/dicabut | Transparansi penuh, bisa dicabut kapan saja |
| Ledger poin lintas-bisnis | Kalau ikut program loyalty lintas-Workspace |
| Dashboard aktivitas pribadi | Riwayat kunjungan/transaksi mereka sendiri di semua Workspace yang mereka izinkan — **hanya mereka yang lihat**, bukan dipakai pihak lain untuk profiling (prinsip "Jalur B" yang sudah dikunci) |

**Catatan desain:** ini kemungkinan **tidak** dibranding "Forma" secara terbuka ke konsumen akhir — perlu keputusan produk terpisah apakah forma/id tampil sebagai white-label per-Workspace atau brand tersendiri yang dikenal publik.

---

## 4. Proposal Baru: `forma/mcp` — AI Assistant sebagai Klien Resmi Forma

Ini menjawab pertanyaan konkret: kalau klien (Workspace Owner atau App Owner) ingin memakai AI assistant untuk berinteraksi dengan aplikasi mereka, bagaimana AI itu tahu Workspace mana yang dilayani dan data apa yang boleh disentuh?

### 4.1 Kabar Baik: Ini Tidak Butuh Mekanisme Keamanan Baru

Forma sudah punya jawabannya secara struktural, tinggal ditambah satu delivery channel. Core Basic Spec Section 14 (Delivery Spec) sudah mendefinisikan HTTP (wajib) dan WebSocket (wajib) sebagai cara resource diakses — **MCP tinggal jadi channel ketiga**, bukan mekanisme terpisah:

- Setiap tool MCP yang diekspos = satu action resource yang sudah ada, dengan permission yang **sama persis** dengan yang sudah dideklarasikan di `permissions` resource YAML. Tidak ada "mode AI" yang punya akses lebih longgar.
- **Sesi MCP harus melekat ke identitas user yang login, bukan cuma ke Workspace.** Koreksi penting dari draf awal: kalau token MCP cuma discope ke Workspace (bukan ke user individu di dalamnya), AI akan punya akses yang sama persis baik yang bertanya manager atau operator — padahal keduanya harus dapat jawaban berbeda. Yang benar: AI "meminjam" identitas dan hak akses user yang sedang mengajaknya bicara, sama seperti kalau user itu memanggil HTTP API langsung. Mesinnya sudah ada — Basic RBAC (Core Basic Section 11.5) dan Field-level Security (Core Extended Section 9) sudah cukup: manager dengan `invoices.view_financial` dapat field `profit_margin` di jawaban AI, operator tanpa permission itu tidak pernah dapat field itu sampai ke AI — ditolak di level query, bukan cuma disembunyikan di response.
- Audit trail yang sudah ada otomatis mencatat panggilan lewat MCP — asal ditambah satu field `actor_type: ai_agent` dan `on_behalf_of: {user_id}` di audit log, supaya jelas AI bertindak atas nama siapa, bukan sebagai identitas independen.

Ini konsisten dengan prinsip "if not declared, blocked always" — AI yang terhubung lewat forma/mcp tidak bisa berbuat lebih dari yang sudah diizinkan lewat permission declaration yang sama yang membatasi kode `native`/`script` manapun, ditambah tidak bisa berbuat lebih dari yang diizinkan untuk *user spesifik* yang sedang diwakilinya.

### 4.2 Fitur yang Perlu Ada di `forma/mcp`

| Fitur | Deskripsi |
|---|---|
| Auto-discovery resource sebagai MCP tools | Setiap action resource yang punya `expose_to_ai: true` (flag baru, opt-in per action) otomatis muncul sebagai MCP tool dengan skema input/output dari resource definition |
| Scoping ketat per identitas user | Satu koneksi MCP = satu Workspace **dan** satu user di dalamnya, tidak ada cara AI "pindah" Workspace atau "naik" ke permission user lain dalam satu sesi |
| Deskripsi tool otomatis dari resource metadata | AI tahu kapan harus pakai tool apa dari `description` yang sudah ada di resource YAML — tidak perlu ditulis ulang khusus untuk AI |
| Rate limit terpisah untuk actor AI | Karena AI bisa memanggil actions jauh lebih cepat dari manusia, `rate_limit` (Core Extended Section 10) perlu scope tambahan `actor: ai_agent` |
| Audit trail bertanda `actor_type` dan `on_behalf_of` | Membedakan aksi yang diinisiasi AI vs manusia langsung, dan atas nama user siapa — penting untuk investigasi insiden nanti |
| Consent eksplisit untuk actions sensitif | Actions dengan `data_classification: restricted` (Core Extended Section 9) tetap butuh konfirmasi manusia sebelum AI bisa eksekusi, meski AI punya permission — pola "AI mengusulkan, manusia menyetujui" untuk hal berisiko tinggi |
| **Permission analitis, terpisah dari permission CRUD** | Lihat 4.2b — ini bukan detail kecil, ini kelas permission baru yang harus eksplisit |

### 4.2b Kenapa "Boleh Buat Invoice" Tidak Sama dengan "Boleh Tanya Insight Agregat"

Ini poin yang lebih dalam dari sekadar scoping per user. Kemampuan transaksional (create/update satu record — invoice, order) dan kemampuan analitis (agregasi lintas ribuan record — "produk paling laris di area tertentu") adalah dua kelas berbeda, dan selama ini tidak ada framework yang memisahkan keduanya sebagai permission terpisah — karena secara historis keduanya dipisahkan oleh **friksi**: untuk bisa menjawab pertanyaan analitis, dulu harus ada developer yang sengaja membangun dashboard/report-nya dulu. Forma sendiri sudah punya pola ini lewat **Summary Spec** (Core Extended) — resource `characteristics: [summary]` dengan `group_by`/`compute` yang dideklarasikan dan direview developer sebelum jadi laporan yang bisa diakses.

AI menghilangkan friksi itu — siapa pun yang bisa "bertanya dalam bahasa natural" sekarang berpotensi memicu agregasi ad-hoc yang dulu butuh developer membangunnya dulu. Karena itu `forma/mcp` membedakan dua jalur:

- **Jalur aman (default):** AI hanya boleh baca Summary resource yang sudah ada — bentuk agregasinya sudah divalidasi manusia sejak desain (`group_by: [region]`, bukan `group_by: [employee]`), bukan disusun bebas saat itu juga oleh AI.
- **Jalur berisiko (butuh izin eksplisit terpisah):** AI menyusun agregasi ad-hoc sendiri lewat Query Builder. Ini **tidak pernah** otomatis didapat dari kombinasi permission CRUD apa pun:

```yaml
permissions:
  infrastructure:
    - db.readonly           # boleh baca data individual (invoice, product)
  analytics:
    - adhoc_query: false    # WAJIB eksplisit true — tidak pernah default,
                             # tidak pernah "ikut" dari db.readonly atau resource.list
```

Manager yang punya `invoices.create` + `db.readonly` **tidak otomatis** dapat `analytics.adhoc_query` — itu izin ketiga yang berdiri sendiri, harus diberikan sadar oleh siapa pun yang mengatur role di Workspace itu (lewat forma/workspace-admin).

### 4.3 Kenapa Ini Layak Dibangun Lebih Awal dari yang Terlihat

Ini bukan cuma fitur "nice to have" — ini selaras dengan posisi Forma sebagai jawaban terhadap era AI coding yang sudah jadi tema sentral sejak Investment Memorandum bagian 2. Kalau Forma sudah jadi "pagar pembatas" untuk AI yang **menulis** kode, `forma/mcp` memperluas prinsip yang sama ke AI yang **menjalankan** aksi bisnis atas nama pengguna — dua sisi masalah yang sama, satu prinsip governance.

**Nilai jual konkret ke Workspace Owner kecil:** "Anda bisa suruh AI assistant cek omzet minggu ini atau kirim reminder ke pelanggan yang belum bayar — AI itu cuma bisa lakukan yang sudah diizinkan developer Anda, tercatat semua yang dia lakukan, dan tidak bisa diam-diam mengintip data yang tidak relevan." Ini konsisten dengan seluruh argumen kedaulatan data yang sudah dibangun.

### 4.4 Pertanyaan Terbuka Khusus `forma/mcp`

- Siapa yang menerbitkan token MCP per user — lewat forma/workspace-admin (self-service oleh Workspace Owner mengatur akses tiap staf mereka ke AI), atau App Owner yang setup di awal?
- Bagaimana menangani AI assistant pihak ketiga (bukan first-party Forma) yang ingin terhubung — apakah forma/mcp jadi server publik yang bisa dipakai model apa pun, atau eksklusif untuk partner tertentu dulu?
- Perlu policy eksplisit: actions mana yang defaultnya `expose_to_ai: false` sampai App Owner sadar dan eksplisit meng-opt-in — supaya tidak ada resource lama yang tiba-tiba "bocor" ke AI tanpa App Owner sadar setelah forma/mcp dirilis.
- **Risiko agregasi lewat banyak panggilan (belum ada jawaban baku di industri manapun):** AI agentic bisa memanggil puluhan tool call dalam satu sesi dan menyimpulkan sesuatu dari kombinasinya — mis. operator yang tidak boleh lihat gaji individu memancing lewat pertanyaan bertubi-tubi ("berapa pengeluaran departemen A minggu ini kalau cuma 2 orang di sana") sampai AI tidak sengaja membocorkan info individu lewat inferensi, padahal tiap query individualnya sah secara permission. Membatasi default ke Summary resource (4.2b) meredam risiko ini tapi tidak menghilangkannya sepenuhnya kalau Summary-nya sendiri granular. Perlu riset lebih lanjut sebelum diklaim selesai.
- **Bagaimana forma/workspace-admin menjelaskan ini ke Workspace Owner yang awam:** pemilik bengkel/klinik kemungkinan tidak paham beda "boleh lihat data" vs "boleh minta AI menganalisis data secara agregat." UI pemberian izin AI ke staf mereka harus menerjemahkan `analytics.adhoc_query` ke bahasa yang masuk akal buat non-teknis, bukan menampilkan nama permission mentah.

---

## 5. Tabel Ringkasan Seluruh Katalog

| Aplikasi | Audiens Utama | Bentuk | Status |
|---|---|---|---|
| `forma/workspace-admin` | Workspace Owner | Web/PWA | Perlu dibangun |
| Admin panel bisnis (auto-generated) | Staf & Workspace Owner | Web/PWA | Sudah di-spec (Core Basic) |
| `forma` CLI | App Owner | CLI | Sudah sebagian besar di-spec |
| `forma/studio` | App Owner non-teknis | Web | Roadmap, belum di-spec detail |
| `forma/store` | App Owner + Module Owner | Web | Perlu dibangun |
| `forma/module-admin` | Module Owner | Web | Perlu dibangun |
| `forma-ctl` CLI | Cloud Owner/infra-security | CLI | Sudah di-spec (Core Extended) |
| `forma/ops` | Cloud Owner (internal Forma) | Web internal | Konsep awal, tumpang tindih dengan observe perlu diselesaikan |
| `forma/observe` | App Owner + Cloud Owner | Web/dashboard | Sudah di-spec (Core Extended) |
| `forma/id` | Pengguna individu | Mobile PWA | Konsep, belum di-spec teknis |
| `forma/mcp` | AI assistant (atas nama semua role) | MCP server | **Baru diusulkan dokumen ini** |

---

## 6. Pertanyaan Terbuka

- Apakah `forma/workspace-admin` dan `forma/studio` sebaiknya jadi satu aplikasi dengan mode berbeda (Workspace Owner vs App Owner non-teknis), atau memang harus dua aplikasi terpisah karena audiensnya beda kematangan teknis?
- Prioritas pembangunan: dari sepuluh item di tabel ringkasan, mana yang paling kritis untuk M2–M3 (Investment Memorandum) vs yang realistis ditunda ke M4–M5?
- `forma/id` — brand terpisah atau di bawah nama Forma? Ini keputusan yang harus diambil sebelum desain UI dimulai, bukan setelahnya.
- `forma/mcp` — apakah masuk sebagai bagian dari Core Basic/Extended Spec resmi (jadi bagian delivery channel wajib), atau tetap modul opsional seperti `forma/observe`?
- Siapa pemilik produk (product owner) internal untuk tiap aplikasi ini — apakah satu tim kecil mengerjakan semua di M1–M3, atau perlu dipetakan pembagian tanggung jawab sejak awal supaya tidak semua dikerjakan setengah-setengah?

---

*Dokumen ini adalah rangkuman kerja dari sesi diskusi. Tujuannya menyimpan alur penalaran dan argumen inti agar tidak hilang. Bukan keputusan final — perlu direview sebelum masuk sebagai revisi resmi Investment Memorandum atau spesifikasi teknis produk.*

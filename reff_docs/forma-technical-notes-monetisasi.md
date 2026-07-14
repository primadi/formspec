# Forma Technical Note: Monetisasi Module & App — Registry, Store, Metering, dan Realita Proteksi di Era AI

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec.**
**Status: bahan eksplorasi strategi & desain, belum committed. Beberapa kesimpulan di sini bersifat strategis (bukan cuma teknis) dan disengaja ditulis jujur soal batasnya, bukan menjanjikan proteksi yang tidak bisa ditepati.**

---

## 0. Latar Belakang

Diskusi ini bermula dari pertanyaan praktis — "registry atau store?" — lalu berkembang jauh lebih dalam ke pertanyaan fundamental: **di dunia di mana spec Forma sengaja dibuat terbuka dan mudah dibaca (sebagai fitur trust), dan AI membuat reverse-engineering nyaris tanpa biaya, apa sebenarnya yang bisa dimonetisasi, dan oleh siapa?** Kesimpulan akhir mengubah cara memandang siapa pembeli utama Forma dan di mana uang sebenarnya mengalir dalam ekosistem ini.

---

## 1. Registry vs Store — Dua Kanal, Dua Audiens

| | Registry | Store |
|---|---|---|
| Audiens | Developer/software house yang compose sendiri | End-user non-teknis (business owner) |
| Isi | Module — unduh bebas | App siap-pakai (sebenarnya: instance produksi + Module berlisensi di baliknya, kemungkinan lewat Forma Cloud managed hosting) |
| Model transaksi | Bebas unduh, bayar saat masuk produksi | Beli langsung app jadi — kompleksitas Registry/Module/lisensi disembunyikan dari user |

**Keputusan: Registry, bukan Store-dengan-komitmen-beli-di-depan, untuk distribusi Module.**

**Alasan:**
- Sejalan dengan keputusan yang sudah final: *"fees follow payment rails, bukan listing visibility"* — ini cuma masuk akal kalau discovery/download memang bebas dan fee dikutip di titik transaksi/produksi, bukan di titik unduh.
- Audiens Registry adalah developer yang mengevaluasi sendiri (bottom-up motion) — butuh coba dulu di dev sebelum putus, bukan komitmen beli di muka ala enterprise procurement.
- Model "return dalam 30 hari" tidak cocok untuk software/kode — begitu source/logic diakses dan dipakai, tidak ada pengembalian yang berarti; risikonya justru disalahgunakan (unduh, pakai sebentar, refund).
- Konsisten dengan model FSL yang sudah dipilih Forma sendiri untuk platform: bebas pakai, restriksi baru berlaku pada kasus tertentu (produksi/kompetisi komersial). Satu bahasa lisensi di seluruh ekosistem, bukan dua paradigma berbeda (platform Registry+FSL vs Module Store) yang membingungkan developer.

**Mekanisme enforcement — reuse infrastruktur yang sudah ada:**
- Environment label sudah ada (`forma.dev/environment: production`, `FORMA_ENV=production`) — license check menempel di sini: Module bebas dipakai di `development/test/staging`, gate aktif hanya di `production`.
- Deployment Policy & approval flow (`forma promote --to production`) sudah punya mekanisme gate — license verification jadi bagian dari gate yang sama, bukan sistem terpisah.

**First-party vertical App (banking, ERP, dll — revenue layer ketujuh) sengaja dipisahkan dari model ini** — sasarannya end customer lewat motion B2B closed-source biasa (demo, kontrak), bukan unduh bebas dari Registry. Dua jenis produk berbeda pembeli, jangan dipaksa satu mekanisme.

---

## 2. App Tidak Punya Lisensi Sendiri — Module Adalah Satu-Satunya Unit Lisensi

**Keputusan:** App tidak pernah punya skema lisensi sendiri. App = resep komposisi (Navigation + Menu + Auth + Theme-ref + referensi ke Module), **selalu gratis**, dilisensikan oleh gate di level Module+environment, bukan di level App.

**Alasan:**
- Gate produksi sudah didesain di level Module+environment (§1) — begitu Workspace Owner sudah punya lisensi produksi untuk Module A, Module A bebas dipakai App mana pun dalam Workspace yang sama, termasuk App custom buatan sendiri yang me-remix Module dari App yang dibeli. Ini konsekuensi benar dari desain "lisensi menempel ke Module+Workspace", bukan celah.
- App sebagai artefak berlisensi sendiri di atas ini akan jadi double-licensing yang membingungkan dan tidak perlu.
- Analog Docker Compose (file compose gratis, image di dalamnya yang berlisensi) atau Helm chart (chart gratis, container image yang berbayar).

**Peran App di Registry/Store:** starting point/reference — Module Owner bisa publish "DemoApp" bersama Module-nya sebagai onboarding artifact (semacam README interaktif yang bisa langsung `forma dev`), bukan produk terpisah yang dijual sendiri.

### 2.1 App-Reselling Bukan Ancaman — Justru Channel Distribusi

Karena App gratis dan gate ada di level Module, developer B yang download App milik developer A lalu menjual ulang sebagai "App baru" **tidak merugikan Module Owner asli** — selama Module di dalamnya tetap Module asli yang sama, Module Owner tetap dapat lisensi fee di titik `forma promote --to production`. Developer B di sini berfungsi sebagai reseller/agency (jual jasa pemasangan+kustomisasi), bukan pencuri pendapatan — pola yang sama dengan ekosistem reseller Shopify/WordPress yang sehat: lebih banyak reseller = lebih banyak distribusi = lebih banyak lisensi teraktivasi.

**Ancaman sesungguhnya cuma satu titik:** kalau developer B mengganti Module asli dengan hasil reverse-engineering-nya sendiri di dalam App yang dijual ulang. Ini bukan masalah baru — ini persis masalah reverse-engineering Module yang dibahas di §5–§6.

### 2.2 Open Question — Deferred

Kalau suatu saat komposisi App itu sendiri (Navigation/Menu/Theme yang sudah teruji dan sangat bagus) punya nilai jual nyata di luar Module di dalamnya — apakah App Owner boleh memberi harga atas komposisinya? **Deferred** — mulai dengan App selalu gratis (simple), revisit kalau ada bukti riil developer memintanya.

---

## 3. Gate Produksi — Level Control Plane, Bukan di Dalam Handler

**Keputusan awal (lalu direvisi, lihat §4):** metering tidak boleh ditaruh di dalam `impl.compiled` handler saja.

**Alasan:**
- Tidak semua Module punya compiled handler — Module yang cuma berisi Entity+Form tanpa custom logic tidak punya "tempat" untuk cek lisensi.
- Duplikasi beban ke tiap Module Owner untuk reimplementasi mekanisme lisensinya sendiri — melanggar prinsip closed-set primitives.
- Binary bisa dipatch/diganti — proteksi yang bergantung sepenuhnya ke satu artefak yang dikuasai operator (self-host) itu rapuh.

**Tempat yang benar:** gate di level aktivasi Module oleh `forma-server`/`forma-control`, independen dari isi Module. Reuse mekanisme yang sudah ada, analog persis dengan cara Mockup System bekerja (overlay/keputusan dibuat oleh runtime environment detection saat boot, bukan oleh kode di dalam Module):

```
forma promote billing-tax --to production
  → cek existing: signature, staging duration, approval
  → TAMBAHAN: cek lisensi produksi Module ini untuk Workspace ini
      (validasi ke forma-control, bukan ke dalam kode Module)
  → tidak ada lisensi valid → deployment ditolak, Module tidak
    pernah ter-load di resource plane production
```

Moat sesungguhnya bergeser ke **forma-server sebagai gatekeeper tertutup**, bukan ke performa runtime atau ke Module itu sendiri — memperjelas bagian mana dari runtime yang jadi moat riil dalam positioning "moat Forma bukan runtime engine, tapi ekosistem & distribusi."

---

## 4. Kegagalan Gate Berbasis Nama/ID — Rename Attack

**Masalah:** kalau gate cuma cek `module.id`/`metadata.name`, Workspace Owner tinggal ubah nama Module jadi milik "sendiri" — `forma-control` tidak lagi mengenali Module itu sebagai Module berbayar. Trivial dibobol.

**Revisi:** identitas Module tidak boleh berdasarkan nama self-declared. Gunakan prinsip yang sama dengan keputusan **keypair-based developer identity** yang sudah ada untuk Store federation — identitas Module adalah signature kriptografis dari private key Module Owner asli, menempel ke isi spec:

```yaml
kind: Module
metadata:
  name: billing-tax          # cuma label — TIDAK dipercaya untuk lisensi
  publisher_key: forma1a2b3c...
  signature: 9f8e7d6c...          # sign atas content hash spec ini
```

Rename `metadata.name` membuat signature invalid, kecuali Workspace Owner menghapus signature dan membuat versi sendiri — di titik itu dia keluar dari "pakai Module vendor X" menjadi klaim "buatan sendiri", yang membuka pertanyaan di §5–§6.

---

## 5. Content Fingerprinting — Kenapa Ditolak Sebagai Gate Otomatis

**Masalah yang diangkat:** kalau fingerprint dihitung dari bentuk Entity (`customer: nama, alamat, telpon`), hampir semua Module akan false-positive — bentuk generik semacam ini muncul di hampir semua sistem, tidak unik ke satu vendor.

**Analisis:** kesalahan awal ada di unit analisis yang terlalu kecil. Prinsip yang benar (analog hukum cipta — fakta dan struktur umum tidak dilindungi, yang dilindungi ekspresi spesifik dalam kombinasi besar): unit analisis harus **satu Module utuh** (puluhan entity + business rule + action + permission + wording pesan error), bukan satu entity. Kemungkinan dua Module Owner independen kebetulan sama di *kombinasi* nama field + urutan rule + threshold + wording pesan di puluhan tempat sekaligus jauh lebih kecil daripada kesamaan satu field individual.

**Keputusan akhir: fingerprint TIDAK dipakai sebagai gate teknis otomatis sama sekali.** Deteksi berbasis konten tetap probabilistik — false positive memblokir developer jujur, false negative meloloskan penjiplak yang sedikit ubah struktur. Diganti dengan **provenance record**, bukan deteksi konten ulang:

```
Saat Workspace install Module dari Registry:
  → forma-control catat: "Workspace X install Module Y v1.2,
    signed publisher Z, tanggal T" — record eksplisit, bukan
    hasil analisis ulang konten

Saat forma promote --to production:
  → cek: apakah Module ini punya provenance record yang match
    dan lisensinya aktif?
  → Ya → jalan
  → Tidak ada record (mis. Workspace klaim "tulis sendiri") →
    TIDAK otomatis diblokir — sistem tidak tahu itu jiplakan
    atau kebetulan mirip
```

Kalau ternyata itu memang rename/copy tersembunyi — itu jadi **sengketa lisensi/hukum**, diselesaikan lewat proses manual (mirip DMCA takedown), **bukan** diputuskan otomatis oleh runtime saat deploy. Content-similarity boleh dibangun nanti sebagai **sinyal untuk investigasi manusia** (notifikasi ke tim trust Forma Cloud), bukan trigger block otomatis — deferred, bukan prasyarat gate produksi.

---

## 6. Batas Fundamental yang Tidak Bisa Ditutup Teknis

### 6.1 Rename dan Re-upload sebagai Module Internal

Kalau Workspace B sekadar copy isi YAML Module dan `forma apply` sebagai Module "buatan sendiri" — tanpa pernah melalui `forma install` dari Registry — **tidak ada event apa pun yang tersentuh oleh `forma-control`**. Dari sudut pandang sistem, itu identik dengan Workspace B menulis Module dari nol. Tidak ada sinyal untuk dideteksi, bukan karena algoritma kurang canggih, tapi karena memang tidak ada perbedaan yang bisa diamati.

**Ini konsekuensi langsung dari keputusan yang sudah diambil sendiri:** spec dirancang human-readable sebagai fitur ("spec sebagai lapisan trust yang auditable"). Sesuatu yang sengaja dibuat terbaca dan bisa diaudit manusia, secara definisi juga bisa dibaca dan disalin manusia. Tidak bisa punya "spec transparan untuk trust" sekaligus "spec tersembunyi dari penyalinan" — dua tujuan itu bertentangan secara inheren.

### 6.2 Reverse-Engineering via AI — Clean-Room, Legal, Tak Terhindarkan

Pola: unduh App/Module lengkap → minta AI baca spec dan buat dokumentasi → dari dokumentasi minta AI generate ulang spec dengan modifikasi. Ini persis pola **clean-room reverse engineering** yang sudah lama diakui sah secara hukum (dipakai industri chip/BIOS puluhan tahun). AI cuma mempercepat proses dari berbulan-bulan jadi berjam-jam — tidak mengubah status hukumnya.

**Prinsip dasar hukum cipta yang berlaku:** ide/fungsi tidak pernah dilindungi, cuma ekspresi literal yang dilindungi. Begitu ada langkah "tulis ulang dari dokumentasi", ekspresi literal sudah berubah — itu legal, terlepas dari niat di baliknya. Ini bukan celah Forma, ini realita hukum cipta software sejak dulu; AI membuat biayanya nyaris nol.

**Implikasi penting:** jangan pernah posisikan Registry+lisensi sebagai perlindungan terhadap reverse-engineering fungsional — klaim itu tidak bisa ditepati. Posisi yang jujur: *"lisensi melindungi dari orang yang malas (copy-paste literal), bukan dari kompetitor yang benar-benar niat membangun ulang."*

---

## 7. Lapisan Proteksi Riil yang Tersisa

Karena spec+Module tidak bisa dilindungi teknis dari copy/regenerasi, proteksi bergeser sepenuhnya ke empat hal yang tidak bisa direplikasi lewat baca-spec-lalu-tulis-ulang:

1. **`impl.compiled` (binary handler)** — satu-satunya proteksi *teknis* sungguhan, karena memang tidak dirancang readable. Algoritma/business-logic bernilai tinggi (pricing dinamis, risk scoring) harus ditaruh di sini, bukan di spec.
2. **Data operasional & jaringan tenant (network effect)** — data ribuan tenant riil yang sudah berjalan di Module asli (dipakai untuk benchmark, tuning, rekomendasi) tidak ikut ter-regenerasi oleh siapa pun yang menulis ulang spec.
3. **Aliran maintenance** — copy/regenerasi adalah snapshot beku; tidak otomatis ikut update tarif pajak/regulasi/bugfix yang terus mengalir dari Module Owner asli. Ini proteksi *ekonomi*, paling kuat untuk vertikal yang sering berubah (pajak, payroll, kepatuhan).
4. **Lisensi + jalur hukum (FSL-style di level Module) + trust/reputasi Store** — proteksi *legal* dan *distribusi*. Efektif karena target pasar adalah software house/developer B2B yang punya eksposur reputasi dan kontrak nyata, bukan pihak anonim. Copy-an tidak dapat verified badge, tidak dapat listing resmi, tidak dapat kepercayaan pembeli berikutnya.

**Prinsip untuk didokumentasikan ke Module Owner:** jangan janjikan spec "terlindungi" — sampaikan jujur bahwa spec terbuka by design, proteksi riil ada di compiled logic + data/network effect + maintenance + legal + reputasi Store. Ini sama persis dengan cara Forma sendiri memposisikan core spec-nya CC0 sementara moat riil ada di ekosistem/distribusi, bukan kode.

---

## 8. Implikasi Strategis: AI, Daya Tawar Developer, dan Bifurkasi Pasar

### 8.1 Kenapa Forma Sangat Rentan pada Dinamika Ini

Forma sengaja dirancang spec-first, closed-set primitives, convention-based — ini kondisi ideal untuk AI code-gen bekerja baik. AI jauh lebih reliable di ruang solusi yang dibatasi (mengisi slot Entity/Action/ViewKind yang sudah didefinisikan) dibanding codebase bebas. Efek "AI menurunkan daya tawar developer" akan terjadi **lebih cepat** di ekosistem Forma dibanding di framework generik — ini konsekuensi langsung dari desain Forma sendiri, bukan tren eksternal semata.

### 8.2 Bifurkasi Pasar Module — Bukan Keruntuhan Total

Kalau originator selalu bisa disalip oleh developer yang cuma modifikasi sedikit (lebih murah daripada bikin dari nol, dan sulit disebut plagiat karena hasil AI-regenerate bisa berubah banyak/subyektif), insentif ekonomi jadi berat sebelah — pola *tragedy of the commons* klasik. Tapi ini sudah pernah terjadi di ekosistem lain (plugin WordPress, theme Shopify), dan hasilnya bukan keruntuhan total, melainkan bifurkasi:

| | Lapisan Komoditas | Lapisan Trust/Vertikal Kompleks |
|---|---|---|
| Contoh | App sederhana bervolume tinggi (barbershop, salon, laundry) | Vertikal ter-regulasi (pajak, payroll, kepatuhan) atau butuh kustomisasi tinggi |
| Moat pengembangan fitur | Menipis mendekati nol setelah matang — AI/software house lain bisa reproduksi cepat | Tetap tebal — perubahan regulasi eksternal terus memaksa update, kesalahan mahal |
| Siapa yang menang | Distribusi & akuisisi pelanggan, bukan kecanggihan kode | Sertifikasi, dukungan berkelanjutan, akurasi kepatuhan |
| Peran Forma yang disarankan | First-party vertical app / distributor langsung (Store/Forma Cloud) | Ekosistem software house tetap relevan sebagai spesialis |

**Yang tersisa untuk vertikal komoditas bukan "jual kode" tapi bergeser jadi:**
- Distribusi & akuisisi pelanggan (agency yang efisien jual+onboarding menang, bukan pembuat Module)
- Hubungan & support manusia (printer struk error, training staf, integrasi WhatsApp Business API) — tidak hilang meski software beku bertahun-tahun, justru inilah yang mencegah churn
- Agregasi data lintas tenant — moat yang tidak bisa direplikasi App custom tunggal; cuma dimiliki operator yang menjalankan Module yang sama di banyak tenant (benchmark, rekomendasi harga)
- Lapisan monetisasi tambahan (pembiayaan/marketing/hardware berbasis data operasional) — sejalan model Forma ID Jalur B (consented data broker, bukan diam-diam)

### 8.3 Forma Cloud Tetap Moat yang Tidak Tergantikan

Berbeda dari Module/spec, Forma Cloud tidak bisa direplikasi lewat baca-spec-lalu-tulis-ulang — mereplikasinya butuh modal riil (server, SLA, tim ops), kepercayaan operasional yang terbangun lewat waktu (uptime record), dan jaringan sisi-permintaan. Satu-satunya jalan replikasi adalah **vendor infra lain yang mengadopsi sistem Forma Cloud** — bukan sekadar menyalin spec Module.

### 8.4 Rekomendasi Strategis

1. **Registry/Module marketplace komisi kemungkinan mengalami komoditisasi cepat untuk Module sederhana** — harga akan turun mendekati nol untuk lapisan ini. Ini bukan kegagalan, tapi realita pasar yang perlu diterima sejak awal, bukan dilawan dengan lebih banyak proteksi teknis.
2. **Pusat gravitasi pendapatan Forma perlu makin condong ke Forma Cloud (infra/hosting) dan first-party vertical app** — dua hal yang punya moat riil (operasional dan data-network-effect), bukan Registry/Module komisi semata.
3. **Developer/software house perlu diarahkan** ke tiga peran yang tetap bernilai di era AI: (a) vertikal kompleks/ter-regulasi, (b) layanan operasional-manusia berkelanjutan, (c) reseller/distributor App (yang sudah dikonfirmasi tidak masalah — lihat §2.1) — bukan berharap jadi pencipta Module tunggal yang untung besar dari originalitas semata.
4. **Store/Forma Cloud sebagai jalur langsung ke business owner** menjadi makin penting dibanding mengandalkan ribuan software house kecil menjual App sederhana satu-satu — pergeseran target audiens utama yang layak direvisit di dokumen strategi/positioning, bukan cuma Technical Note teknis.

---

## 9. Root-Trust Eksternal — Prasyarat Teknis yang Masih Deferred

Semua mekanisme metering di atas (§3–§4) mengasumsikan `forma-control` bisa memverifikasi lisensi ke otoritas yang tidak sepenuhnya dikuasai operator self-host. Kalau operator menguasai penuh instance `forma-control` miliknya (self-hosted), secara teori dia bisa mem-patch validasi lisensi di situ juga — ini bukan celah unik Forma (masalah yang sama dihadapi semua software self-hosted berlisensi seperti GitLab EE, Elastic X-Pack).

**Solusi standar industri:** token lisensi bertanda-tangan dengan masa berlaku, perlu verifikasi berkala ke otoritas eksternal (Forma Cloud sebagai root, bukan `forma-control` lokal). Ini menyatu dengan item yang sudah ditandai deferred: **"Unified root trust infrastructure across Forma Cloud instances."** Detail desainnya tetap **deferred** — belum ada Module berbayar riil beredar — tapi dicatat sebagai keharusan struktural begitu monetisasi Module berjalan, bukan opsional.

---

## 10. Ringkasan Keputusan

| Topik | Keputusan | Status |
|---|---|---|
| Registry vs Store | Registry untuk Module (developer), Store untuk App jadi (end-user, via Forma Cloud) | Final |
| Unit lisensi | Module, bukan App — App selalu gratis | Final |
| App-reselling | Bukan ancaman — Module Owner tetap dapat fee lewat gate produksi | Final |
| Gate produksi | Level control plane (`forma-control`), bukan di dalam handler | Final |
| Identitas Module | Signature kriptografis (publisher key), bukan nama/ID self-declared | Final |
| Content fingerprinting | Ditolak sebagai gate otomatis — diganti provenance record + investigasi manual | Final |
| Rename & re-upload sebagai Module internal | Tidak bisa dicegah teknis — diterima sebagai batas struktural | Diterima, bukan "diselesaikan" |
| Reverse-engineering via AI (spec→dokumentasi→regenerasi) | Legal (clean-room), tak terhindarkan — jangan dijanjikan proteksi | Diterima, bukan "diselesaikan" |
| Proteksi riil yang tersisa | impl.compiled, data/network effect, maintenance flow, legal+reputasi Store | Final |
| Root-trust eksternal untuk verifikasi lisensi | Prasyarat struktural, desain detail deferred | Deferred |
| Alokasi taruhan bisnis jangka panjang | Geser fokus ke Forma Cloud + first-party vertical app; Registry Module komoditas diterima apa adanya | Diusulkan — perlu didiskusikan di level strategi/positioning |
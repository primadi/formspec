# Forma Technical Note: Kedaulatan Data — Pemisahan Data Owner, App/Module Owner, dan Global Customer ID sebagai Alasan Forma Harus Dibuat

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec**
**Status: bahan argumen inti untuk positioning & Investment Memorandum, belum committed ke spesifikasi teknis final.**

---

## 0. Latar Belakang

Sejauh ini Forma dijelaskan lewat argumen efisiensi teknis: konvensi mengurangi utang teknis, sembilan primitif infrastruktur menghilangkan kerja berulang, spec-first menjaga konsistensi di era AI coding. Semua itu benar, tapi argumen efisiensi mudah ditandingi kompetitor — siapa pun bisa klaim "framework kami juga lebih efisien."

Diskusi ini menemukan argumen yang lebih sulit ditandingi: **kedaulatan data**. Ini bukan soal seberapa cepat developer bekerja, tapi soal siapa yang secara struktural bisa dan tidak bisa melihat data bisnis seseorang — dan Forma adalah satu-satunya yang menjadikan batasan itu properti bawaan framework, bukan janji kontrak atau kebijakan perusahaan.

**Klaim inti dokumen ini:** di ekosistem software custom/SaaS yang ada sekarang, pemilik bisnis kecil dipaksa "telanjang" di hadapan pembuat aplikasinya — menyerahkan seluruh visibilitas omzet, pelanggan, dan pola operasional kepada satu pihak (developer/vendor) yang sering kali juga melayani kompetitor mereka. Forma membuat ini secara teknis tidak mungkin terjadi tanpa consent eksplisit, bukan sekadar tidak dianjurkan.

---

## 1. Masalah yang Ada Sekarang: Data Owner "Telanjang" di Hadapan App Owner

Bayangkan skenario yang sangat umum: satu developer/software house membangun aplikasi manajemen bengkel untuk banyak klien di kota yang sama — beberapa di antaranya saling bersaing.

Di dunia software custom biasa (dan sebagian besar SaaS vertikal kecil), developer itu **secara teknis** memegang akses ke server/database semua kliennya sekaligus. Konsekuensinya, terlepas dari niat baik developer:

- Dia tahu omzet bulanan Bengkel A dan Bengkel B, dan bisa membandingkan siapa lebih laris.
- Dia tahu komponen apa yang paling sering diminta di suatu wilayah — informasi yang punya nilai kompetitif nyata.
- Dia tahu pola pelanggan siapa yang paling loyal, kapan periode sepi, berapa margin per servis — hal-hal yang pemilik bengkel anggap rahasia dagang.

Ini bukan skenario hipotesis jahat. Ini konsekuensi wajar dari arsitektur "satu pihak memegang infrastruktur banyak klien sekaligus" — model yang berlaku di hampir semua software custom lokal dan sebagian besar SaaS vertikal kecil yang belum punya kematangan governance seperti enterprise besar.

**Akar masalahnya:** kepercayaan yang diberikan pemilik bisnis ke developer/vendornya *tidak dibatasi secara teknis* — batasnya murni norma sosial dan kontrak kerja yang bisa dilanggar tanpa jejak yang mudah diaudit.

---

## 2. Yang Sudah Dikunci di Forma yang Menjawab Ini

Forma sudah punya dua prinsip arsitektur keras yang, kalau dikombinasikan, langsung menjawab masalah di atas:

1. **Workspace sebagai satu-satunya model tenancy** — data bisnis satu Workspace tidak pernah bocor ke Workspace lain di level infrastruktur (lihat Technical Note Tiering Isolasi).
2. **Four-layer actor model** — Workspace Owner, App Owner, Module Owner, dan Cloud Owner masing-masing punya identitas akuntabel yang terpisah. App Owner (developer yang membangun aplikasi bengkel) secara arsitektur **bukan** pemegang kredensial infrastruktur Workspace kliennya — itu domain Workspace Owner/Cloud Owner.

Konsekuensi langsung: App Owner yang membangun aplikasi bengkel untuk Bengkel A dan Bengkel B **tidak otomatis** bisa membandingkan data keduanya, karena dia tidak pernah memegang akses lintas-Workspace secara default. Kalau dia butuh melihat data satu Workspace untuk keperluan dukungan teknis, itu harus lewat mekanisme consent eksplisit, time-boxed, dan tercatat di audit log — bukan akses permanen yang melekat begitu saja pada perannya sebagai pembuat aplikasi.

Ini konsisten dengan prinsip yang sudah berlaku di tingkat lebih tinggi: **Control Plane tidak pernah membaca data bisnis** — sekarang ditegaskan berlaku juga satu tingkat di bawahnya, antara App Owner dan Workspace Owner.

---

## 3. Ini Bukan Ide Baru — Tapi Belum Ada yang Menjadikannya Properti Framework Generik

Penting untuk jujur soal presedennya, supaya klaim ke investor tidak terkesan mengada-ada:

**Precedent yang sudah ada — sebagai kebijakan platform tunggal:**
- **Salesforce Login Access** — ISV (pembuat aplikasi pihak ketiga) harus meminta izin eksplisit dari pelanggan untuk masuk ke organisasi mereka. Pelanggan yang menentukan durasi akses, bisa mencabut kapan saja, dan ada pengaman platform yang mencegah ISV mengekspor data lewat sesi itu.
- **Shopify App Scopes** — aplikasi pihak ketiga harus meminta scope data spesifik yang disetujui merchant saat instalasi, dengan program tambahan ("Protected Customer Data") untuk data pelanggan sensitif.

**Bedanya dengan Forma:**

| | Salesforce/Shopify | Forma |
|---|---|---|
| Siapa menegakkan | Satu perusahaan platform, untuk ekosistemnya sendiri | Properti bawaan framework, berlaku di mana pun di-deploy (self-hosted, Forma Cloud, BYOI) |
| Sifat jaminan | Kebijakan/fitur admin — bisa saja dilewati kalau developer punya akses infrastruktur lain | Struktural — App Owner secara arsitektur tidak pernah pegang kredensial infrastruktur Workspace |
| Siapa yang dapat manfaat | Developer di ekosistem satu platform besar saja | Siapa pun App Owner, di ekosistem manapun mereka jual aplikasinya |

**Kesimpulan jujur:** prinsip "vendor tidak bisa lihat data pelanggan tanpa consent" bukan hal baru — sudah ada di SaaS besar. Yang belum ada presedennya adalah menjadikan ini **jaminan bawaan sebuah framework application-building generik**, bukan fitur satu platform tunggal.

---

## 4. Lapisan Tambahan: Global Customer ID (Forma ID)

Selain isolasi Workspace-ke-Workspace, diskusi terpisah menghasilkan konsep **Forma ID** — identitas pelanggan tunggal yang bisa dipakai lintas Workspace (mis. satu ID dipakai pelanggan yang sama di Bengkel A, Barbershop B, Klinik C), dengan model:

- Forma ID **tidak menyimpan data bisnis** — hanya identitas + ledger consent. Data tetap di Workspace masing-masing.
- Setiap akses lintas-Workspace butuh **consent eksplisit dari pelanggan**, bukan otomatis mengalir karena sama-sama pakai Forma ID.
- Presedennya: **Account Aggregator/DEPA di India** (consent-based data sharing, diregulasi RBI, sudah dipakai 250+ juta pengguna) untuk pola teknisnya, dan **coalition loyalty program** (Air Miles, Nectar) untuk pola insentif poin lintas-bisnisnya.
- Perbedaannya dari kedua presedan itu: keduanya beroperasi di atas platform terpusat tunggal. Forma ID beroperasi di atas **banyak Workspace independen**, dibangun oleh App Owner berbeda-beda, yang belum tentu saling kenal satu sama lain.

**Prinsip keras yang sudah disepakati untuk Forma ID (lihat diskusi Jalur A vs B):**

> Forma ID menghasilkan uang dari memfasilitasi transaksi yang di-consent (fee verifikasi, fee redemption poin) — **tidak pernah** dari menganalisis atau menjual data perilaku personal untuk iklan/profiling.

Ini bukan detail teknis kecil — ini keputusan yang menentukan Forma ID jadi produk kepercayaan (utility) atau produk data broker. Jalur kedua bertentangan langsung dengan seluruh brand Forma yang dibangun di atas auditability dan governance.

---

## 5. Kenapa Ini Alasan Kuat "Mengapa Forma Harus Dibuat"

Argumen efisiensi teknis (bagian 2 Investment Memorandum) menjawab pertanyaan *"kenapa developer harus pakai Forma dibanding menulis sendiri."* Argumen kedaulatan data ini menjawab pertanyaan yang lebih dalam dan lebih sulit ditandingi kompetitor: **"kenapa ekosistem software bisnis butuh Forma, bukan cuma developer individu."**

Pitch konkretnya ke pemilik bisnis kecil (end user tidak langsung Forma):

> "Developer yang sama boleh membangun aplikasi untuk kompetitor Anda, tapi dia tidak akan pernah bisa melihat data Anda tanpa izin eksplisit dari Anda — bukan karena dia berjanji, tapi karena sistemnya tidak mengizinkan."

Ini kalimat yang tidak bisa diucapkan jujur oleh kompetitor mana pun yang arsitekturnya tidak memisahkan App Owner dari Workspace Owner secara struktural — termasuk kebanyakan software house yang membangun custom solution untuk banyak klien di infrastruktur yang sama.

**Kenapa ini sulit ditiru:** kompetitor bisa menulis kebijakan "kami tidak akan melihat data Anda," tapi mengubah itu jadi jaminan struktural butuh membongkar ulang cara mereka membangun software — persis apa yang sudah dikerjakan Forma sejak spesifikasi awal (Workspace tenancy, four-layer actor model, Control Plane yang tidak baca data bisnis). Ini bukan fitur yang bisa ditambahkan belakangan; ini konsekuensi dari keputusan arsitektur yang sudah diambil sejak hari pertama.

---

## 6. Trade-off yang Harus Diterima Sadar

Supaya argumen ini tidak terdengar seperti solusi tanpa biaya — dua trade-off nyata:

**Kehilangan insight benchmark lintas-tenant secara default.** Fitur seperti "omzet Anda bulan ini di bawah rata-rata bengkel sejenis di kota Anda" butuh melihat data agregat banyak Workspace, dan itu justru yang diblokir arsitektur ini secara default. Solusinya bukan menghilangkan fitur ini selamanya, tapi menjadikannya **opt-in eksplisit** — mis. modul terpisah (`forma/benchmark-bengkel`) dengan consent granular, bukan diam-diam terkumpul karena developer kebetulan punya akses infrastruktur.

**Kompleksitas hukum Forma ID belum selesai bahkan di pasar paling maju.** Bahkan Account Aggregator India yang sudah berjalan sejak 2021 masih menghadapi ketegangan regulasi terbaru soal manajemen consent. Forma ID di Indonesia kemungkinan besar butuh pendampingan hukum serius sebelum diluncurkan — bukan sekadar keputusan desain teknis yang benar.

**Nilai argumen ini juga tidak seragam di semua segmen.** Paling kena untuk skenario "satu developer/software house, banyak klien vertikal yang saling bersaing" — persis model GTM yang sedang disusun Forma. Untuk bisnis tunggal tanpa kompetitor di ekosistem yang sama, argumennya tetap benar secara prinsip, tapi kurang jadi pemicu keputusan beli yang konkret.

---

## 7. Ancaman dari Dalam Forma Sendiri — Janji Institusi Saja Tidak Cukup

Semua yang dibahas di atas menjawab ancaman App Owner terhadap Workspace Owner. Tapi ada lapisan lain yang harus dijawab jujur: **bagaimana kita menjamin Forma sendiri — institusinya maupun pegawainya — tidak mengintip data Workspace yang dihosting di Forma Cloud?**

"Kami janji tidak akan mengintip" adalah janji kosong tanpa mekanisme teknis di baliknya — kalau operasional sehari-hari (backup, restore, migrasi skema, debugging insiden) tetap butuh manusia memegang akses mentah ke database, satu pegawai nakal cukup untuk membatalkan seluruh klaim kedaulatan data di atas. Ini bukan masalah unik Forma — ini masalah klasik semua cloud provider — tapi ada pola nyata dari industri untuk meminimalkannya, bukan sekadar berharap baik:

**a. Enkripsi dengan kunci yang dipegang pelanggan (Customer-Managed Key / BYOK).** Data dienkripsi at-rest dengan kunci yang dipegang Workspace Owner sendiri, bukan Forma. Pegawai Forma yang mengakses raw disk/backup untuk keperluan operasional hanya melihat ciphertext. Batasan jujur: tidak semua kolom bisa dienkripsi seperti ini — kolom yang dipakai `WHERE`/`JOIN`/`ORDER BY` harus tetap terbaca query engine — jadi ini realistis untuk field paling sensitif, bukan seluruh database.

**b. Split-key/quorum untuk kunci master.** Kunci master dipecah lewat skema *M-of-N* (mis. Shamir Secret Sharing) sehingga tidak ada satu pegawai atau satu HSM yang pegang kunci utuh — butuh beberapa orang sepakat untuk merekonstruksi. Pola yang sama dengan prinsip "no self-approval" yang sudah dikunci di Forma Control, diterapkan ke akses data.

**c. Break-glass access — bukan akses permanen.** Pegawai yang butuh menyentuh data pelanggan tidak punya akses baku. Mereka request akses time-boxed, disetujui pihak lain (bukan self-approval), otomatis kadaluarsa, dan tercatat ke log immutable/hash-chained yang sama seperti audit trail Control Plane. Syarat pentingnya: log ini harus **terlihat oleh Workspace Owner sendiri**, bukan cuma disimpan internal Forma untuk kepatuhan — kalau cuma Forma yang bisa lihat log-nya, itu tetap "percaya kami."

**d. Confidential computing (opsi lanjutan, tier tertinggi).** Teknologi seperti AMD SEV-SNP/Intel TDX memungkinkan data terenkripsi bahkan saat diproses di memori — operator infrastruktur secara teknis tidak bisa membaca isi memori mesin yang menjalankan workload klien. Masih mahal, belum jadi standar industri, tapi ini arah paling jauh menuju "zero-knowledge infrastructure operator."

**e. Kontrol organisasi (background check, NDA, separation of duties)** tetap perlu sebagai lapisan dasar, tapi tidak cukup sendirian — inilah kenapa (a)–(d) di atas wajib berjalan bersama, bukan alternatif satu sama lain.

**Kejujuran yang perlu ditegaskan:** "Forma secara teknis tidak mungkin mengintip" hanya benar penuh di tier Mahal/Enterprise/self-hosted (Workspace Owner pegang semua kunci). Di tier managed murah, Forma *bisa* secara teknis — tapi akses itu dibatasi break-glass + quorum + audit yang bisa diperiksa pelanggan sendiri. Ini argumen tambahan untuk struktur tiering yang sudah ada: bukan cuma beda harga karena beda infra, tapi beda **level jaminan kedaulatan data** — semakin mahal, semakin dekat ke "secara teknis tidak mungkin," bukan cuma "secara kebijakan dilarang."

---

## 8. Resep Konkret: Role Database yang Blackbox untuk Pegawai Internal

Pertanyaan konkretnya: bisakah pegawai Forma yang menjalankan tugas admin database (backup, migrasi skema) diberi akses yang secara teknis **tidak bisa** membaca isi tabel? Jawabannya berbeda per jenis tugas.

**Backup/restore — bisa 100% blackbox.** Backup/restore tidak butuh koneksi SQL sama sekali kalau dilakukan benar. Tools seperti `pg_basebackup`, WAL-G, atau pgBackRest bekerja di level file/WAL stream, bukan query — menyalin byte mentah dari data directory dan write-ahead log. Role yang dibutuhkan cukup atribut `REPLICATION`, **tanpa** privilege `SELECT`/`INSERT`/`UPDATE`/`DELETE` di tabel manapun. Secara teknis, tidak ada tombol untuk membaca isi tabel lewat jalur ini.

**Schema management (migrasi, index) — bisa dipisah, tapi butuh kedisiplinan konfigurasi.** Postgres punya granularitas privilege yang cukup tajam untuk membuat role dengan `CREATE`/`ALTER` di level schema tanpa pernah diberi hak baca/tulis data. Titik rawan: siapa pun yang punya `GRANT OPTION` atau `CREATEROLE` secara teknis bisa menaikkan privilege dirinya sendiri — jadi role DDL-only harus eksplisit `NOSUPERUSER`, tanpa `GRANT OPTION`, tanpa `CREATEROLE`, dan `REVOKE ALL` aktif untuk DML dari semua tabel (bukan cuma "tidak diberi").

**Yang tidak bisa diselesaikan di level role SQL saja:** siapa pun yang punya akses OS/filesystem ke mesin database (root server, akses hypervisor) bisa baca file data mentah di disk, di luar Postgres sama sekali. Superuser Postgres (kalau ada yang pegang) bisa bypass semua GRANT/REVOKE termasuk Row Level Security. Karena itu, **tidak ada pegawai individu yang boleh pegang kredensial superuser** di cluster multi-tenant — superuser hanya untuk automation terkontrol atau break-glass quorum. Inilah kenapa enkripsi at-rest dengan BYOK (bagian 7) tetap perlu berjalan bersama — role database menutup jalur SQL, enkripsi menutup jalur file mentah yang levelnya di bawah database.

**Resep peran (rujukan teknis):**

```
forma_ops_backup   → REPLICATION saja, tidak pernah login psql interaktif,
                      dipakai otomatis oleh WAL-G/pgBackRest
forma_ops_ddl      → CREATE, ALTER di level schema, NOSUPERUSER,
                      tanpa GRANT OPTION, tanpa CREATEROLE,
                      REVOKE ALL eksplisit untuk SELECT/INSERT/UPDATE/DELETE
                      dari semua tabel — termasuk default privilege baru
(tidak ada role manusia dengan SUPERUSER di cluster produksi multi-tenant)
```

Ditambah kebiasaan operasional: audit berkala (`\du`, `pg_roles`, `default_acl`) untuk memastikan tidak ada role yang "melebar" privilege-nya diam-diam — kesalahan konfigurasi kecil di sini yang biasanya jadi celah nyata, bukan kegagalan konsepnya.

---

## 9. Automasi Operasional Harian — Manusia Hanya Datang Saat Insiden

Selain membatasi *apa* yang bisa diakses manusia, cara lain memperkecil risiko adalah membuat manusia **jarang perlu** menyentuh infrastruktur sama sekali untuk tugas rutin. Ini bukan cuma soal efisiensi biaya — makin sedikit manusia rutin menyentuh infra, makin sedikit titik break-glass yang perlu dibuka, makin kuat klaim "akses manusia itu pengecualian, bukan kebiasaan."

**Bisa otomatis penuh — tidak butuh tangan manusia sama sekali:**

| Aktivitas | Mekanisme |
|---|---|
| Backup rutin | `forma backup create` terjadwal, pakai role `REPLICATION`-only (bagian 8) |
| Provisioning Workspace baru | API `forma-control`, bikin schema/DB, seed default role |
| Scaling naik/turun | KEDA berbasis metrik Redis yang sudah dicatat framework |
| Restart otomatis saat crash | Liveness/readiness probe K8s, circuit breaker yang sudah di-spec |
| Rotasi sertifikat TLS | ACME/Vault PKI terjadwal |
| Rotasi kunci non-master | KMS scheduled rotation |
| Arsip data lama | `forma archive run` terjadwal |
| Penegakan kuota | Sampling `pg_schema_size`, soft/hard-block otomatis |
| Patch OS/base image rutin | Rebuild image + rolling deploy via pipeline dengan test gate otomatis |

Semua ini berulang, berbasis aturan tetap, tidak butuh judgment baru setiap dijalankan — kandidat ideal untuk full automation.

**Sengaja tetap butuh manusia — bukan kekurangan, ini fitur keamanan:**

- **Approval break-glass access** — kalau diotomatisasi, seluruh mekanismenya jadi percuma. Poinnya justru ada gesekan manusiawi sebelum data pelanggan tersentuh.
- **Rekonstruksi kunci master (quorum M-of-N)** — sengaja butuh beberapa orang sepakat.
- **Approval artifact di governance tier tinggi ("no self-approval")** — butuh mata kedua yang independen dari yang menulis kode.
- **Insiden yang belum pernah terjadi** — begitu masuk kategori tidak diantisipasi automation, otomatis balik ke manusia.
- **Keputusan patch darurat untuk zero-day** — trade-off stabilitas vs urgensi butuh judgment.
- **Sengketa consent di Forma ID / investigasi fraud** — butuh manusia menimbang konteks.

---

## 10. Menambah Kapasitas Server — Otomatis vs Butuh Keputusan Manusia

**Menambah kapasitas di region/provider yang sudah ada — bisa full otomatis.** Cluster Autoscaler/Karpenter mendeteksi pod yang tidak bisa dijadwalkan karena resource habis, otomatis meminta VM baru ke cloud provider, node itu join cluster, KEDA menjadwalkan pod tenant ke node baru — tanpa manusia menyentuh proses dari ujung ke ujung. Node baru join cluster lewat `cloud-init`/bootstrap otomatis dengan kredensial dari Vault/KMS, bukan diketik manual — manusia tidak pernah SSH ke mesin barunya.

**Memperluas ke region/provider baru, atau dedicated hardware fisik — butuh gerbang keputusan manusia,** karena ini soal bisnis/hukum (data residency, kontrak provider, biaya), bukan lagi soal teknis. Begitu keputusan diambil, eksekusinya tetap bisa full Infrastructure-as-Code (Terraform/Pulumi) — tidak perlu manusia mengklik-klik console cloud provider. Hanya dedicated bare-metal fisik untuk klien tier Mahal yang benar-benar butuh tangan manusia memasang perangkat di rak data center — kasus yang seharusnya jarang karena kebanyakan kebutuhan tier Mahal cukup dedicated *virtual* instance.

**Pagar pengaman yang perlu ditambahkan:** auto-provisioning tanpa batas berisiko sendiri (bug loop scaling, atau tenant sengaja memicu beban palsu untuk scale-out besar-besaran). Perlu hard ceiling (batas maksimum node per cluster/region) plus alert ke manusia saat mendekati batas — bukan mematikan automation, tapi memberi rem darurat. Pola yang sama dengan "otomatis untuk rutin, manusia untuk anomali."

---

## 11. Menghilangkan Connection String Statis — Kredensial Dinamis, Bukan Janji Auto-Swap Engine Total

Ide yang muncul: apa tidak perlu ada connection string Postgres sama sekali — semua akses ke DB lewat service yang auditable, dan bahkan DB bisa diganti teknologi apapun di masa depan? Ini layak dipisah jadi dua klaim, karena satu realistis dan sangat bernilai, satu lagi perlu diperjelas batasnya.

**Yang realistis dan bernilai tinggi: kredensial dinamis, bukan connection string statis.** Masalah nyata dari `FORMA_DB_DSN` sebagai bootstrap config statis (Section 5.1 Core Basic Spec) adalah dia jadi *secret berumur panjang* yang duduk di env var/Kubernetes Secret — siapa pun dengan akses ke situ bisa memakainya kapan saja, tanpa jejak siapa yang memakainya, sampai ada yang sadar dan merotasinya manual. Solusi yang sudah matang di industri: **HashiCorp Vault Dynamic Database Credentials** (atau setara) — setiap kali `forma-server` butuh koneksi, dia minta kredensial ke Vault, Vault membuatkan role Postgres baru yang unik dan bertenggat waktu (mis. berlaku 1 jam), lalu otomatis mencabutnya setelah TTL habis. Setiap penerbitan kredensial tercatat: siapa/apa yang minta, kapan, untuk tenant/schema mana.

**Ini mencapai tujuan auditability yang diminta tanpa mengorbankan performa** yang sudah jadi prinsip desain (Location transparency: "same process — direct function call, zero network" untuk resource call, dan `ctx.db` dirancang zero-overhead). `forma-server` tetap konek langsung ke Postgres untuk tiap query — yang berubah cuma cara dia *mendapatkan* kredensial, bukan menambah hop jaringan di jalur query itu sendiri.

**Yang perlu diperjelas batasnya: "DB bisa diganti teknologi apapun di masa depan."** Ini ambisi yang mahal untuk dipenuhi penuh, dan perlu jujur soal trade-off-nya:

- Business logic developer **sudah** engine-agnostic di level yang benar — mereka selalu lewat `ctx.db`, tidak pernah bicara langsung ke driver Postgres. Ini bagian yang memang sudah portable by design.
- Tapi banyak keputusan operasional yang sudah dikunci di seluruh diskusi kita **spesifik Postgres**: `pg_schema_size` untuk kuota (bagian 7 Technical Note Tiering), schema-per-tenant sebagai tier Murah, Row Level Security sebagai opsi, index-check via `EXPLAIN`, hybrid JSONB persist model. Mengganti engine berarti membangun ulang seluruh lapisan operasional ini dari nol untuk engine baru — bukan sekadar ganti connection string.
- Menjanjikan "swap kapan saja tanpa biaya" ke investor/pelanggan berisiko jadi klaim yang tidak bisa ditepati — lebih jujur untuk bilang: **lapisan bisnis sudah portable, lapisan operasional terikat Postgres by design saat ini, dan itu keputusan sadar** (Postgres dipilih karena fitur native seperti schema, JSONB, RLS yang justru jadi dasar banyak mekanisme keamanan/kuota di dokumen ini) — bukan keterbatasan yang perlu diselesaikan.

**Ringkas:** kredensial dinamis (Vault) — ya, ini harus masuk sebagai standar, dan memperkuat cerita audit di bagian 7–8. Engine-agnostic penuh — bukan prioritas sekarang, dan mengklaimnya sebagai fitur siap pakai akan menambah utang teknis nyata untuk manfaat yang belum jelas nilainya di tahap ini.

---

## 12. Kesimpulan: Kenapa Forma Cloud Harus Tetap Ada

Semua pembahasan di atas mengarah ke satu kesimpulan yang membalik logika lama industri hosting: Forma Cloud bukan sekadar opsi kenyamanan dibanding self-hosting — untuk sebagian besar segmen, Forma Cloud adalah **satu-satunya jalan** menuju kedaulatan data yang sebenarnya.

**Klien kecil (bengkel, klinik, barbershop) tidak punya tim infra sendiri.** Kalau App Owner menawarkan "biar saya self-hosting-kan," itu secara default berarti App Owner yang pegang kredensial infrastruktur — persis skenario yang dibongkar di bagian 1 (App Owner bisa intip omzet lintas kliennya yang saling bersaing). Bagi klien kecil, "self-hosted" bukan berarti "saya kontrol data saya" — itu ilusi, karena mereka tidak punya kapasitas mengoperasikan server sendiri. Yang benar-benar terjadi: App Owner yang kontrol, klien cuma pakai. Forma Cloud, dengan jaminan struktural di bagian 2 (App Owner tidak pernah pegang kredensial Workspace), justru satu-satunya opsi yang benar-benar memisahkan App Owner dari akses data.

**Klien besar yang punya tim infra sendiri** menghadapi pertanyaan berbeda: bukan "bisa atau tidak self-host," tapi "siapa yang track record disiplin operasionalnya lebih bisa dipercaya — tim internal yang ad hoc dengan banyak prioritas lain, atau operator spesialis yang hidup-matinya bisnis bergantung penuh pada tidak pernah gagal?" Ini pola yang sama dengan kenapa perusahaan besar mempercayakan keamanan ke MSSP spesialis dibanding SOC internal — bukan soal kemampuan teknis, tapi soal konsistensi disiplin proses (separation of duties, audit rutin, break-glass yang benar-benar ditegakkan) yang lebih mudah dijaga oleh pihak yang seluruh reputasinya taruhannya di situ.

**Konsekuensi yang harus diakui, bukan cuma dirayakan:** begitu "kepercayaan" jadi alasan utama orang pakai Forma Cloud — bukan cuma harga/kenyamanan — **satu insiden kebocoran data membatalkan seluruh proposisi nilai**, bukan cuma kehilangan satu pelanggan seperti downtime biasa. Ini beda dari kegagalan produk SaaS biasa yang bisa dipulihkan lewat perbaikan fitur.

**Implikasi konkret:**

1. **Klaim kepercayaan tidak boleh cuma pernyataan.** Harus independently verifiable: sertifikasi pihak ketiga (SOC 2/ISO 27001 begitu skala cukup besar), transparency log yang bisa diaudit pelanggan sendiri (bagian 7), dan idealnya laporan insiden publik kalau pernah ada masalah — kejujuran soal kegagalan memperkuat kredibilitas jangka panjang dibanding menyembunyikannya.
2. **Di tahap awal, Forma belum punya reputasi untuk dijual.** Pitch "reputasi kami terbaik" tidak bisa dipakai di M1–M4 karena belum ada rekam jejak. Yang bisa dijual di tahap awal adalah jaminan arsitektur yang bisa diverifikasi hari ini (spec CC0 terbuka, mekanisme audit yang bisa diperiksa siapa saja) — reputasi menyusul lewat rekam jejak nyata, bukan diklaim di depan. Pilot enterprise pertama (M4, lihat Investment Memorandum) jadi krusial bukan cuma untuk validasi revenue, tapi untuk mulai membangun rekam jejak kepercayaan itu sendiri.
3. **Automasi operasional (bagian 9–10) bukan cuma soal margin.** Makin sedikit manusia menyentuh infra secara rutin, makin kecil peluang human error jadi penyebab insiden yang bisa menghancurkan reputasi. Automasi dan kedaulatan data mendukung tujuan yang sama.

---

## 13. Pertanyaan Terbuka

- Bagaimana mekanisme consent App Owner-ke-Workspace Owner ini secara teknis diimplementasikan — apakah lewat `forma-control` (karena sifatnya governance), atau primitif terpisah di level Resource Plane?
- Apakah "dukungan teknis App Owner ke Workspace" butuh dirancang sebagai fitur resmi Forma (mirip Salesforce Login Access: time-boxed, dicatat pelanggan sendiri durasinya, auditable), atau cukup jadi guideline yang diserahkan ke masing-masing App Owner membangun sendiri di atas primitif yang ada?
- Modul benchmark opt-in (bagian 6) — siapa yang membangun dan mengoperasikannya? Apakah ini business service biasa di Resource Plane (developer bisa bangun sendiri), atau layanan first-party Forma yang lebih dipercaya karena posisinya netral?
- Perlu diputuskan: apakah argumen kedaulatan data ini masuk sebagai bagian Diferensiasi (bagian 6 Memorandum) yang berdiri sendiri, atau dijalin ke Visi & Misi (bagian 9) sebagai bagian dari "mengapa Forma harus ada," mengingat sifatnya lebih dekat ke pernyataan nilai daripada fitur teknis.
- Siapa yang mengoperasikan Vault/KMS untuk kredensial dinamis (bagian 11) — apakah ini bagian `forma-control`, atau komponen infra terpisah yang jadi bagian tawaran Forma Cloud saja (tidak relevan untuk self-hosted murni)?
- Hard ceiling auto-provisioning (bagian 10) — berapa angka realistis untuk tahap awal, dan siapa yang berwenang menaikkannya saat bisnis tumbuh?
- Perlu diputuskan format sertifikasi/audit pihak ketiga mana yang jadi prioritas pertama (SOC 2 Type II vs ISO 27001) dan di milestone mana (M4? M5?) ini realistis mulai dikejar, mengingat biaya dan waktu audit yang tidak kecil untuk tim kecil.

---

*Dokumen ini adalah rangkuman kerja dari sesi diskusi. Tujuannya menyimpan alur penalaran dan argumen inti agar tidak hilang. Bukan keputusan final — perlu direview sebelum masuk sebagai revisi resmi Investment Memorandum.*

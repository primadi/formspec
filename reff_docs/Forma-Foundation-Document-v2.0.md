# Forma Foundation Document

**Versi:** 2.0
**Status:** D1–D46 Final; D47–D48 diusulkan (Plane Protocol v0.1.0 + Core Extended v0.2.0 — menunggu review)
**Fungsi dokumen:** Titik awal dan acuan tertinggi proyek Forma. Semua dokumen lain (core spec, technical note, memorandum) diturunkan dari atau harus konsisten dengan dokumen ini. Dokumen-dokumen sebelum tanggal dokumen ini berstatus **arsip/referensi** — berisi jejak penalaran, bukan acuan.

---

## 1. Visi

**Forma adalah ekosistem lengkap untuk mempermudah pembuatan aplikasi bisnis menggunakan Go.**

Forma bukan tiruan proyek mana pun — Forma adalah **sintesis hal-hal terbaik dari banyak proyek**, masing-masing diambil dengan alasan eksplisit (bagian 2): Frappe (entity & workflow deklaratif), PocketBase (satu definisi → banyak permukaan), Dapr (pola kontrak infra), OPA (governance), Laravel (kelengkapan ekosistem & model bisnis), Kubernetes (format deklaratif seragam).

Konteksnya: ekosistem Go hari ini mirip PHP sebelum Laravel — bahasanya cepat dan hemat resource, tapi membangun aplikasi bisnis berarti merakit library terpisah tanpa konvensi bersama. Forma mengisi kekosongan itu. Frasa *"Laravel-nya Go"* boleh dipakai sebagai jembatan komunikasi (terutama ke developer PHP/Laravel yang mencari jalan ke Go), tapi **bukan definisi scope** — Forma melangkah lebih jauh: spec-first, governance control plane, scripting runtime, dan format deklaratif menyeluruh.

Tiga karakter yang membedakan Forma:

1. **Spec-first.** Forma adalah open standard (CC0). Implementasi resmi dibangun di atas spec, bukan sebaliknya.
2. **Deklaratif di pusatnya.** Satu definisi resource menjadi sumber kebenaran untuk API, admin panel, frontend, dokumentasi, tipe data, validasi, state machine, dan permission.
3. **Dirancang untuk era AI-assisted development.** Struktur deklaratif + konvensi ketat + hanya tiga jenis file menjadikan Forma pagar pembatas bagi AI coding assistant: hasil generate konsisten, mudah direview, sulit melewatkan hal kritis.

Prinsip pragmatis: **tetap sederhana, manfaatkan open source yang sudah matang; kalau tidak ada yang pas, bangun sendiri.**

### 1.1 Apa yang dimaksud "aplikasi bisnis" — dan pain yang diselesaikan

Aplikasi bisnis Forma = sistem transaksional multi-user dengan aturan domain: POS multi-cabang, inventory, billing/invoicing, klinik, sekolah, HRM, order management. **Contoh kanonik: Order-to-Cash** (order → reservasi stok → pembayaran → invoice → pengiriman), dipakai di semua dokumen publik dengan narasi dua kolom *tanpa Forma vs dengan Forma*.

Skenario ini dipilih karena secara alami memunculkan semua pain yang biasanya baru disadari developer setelah kejadian di production:

| Pain nyata | Tanpa ekosistem | Di Forma |
|---|---|---|
| Dua kasir mengurangi stok bersamaan (race condition) | Developer harus sadar sendiri, pasang locking manual | `ctx.lock` — konvensi eksplisit |
| Webhook pembayaran dikirim 2× → invoice dobel | Idempotency sering terlupakan | Idempotency key by convention |
| Nomor invoice urut di bawah konkurensi | SELECT MAX+1 yang salah | Sequence/lock terkelola |
| Event "order paid" hilang saat crash | Butuh outbox pattern, jarang yang benar | Reliable events bawaan |
| Deploy versi baru + migrasi schema | Downtime, skrip manual | Artifact tertandatangani + policy Control Plane |



---

## 2. Enam Sumber Pembelajaran

Forma belajar dari enam proyek, masing-masing dengan pelajaran spesifik dan keputusan adopsi yang eksplisit.

### 2.1 Frappe — cara mendefinisikan business entity, workflow, dan modul

**Yang diambil:**
- DocType → resource `type: entity`: definisi deklaratif berisi fields, state machine, actions, events, permission.
- Business module sebagai unit pengelompokan (`modules/billing/`, `modules/inventory/`) dan unit distribusi (module registry).
- Modul vertikal siap pakai (accounting, HRM, inventory, dst.) sebagai bagian dari ekosistem, bukan sekadar contoh.

**Yang juga diadopsi — spesifikasi detail menyusul:**
- Metadata layout form di definisi entity (urutan field, section, kolom) — UI hints untuk admin panel.
- Print format / dokumen cetak deklaratif.
- Approval workflow deklaratif di level state machine ("transisi X butuh approval role Y") — melengkapi guard conditions yang sudah ada.

### 2.2 PocketBase — dari definisi data langsung jadi CRUD API, form, dan auth

**Yang diambil:**
- Prinsip "One Definition, Many Protocols": dari satu resource definition otomatis lahir endpoint HTTP, WebSocket handler, admin panel UI, dokumentasi API, dan tipe data hasil generate.
- Auth wajib by default; akses anonim harus dideklarasikan eksplisit.
- Standar DX: `forma dev` satu perintah, semuanya langsung jalan.

**Yang juga diadopsi — spesifikasi detail menyusul:**
- Konvensi realtime subscription per-entity/per-record otomatis (di atas primitif `ctx.pubsub` + WebSocket yang sudah ada).
- Admin panel instan tanpa setup sebagai tolok ukur DX yang mengikat, bukan sekadar aspirasi.

### 2.3 Dapr — pola infrastruktur, BUKAN dependency runtime

**Keputusan (final):** Dapr adalah **referensi pola, bukan infra di balik Forma.**

Alasan:
- Enam primitif `ctx.*` Forma adalah closed set yang menjadi titik penegakan tenant isolation, permission, dan Signed Query Registry. Building block Dapr tidak tenant-aware — memakainya berarti Forma tetap harus membungkusnya lagi: lapisan ganda tanpa nilai tambah.
- Dapr mewajibkan sidecar `daprd` per proses — merusak cerita single-binary dan `forma dev` yang ringan.
- Registry, load balancing `tenant_affinity`, dan circuit breaker Forma punya semantik yang tidak disediakan Dapr apa adanya.

**Yang tetap diambil dari Dapr:**
- Pola kontrak sidecar HTTP/gRPC untuk polyglot business logic (impl type `sidecar`).
- Component model sebagai inspirasi: backend di balik primitif bisa di-swap (Valkey ↔ Redis ↔ NATS) tanpa mengubah kontrak `ctx.*`.

### 2.4 OPA — governance di Control Plane

**Keputusan (diusulkan, perlu konfirmasi):** OPA/Rego di-embed sebagai **policy engine Control Plane**.

- OPA pure Go → masuk ke binary `forma-control` sebagai library, tanpa proses tambahan — konsisten filosofi single-binary.
- Rego mengekspresikan aturan governance deklaratif dan auditable: "artifact ke production wajib 2 approver dari tim berbeda", "tidak ada deploy Jumat malam", aturan rotasi key, dst.
- **Batas tegas:** OPA hanya untuk governance Control Plane (deployment, promotion, approval, key management). Authorization data bisnis di Resource Plane tetap domain `required_permission` di resource definition. Tidak boleh ada dua sistem permission yang tumpang tindih.

---

### 2.5 Laravel — kelengkapan ekosistem, DX, dan model bisnis

Laravel adalah **inspirasi, bukan cetakan**. Yang ditiru, dengan alasan eksplisit:

- **Kelengkapan ekosistem resmi** (Horizon, Pulse, Filament, Forge, Cashier) — bukti bahwa framework menang karena lapisan di sekelilingnya, bukan runtime-nya saja. Padanan Forma: `forma.observe`, admin panel, registry, Forma Cloud, agent skill.
- **Standar DX:** satu perintah untuk mulai, convention over configuration, dokumentasi kelas dunia.
- **Model bisnis terbukti:** kode terbuka + layanan berbayar di sekelilingnya.
- **Metode kerja:** peta "Laravel → Forma" (Appendix A) dipakai sebagai *checklist kelengkapan* — setiap fitur harian Laravel wajib punya jawaban: ada padanannya / sengaja beda (dengan alasan) / gap yang diakui. Exercise ini sudah menghasilkan D12–D13.

Yang **tidak** ditiru: framework-first (Forma spec-first), ORM (Forma anti-ORM), template server-side (Forma: API + frontend deklaratif), ketiadaan governance layer.

### 2.6 Kubernetes — format deklaratif seragam (`kind`)

**Yang diambil:**
- Pola `apiVersion` + `kind` + `metadata` + `spec`: **semua** konsep Forma — entity, service, config, module, page, form, dashboard, menu — dideskripsikan dalam format YAML yang sama, bisa dipecah ke banyak file/folder per concern, dan diproses tooling generik (validasi, diff, `forma apply`, GitOps).
- Disiplin *desired state*: YAML mendeskripsikan keadaan yang diinginkan, bukan logika imperatif.
- Konsekuensi radikal: **seluruh proyek Forma hanya berisi tiga jenis file — `yaml` (deskripsi), `script` (logika), `asset` (statis/bundle).**

**Batas yang dijaga (pelajaran dari kegagalan "UI dalam YAML" di tempat lain):** kind frontend hanya untuk UI berpola (Page, Form, Table, Dashboard, Menu — mencakup ±80% UI aplikasi bisnis). UI arbitrer tidak dipaksakan ke YAML — escape hatch resmi: custom component sebagai `asset` yang direferensikan dari YAML. YAML tidak boleh berevolusi menjadi bahasa pemrograman.

---

## 3. Prinsip Inti

1. **Everything is a Resource.** Semua konsep aplikasi bisnis dimodelkan sebagai resource: `entity` (stateful, CRUD, state machine) atau `service` (stateless, pure computation). Integrasi eksternal wajib dibungkus service.
2. **One Definition, Many Protocols.** Resource YAML adalah single source of truth untuk semua permukaan (API, UI, docs, types).
3. **Convention over Configuration.** Default masuk akal untuk semuanya; override hanya yang perlu.
4. **Security by Default.** Auth wajib, tenant isolation otomatis dan tidak bisa di-bypass, tidak ada permission implisit, cross-tenant access mengembalikan 404 (bukan 403).
5. **Location Transparency.** Pemanggil tidak pernah perlu tahu di mana resource berjalan — registry yang menyelesaikan.
6. **Closed set of primitives.** Akses infrastruktur hanya lewat enam primitif: `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage` (+ `ctx.config`, `ctx.kvstore`, `ctx.log` sebagai pendukung). User tidak boleh mendefinisikan infrastructure service sendiri.
7. **Kontrak ditulis sebelum implementasi.** Urutan kerja selalu: resource YAML dulu (fields, state machine, actions, events, permissions), baru `impl`.
8. **Satu format untuk semua (`kind`).** Backend, frontend, config, modul — semuanya YAML `apiVersion/kind/metadata/spec`, bisa dipecah per folder/file sesuai concern.
9. **Tiga jenis file.** Proyek Forma hanya berisi `yaml` (deskripsi), `script` (logika), `asset` (statis/custom component). Tidak ada jenis keempat.

---

## 4. Arsitektur: Dua Plane

### 4.1 Forma Control Plane (`forma-control`)

Governance dan keamanan: definisi environment, deployment policy (dievaluasi via OPA/Rego), signing & key management (HSM/Vault/KMS), approval workflow dengan no-self-approval, audit store immutable hash-chained, emergency control (`forma-ctl freeze`, revoke sessions, key rotate).

**Larangan mutlak:** Control Plane tidak pernah membaca data bisnis tenant, tidak mengeksekusi business handler.

Compute-nya stateless (bebas direplikasi), state didelegasikan ke storage terpisah (Postgres schema `forma_control`).

### 4.2 Forma Resource Plane (`forma-resource`)

Eksekusi resource: CRUD, actions, validasi, state machine. Akses data via enam primitif tertutup. Permukaan protokol (HTTP, WebSocket, docs) diturunkan otomatis dari resource definition. Registrasi ke registry (Valkey), load balancing (round_robin / least_connections / latency_aware / tenant_affinity), circuit breaker per instance.

**Relasi antar plane:** Resource Plane membaca policy saat boot + refresh tiap 5 menit via mTLS. **Tidak pernah menulis balik ke Control Plane.** Satu Control Plane mengelola banyak Resource Plane (per environment, per modul). Resource Plane saling memanggil lewat registry + location transparency; lintas mesin memakai gRPC (sync) dan NATS (pub/sub, request-reply).

### 4.3 Resolusi inkonsistensi "Single-Binary"

Spec lama dan Investment Memorandum bertentangan (proses terpisah vs satu proses). **Keputusan (diusulkan):**

- **Dua proses selalu**, bahkan di development — developer terbiasa arsitektur dua-binary sejak hari pertama.
- Istilah tier gratis **"Single-Binary" diganti** (misal: "Forma Standalone") karena secara harfiah menyesatkan — yang dimaksud adalah *self-contained deployment tanpa dependency cloud*, bukan satu proses.

---

## 5. Model Eksekusi Business Logic — Lima Impl Type

| Impl | Bentuk | Sandbox | Update tanpa redeploy | Catatan |
|---|---|---|---|---|
| `native` | Fused single binary Go | Tidak (trust penuh) | Tidak | Bisa di-build lokal (konsekuensi lisensi FSL, D8) |
| `compiled` | Go plugin `.so` / WASM | Parsial (WASM) | Ya (load runtime) | Bisa dibangun lokal |
| `script` | Starlark inline | Ya (no network/fs, limit memory & waktu) | Ya | Prototipe / rule kecil |
| `script_ref` | Starlark tersimpan di DB | Ya | Ya — editable dari admin panel, berversi, bisa rollback | Business rule yang sering berubah |
| `sidecar` | Container bahasa lain (PHP/Python/Node/Java) via Unix socket | Container terpisah, trust = native | Ya (deploy container) | Demi ekosistem bahasa, bukan bahasanya |

Aturan pemilihan: logic sering berubah → `script_ref`; stabil & performa kritis → `native`; butuh ekosistem bahasa lain → `sidecar`.

Trust model sidecar **sama dengan native** — proteksi (proxy identitas Redis, Signed Query Registry untuk raw SQL) berlaku sama terlepas bahasa.

---

## 6. Tenancy: Workspace — Satu-satunya Model (D29)

Multi-tenancy di Forma hanya punya **satu** bentuk: **Workspace**.

```
Workspace (milik Data Owner — data, compute, billing, identitas tenant)
  └── App (unit deployment & trust boundary; publish/grant antar app)
        └── Module (packaging kode — tanpa tenant, tanpa data)
              └── Resource (Entity, Service, Workflow, ...)
```

Aturan kepemilikan tunggal (D27): **data selalu milik pemilik workspace
tempat resource berjalan.** Modul terpasang di workspace Anda → datanya milik
Anda; vendor hanya menjual kode + update. Provider app berjalan di workspace
vendor → vendor adalah Data Owner atas datanya sendiri; konsumen hanya
memegang grant ke interface.

Konsekuensi radikal: **aplikasi ditulis 100% buta-tenancy.** Tidak ada
`FORMA_TENANCY`, tidak ada mode single/multi, tidak ada kode tenant di app.
`tenant_id` tinggal sebagai mekanisme isolasi internal runtime yang dikunci
ke workspace. Memasang beberapa app ke satu workspace menyamakan identitas
tenant lintas app — dasar cross-app grant.

Tenant besar yang ingin mengelola server sendiri tidak mendapat "mode lain":
dia membeli **enterprise license dan menjalankan Forma Cloud-nya sendiri** —
menjadi Platform Operator bagi dirinya. Amandemen: model distribusi ganda
di D21 menyempit — jalur "self-managed tenancy di dalam app" dihapus;
istilah sub-tenant (D7) tidak lagi dipakai. Batas D7 yang tetap hidup:
Control Plane mengelola *lifecycle* workspace (provisioning, billing,
consent) tanpa pernah membaca *isi* datanya.

---

## 7. Bentuk Deployment — Empat Sumbu Independen

1. **Siapa menjalankan Control Plane:** Standalone gratis · Forma Control Cloud · Forma Control Enterprise (self-hosted berbayar).
2. **Siapa menjalankan compute Resource Plane:** Forma Cloud fully-managed · Forma Cloud BYOI · self-hosted murni.
3. **Orkestrasi:** Standalone (Docker/bare-metal, `forma scale` manual) · K8s-aware · K8s-native (CRD, GitOps, KEDA).
4. **Bentuk eksekusi:** lima impl type di bagian 5.

Development selalu satu bentuk: `forma dev` → Docker Compose (Postgres, Valkey, Mailpit, MinIO, `forma-control` policy-relaxed, `forma-resource` hot-reload; container bahasa ditambahkan otomatis untuk project polyglot).

**Syarat keras:** Forma harus bisa jalan di cloud provider mana pun, termasuk self-hosted penuh.

---

## 8. Ekosistem & Model Bisnis (Ringkas)

- **Spec:** open standard, CC0, netral vendor.
- **Implementasi** (`forma-resource`, `forma-control`): **FSL sejak hari pertama** — source terbuka, bebas dipakai membangun aplikasi komersial apa pun; larangan tunggal: menjual Forma sebagai managed service kompetitor; tiap versi otomatis Apache 2.0 setelah 2 tahun. Konsekuensi penting: `impl.native` bisa di-build lokal siapa saja — **Compile Service tidak diperlukan lagi**.
- **Lisensi fitur enterprise:** license token kriptografis, divalidasi lokal, tidak wajib call-home (mendukung air-gapped) — meng-gate fitur governance (HSM signing, audit immutable, SSO/SCIM), bukan menyembunyikan kode.
- **Pertahanan struktural:** trademark "Forma" terdaftar; registry resmi sebagai hub ekosistem (fork kehilangan brand + marketplace + trust chain).
- **Modul resmi standar (bukan primitif):** `forma/scheduler` (cron/scheduled jobs), `forma/mail` (di atas `ctx.queue`), `forma/notify` (di atas `ctx.pubsub` — extensible ke WA, SMS, push), `forma/seed` (seeder & factory untuk dev/testing). Prinsipnya: primitif hanya hal benar-benar dasar; kebutuhan umum dibangun sebagai modul resmi di atas primitif, sehingga bisa berevolusi tanpa menyentuh closed set `ctx.*`.
- **REPL (`forma repl`):** konsol Starlark interaktif dengan akses `ctx.*` — alat debugging first-class, sekaligus permukaan untuk Forma Agent Skill (AI mencari bug jauh lebih cepat lewat REPL daripada menebak dari kode).
- **Ekosistem resmi:** module registry + Verified Badge, modul vertikal (accounting, HRM, inventory, POS, klinik, sekolah, ...), modul locale (`forma/locale-id`: PPN, e-Faktur, BPJS, ...), mockup system untuk integrasi pihak ketiga, tools migrasi (Laravel/Rails/Django/Postgres → resource YAML), `forma.observe` (padanan Pulse/Horizon), **Forma Agent Skill** untuk AI coding assistant.
- **Tiering storage** (Forma Cloud): shared schema (passthrough `ctx.db` dimatikan) · database logis terpisah · instance terpisah.

---

## 9. Decisions Log

| # | Keputusan | Status |
|---|---|---|
| D1 | Dapr = referensi pola (kontrak sidecar, component model), bukan dependency runtime | **Final** |
| D2 | OPA/Rego embedded sebagai policy engine Control Plane, terbatas pada governance (bukan authorization data bisnis) | **Final** |
| D3 | Dua proses (`forma-control` + `forma-resource`) selalu, termasuk dev; tier "Single-Binary" di-rename menjadi "Forma Standalone" | **Final** |
| D4 | Nama plane: Control Plane & Resource Plane; binary: `forma-control` & `forma-resource` | Final |
| D5 | Enam primitif `ctx.*` sebagai closed set; user tidak boleh definisikan infrastructure service | Final |
| D6 | Trust model sidecar = native; proteksi berlaku sama semua bahasa | Final |
| D7 | Sub-tenant dikelola sebagai modul bisnis (`forma/tenant-billing`) di Resource Plane, bukan di Control Plane | Final |
| D8 | **Lisensi (revisi):** spec CC0 · `forma-resource` & `forma-control` di bawah FSL (Functional Source License — source terbuka, bebas dipakai membangun aplikasi apa pun, dilarang menjual Forma sebagai managed service kompetitor; tiap versi otomatis jadi Apache 2.0 setelah 2 tahun) · fitur governance enterprise di-gate license token. Dipasang sejak hari pertama — tidak ada relicense di tengah jalan. Posisi komunikasi: "fair source", bukan klaim "open source" OSI | **Final** |
| D9 | Bahasa dokumen kerja: Indonesia (personal project); spec publik: Inggris | Final |
| D10 | Fitur adopsi Frappe (form layout hints, print format, approval-in-state-machine) dan PocketBase (realtime subscription convention, admin panel instan) masuk scope inti, bukan backlog opsional | **Final** |
| D11 | **Pertahanan struktural non-lisensi:** trademark "Forma" didaftarkan sejak awal; module registry + Verified Badge (trust chain ed25519) dioperasikan sebagai hub resmi terpusat; Control Plane governance & modul locale hidup (`forma/locale-id`) sebagai moat layanan yang tidak bisa di-fork | **Final** |
| D12 | Scheduler, mail, notification, seeder/factory = **modul resmi** di atas primitif (mail→`ctx.queue`, notif→`ctx.pubsub`), bukan primitif baru; closed set `ctx.*` tetap enam | **Final** |
| D13 | REPL (`forma repl`, berbasis Starlark + akses `ctx.*`) adalah fitur first-class, termasuk sebagai permukaan Forma Agent Skill untuk AI debugging | **Final** |
| D14 | Frontend dideskripsikan sebagai spec YAML dengan pola `kind` ala K8s; seluruh proyek hanya tiga jenis file: `yaml`, `script`, `asset`; kind frontend terbatas UI berpola, custom UI via escape hatch `asset` | **Final** |
| D15 | Positioning: Laravel (dan lainnya) = sumber inspirasi dengan alasan eksplisit, bukan batas scope; Forma = sintesis enam sumber; "Laravel-nya Go" hanya jembatan komunikasi | **Final** |
| D16 | Contoh kanonik: **Order-to-Cash** (companion lama ditulis ulang mengikuti spec baru), format narasi dua kolom "tanpa Forma vs dengan Forma", dipakai di semua dokumen publik | **Final** |
| D17 | **Peta concern → kind** (Appendix B): katalog kind ditentukan per concern, dengan prinsip *derived by default* — CRUD API, admin panel, docs lahir otomatis dari Entity; kind exposure/UI (`Api`, `Form`, dst.) hanya untuk override | **Final** |
| D18 | **Kind extensible tiga lapis**: bawaan spec (`Module, Entity, Service, Config, Migration`) → modul resmi (`Seed, Schedule, MailTemplate, ...` via `KindDefinition`, ala CRD) → modul pihak ketiga (ter-namespace, tunduk Verified Badge). Guardrail: developer aplikasi hampir tidak pernah membuat kind — butuh kind baru = memperluas framework; 95% kasus jawabannya `Entity` | **Final** |
| D19 | **Desain `kind: Module`**: `metadata.name` = identitas sekaligus namespace permission (tanpa alias); vendor path untuk registry/dependency; isi modul ditemukan via scan (tidak didaftar); config default level modul; **tanpa** blok permission tambahan | **Final** |
| D20 | **Model permission eksplisit untuk semua impl type**: setiap action mendeklarasikan `required_permission` (guard pemanggil) dan `uses` (akses kode: db tier, resource lain, primitif). Grant tidak pernah diturunkan dari pemakaian. Runtime enforcement via identity proxy untuk semua type; auto-scan script hanya sebagai verifikator kejujuran (`forma validate`); footprint modul = agregat deklarasi, ditampilkan sebagai consent saat install; `ctx.auth.has()` hanya boleh mereferensikan permission yang terdeklarasi | **Final** |
| D21 | **Dua model distribusi aplikasi**: (A) self-managed tenancy — developer kelola multi-tenant sendiri, wajib tetap didukung (kredibilitas FSL, kasus density ekstrem); (B) **platform-managed tenancy — jalur default yang direkomendasikan**: app ditulis buta-tenancy, Forma Cloud memutuskan topologi (instance terisolasi atau pooled via mesin `tenant_id` yang sama). Pemisahan role: **App Owner** (artifact, versi, metrics tersanitasi — tanpa akses data tenant) vs **Data Owner** (users, data, backup, consent footprint per D20, re-consent saat footprint berubah). Support access = impersonation grant dari Data Owner, time-boxed, teraudit | **Final** |
| D22 | **Marketplace + revenue sharing** sebagai pilar ekosistem: dependency graph deklaratif (`depends`) = basis metering pemakaian modul lintas owner. **Syarat komersial (harga, bagi hasil) hidup di registry, bukan di manifest** — manifest tetap murni teknis; modul yang sama bisa gratis self-hosted dan berbayar di marketplace tanpa perubahan YAML. Menjawab sebagian Q6 (tier ISV/Platform Partner) | **Final** |
| D23 | **Tenancy di manifest = semantik saja, strategi keluar total.** Setiap Entity **tenant-isolated by default, tanpa kecuali dan tanpa derivasi implisit**. Global/reference adalah pengecualian eksplisit: `tenant.isolated: false` hanya valid jika `characteristics: [reference]` (divalidasi). Kepemilikan mengikuti: data entity reference dimiliki **App Owner** (dikirim & diupdate via rilis aplikasi/seed; Data Owner read-only), data tenant-isolated dimiliki **Data Owner**. Seluruh strategi/topologi (single/multi, pooled/isolated, tiering) bukan urusan app spec — diputuskan saat deployment (D21) | **Final** |
| D24 | **Manifest tidak pernah dienkripsi** — keterbacaan adalah fitur (consent D20, `forma validate`, diff review, AI). Proteksi nilai modul komersial: (1) IP sesungguhnya boleh biner via `impl` native/compiled tanpa source, (2) signed provenance vendor (ed25519, D11) + policy "Verified-only" di Control Plane — salinan ganti-ID tidak bisa memalsukan tanda tangan dan tidak bisa masuk marketplace/environment ter-governance, (3) ekonomi update-support-liability yang tidak ikut tersalin, (4) legal via lisensi modul | **Final** |
| D25 | **App sebagai lapisan first-class**: Workspace → App → Module → Resource. App = unit deployment, trust boundary, publish interface (`kind: App`, root project); Module = packaging kode statis tanpa tenant/data; default private — akses lintas app hanya via publish → request → **grant yang disetujui Data Owner**, tercatat, revocable, dan menjadi titik metering revenue sharing (D22) | **Final** |
| D26 | **Tidak ada storage global sama sekali** (menggantikan mekanisme `isolated: false` di D23): `characteristics: reference` tinggal sebagai penanda domain (read-only bagi Data Owner, seeded). Lookup kecil-stabil → seed per-tenant milik App Owner via rilis; data hidup/besar (kurs, ICD-10) → **provider app** milik vendor yang mem-publish service | **Final** |
| D27 | **Kepemilikan mengikuti workspace tempat resource berjalan** — satu aturan untuk semua kasus. Modul yang dibeli/disewa tidak pernah menguasai data; compute = tanggung jawab Data Owner. Lisensi kedaluwarsa → modul **degradasi read-only**: actions/services mati, tapi `list/find/export/backup` **tidak pernah bisa digerbang lisensi, tanpa expired** — dijamin normatif di runtime (license token menolak gating standard read) | **Final** |
| D28 | **Glossary kanonik (Appendix C) mengikat** untuk semua dokumen dan diskusi: Workspace, App, Module, Resource, Tenant, Data Owner, App Owner, Module Vendor, Provider App, Grant, Platform Operator | **Final** |
| D29 | **Workspace = satu-satunya model multi-tenancy Forma** (amandemen D21: jalur self-managed tenancy dalam app dihapus; D7: istilah sub-tenant tidak dipakai lagi). Aplikasi 100% buta-tenancy; `FORMA_TENANCY` dihapus. Tenant besar yang ingin server sendiri → enterprise license → menjalankan Forma Cloud sendiri sebagai Platform Operator. Primitif workspace ada di semua tier (termasuk Standalone — dibutuhkan bahkan untuk dev); yang berbayar adalah *management plane* skala besar (provisioning otomatis, billing, marketplace) | **Final** |
| D30 | **Integritas contract (grant, consent, lisensi, metering) via transparency log, blockchain ditolak.** Mekanisme: (1) audit store = Merkle tree append-only dengan inclusion proof, (2) setiap contract ditandatangani kedua pihak (ed25519), non-repudiation kriptografis, (3) root checkpoint dipublikasikan/di-mirror di luar kendali operator. Blockchain menjawab masalah yang Forma tidak punya (konsensus tanpa otoritas pusat — padahal Control Plane adalah otoritas pusat by design dan registry terpusat = moat D11); dievaluasi ulang hanya jika federasi multi-operator terwujud (Q19) | **Final** |
| D31 | **Credible Exit Guarantee** — operator tidak perlu dipercaya karena meninggalkannya murah, ditegakkan pasar bukan chain. Empat jaminan normatif di spec: (1) identitas self-custodied — kunci workspace/app/vendor dipegang pemiliknya, platform hanya menyimpan public key, (2) contract = dokumen portabel bertanda tangan dua pihak, salinan dipegang masing-masing + inclusion proof — dapat dibuktikan ke operator baru tanpa kerja sama operator lama (license token D8 sudah portabel by design), (3) format backup/export normatif di open spec — implementasi conforming mana pun bisa restore, digabung D27 (export tak pernah tergerbang lisensi), (4) checkpoint log di-mirror pihak ketiga (D30). Positioning jujur: *operator terpusat yang dapat diverifikasi dan mudah ditinggalkan* — bukan trustless | **Final** |
| D32 | **Idempotency & concurrency framework-enforced** (hasil test drive Companion + diskusi): (1) `idempotent: true` ditegakkan runtime via **idempotency store** `(tenant, action, key) → pending\|completed + response tersimpan` — duplikat setelah completed me-**replay response asli** (webhook retry dapat 200-nya lagi), duplikat saat pending → tunggu/409; key dari client (header `Idempotency-Key`/field — webhook, outbox) atau **diterbitkan server via prepare-step** (double-submit form create); retensi via expiry, bukan delete-on-commit. Handler bebas cek manual. (2) Kolom `version` (§20.3) diberi semantik **optimistic concurrency**: CAS pada update, mismatch → 409 + versi terkini, default aktif semua Entity. (3) `last_updated`/`updated_at` = metadata audit murni, ditulis DB, **tidak pernah** jadi token perbandingan (rapuh: resolusi jam, NTP, non-monoton) | **Final** |
| D33 | **Garis deklaratif vs imperatif — litmus test tiga pertanyaan**: (1) fakta/jaminan yang framework tegakkan → YAML; prosedur berurutan → handler; (2) **vocabulary YAML = closed set** — tidak pernah tambah sintaks per kasus bisnis; escape hatch = ekspresi Starlark di dalam konstruksi yang ada (guard, conditions) — *declared location, scripted expression*; (3) konsekuensi event → `deliver` (deklaratif); langkah dalam action → handler. Konsekuensi konkret: **`channel: queue` ditambahkan ke deliver §12.3** (email/WA/PDF = konsekuensi event `paid`, bukan langkah mark-paid) | **Final** |
| D34 | **Spec tooling ladder** (jawaban verbosity): (1) JSON Schema per kind dipublikasikan (forma.dev/schemas) + LSP — autocomplete, hover docs, validasi realtime; hampir gratis berkat format `apiVersion/kind`; (2) scaffold CLI `forma new <kind>`; (3) visual editor di admin panel ala DocType editor Frappe — **menulis YAML ke file/PR, bukan DB tersembunyi; git tetap source of truth**; (4) Agent Skill = spec editor untuk AI. Tidak pernah ada format biner proprietary | **Final** |
| D35 | **`kind: Subscription` (Core Basic)** — berlangganan event resource lain dari modul pelanggan, tanpa menyentuh manifest penerbit; prasyarat struktural ekosistem modul (modul ter-sign tidak bisa diedit). Vocabulary `deliver` sama persis (closed set, D33). **Garis pembagi = kepemilikan jaminan**: konsekuensi yang merupakan kontrak penerbit (jurnal ← janji billing) tetap di `deliver` publisher; reaksi opsional/pihak-lain/lahir-belakangan → Subscription. Penangkal "tersebar tak disadari": (1) fan-out wajib terkompilasi — `forma describe` + admin panel menampilkan gabungan deliver + semua Subscription penunjuknya, (2) Subscription masuk footprint consent install (D20), (3) kontrak durability tetap dua sisi (§12.1). Simetri: lintas modul = consent install; lintas app = grant (D25) | **Final** |
| D36 | **Arsitektur frontend** (Frontend Spec v0.4.0): (1) **renderer interpretatif** — SPA shell membaca manifest via meta API, bukan codegen UI; renderer juga **embeddable sebagai library** (`<FormaPage/>` dalam app React eksis — adopsi dua arah); (2) **kontrak komponen** = ES module `mount(el, props, forma)` framework-agnostic + client `forma.api` (berjalan sebagai user login) + **`forma.ui`** services standar (toast/dialog/confirm/drawer) + base component library tertutup + scoped CSS + CSP; full-custom page = Page berisi satu komponen; (3) **garis bahasa**: ekspresi deklaratif = **FormaExpr** (subset ekspresi Starlark, AST interpreter kecil di JS, satu grammar dengan guard server, UX-only) dengan vocabulary behavior tertutup `visible_when/readonly_when/required_when/compute` (reaktif); kode imperatif = JS/TS via komponen — full Starlark di browser ditolak; (4) realtime convention `entity:{module}.{name}` filter permission per-message (Q9); (5) `kind: Print` server-side PDF (D10); (6) **`kind: Theme`** — tokens + CSS layer + widget skins, dipaketkan dalam modul, signed, dijual di marketplace (D22), dipilih Data Owner per workspace; Page mendapat varian `tabs` untuk data master/config. selebihnya komponen. (7) **`kind: Widget`** — modul menyumbang widget ke katalog app (visibilitas derived dari permission); **dashboard customizable** dengan prinsip normatif baru: *manifest mendefinisikan yang mungkin, preferensi mencatat yang dipilih* — layout user = data runtime di forma.core, tidak pernah ditulis balik ke YAML; **`kind: Report`** — laporan berparameter + grup + total + export async. (8) **Transfer manager** (`forma.files` + upload/download tray) = infrastruktur renderer, bukan kind — download tray murni berlangganan job.completed dari mekanisme async yang sudah ada. Sembilan kind total | **Final** |
| D37 | **Identitas workspace, membership per-app**: user identity = level workspace (satu akun per manusia — prasyarat cross-app grant, audit, SSO); membership + role assignment = per-app (`forma.core` + entity `app-membership`); populasi user tiap app boleh berbeda total dalam satu workspace. Definisi role tetap milik modul → otomatis per-app | **Final** |
| D38 | **Otorisasi berbasis task/page = lapisan administrasi, bukan enforcement.** Otoritas implisit berbasis halaman ("boleh page → implisit boleh simpan entity-nya") **ditolak**: provenance UI tidak bisa diverifikasi server (confused deputy) — dan client unmanaged (Flutter/API) tidak lewat page sama sekali. Sebagai gantinya: setiap Page punya **capability footprint derived** dari komposisinya (Form→actions, Table→list, component→deklarasi `needs:` eksplisit); admin memberi role akses per-page → footprint **dimaterialisasi** menjadi permission resource nyata, terlihat di `forma describe page`, teraudit. Enforcement selamanya di resource (D20). Pelengkap: headless form engine `forma.form()` untuk JS penuh; unmanaged mobile (Flutter) = API consumer kelas satu hari ini (HTTP+WS+codegen Dart/TS); wizard multi-entity → composite action server-side untuk atomisitas | **Final** |
| D39 | **Arsitektur Control Plane** (Control Spec v0.1.0): (1) `kind: Policy` = **YAML terstruktur + escape hatch Rego** (pola D33 versi governance; structured keys dikompilasi ke Rego — satu engine, satu decision log) — menjawab Q10; policy floor tak bisa dimatikan: no self-approval, no unsigned artifact non-dev, no environment override; (2) **dua kelas kunci**: owner keys self-custodied (Control simpan public key saja) vs platform keys HSM/KMS; (3) **contract model tunggal** untuk grant/consent/license — dokumen portabel bertanda tangan dua pihak + inclusion proof Merkle + checkpoint terpublikasi (D30/D31); license token tak bisa menggerbang list/find/export/backup (floor D27); (4) **consent delta = approval**: perubahan footprint versi baru memblokir deploy sampai Data Owner re-consent; (5) **REPL governance** (Q13): dev penuh, staging read-write teraudit, production read-only default + write butuh approval time-boxed + sesi masuk transparency log; (6) impersonation hanya via grant Data Owner bertanda tangan & time-boxed — tidak ada backdoor operator | **Final** |
| D40 | **Model aktor & kepemilikan**: empat owner simetris — **Workspace Owner** (alias Data Owner), **App Owner**, **Module Owner** (alias Module Vendor), **Cloud Owner** (alias Platform Operator); satu identitas boleh memegang beberapa role. **Owner = tepat satu identitas** (email + owner key self-custodied). **Admin via delegation certificate**: admin ber-key sendiri + sertifikat delegasi ditandatangani owner (scope, masa berlaku, revocable) — tanda tangan admin selalu membawa sertifikatnya, rantai kriptografis tetap ke owner; owner-only (non-delegable): terima/lepas kepemilikan, terbitkan/cabut delegasi, rotasi owner key. **Transfer dua jalur**: konsensual (dua tanda tangan, Cloud Owner memfasilitasi & mencatat — bukan memberi izin) dan pemulihan (operator-mediated dengan due process normatif: re-attestation, masa tunggu, notifikasi semua admin, entry transparency log publik) — transfer sepihak oleh operator = backdoor, ditolak | **Final** |
| D41 | **Governance backup/restore**: hanya **workspace data** yang butuh backup customizable (satu-satunya yang tak reproducible) — diatur Workspace Owner: jadwal, retensi, cakupan, **target eksternal milik owner** (Credible Exit yang hidup — salinan harian di tangan owner; tak pernah tergerbang lisensi per D27); app/module artifact = reproducible dari git + registry (tanpa backup sendiri; retensi registry = Cloud); contract/log/membership = Cloud Owner + salinan kontrak di tiap pihak (D31); durability infra = SLA Cloud. Aturan: `backup create` delegable, **`restore` menimpa = owner-signature atau delegasi ber-scope eksplisit + transparency log**; enkripsi per-tenant dengan opsi owner-supplied key; konsistensi point-in-time per-app, lintas-app mendekati + rekonsiliasi outbox pasca-restore | **Final** |
| D42 | **Marketplace & model ekonomi** (spec sendiri: `forma-marketplace.md`): (1) **satu marketplace** — module/app/theme/widget = kategori listing di satu infrastruktur (semuanya artifact ter-sign di registry); syarat komersial di listing, tidak pernah di manifest (D22). (2) **Vocabulary pricing tertutup**: free · one_time · subscription · per_seat (← membership D37) · per_call (← grant metering D25) · per_transaction (← metering event count-only, Control tak pernah baca payload) · metered_passthrough. (3) **Metering terverifikasi**: angka ditandatangani Resource Plane + anchor transparency log (D30) — vendor verifikasi penjualannya, Workspace Owner verifikasi tagihannya, terhadap operator sekalipun. (4) **Satu ledger per owner**: debit (infra, langganan) + kredit (payout, top-up), pendapatan meng-offset tagihan; settlement **prepaid default** (top-up lokal; saldo habis → grace → degradasi read-only D27) + **postpaid tier kepercayaan** (invoice/PO enterprise). (5) **Lisensi banyak jenis** (trial/perpetual/subscription/usage — semua berujung license token D8 dengan tipe & masa berlaku) vs **infra metering satu jenis seragam** — dua garis di satu tagihan. (6) **Scaling transparan**: mekanika tak pernah di app spec (konsekuensi D29 + location transparency); owner mengatur resource plan + **budget cap** (prepaid = rem alami), operator memutar tuas; per tier: managed autoscale / BYOI / self-hosted manual. (7) Pagar tetap: read-only degradation + export tak tergerbang (D27), Verified Badge wajib untuk listing berbayar (D11), token portabel air-gap-safe (D8), jalur gratis/self-hosted FSL utuh. Angka fee/refund/grandfathering = keputusan bisnis (Q16 menyempit) | **Final** |
| D43 | **Konsol resmi = dogfooding wajib**: tiga Forma app per persona — `forma/console` (Workspace Owner+admin), `forma/studio` (App/Module Owner — kreator), `forma/ops` (Cloud Owner+admin); konsol = proving ground permanen spec. Arsitektur: konsol berjalan di Resource Plane workspace ops, akses Control Plane via bridge `kind: Service` (larangan "Control tak eksekusi business logic" utuh); tanda tangan owner key = **client-side** (custom component + WebCrypto — kunci self-custodied D31 tak pernah menyentuh server). **Bedrock exception**: jalur darurat `forma-ctl` CLI = kode konvensional dalam binary `forma-control`, tidak pernah bergantung platform yang diperbaikinya; web console = kenyamanan, bukan satu-satunya jalan. Dokumentasi = manifest YAML konsol itu sendiri + satu architecture note; lapisan admin lengkap di Appendix D | **Final** |
| D44 | **Prinsip owner non-teknis** — Workspace Owner bisa jadi pemilik bisnis gaptek; aktivitas wajibnya minimal: (1) **owner key = passkey (WebAuthn) sebagai bentuk default** — private key di secure enclave perangkat (sync iCloud/Google = jawaban HP hilang), platform simpan public key saja: D31 utuh, UX = "login sidik jari"; kontrak di-hash ke challenge → tanda tangan = notifikasi + tap + biometrik; raw ed25519 via CLI tetap ada untuk power user — **dua amplop tanda tangan, satu model kontrak** (§6 Control); (2) **kewajiban owner dikunci minimal**: approve consent/footprint-delta/grant/billing-di-atas-ambang/restore/penunjukan-admin/transfer — sisanya delegasi (D40), termasuk pola software house sebagai admin ber-sertifikat (ter-scope, teraudit, revocable satu tap); (3) **layar consent wajib bahasa manusia** dengan detail teknis expandable — syarat normatif; (4) anti-fatigue: prepaid top-up = persetujuan billing itu sendiri, budget cap = auto-approve di dalamnya (D42) | **Final** |
| D45 | **Larangan unmanaged storage di workspace**: data di luar primitif `ctx.*` = keluar diam-diam dari semua jaminan (backup D41, Credible Exit D31, export D27, isolasi, consent D20) — "Forma module" adalah janji. Tangga resmi kebutuhan lanjut: (1) `ctx.db` raw SQL (sudah ada), (2) **tabel milik modul via `kind: Migration`** — struktur bebas, tapi **wajib berkolom `tenant_id`** + tinggal di schema kategorinya (tetap ter-backup/isolasi/audit), (3) engine eksotis (search/vector/graph) → provider app milik vendor atau dibungkus `kind: Service`. Workspace Owner **tidak pernah** menyediakan raw storage untuk modul (BYOI = primitif managed di infra owner, bukan akses mentah). **Sidecar wajib stateless**: scratch disk ephemeral boleh, state persisten hanya via `ctx.*` | **Final** |
| D46 | **Pertahanan berlapis `ctx.db` & binary handler** (skenario: modul gratis jahat ber-native mengincar data GL): (1) scope db **default = modul sendiri**; cross-module wajib deklarasi → muncul di consent; **cross-module write = high-risk consent** (presentasi berbeda); (2) pelanggaran runtime dinormatifkan: undeclared access → blocked + alert + **modul auto-suspend** + incident audit — mengecek keberadaan data pun memicu; (3) **pengakuan jujur**: enforcement native = best-effort (in-process full trust, kredensial di-broker tapi memori bisa dikorek) — hukum alam semua plugin biner; pertahanan sesungguhnya = **provenance: impl type digerbang tier kepercayaan** — unverified/gratis: hanya sandbox (script/script_ref/WASM); Verified Badge: + sidecar (terkurung proxy); Verified + security scan + approval berlapis: + native; (4) blast radius: tenant injection di lapisan ctx membatasi kerusakan ke workspace yang meng-consent; pemulihan via audit + backup eksternal D41 | **Final** |
| D47 | **Model dua kanal antar-plane** (Plane Protocol v0.1.0) — mempertajam aturan "no write-back": (1) **desired-state channel** (Control→Resource, pull-only): snapshot ter-sign & ber-versi monoton (proteksi rollback attack) berisi policy, desired deployments, trust anchors, grants, revocations, membership; Resource tidak pernah bisa memutasi governance state; (2) **evidence channel** (Resource→Control, append-only): deploy status, metering (count-only), audit anchor (Merkle root segmen lokal — audit workspace terikat ke transparency log tanpa mengirim isinya), violation incidents — bukti bisa ditambah, tidak pernah diedit, semua ter-anchor di log (bukti tentang operator pun tamper-evident); tanpa kanal ini metering terverifikasi D42 dan insiden D46 tak punya jalur. Outage: serve-forever di snapshot terakhir + penolakan lokal operasi high-governance melewati ambang; fail-closed pada konstruksi trust/versi mayor tak dikenal | Diusulkan |
| D48 | **Core Extended v0.2.0** — tiga kind baru + normatisasi stub: (1) **`kind: Workflow`** (menuntaskan Q8/D10): approval menempel ke transisi state machine tanpa mengubah Entity (pola D35); steps sekuensial ber-quorum + guard, no self-approval (floor D39 naik ke workflow bisnis), tampil di `forma describe` merged; (2) **`kind: Api`**: override exposure (path/versi/disable/gRPC eksternal) — surface tidak pernah memperluas akses; (3) **`kind: KindDefinition`** (menjawab Q14): namespacing via **apiVersion group ala CRD** (`seed.forma.dev/v1`) — kolisi mustahil struktural, handler berjalan di bawah `uses` modul, schema otomatis feed LSP (D34); (4) **`kind: Webhook` & `kind: Mockup`** dinormatisasi dari stub + environment binding config-driven (bisnis handler tak pernah branch environment; `mock_enabled` per env) + state & fault-injection mockup; (5) Tier-2 streaming dilebur ke `kind: Subscription` (D35) — **dynamic subscription = data, bukan manifest**; (6) notification/registry/control content dipindah ke rumah barunya (forma/notify, marketplace, Control Spec) | Diusulkan |

## 10. Open Questions

| # | Pertanyaan |
|---|---|
| Q1 | Runtime async PHP untuk sidecar: Swoole vs RoadRunner vs ReactPHP |
| Q2 | Gaya error handling sidecar: exception idiomatic vs `ok()/fail()` konsisten Starlark |
| Q3 | Detail penerapan FSL: FSL-1.1-Apache-2.0 apa adanya atau perlu penyesuaian klausul; kebijakan CLA/DCO untuk kontributor eksternal |
| Q4 | Trademark: yurisdiksi pendaftaran (Indonesia dulu, lalu Madrid Protocol?) dan kebijakan pemakaian nama oleh komunitas |
| Q5 | Strategi scaling audit hash-chain di Control Plane shared |
| Q6 | Model billing data transfer BYOI; tier ISV/Platform Partner |
| Q7 | Shared sidecar (DaemonSet-per-node) sebagai varian lanjutan |
| Q8 | ~~Fitur adopsi Frappe~~ — **terjawab penuh**: form layout & print di Frontend Spec (D36); approval → kind Workflow di Extended §1 (D48) |
| Q9 | ~~Spesifikasi detail realtime subscription per-entity~~ — **terjawab** Frontend Spec §8 (D36) |
| Q10 | ~~Scope OPA: 100% Rego atau hybrid~~ — **terjawab** Control Spec §3 (D39): YAML terstruktur + escape hatch Rego, satu engine |
| Q11 | Migrasi penamaan spec: adopsi penuh `apiVersion`/`kind` menggantikan `type:` di resource definition (dilakukan saat penulisan ulang Core Basic) |
| Q12 | ~~Katalog kind frontend + kontrak escape hatch~~ — **terjawab** Frontend Spec v0.1.0 (D36): enam kind + mount contract + FormaExpr; sisa detail = F1–F4 di spec tersebut |
| Q13 | ~~Desain REPL~~ — **terjawab** Control Spec §9 (D39): dev penuh, staging read-write teraudit, production read-only + approval untuk write + transparency log |
| Q14 | ~~Mekanisme KindDefinition~~ — **terjawab** Extended §5 (D48): namespacing via apiVersion group ala CRD, handler di bawah uses modul, schema feed LSP |
| Q15 | Desain pooling density untuk jalur platform-managed (D21): kapan instance terisolasi vs pooled, batas resource, noisy neighbor |
| Q16 | Angka bisnis marketplace (mekanisme sudah di D42): besaran platform fee per kategori, kebijakan refund/dispute, grandfathering harga, jadwal payout |
| Q17 | Alur re-consent saat update footprint (D21): blocking update vs grace period, notifikasi Data Owner, rollback otomatis |
| Q18 | Grant lintas workspace (B2B sejati) dan detail publish interface `kind: App`: versioning interface, deprecation, SLA grant provider app |
| Q19 | Shared ledger antar Platform Operator: dievaluasi hanya saat federasi multi-operator terwujud; prasyarat = D30 (transparency log) + D31 (identitas self-custodied & contract portabel) |

---

## 11. Struktur Dokumen ke Depan

Prinsip pemisahan: **spec** adalah kontrak netral (CC0, bisa diimplementasikan siapa pun), **impl** adalah dokumentasi binary resmi (mengikuti kode, lisensi FSL). Core Basic/Extended lama mencampur keduanya — Control Plane (Extended §13) ikut di dalam spec resource model; struktur baru memisahkannya per plane.

```
docs/
  foundation/
    Forma-Foundation-Document.md   ← dokumen ini (acuan tertinggi)

  spec/                            ← open standard, CC0 (bahasa Inggris saat publik)
    forma-core-basic.md            ← resource model minimum — kontrak Resource Plane
    forma-core-extended.md         ← resource model lanjutan (hooks, LB, circuit breaker,
                                     storage, mockup, summary, dll.)
    forma-control.md               ← kontrak Control Plane: environment, deployment policy
                                     (OPA/Rego), signing, approval, audit hash-chained
    forma-frontend.md              ← katalog kind UI, renderer, komponen, realtime (D36)
    forma-marketplace.md           ← ekonomi: listing, pricing, ledger, metering
                                     terverifikasi, payout (D42)
    forma-plane-protocol.md        ← kontrak ANTAR plane: mTLS, policy pull (boot + 5 menit),
                                     format artifact/checksum, larangan tulis-balik

  impl/                            ← dokumentasi implementasi resmi (FSL, hidup bareng kode)
    forma-resource/                ← arsitektur internal, tuning, deployment, sidecar polyglot
    forma-control/                 ← arsitektur internal, HSM/KMS, license token, ops
    forma-cli/                     ← forma, forma-ctl, forma dev

  notes/                           ← technical note baru per topik (bahasa Indonesia)
  archive/                         ← semua dokumen lama (referensi sejarah)
```

Pemetaan: `forma-resource` mengimplementasikan `forma-core-basic` + `forma-core-extended` + sisi klien `forma-plane-protocol`; `forma-control` mengimplementasikan `forma-control.md` + sisi server protokol. Konsekuensi migrasi: saat menulis ulang Core Extended, §13 (Control Plane) dipindah keluar menjadi `spec/forma-control.md`.

Mekanisme kerja: setiap keputusan baru masuk ke Decisions Log dokumen ini lebih dulu, baru diturunkan ke spec/impl/notes.

---

## Appendix A — Peta Laravel → Forma (checklist kelengkapan)

| Laravel | Forma | Status |
|---|---|---|
| Artisan | `forma` CLI | Padanan |
| Tinker (REPL) | `forma repl` (Starlark + `ctx.*`) | Padanan (D13) |
| Horizon / Pulse | `forma.observe` | Padanan |
| Filament / Nova | Admin panel otomatis + form layout hints (D10) | Padanan |
| Packagist | Module registry + Verified Badge | Padanan |
| Queue & Jobs | `ctx.queue` + deklarasi di resource | Padanan |
| Events & Listeners | Reliable events di resource definition | Padanan |
| Validation | Deklaratif di fields | Padanan |
| Policies / Gates | `required_permission` di resource | Padanan |
| Sanctum / auth | Auth wajib by default | Padanan |
| Scheduler (`schedule:run`) | `forma/scheduler` (modul resmi) | Padanan (D12) |
| Mail / Notifications | `forma/mail`, `forma/notify` (modul resmi) | Padanan (D12) |
| Seeder / Factory | `forma/seed` (modul resmi) | Padanan (D12) |
| Forge / Vapor | Forma Cloud / Standalone | Padanan |
| `routes/` | Tidak ada — route diturunkan dari resource | **Sengaja beda**: satu sumber kebenaran |
| Eloquent (ORM) | Tidak ada — entity auto-CRUD + `ctx.db` | **Sengaja beda**: anti-ORM |
| Folder `config/` | Resource `kind: Config` + environment dari Control Plane | **Sengaja beda**: config = resource ber-governance |
| Blade / Livewire | Kind frontend (Page, Form, ...) + `asset` | **Sengaja beda**: deklaratif, bukan template |
| Migrations | Diturunkan dari perubahan definisi entity | **Sengaja beda** (detail di spec baru) |
| — (tidak ada di Laravel) | Control Plane, signing, OPA policy, multi-tenancy bawaan, location transparency, polyglot sidecar | Melampaui |

Aturan pemeliharaan: setiap fitur Laravel yang belum terpetakan wajib masuk tabel ini dengan salah satu status — *Padanan*, *Sengaja beda + alasan*, atau *Gap (→ Open Question)*.

---

## Appendix B — Peta Concern → Kind (D17, D18)

Tiga prinsip katalog:

1. **Derived by default, kind to override.** CRUD API, admin panel, dan docs lahir otomatis dari `Entity`. Kind di concern exposure/UI hanya untuk menyimpang dari default. Granularitas: satu kind per concern-unit, bukan per layar (`Form` dengan `spec.mode`, bukan `EntityCreateUI`).
2. **Kind extensible tiga lapis** (D18): bawaan spec → modul resmi via `KindDefinition` → pihak ketiga (ter-namespace + Verified Badge).
3. **Data bukan manifest.** General-ledger = *instance* `kind: Module`; akun "1-1001 Kas" = baris database (instance runtime Entity), bukan manifest.

| # | Concern | Kind | Letak |
|---|---|---|---|
| 1 | Packaging | `Module` | Core Basic |
| 2 | Domain model | `Entity`, `Service` | Core Basic |
| 3 | Proses bisnis | state machine inline di Entity; `Workflow` terpisah untuk approval/lintas-role (wadah D10) | Extended |
| 4 | Konfigurasi | `Config` (`FeatureFlag` kandidat) | Core Basic |
| 5 | Exposure/API | derived by default; `Api` (override REST/gRPC eksternal), `Webhook` (inbound) | Extended |
| 6 | UI/Frontend | derived by default; `Page`, `Form`, `Table`, `Dashboard`, `Menu`, `Print` | Extended (spec frontend) |
| 7 | Data lifecycle | `Migration` (custom DDL saja; structural derived dari diff Entity) | Core Basic |
| 8 | Seeding/testing | `Seed`, `Factory` | Modul `forma/seed` |
| 9 | Async/komunikasi | `Schedule`, `MailTemplate`, `NotificationChannel` | Modul resmi masing-masing |
| 10 | Integrasi eksternal | dibungkus `Service`; `Mockup` untuk simulasi pihak ketiga | Extended |
| 11 | Governance | `Environment`, `Policy` (Rego) | forma-control.md |
| 12 | Reaksi lintas-modul terhadap event | `Subscription` (D35) — deliver vocabulary sama dengan publisher | Core Basic |

---

## Appendix C — Glossary Kanonik (D28, mengikat)

Hierarki dalam satu kalimat: **Workspace berisi App, App tersusun dari
Module, Module berisi Resource; data selalu milik pemilik Workspace tempat
resource berjalan.**

| Istilah | Definisi |
|---|---|
| **Workspace** | Payung milik satu Data Owner: tempat data & compute hidup, identitas tenant lintas app, batas billing |
| **App** | Unit deployment & trust boundary; dipasang ke workspace; tersusun dari modul; mem-publish interface; `kind: App` = root project |
| **Module** | Unit packaging kode — statis, tanpa tenant, tanpa data; dibeli/disewa, tidak pernah "memiliki" |
| **Resource** | Entity/Service/Workflow/dll di dalam modul |
| **Tenant** | Representasi workspace di dalam storage sebuah app — mekanisme isolasi internal runtime, bukan konsep bisnis; satu workspace = identitas tenant yang sama di semua app-nya |
| **Data Owner** | = **Workspace Owner** (istilah kanonik sejak D40): satu identitas pemilik workspace — data, users, grants, compute, billing |
| **App Owner** | Vendor aplikasi (satu identitas): artifact, versi, konten seed reference; tanpa akses data production |
| **Module Vendor** | = **Module Owner** (D40): penerbit modul di marketplace (bisa ≠ App Owner) |
| **Provider App** | App yang tujuannya mem-publish service ke app lain; vendornya = Workspace Owner atas workspace-nya sendiri; menggantikan konsep "data global" |
| **Grant** | Izin akses lintas-app: di-request konsumen, disetujui Workspace Owner penyedia, tercatat, revocable; titik metering revenue sharing |
| **Platform Operator** | = **Cloud Owner** (D40): pemilik instance Forma Cloud — Forma Cloud resmi, atau pemegang enterprise license |
| **Admin** | Identitas ber-key yang bertindak atas nama owner via **delegation certificate** (scope + masa berlaku + revocable, D40); tanda tangannya selalu membawa sertifikat — rantai audit sampai ke owner |

---

## Appendix D — Empat Lapisan Administrasi (klarifikasi D37/D40/D43)

Admin **dalam** aplikasi bisnis bukan persona platform — dia lapisan
terbawah yang mekanismenya sudah lengkap:

| Lapisan | Contoh (app klinik) | Mekanisme | UI |
|---|---|---|---|
| **L1 — admin dalam app** | admin klinik mengelola user & role dokter/kasir | RBAC biasa: role modul (D19) ber-permission `core.members.*` atas entity forma.core ter-scope app (D37) | UI app itu sendiri — derived admin panel atas forma.core; developer tidak pernah membangun user management |
| **L2 — Workspace Owner + admin** | pemilik klinik | D40 (owner tunggal + delegation cert); install, consent, grant, backup (D41), tagihan (D42) | `forma/console` |
| **L3 — App/Module Owner + admin** | vendor app klinik; atau workspace owner yang membuat app internal sendiri (rangkap L2+L3, tanpa marketplace) | D40, rilis & listing (D42) | `forma/studio` + CLI |
| **L4 — Cloud Owner + admin** | operator Forma Cloud | D40 + Control Spec; jalur darurat = bedrock `forma-ctl` (D43) | `forma/ops` |

Dokumentasi fitur konsol L2–L4 = manifest YAML app-nya sendiri (D14/D43)
+ satu architecture note; L1 tidak butuh dokumen — dia pola pemakaian
biasa, dicontohkan di Companion.

**Ujian Module vs App** (pertanyaan pertama setiap developer baru):

> **Module dipakai saat *build/compose* — kodenya masuk ke App Anda, datanya jadi milik workspace Anda.**
> **App dipakai saat *runtime* — lewat grant ke interface-nya, datanya tetap milik workspace dia.**

Pembedanya *bisa deploy sendiri atau tidak*, bukan "dipakai pihak lain atau
tidak" — App juga dipakai pihak lain, lewat grant. Alur lazim: mulai dari
App (`forma new` = App); Module lahir via ekstraksi logika reusable, atau
sengaja dibuat untuk marketplace. Satu repo boleh mendaftarkan **dua bentuk
distribusi** ke registry sekaligus (app-nya dan modul-modulnya, versi
independen). Simetri deklarasi di `kind: App`: `publishes` (interface yang
ditawarkan) ↔ `consumes` (interface app lain yang dibutuhkan — memicu grant
request saat install, tampil dalam satu layar consent bersama footprint
modul).

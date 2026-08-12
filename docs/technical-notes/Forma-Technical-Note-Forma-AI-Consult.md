# FormSpec Technical Note: FormSpec AI — `formspec consult` sebagai Konsultan Bisnis & Spec Author

**Catatan internal — hasil diskusi tim, bukan bagian resmi FormSpec Core Spec**
**Status: arah desain awal, belum ditulis ke Core Basic/Extended Spec resmi. Banyak bagian masih proposal, bukan keputusan final.**

---

## 0. Latar Belakang

Kebutuhan awal: developer FormSpec App (mis. membuat aplikasi barbershop) ingin AI membantu bukan cuma menulis spec, tapi juga berperan sebagai **konsultan bisnis** — aktif bertanya soal tujuan aplikasi, mengusulkan alur sistem, diskusi dengan business owner, baru kemudian menerjemahkan hasil diskusi jadi spec FormSpec yang valid.

Dua kapabilitas ini sering gagal disatukan tanpa jembatan yang jelas:
1. **AI sebagai partner diskusi bisnis** — perlu lancar bicara bahasa awam, proaktif tanya, tidak kaku.
2. **AI sebagai penulis spec FormSpec yang akurat** — tidak boleh mengarang nama field/kind/aturan yang tidak ada.

Kalau digabung tanpa struktur, hasilnya AI yang lancar ngobrol tapi specnya asal, atau sebaliknya. Desain di bawah memisahkan keduanya secara eksplisit dengan lapisan **grounding** (tools nyata, bukan sekadar prompt) dan **validasi wajib** yang independen dari perilaku LLM.

---

## 1. Keputusan Awal (Sudah Disepakati)

- **Mulai dari CLI** (`formspec consult`), bukan FormSpec Studio. Studio (Studio Lite) adalah upgrade jangka menengah setelah CLI terbukti.
- **Industry template awal 100% dari FormSpec sendiri** — community-contributed template adalah kemungkinan masa depan, bukan target M1/M2.
- **Tidak perlu deteksi role** (business owner vs developer). Batasan domain tidak jelas dan sering campur dalam satu sesi — AI cukup adaptif secara alami (bahasa awam default, teknis kalau developer jelas menanyakan hal teknis), tanpa mode switch eksplisit.
- **Sesi chat disimpan di folder khusus**, hasil draft-nya bisa di-diff terhadap implementasi yang sudah ada.
- **Implementasi berbasis Spec FormSpec** — karena tidak ada tahap compile (sudah diputuskan di technical note vendoring), diff yang relevan adalah **diff spec-ke-spec**, bukan spec-ke-kode-tergenerate. Spec FormSpec *adalah* implementasinya.
- **Module vertikal (payment gateway, GL, CRM, dst.) harus bisa di-reuse** — AI perlu tahu module apa saja yang tersedia supaya bisa mengusulkan integrasi, bukan reinvent dari nol.
- **MCP perlu index module** — tiap module yang terdaftar mendeskripsikan kategori, fitur, cara integrasi, dan "skill" yang bisa ditangani, supaya AI bisa mengusulkan module yang tepat.

---

## 2. Arsitektur: Lima Lapisan Terpisah

```
┌──────────────────────────────────────────┐
│ 1. Client (formspec-consult, TS/Bun)        │  ← REPL, kelola sesi, tampilkan diff
├──────────────────────────────────────────┤
│ 2. LLM Provider Layer (Vercel AI SDK)    │  ← Bisa ganti-ganti provider
├──────────────────────────────────────────┤
│ 3a. formspec-local-mcp (stdio, Go)          │  ← Grounding: workspace, project ini
│ 3b. formspec-remote-mcp (HTTP, FormSpec Cloud) │  ← Grounding: template & registry ekosistem
├──────────────────────────────────────────┤
│ 4. Validation Gate (server-side, wajib)  │  ← Safety net, independen dari LLM
└──────────────────────────────────────────┘
```

Prinsip pemisahan: kelemahan satu lapisan (mis. LLM yang kurang reliable ikuti instruksi) tidak boleh merembet ke lapisan lain. Ini penerapan langsung prinsip "safety via structure, not documentation" yang sudah dipegang di bagian arsitektur lain — jangan bergantung pada LLM "berperilaku baik". Lapisan 3 dipecah dua (FA-m) karena beda kepemilikan data — bukan pelanggaran prinsip pemisahan, justru penerapannya: data project (lokal) dan data ekosistem (bersama) punya lifecycle dan trust boundary berbeda, jadi dipisah server juga.

### 2.1 Client — `formspec consult` Berjalan 100% Mandiri (Bukan Bergantung Client Lain)

**Revisi penting dari draf sebelumnya:** `formspec consult` adalah MCP client-nya sendiri — bukan "fallback" kalau Claude Code/Cursor/VS Code tidak terpasang. Ini keharusan struktural, bukan preferensi: spec bisnis ada di local folder (`modules/`, `vendors/`, `formspec.lock`), dan `formspec-local-mcp` perlu baca langsung dari disk. Kalau jalur utamanya lewat client cloud/remote (yang cuma dukung konektor MCP remote via HTTP publik), server lokal harus di-tunnel ke internet — spec bisnis klien (harga, struktur komisi, dst.) sempat lewat jaringan publik hanya untuk divalidasi. Ini bertentangan dengan prinsip data sovereignty yang sudah dipegang di tempat lain (BYOK, App Owner/Workspace Owner terpisah).

**Revisi bahasa implementasi (lihat FA-o/FA-l):** karena tooling multi-provider+MCP jauh lebih matang di TypeScript (Vercel AI SDK 6) dibanding Go saat ini, `formspec-consult` (binary terpisah, huruf sambung sesuai konvensi CLI, bukan `formspec consult` dengan spasi lagi setelah dipisah bahasanya) diimplementasikan TypeScript + Vercel AI SDK, di-compile jadi standalone binary via `bun build --compile`. `formspec` (CLI utama Go — generate, module install, apply, server) tidak berubah, tetap murni Go.

Arsitektur mandiri:

```
formspec                    (Go — CLI utama + subcommand `formspec mcp-serve`)
  └─ mcp-serve: expose formspec-local-mcp sebagai proses stdio
     (pembungkus tipis formspec-core, FA-q — BUKAN binary terpisah)

formspec-consult             (TypeScript + Vercel AI SDK, compile via bun build --compile)
  ├─ spawn `formspec mcp-serve` sebagai child process (stdio, satu mesin)
  ├─ ToolLoopAgent (Vercel AI SDK) sebagai tool-use loop
  └─ panggil LLM API langsung lewat provider adapter Vercel AI SDK (BYOK)
```

Client MCP eksternal (VS Code/Claude Code/Cursor) dan `formspec-consult` (built-in) sama-sama menjalankan command yang identik (`formspec mcp-serve`) — satu implementasi server Go, dua cara pakai. Tidak perlu agentic framework besar untuk `formspec-consult` — `ToolLoopAgent` dari Vercel AI SDK sudah menyediakan siklus kirim-cek-eksekusi-kembalikan (2.1.1) sebagai abstraksi first-class, plus adapter provider (Anthropic/OpenAI/dst.) dan MCP client bawaan — sejalan prinsip "manfaatkan open source dulu", cuma sekarang di ekosistem TypeScript, bukan Go, untuk komponen spesifik ini. Tugas `formspec-consult`: kelola sesi chat, jalankan tool loop, render diff, simpan sesi ke folder (bagian 3). Kontributor yang cuma kerja di `formspec-core`/`formspec-server`/module system tidak pernah perlu sentuh TypeScript.

#### 2.1.1 Alur Tool-Use Loop — LLM Tidak Pernah Panggil MCP Langsung

Poin penting yang sering disalahpahami: LLM tidak punya kemampuan eksekusi/jaringan sendiri — dia cuma model teks. `formspec-consult` (client) adalah perantara wajib di setiap putaran — secara internal ditangani `ToolLoopAgent` (Vercel AI SDK), tapi mekanismenya tetap:

```
1. formspec-consult kirim [riwayat percakapan] + [daftar tool, JSON Schema] ke LLM API
2. LLM balas blok terstruktur "tool_use" (nama tool + input parameter)
   — ini CUMA teks/JSON, belum ada eksekusi apa pun
3. formspec consult BACA tool_use itu, lalu DIA SENDIRI melakukan panggilan MCP
   nyata ke formspec-local-mcp (stdio) dengan parameter dari LLM
4. formspec-local-mcp eksekusi (wrapper tipis atas formspec-core, bagian 2.4/2.5),
   kembalikan hasil ke formspec consult
5. formspec consult bungkus hasil jadi "tool_result", kirim balik ke LLM API
   sebagai giliran berikutnya
6. LLM lanjut — bisa minta tool lagi (ulang dari langkah 2),
   atau langsung kasih jawaban teks final ke developer
```

Satu giliran percakapan bisa berisi beberapa siklus tool_use/tool_result berturut-turut sebelum LLM akhirnya menjawab — itu normal. Inilah yang dimaksud "loop" yang harus diimplementasikan sendiri di `formspec consult` (SDK Anthropic sediakan helper konversi tool MCP ↔ format API, tapi loop-nya sendiri ditulis di CLI) — karena fitur `mcp_servers` bawaan Anthropic cuma jalan untuk server remote HTTP, bukan stdio lokal (lihat 2.2).

**Catatan penting soal biaya:** karena API stateless, setiap panggilan API memang wajib sertakan ulang seluruh riwayat pesan + daftar skema tool (tidak bisa dihindari, sifat dasar protokol). Tapi ini tidak sama dengan masalah katalog yang dihindari di FA-j/FA-u — ada dua budget token berbeda: **deklarasi tool** (nama+skema JSON parameter) tetap kecil dan konstan berapa pun besar data di baliknya (`list_skills` tetap satu skema kecil, mau ada 5 atau 500 skill), sementara **isi/data hasil panggilan tool** (daftar skill, modul, dst.) tidak pernah dideklarasikan di depan — cuma diambil on-demand. Justru ini alasan konkret kenapa "semua via MCP tool" (revisi FA-j) menyelesaikan masalah skala: yang tumbuh besar tidak pernah masuk ke bagian yang wajib diulang tiap panggilan API. Ditambah **prompt caching** (kalau daftar tool + system prompt identik antar turn, provider cache prefix-nya) — biaya/latensi pengulangan ini di praktiknya jauh lebih murah daripada kelihatannya di atas kertas.

**Attach ke client MCP lain (Claude Code, Cursor, VS Code, OpenCode, dst.) bersifat opsional** — bonus reuse untuk developer yang sudah pakai tool tersebut sehari-hari dan lebih suka attach `formspec-local-mcp`/`formspec-remote-mcp` di situ, bukan prasyarat. **OpenCode** (dipertimbangkan sempat sebagai fondasi `formspec-consult` sendiri, lihat FA-l riwayat) tetap masuk kategori ini — aplikasi jadi (TUI+session+persona sendiri) yang bagus dipakai langsung apa adanya (termasuk untuk akses provider seperti DeepSeek), tapi bukan substrat yang tepat untuk `formspec-consult` built-in karena flow konsultasi khusus FormSpec (Discovery→Proposal→Draft, safety-gate, struktur sesi) tetap harus ditulis custom di atasnya — sama besar effort-nya dengan menulis di atas Vercel AI SDK, tapi harus melawan asumsi aplikasi besar yang tidak kita kontrol dulu. Catatan: dukungan resources/prompts (dua primitif MCP selain tools) lebih tidak merata antar client eksternal dibanding tools yang nyaris universal — perlu dicek per client kalau fitur itu dipakai.

```
formspec consult
  → default: jalan built-in client, 100% lokal, tidak butuh aplikasi lain
  → opsional: developer bisa attach formspec-local-mcp ke client MCP lain yang sudah terpasang
```

### 2.2 LLM Provider Layer — Apakah Semua LLM Sama? Tidak.

Tiga alasan:
- **Reliabilitas tool-calling berbeda jauh** antar model — model kecil/murah sering skip panggil tool atau salah format argumen, terutama di percakapan panjang (discovery bisa 20+ turn).
- **Instruction-following untuk sesi panjang** — consultant behavior (nanya dulu, jangan lompat ke YAML) butuh model yang konsisten ikuti system prompt sampai turn ke-20+, bukan cuma turn pertama.
- **Tidak semua model dukung MCP tool-use** (terutama model lokal/open-weight lama).

Keputusan: **BYOK** (developer bawa API key sendiri — konsisten dengan prinsip BYOK yang sudah dipakai untuk data sovereignty di bagian lain) + **minimum capability bar** (harus lolos test tool-calling + context window minimum) + daftar provider yang sudah divalidasi FormSpec. Tidak ada klaim "semua LLM support" secara default. FormSpec tidak jadi reseller AI — cost inference sepenuhnya ditanggung developer.

**Format tool-call di API mentah berbeda per provider — ini alasan konkret kenapa layer ini perlu ada, bukan cuma teoretis.** Anthropic pakai arsitektur content-block (`type: "tool_use"`, `input` sudah berupa objek JSON native). OpenAI pakai `message.tool_calls[].function.arguments` — tapi `arguments` itu **string JSON**, perlu di-parse manual. DeepSeek sengaja kompatibel dengan format OpenAI (drop-in replacement), jadi ikut bentuk yang sama. Gemini beda lagi (`functionCall` di dalam `parts`). **[Revisi, lihat FA-o]** Normalisasi ini tidak lagi ditulis manual sebagai adapter Go — sejak `formspec-consult` pindah ke TypeScript + Vercel AI SDK, penerjemahan format tool-call lintas-provider ditangani `ToolLoopAgent`+provider adapter Vercel AI SDK yang sudah matang untuk 25+ provider. `formspec-local-mcp` (Go, subcommand `formspec mcp-serve`) tidak perlu berubah per provider — protokol MCP itu sendiri sudah bahasa/provider-agnostik by design.

**Penyimpanan API key — reuse open source, bukan built from scratch. Revisi: `zalando/go-keyring`, bukan `99designs/keyring`.** Untuk project baru tanpa kebutuhan enterprise yang nyata (1Password/Vault), `zalando/go-keyring` lebih tepat — lebih dirawat aktif (original `99designs/keyring` sudah ditinggalkan, perlu fork pihak ketiga), dan backend tambahan yang ditawarkan `99designs` belum relevan tanpa sinyal permintaan nyata (defer non-essential complexity).

**Bukan fallback file terenkripsi** — itu kompleksitas kripto (key derivation, penyimpanan passphrase) yang tidak sepadan untuk M1. Cukup dua tingkat cek yang praktis gratis:
```
1. OS keyring (zalando/go-keyring)   → kasus normal, desktop
2. Environment variable               → kasus headless/CI (Anthropic Go SDK sendiri
                                          sudah default baca ANTHROPIC_API_KEY)
3. (tidak ada apa pun)                → error jelas: minta set env var atau jalankan di desktop
```

Dibungkus di belakang interface kecil (`CredentialStore`) di kode sendiri — bukan menambah kompleksitas sekarang, tapi menjaga kalau nanti ada kebutuhan enterprise nyata (Vault, dst.), penggantian implementasi tidak menyentuh titik pemanggilan di banyak tempat.

**Klarifikasi:** Anthropic Console punya manajemen API key, tapi itu untuk developer/organisasi mengelola key mereka sendiri di dashboard — bukan fitur untuk aplikasi pihak ketiga (`formspec consult`) menyimpan BYOK key milik end-user secara lokal. Penyimpanan lokal tetap tanggung jawab FormSpec sendiri lewat mekanisme di atas.

**[Revisi, lihat FA-o]** Sebelumnya dinilai tidak ada library Go yang pas untuk "manage multi-provider" (`langchaingo`/`genkit-go` terlalu berat, alasan sama seperti menghindari agentic framework penuh) — sehingga sempat direncanakan built from scratch di Go. **Setelah `formspec-consult` pindah ke TypeScript**, ini tidak lagi berlaku: Vercel AI SDK 6 (`ToolLoopAgent` + provider adapter) menyediakan tepat kebutuhan ini sebagai library matang, bukan framework berat ala LangChain — jadi prinsip "manfaatkan open source dulu" tetap terpenuhi, cuma di ekosistem TypeScript untuk komponen ini secara spesifik.

### 2.3 Dua MCP Server — `formspec-local-mcp` dan `formspec-remote-mcp`

**Revisi pembagian (lihat FA-m):** bukan satu server dengan banyak tool, tapi dua server terpisah berdasarkan kepemilikan data — "tentang project saya" (lokal) vs "tentang ekosistem FormSpec" (pusat, Streamable HTTP).

```
formspec-local-mcp (stdio, subcommand `formspec mcp-serve`)
  list_kind_schemas(kind)            → schema resmi tiap kind (Entity, Form, Extension, dst)
  read_workspace_manifest()          → formspec.yaml — App, Navigation, Menu, Auth, Theme, uses: aktif
  list_installed_modules()           → modul yang SUDAH ter-install di modules/+vendors/ project ini
  read_module_spec(module,kind,name) → detail satu spec
  propose_spec_file(path, content)   → tulis draft + validasi otomatis (bagian 2.4)
  apply_draft(session, file)         → pindahkan draft ke lokasi asli (FA-w), guard read-only vendors/ (FA-x)
  validate_spec / check_naming_conflict
  restart_server() / get_server_status() / stop_server()  (FA-w)
  list_skills() / read_skill(name)   → FormSpec Skill, dibundel bersama instalasi formspec (FA-k)

formspec-remote-mcp (Streamable HTTP, hosted FormSpec Cloud)
  list_business_templates()          → pattern bisnis yang sudah dikenal (bagian 4), FA-u
  search_modules_registry(query)     → pure retrieval (pgvector) module dari registry publik, top-K berperingkat
  get_module_detail(name)            → detail satu module (termasuk skills_for_ai)
```

AI memanggil tools ini sebelum menulis YAML — bukan mengandalkan ingatan model soal spec FormSpec. Ini juga menjaga posisi netral-vendor FormSpec: konsultasi tidak terkunci ke satu provider AI, karena groundingnya ada di MCP server, bukan di model tertentu.

**Catatan pembagian index vs retrieval (lihat FA-u):** `list_business_templates()` kembalikan seluruh daftar apa adanya dalam satu response (katalog kecil, terkurasi, FA-b) — LLM konsultan yang mencocokkan, bukan embedding. `search_modules_registry` sebaliknya **tidak** bisa dikirim utuh — katalog module registry berpotensi besar dan terus tumbuh (community tier akan dibuka), jadi perlu narrowing lewat vector similarity (pgvector, di `formspec-remote-mcp`) sebelum kembalikan top-K kandidat berperingkat — bukan panggilan LLM di dalam tool, supaya tidak dobel biaya reasoning dan tidak merusak netralitas provider. Consultant LLM di luar yang menimbang mana paling cocok untuk konteks bisnis yang sedang didiskusikan — tool tidak memutuskan sepihak.

**MCP server sebagai index/interface, bukan tempat eksekusi baru.** Perlu ditegaskan: setiap tool di `formspec-local-mcp` (`validate_spec`, `check_naming_conflict`, dst.) tidak membawa logic baru — semuanya cuma pembungkus terstruktur di atas `formspec-core` (bagian 2.5) yang sudah ada. Beda MCP tool dengan "AI shell out ke CLI `formspec validate`" bukan soal *di mana* logic-nya jalan (sama-sama lokal), tapi soal **kontrak antarmuka**: MCP tool punya JSON Schema input/output terstruktur yang langsung dipakai tool-calling API, sementara shell out butuh AI menyusun string command bebas dan menafsirkan stdout/stderr teks bebas — jauh lebih rawan salah dibanding pola call-dengan-schema yang memang didesain untuk LLM.

Konsekuensinya: **`formspec-consult` sendiri juga tetap lewat MCP** untuk kedua server ini, bukan panggil `formspec-core` langsung in-process meski sama-sama dibuat FormSpec. Ini supaya cuma ada satu jalur integrasi ("bagaimana AI mengakses kapabilitas FormSpec") yang dipakai baik built-in client maupun client eksternal (Claude Code/Cursor, FA-o) — kalau built-in client punya jalur pintas sendiri yang beda, itu risiko divergen yang sama seperti disebutkan di FA-p, kali ini soal *cara akses*, bukan soal *validitas*. Overhead serialisasi stdio/HTTP-nya diabaikan karena tool-tool ini dipanggil beberapa kali per sesi, bukan hot path performa tinggi.

### 2.4 Validation Gate — Wajib di Server, Bukan Bergantung Client

**Revisi penting:** karena client yang dipakai bisa jadi eksternal (Claude Code/Cursor, bagian 2.1) yang tidak FormSpec kontrol, validasi **tidak boleh** bergantung pada CLI `formspec consult` sendiri yang memanggilnya — harus jadi bagian dari perilaku tool itu sendiri di server, supaya client apa pun (built-in atau eksternal) otomatis dapat proteksi yang sama.

Solusinya bukan expose tool "tulis file" mentah, tapi tool composite `propose_spec_file(path, content)`:
```
propose_spec_file(path, content)
  → server tulis ke .formspec/consult/{session}/draft/{path}
  → server jalankan validate_spec secara internal
  → return { written: true, validation: {...} } ke LLM
```
Validasi jadi bagian dari *apa yang dilakukan tool*, bukan langkah terpisah yang berharap LLM mau memanggilnya sendiri. Ini mitigasi utama untuk model yang lemah/berhalusinasi atau client yang tidak disiplin memanggil `validate_spec`: kualitas hasil akhir tidak bergantung pada kedisiplinan LLM/client mengikuti instruksi.

### 2.5 `validate_spec` — Lokal, Reuse Library yang Sama dengan `formspec-server` Boot

**Penting untuk konsisten dengan D-a** (spec diinterpretasi saat boot, tidak ada compile): `validate_spec` di `formspec-local-mcp` **bukan** logic terpisah yang ditulis khusus untuk MCP tool ini. Kalau dibuat terpisah, muncul risiko dua implementasi "apa itu spec valid" diam-diam divergen — spec yang lolos `validate_spec` saat consult tapi gagal saat `formspec-server` betulan boot (atau sebaliknya), kelas bug paling susah dilacak.

Solusi: satu package Go internal (`formspec-core`), tiga pemanggil:

```
formspec-server              → boot betulan, load spec, serve traffic
formspec apply --dry-run     → CLI, load spec, validasi, TANPA serve traffic
formspec-local-mcp          → panggil fungsi validasi yang SAMA, in-process
  (validate_spec tool)       (bukan HTTP call ke server yang jalan, bukan reimplementasi)
```

`validate_spec` panggil fungsi Go langsung, in-process, sejalan `formspec-local-mcp` yang memang sudah lokal (bagian 2.1) — tidak ada server terpisah yang perlu di-spawn untuk sekadar validasi.

**Batasan scope — structural, bukan runtime data:** `validate_spec` cuma cek hal statis (schema, referensi `depends_on`/Extension/Override valid, bentrok nama — bagian 3 technical note vendoring). **Di luar scope:** validasi data runtime yang butuh instance `formspec-server` + DB beneran jalan (mis. "apakah `natural_key` ini sudah dipakai row lain di produksi") — itu bukan urusan sesi consult.

**Pengecualian kecil terhadap "semua offline":** verifikasi signature/trust-tier vendor module (official/verified/community) berpotensi butuh cek revocation list ke registry pusat — beda dari validasi spec murni yang 100% offline. Kalau draft menyentuh module vendor yang perlu diverifikasi ulang trust tier-nya, itu jalur terpisah yang boleh online, dan harus eksplisit (bukan diam-diam ikut terjadi saat `validate_spec` dipanggil).

---

## 3. Penyimpanan Sesi & Mekanisme Diff

```
project/
  .formspec/consult/
    2026-07-18-barbershop-initial/
      transcript.md          # log percakapan penuh
      discovery-summary.md   # ringkasan alur, bahasa awam, untuk konfirmasi owner
      draft/
        modules/barbershop/service.resource.yaml
        modules/barbershop/visit.resource.yaml
```

- `formspec consult` → sesi baru dibuat, transcript ditulis real-time.
- Draft spec ditulis ke `draft/`, mencerminkan struktur folder project asli (`modules/`, atau `overrides/`/`Extension` kalau targetnya vendor module).
- `formspec consult diff` → bandingkan `draft/` vs `modules/`/`vendors/` yang sebenarnya (unified diff biasa, karena keduanya YAML — tidak butuh mekanisme diff khusus, sejalan prinsip tidak ada tahap compile).
- Developer accept/reject per file — accept memindahkan file dari `draft/` ke lokasi asli project.

---

## 4. Industry Template — Supaya AI Tidak Mulai dari Kosong

Template pattern bisnis (bukan spec lengkap), dirujuk lewat `list_business_templates()`, mempercepat discovery dengan pertanyaan pemandu:

```yaml
# templates/appointment-based-service.pattern.yaml
pattern: appointment-based-service
applies_to: [barbershop, salon, klinik, spa]
probing_questions:
  - "Apakah pelanggan booking dulu, atau bisa walk-in tanpa janji?"
  - "Apakah tiap staff punya harga/skill berbeda, atau harga fixed per layanan?"
  - "Ada komisi staff dari tiap transaksi?"
  - "Jual produk juga (retail), atau murni jasa?"
candidate_entities:
  - Customer   [characteristics: master]
  - Staff      [characteristics: master]
  - Service    [characteristics: master]
  - Appointment [characteristics: transaction, state_machine]
  - Sale       [characteristics: transaction]
```

**[Revisi, lihat FA-m]** Template ini hidup di `formspec-remote-mcp` (hosted FormSpec Cloud, Streamable HTTP) — bukan di `formspec-local-mcp` seperti draf sebelumnya. Alasan revisi: template itu meta-knowledge ekosistem FormSpec (dikurasi FormSpec, berpotensi multi-kontributor nanti sesuai FA-b), sama kelasnya dengan module registry — bukan sesuatu yang spesifik ke satu workspace developer, jadi ditaruh sejak awal di server yang sama dengan `search_modules_registry`, supaya tidak perlu migrasi arsitektur saat community template dibuka nanti. Tetap bukan bagian mekanisme `modules/`/`vendors/` — ini meta-knowledge untuk discovery, bukan module yang di-install ke aplikasi.

Sesuai keputusan bagian 1: seluruh template awal dari FormSpec. Community-contributed template adalah kemungkinan masa depan.

---

## 5. Module Index untuk AI — Reuse Module Vertikal

Supaya AI bisa mengusulkan "pakai module X" alih-alih reinvent, `ModulePublish` (kind yang sudah ada untuk trust tier) diusulkan diperluas dengan blok `ai_index`:

```yaml
ai_index:
  category: payment
  features: [charge, refund, webhook_callback, virtual_account]
  integration_pattern: |
    depends_on: {module: payment-gateway-xendit, access: service_call}
  skills_for_ai: |
    Pakai module ini kalau bisnis butuh terima pembayaran online.
    Integrasi umum: buat Transaction yang refer ke charge service module ini,
    listen event payment.completed untuk update status Sale lokal.
```

`list_installed_modules()` di `formspec-local-mcp` mengembalikan index ini untuk module yang sudah ter-install (`vendors/`). Untuk module dari registry publik yang belum di-install (diusulkan sebagai `formspec module install` baru), index yang sama diambil lewat `search_modules_registry`/`get_module_detail` di **`formspec-remote-mcp`** (FA-m) — bukan server yang sama, karena registry publik itu data bersama lintas developer, bukan spesifik satu workspace.

### 5.1 Risiko: `skills_for_ai` sebagai Untrusted Input

`skills_for_ai` adalah teks dari vendor pihak ketiga. Begitu community tier dibuka, teks ini jadi **data tidak terpercaya** yang masuk ke sesi AI — berisiko prompt injection (deskripsi module dibuat menyesatkan supaya AI "disuruh" merekomendasikan module tertentu, atau menyisipkan instruksi tersembunyi yang dibaca AI sebagai perintah, bukan deskripsi).

Karena rencana saat ini "semua dari FormSpec dulu, community nanti" (bagian 1), risiko ini otomatis termitigasi di M1/M2 — seluruh index masih first-party dan terpercaya. Tapi ini **wajib dicatat sebagai prasyarat** sebelum community module dibuka: `skills_for_ai` dari tier non-official harus diperlakukan sebagai data untuk dibaca AI, bukan instruksi untuk diikuti — pola yang sama dengan penanganan konten pihak ketiga tidak terpercaya di tempat lain.

---

## 6. Context Injection: Manifest Ringan di Awal, Detail On-Demand

### 6.1 Kenapa Bukan Dump Spec+Doc Penuh

Mengirim seluruh Core Basic + Extended Spec sebagai context statis di setiap sesi **tidak optimal**, karena dua alasan berbeda dari kebutuhan FormSpec Skill:

| Yang dibutuhkan | Kenapa bukan spec mentah |
|---|---|
| **Procedural/behavioral** — cara jadi konsultan yang baik, kapan panggil `validate_spec`, urutan Discovery → Proposal → Draft | Bukan fakta yang ada di Spec — ini instruksi cara *memakai* Spec. Spec sendiri tidak mengajari AI cara berdiskusi. |
| **Factual/schema** — nama field persis, aturan `characteristics`, whitelist per kind | Dump penuh sebagai context statis menurunkan precision begitu dokumen makin panjang, dan rawan basi begitu versi Spec berikutnya rilis tanpa context yang sudah di-cache ikut update. |

FormSpec Skill dan Spec dua hal yang berbeda kegunaan — bukan salah satu pengganti yang lain.

### 6.2 Mekanisme Terkini: Semua On-Demand via Tool Call, Bukan Inject di Awal

**[Revisi total dari draf awal]** Versi sebelumnya (index kecil di-inject sekali saat sesi mulai) sudah disuperseded oleh FA-j — static injection dihindari sama sekali, termasuk untuk katalog kecil, karena tetap dibayar di setiap pesan sepanjang sesi dan memaksa katalog "kecil selamanya". Mekanisme final:

```
Auto-invoke deterministik (dipicu client, sekali di awal sesi — BUKAN static inject,
tetap tool call biasa, cuma dipicu otomatis bukan inisiatif LLM, lihat FA-v/FA-bb):
  - read_workspace_manifest() + list_installed_modules()  [formspec-local-mcp]  → FA-v
  - list_skills()                                         [formspec-local-mcp]  → FA-bb

On-Demand (dipanggil LLM sendiri, kapan pun dibutuhkan di tengah percakapan):
  - list_kind_schemas("Entity")            [formspec-local-mcp]   → schema penuh, saat authoring Entity
  - list_business_templates()              [formspec-remote-mcp]  → seluruh daftar, katalog kecil (FA-u)
  - search_modules_registry(query)         [formspec-remote-mcp]  → top-K narrowing, katalog besar (FA-u/FA-m)
  - get_module_detail(name)                [formspec-remote-mcp]  → ai_index penuh termasuk skills_for_ai
  - read_skill(name)                       [formspec-local-mcp]   → isi skill penuh, saat topik cocok (FA-bb)
  - validate_spec(yaml)                    [formspec-local-mcp]   → hasil validasi
```

Alasan: dua budget token yang beda (2.1.1) — deklarasi tool tetap kecil-konstan, isi data cuma diambil saat dipanggil. Mitigasi risiko `skills_for_ai` (bagian 5.1) otomatis ikut lebih baik: teks vendor pihak ketiga cuma masuk context kalau `get_module_detail` benar-benar dipanggil untuk module itu, bukan didorong penuh ke semua sesi — tetap dibungkus sebagai *data*, bukan instruksi.

### 6.3 Format FormSpec Skill

YAML frontmatter + Markdown body:

```yaml
---
name: entity-authoring
description: >
  Gunakan skill ini saat membuat atau mengubah Entity kind — field types,
  characteristics (master/transaction/reference), natural_key, state_machine.
  Trigger: percakapan menyebut "tambah field", "buat entity baru", "state machine".
applies_to_kind: [Entity, Extension]
min_core_spec_version: "0.1.9"
---

# Entity Authoring

... isi lengkap: aturan detail, contoh, edge case ...
```

- Saat sesi mulai: AI hanya diberi `name` + `description` semua skill (index kecil).
- Begitu topik percakapan cocok dengan salah satu `description`, AI baca isi lengkap file skill itu.
- `min_core_spec_version` menandai skill terikat ke versi Core Spec tertentu — dipakai untuk deteksi skill basi kalau Core Spec naik versi tapi skill belum diupdate (lihat pertanyaan terbuka).
- Granularitas: **dipecah per kebutuhan** (skill "Entity authoring", "Form layout", "Extension authoring" terpisah), bukan satu skill besar "FormSpec Consultant" — supaya footprint awal tetap kecil dan tidak semua behavior ter-load kalau percakapan baru sampai tahap Discovery.

---

## 7. Ringkasan Keputusan

| # | Topik | Keputusan |
|---|---|---|
| FA-a | Titik masuk produk | CLI (`formspec consult`) dulu, FormSpec Studio adalah upgrade jangka menengah. |
| FA-b | Sumber industry template | 100% dari FormSpec di awal; community-contributed adalah kemungkinan masa depan, bukan target M1/M2. |
| FA-c | Deteksi role percakapan | Tidak perlu. AI adaptif secara alami, tanpa mode switch eksplisit business-owner/developer. |
| FA-d | Objek diff | Spec-ke-spec (bukan spec-ke-kode), karena FormSpec tidak punya tahap compile — spec adalah implementasi. |
| FA-e | Penyimpanan sesi | Folder `.formspec/consult/{session}/` — transcript, discovery-summary, draft/ (mencerminkan struktur project asli). |
| FA-f | Provider LLM | BYOK, minimum capability bar (tool-calling + context), daftar provider tervalidasi — bukan klaim semua LLM setara. |
| FA-g | Safety net terhadap LLM lemah | `propose_spec_file` (composite tool di server) jalankan `validate_spec` otomatis saat menulis draft — bukan bergantung CLI/LLM memanggil `validate_spec` terpisah. Proteksi berlaku sama untuk client mana pun (built-in atau eksternal). |
| FA-h | Reuse module vertikal | `ModulePublish` diperluas dengan blok `ai_index` (category, features, integration_pattern, skills_for_ai). |
| FA-i | Risiko `skills_for_ai` community tier | Diperlakukan sebagai untrusted data, bukan instruksi — prasyarat wajib sebelum community module dibuka. |
| FA-j | **[REVISI]** Mekanisme context injection | ~~Manifest ringan di-inject saat sesi mulai untuk katalog kecil~~ — **disuperseded**. Tidak ada lagi context yang di-inject permanen untuk katalog apa pun (skill, template, module). Satu pola untuk semua: **query on-demand via MCP tool**, konsisten dengan FA-q (MCP = index/interface). Alasan revisi: static index dibayar di setiap pesan sepanjang sesi terlepas relevan atau tidak, dan memaksa katalog "kecil selamanya" — bertentangan dengan rencana template komunitas (FA-b). Detail per skala katalog di FA-u. |
| FA-k | Format FormSpec Skill | YAML frontmatter (`name`, `description`, `applies_to_kind`, `min_core_spec_version`) + Markdown body. Dipecah per kebutuhan (bukan satu skill besar). Mengikuti pola terpadu FA-u: daftar skill di-query via tool saat dibutuhkan (bukan diinjeksi permanen di awal sesi); body penuh dibaca hanya saat description cocok topik. |
| FA-l | **[SUPERSEDED oleh FA-o]** Riset SDK — riwayat keputusan | ~~Anthropic Go SDK sebagai referensi implementasi pertama untuk `formspec consult`~~ — **tidak lagi berlaku** setelah FA-o: `formspec-consult` diimplementasikan TypeScript + Vercel AI SDK, bukan Go, jadi `toolrunner`/`client.Beta` Anthropic Go SDK tidak dipakai untuk komponen ini. Riset di bawah ini dipertahankan sebagai catatan sejarah (alasan kenapa opsi Go dipertimbangkan lalu tidak dipilih), bukan keputusan aktif: SDK Anthropic Go ternyata punya `toolrunner` (multi-turn tool loop, Anthropic-only, tidak multi-provider) dan `client.Beta` (integrasi MCP + context management). Referensi arsitektur multi-provider yang sempat dipertimbangkan: **OpenCode** (github.com/sst/opencode, MIT, 160rb+ stars) — TUI-Go + server-Bun/JS, tidak bisa di-import sebagai package Go. **Temuan tambahan (penting):** `opencode-sdk-go`/`@opencode-ai/sdk` yang dipublish OpenCode itu client REST API untuk remote-control OpenCode SERVER yang sudah jalan (session/file/event service) — bukan library untuk embed agent-loop OpenCode ke aplikasi lain. Lebih penting lagi: **arsitektur internal OpenCode sendiri (`SessionPrompt.loop()`) memanggil model AI lewat Vercel AI SDK** — jadi OpenCode bukan alternatif dari Vercel AI SDK, dia konsumen Vercel AI SDK juga, dengan lapisan aplikasi tambahan (TUI, session DB Drizzle/SQLite, tooling instalasi lintas platform, integrasi Slack/VS Code) yang tidak relevan untuk `formspec-consult`. Kandidat Go-native lain (Jetify SDK, Go port OpenAI Agents SDK) dinilai terlalu alpha atau terlalu berat. Keputusan aktif sekarang ada di **FA-o**: `formspec-consult` pakai Vercel AI SDK (`ToolLoopAgent` + MCP client + 25+ provider adapter) di TypeScript, compile via `bun build --compile` — fondasi yang sama dengan yang dipakai OpenCode sendiri di lapisan intinya, cuma tanpa lapisan aplikasi OpenCode yang tidak kita perlukan. **Catatan yang tetap berlaku:** jangan disamakan dengan Claude Agent SDK (dulu Claude Code SDK) — itu framework lebih berat, billing terpisah, bukan yang dipakai di sini (Vercel AI SDK adalah library berbeda, BYOK murni lewat token API biasa). |
| FA-m | **[Revisi]** Pembagian kapabilitas dua MCP server | **Revisi dari draf sebelumnya** (yang sempat taruh template di lokal): prinsip pemisahan bukan "kecil vs besar" tapi **milik siapa data itu**. `formspec-local-mcp` (stdio, per developer): workspace manifest, modul TERINSTALL, `validate_spec`, `apply_draft`, `restart_server`, **FormSpec Skill** (`list_skills`/`read_skill` — keputusan final: tetap lokal, terikat `min_core_spec_version` yang dicek terhadap Core Spec yang sudah dimuat lokal, ikut siklus rilis `formspec`, beda alasan dari template/module) — "tentang project saya". `formspec-remote-mcp` (Streamable HTTP, hosted FormSpec Cloud): **business template DAN vertical module registry** (`search_modules_registry`) — dua-duanya "tentang ekosistem FormSpec", dikurasi FormSpec sekarang tapi berpotensi multi-kontributor nanti (FA-b) — disamakan arsitekturnya sejak awal supaya tidak perlu migrasi ulang saat community template/module dibuka, konsisten dengan alasan revisi FA-j/FA-u. `formspec-consult` kelola dua koneksi MCP: stdio ke lokal (selalu aktif sepanjang sesi) + Streamable HTTP ke remote (dipanggil saat butuh browse template/cari modul). |
| FA-n | Reasoning "module mana yang cocok" | Dilakukan consultant LLM di luar tool (yang sudah baca konteks Discovery/Proposal), bukan LLM di dalam tool server — tool cuma kembalikan top-K kandidat berperingkat, tidak memutuskan sepihak. Menjaga netralitas provider dan menghindari biaya reasoning ganda. |
| FA-o | **[Revisi]** Arsitektur client & pemisahan bahasa | `formspec consult` adalah MCP client mandiri (built-in tool-use loop + LLM API langsung, BYOK) — 100% lokal, tidak bergantung Claude Code/Cursor/VS Code. Attach ke client MCP eksternal bersifat opsional/bonus reuse, bukan prasyarat — karena spec ada di local folder dan tidak boleh keluar mesin developer untuk sekadar divalidasi. **Revisi implementasi bahasa:** karena tooling multi-provider+MCP jauh lebih matang di ekosistem TypeScript (Vercel AI SDK 6 — `ToolLoopAgent`, MCP client stabil, 25+ provider) dibanding Go saat ini (lihat FA-l), `formspec-consult` diimplementasi TypeScript + Vercel AI SDK, di-compile jadi binary standalone via `bun build --compile` — bukan Go. `formspec` (CLI utama: generate, module install, apply, server) **tetap 100% Go, tidak berubah**. Total dua artifact: `formspec` (Go) dan `formspec-consult` (TS/Bun-compiled), terhubung lewat MCP stdio (bahasa-agnostik by design) — bukan monolith satu bahasa, tapi juga bukan pencampuran kode dalam satu binary. Kontributor yang cuma kerja di `formspec-core`/`formspec-server`/module system tidak pernah perlu sentuh TS. **`formspec-local-mcp` BUKAN binary terpisah** — cukup subcommand dari `formspec` (mis. `formspec mcp-serve`), karena dia cuma pembungkus tipis atas `formspec-core` (FA-q), sama-sama Go, tidak perlu build/release pipeline sendiri. Client MCP eksternal (VS Code/Claude Code/Cursor) dan `formspec-consult` (built-in) sama-sama spawn command yang identik (`formspec mcp-serve`) sebagai child process via stdio — satu implementasi server, dua cara pakai. |
| FA-p | Lokasi `validate_spec` | Lokal, in-process — panggil package Go yang sama dengan yang dipakai `formspec-server` boot dan `formspec apply --dry-run` (bukan reimplementasi terpisah, bukan network call). Scope structural/static saja, bukan validasi data runtime. Verifikasi trust-tier/signature vendor module adalah jalur terpisah yang boleh online, eksplisit. |
| FA-q | Peran MCP server | Index/interface terstruktur untuk kapabilitas AI (JSON Schema in/out), bukan tempat logic baru — semua tool cuma pembungkus `formspec-core`. Built-in client `formspec consult` tetap lewat MCP (bukan panggil `formspec-core` langsung), supaya satu jalur integrasi dipakai konsisten oleh built-in maupun client eksternal. |
| FA-r | Model ringan vs berat | Opsional/extensible di `LLM Provider Layer`, bukan wajib di M1. Default M1 satu model untuk semua tahap (Discovery, Proposal, draft-writing). Slot model kedua ("fast model") bisa ditambah nanti kalau ada sinyal nyata biaya inference jadi masalah — bukan dioptimasi di depan tanpa data pemakaian. |
| FA-s | Penyimpanan API key (BYOK) | `github.com/zalando/go-keyring` (bukan `99designs/keyring` — belum ada kebutuhan enterprise nyata untuk backend tambahan). Tiered: OS-native keyring dulu, lalu environment variable (bukan file terenkripsi — kompleksitas kripto belum sepadan untuk M1). Dibungkus interface `CredentialStore` supaya mudah diganti nanti. Bukan fitur dari Anthropic Console — itu untuk developer kelola key sendiri, bukan penyimpanan lokal BYOK end-user. |
| FA-t | Multilingual/sinonim pada pencarian | pgvector sendiri agnostik bahasa — kemampuan lintas-bahasa bergantung pada model embedding, bukan pgvector. Katalog kecil (business template) tidak kena isu ini karena hasil tool call dibaca langsung oleh LLM konsultan (LLM natural menangani "tukang potong rambut" → barbershop, lihat FA-u). Hanya module registry (pure retrieval) yang benar-benar bergantung pada kualitas embedding model. Model disarankan: Voyage AI (rekomendasi resmi Anthropic, keluarga `voyage-3` multilingual) atau BGE-M3 (open source, kuat multilingual, bisa self-host). Karena jargon domain (mis. "buku besar"/"general ledger") berisiko kurang presisi kalau cuma andalkan kedekatan semantik (Bahasa Indonesia kemungkinan kurang terwakili di training data dibanding Inggris), `ai_index` perlu field tambahan `aliases: [...]` yang diisi eksplisit oleh publisher modul dan ikut di-embed bersama deskripsi — hybrid semantic + sinonim eksplisit, bukan murni berharap model menebak sinonim lintas-bahasa. |
| FA-u | Katalog kecil vs besar — pola tool-call terpadu | **Katalog terbatas** (business template saat ini): tool (mis. `list_business_templates()`) kembalikan SELURUH daftar (nama+deskripsi+alias) apa adanya dalam satu response — tanpa embedding, tanpa AI tambahan di dalam tool. LLM konsultan yang sudah jalan di sesi itu yang membaca dan mencocokkan langsung, memanfaatkan kemampuan multilingual-nya secara gratis (tanpa biaya inference tambahan). **Katalog tak terbatas/terus tumbuh** (module registry): tool tetap perlu tahap narrowing (embedding/pgvector atau alias-keyword) sebelum kembalikan top-K kandidat — kandidat tidak muat dikirim utuh berapa pun besar LLM-nya. Reasoning akhir/pemilihan tetap di LLM konsultan luar (FA-n), bukan di dalam tool. **AI di dalam MCP tool untuk retrieval ditolak sebagai pola default**: tidak menghilangkan kebutuhan narrowing di skala besar (context limit tetap berlaku untuk LLM manapun), menduplikasi biaya inference (2x panggilan LLM per pencarian), dan merusak netralitas provider `formspec-local-mcp` (tool butuh API key/model sendiri, independen dari pilihan client) — bertentangan langsung dengan alasan FA-n. Katalog kecil tumbuh besar → migrasi mulus ke pola narrowing yang sama, bukan re-arsitektur ulang (batas pertumbuhan sudah tidak ada sejak FA-j direvisi). |
| FA-v | Workspace-awareness | `formspec consult` bisa baca current folder lewat tool read-only: `read_workspace_manifest()` (`formspec.yaml` — App, Navigation, Menu, Auth, Theme, `uses:` aktif), `list_installed_modules()` (gabungan `modules/`+`vendors/`+`formspec.lock`, status aktif/nonaktif), `read_module_spec(module, kind, name)` (detail satu spec). Tidak ada tool "describe_workspace" dengan narasi baked-in — pertanyaan seperti "sistem apa yang sedang dibuat" dijawab LLM konsultan dengan mensintesis hasil tool-tool di atas, konsisten pola FA-n/FA-u. Untuk modul di `vendors/` (berpotensi community-tier/untrusted per FA-i), tool hanya ekstrak field metadata (nama, versi, sumber, deskripsi) — tidak dump mentah isi spec sebagai teks bebas ke context. **[Revisi]** `read_workspace_manifest()` + `list_installed_modules()` dipanggil **otomatis saat sesi consult mulai** (dipicu deterministik oleh built-in client, bukan bergantung AI ingat memanggilnya — pola safety-net ala FA-g), sekali di awal sesi saja (bukan per-pesan). Ini tidak melanggar FA-j/FA-u karena workspace manifest terbatas by nature (satu project, bukan katalog global) dan nyaris selalu relevan — beda karakter dari katalog template/module yang ditolak untuk static injection. |
| FA-w | Aksi operasional dari dalam sesi | Dua tool: `apply_draft(session, file)` (pindahkan draft ke lokasi asli di workspace) dan `restart_server()`/`get_server_status()`/`stop_server()` (composite ala FA-g: `restart_server()` jalankan `validate_spec` dulu, tolak kalau invalid — sekaligus sinyal runtime tambahan di luar scope structural FA-p). **[Revisi kebijakan konfirmasi]** Operasi lifecycle server (start/restart/stop) di mode Dev: **tanpa konfirmasi** — proses lokal, risiko rendah, gampang diulang. `apply_draft`: **tanpa konfirmasi interaktif juga**, DENGAN SYARAT auto-snapshot sebelum tulis sudah ada (FA-y) — diff tetap ditampilkan untuk visibilitas, tapi tidak blocking; kesalahan trivial di-undo lewat FA-y, jadi confirmation dan safety-net saling menggantikan, tidak perlu dua-duanya. |
| FA-x | Guard read-only `vendors/` | Semua tool tulis (`apply_draft` khususnya) menolak eksplisit kalau target path ada di bawah `vendors/` — bukan cuma konvensi dokumentasi. Kalau AI/developer coba modifikasi konten vendor, tool error dan arahkan ke mekanisme yang benar: Extension (D-i, field baru) atau Override/shadow-copy (D-h, presentasi) dari Technical Note Module Vendoring — konsisten dengan D-b (vendors/ read-only by design). |
| FA-y | Snapshot & undo perubahan | **M1 (scope minimum):** auto-backup file-level sebelum ditimpa `apply_draft`, disimpan di `.formspec/consult/{session}/undo/` — bukan git, cukup copy sederhana. Fungsinya spesifik: memungkinkan "undo perubahan AI terakhir" satu langkah, yang jadi syarat pelonggaran konfirmasi di FA-w. **Bukan** sistem versioning penuh dari nol (multi-snapshot bernama, navigasi bebas maju-mundur) — itu over-engineering untuk kebutuhan M1. **Kalau nanti dibutuhkan lebih** (banyak snapshot, navigasi bebas antar titik waktu): reuse git yang sudah jadi prasyarat teknis project (provenance/Studio, catatan horizon) lewat ref/branch terisolasi (tidak mengotori commit history asli developer) + command tipis (`formspec consult snapshot create/list/restore`) — bukan reinvent version control sendiri. Prinsip "manfaatkan open source dulu" berlaku di sini: git sudah ada dan sudah diasumsikan tersedia. |
| FA-z | Skill — standar provider atau bukan | **Bukan** masalah standar seperti tool-call format (FA-l/2.2) — Skill (FA-k) tidak pernah jadi konsep di level API provider mana pun, murni konvensi client-side: index/isi-lengkap dikirim sebagai teks biasa lewat tool_result, tidak ada field khusus "skill" yang perlu dipahami Anthropic/OpenAI/DeepSeek. Terinspirasi pola SKILL.md Claude Code, tapi itu fitur aplikasi Claude Code, bukan fitur API Claude — `formspec consult` reimplementasi sendiri di control flow-nya, tidak bergantung Claude Code. Tidak perlu adapter per provider untuk ini (beda dari kasus tool-call format). |
| FA-bb | Mekanisme relevansi skill | Penilaian "skill mana yang cocok topik" dilakukan LLM konsultan yang SAMA, di turn yang SAMA — bukan sesi/LLM terpisah. Sama seperti katalog kecil lain (FA-u): `list_skills()` kembalikan index kecil (name+description), LLM baca dan cocokkan langsung via reasoning biasa (tanpa embedding, tanpa nested LLM call) karena katalog skill kecil-terkurasi (100% FormSpec-authored, sejalan FA-b). Sesi/LLM klasifikasi terpisah ditolak dengan alasan sama seperti FA-u: menduplikasi biaya inference tanpa manfaat, karena deskripsi skill cukup pendek untuk dinilai LLM utama tanpa panggilan tambahan. **Pemicu (kapan dicek):** tidak bergantung inisiatif LLM semata (risiko lupa di model lemah/sesi panjang, FA-f) — dipancing deterministik dari client: (1) `list_skills()` otomatis dipanggil saat sesi mulai, bareng workspace manifest (FA-v); (2) re-cek skill yang cocok otomatis jadi bagian composite tool `propose_spec_file` sebelum draft ditulis (pola safety-net ala FA-g yang sudah auto-jalankan `validate_spec`). Mekanisme `read_skill(name)` sendiri cuma tool MCP biasa (loop sama seperti 2.1.1) — `formspec consult` tidak perlu "paham" makna skill, cuma routing mekanis berdasar nama tool + skema input. **Input** (`name: string`) tetap wajib skema JSON, tapi **output** (isi skill) bebas — teks Markdown mentah, tidak perlu dibungkus JSON terstruktur, karena skill dirancang dibaca LLM sebagai instruksi bahasa natural, bukan diparse program deterministik (beda dari isi Entity/Form spec yang wajib ikut skema YAML Core Spec). |
| FA-aa | Kompresi history sesi panjang | Formalisasi dari pola yang sudah dipakai project ini sendiri (numbered decision log D1-D48/FA-a...FA-y, dan compaction sesi ini sendiri): (1) transcript penuh selalu tersimpan di `.formspec/consult/{session}/transcript.md` (FA-e); (2) pantau token usage per turn, picu kompresi mendekati ambang context window (mis. 70-80%); (3) saat kompresi, distilasi turn lama jadi ringkasan terstruktur mirip format `discovery-summary.md` (fakta/keputusan yang sudah disepakati) — bukan dialog eksploratif mentah — turn lama diganti ringkasan ini di context aktif, transcript penuh tetap di disk; (4) sediakan tool baca-balik ke transcript penuh untuk detail spesifik yang tidak tertangkap ringkasan, bukan berharap ringkasan sempurna dari awal. Prinsip: yang dikompresi adalah proses eksploratif, keputusan final tetap utuh — konsisten dengan pemisahan transcript vs discovery-summary yang sudah ada sejak FA-e. **Catatan (lihat juga FA-l):** Anthropic Go SDK `client.Beta` sudah punya fitur context management (automatic compaction, tool use clearing, thinking truncation) — perlu dievaluasi apakah bisa dipakai langsung sebagai basis mekanisme di atas, bukan dibangun manual dari nol; masih beta jadi perlu verifikasi stabilitas sebelum bergantung penuh. |

---

## 8. Pertanyaan Terbuka untuk Iterasi Berikutnya

- Format persis `discovery-summary.md` dan `proposal` (diagram Mermaid? Naratif murni?) — belum diputuskan.
- Minimum capability bar untuk LLM (FA-f) — perlu didefinisikan konkret: benchmark tool-calling seperti apa, context window minimum berapa token?
- Bagaimana `formspec consult diff` menangani draft yang menyentuh module vendor (lewat Extension atau shadow-copy) — apakah draft otomatis diarahkan ke `overrides/`/`Extension` yang sesuai, atau developer harus pindahkan manual?
- Governance untuk `ai_index` — siapa yang berhak isi field `skills_for_ai` (vendor sendiri saat publish, atau ada proses review terpisah), terutama untuk trust tier `verified` yang di bawah `official`?
- Apakah sesi `.formspec/consult/` di-commit ke git (riwayat diskusi ikut ter-audit) atau di-gitignore (dianggap scratch, bukan artifact permanen)?
- Bagaimana FormSpec Skill di-versioning secara mekanis — `min_core_spec_version` di frontmatter cukup untuk deteksi basi (bagian 6.3), atau perlu proses CI yang menjalankan validasi otomatis setiap Core Spec rilis versi baru?
- Siapa yang menulis FormSpec Skill pertama kali (entity-authoring, form-layout, extension-authoring, dst) — tim FormSpec langsung, atau didistilasi dari technical note yang sudah ada (mis. technical note ini sendiri jadi bahan baku skill "module-vendoring")?
- Apakah manifest project state (bagian 6.2 — isi `modules/`+`vendors/`+`uses:` saat ini) perlu diringkas juga kalau project sudah besar (puluhan module), atau selalu dikirim penuh karena ukurannya biasanya kecil dibanding Spec/skill?
- **[Terjawab sebagian oleh revisi FA-m]** Embedding pgvector dihitung/disimpan di `formspec-remote-mcp` (FormSpec Cloud, terpusat) — bukan direplikasi ke tiap `formspec-local-mcp` lokal. Yang masih terbuka: apakah hasil pencarian boleh di-cache sementara di sisi lokal (mis. per sesi) untuk kurangi round-trip jaringan berulang, dan bagaimana autentikasi/rate-limit untuk `formspec-remote-mcp` (perlu API key FormSpec Cloud terpisah dari BYOK LLM, atau bebas akses untuk baca/search)?
- Kalau developer menjalankan `formspec consult` built-in (FA-o) tanpa client eksternal, bagaimana UX render diff di terminal — cukup unified diff teks, atau perlu TUI (mis. pakai library seperti `bubbletea`)?
- Dukungan resources/prompts MCP di client eksternal (Claude Code/Cursor/dst.) perlu diverifikasi per client kalau fitur itu dipakai (mis. persona konsultan sebagai MCP prompt) — belum dicek satu per satu.
- **[FA-y]** Detail teknis auto-backup: berapa lama file di `.formspec/consult/{session}/undo/` disimpan (dihapus otomatis setelah sesi selesai, atau dibiarkan menumpuk sampai dibersihkan manual)?
- **[FA-aa]** Detail teknis kompresi: ambang token pasti berapa persen context window (perlu beda per model karena context window beda-beda)? Juga perlu hati-hati saat kompresi memotong turn lama — API Anthropic/OpenAI mewajibkan setiap blok `tool_use` punya pasangan `tool_result` yang valid di urutan berikutnya, jadi kompresi tidak bisa asal potong turn di tengah pasangan tool_use/tool_result tanpa membuat riwayat percakapan jadi tidak valid untuk dikirim ulang ke API.
- **[FA-l]** Verifikasi mendalam: apakah `toolrunner` dan MCP integration di `client.Beta` (Anthropic Go SDK) kompatibel dengan MCP client stdio lokal buatan sendiri (kasus `formspec-local-mcp`), atau didesain khusus untuk konektor MCP remote saja? Perlu baca dokumentasi detail `5.2 Tool Execution Loop` dan `6.5 Model Context Protocol (MCP)`, bukan cuma ringkasan overview. **(Catatan: sebagian besar sudah tidak relevan lagi setelah FA-o memindah `formspec-consult` ke TypeScript — cek relevansinya dulu sebelum ditindaklanjuti.)**
- **[Baru, diklarifikasi]** Sejauh ini semua kapabilitas `formspec-local-mcp`/`formspec-remote-mcp` diimplementasikan sebagai primitif **Tools** MCP saja. Dua primitif lain (**Resources**, **Prompts**) belum dimanfaatkan. **Resources+subscribe** (kalau dipakai untuk `read_workspace_manifest`): mekanismenya application-controlled, BUKAN model-controlled — `formspec-consult` (kode client, bukan LLM) yang panggil `resources/subscribe(uri)` deterministik di awal sesi (pola sama seperti FA-v), lalu terima notifikasi `notifications/resources/updated` kalau `formspec.yaml` berubah di tengah sesi (mis. `formspec module install` dijalankan di terminal lain) — LLM sendiri tidak pernah "minta" subscribe lewat tool-call, ini di luar siklus tool-use sepenuhnya. **Prompts** untuk persona konsultasi FormSpec masih perlu dievaluasi terpisah — supaya developer yang attach ke client eksternal (Claude Code/Cursor/OpenCode, FA-o) bisa panggil flow Discovery→Proposal→Draft yang sama persis (mis. `/formspec-consultant`), bukan cuma dapat tools mentah tanpa jaminan persona/urutan yang tepat.
- **[FA-w]** Perlu tool tambahan untuk lifecycle proses server: `get_server_status()` (apakah sedang jalan, di port berapa), `stop_server()` — sudah disebut tapi detail belum didalami. Juga belum diputuskan: bagaimana log server ditangkap dan disampaikan balik ke sesi AI kalau boot gagal (streaming penuh, atau ringkasan error saja)?

---

*Dokumen ini adalah catatan kerja awal, bukan keputusan final untuk semua detail. Keputusan FA-a sampai FA-bb di bagian 7 sudah disepakati sebagai arah desain; detail teknis di bagian 8 masih perlu didalami sebelum masuk ke Core Basic/Extended Spec resmi atau dokumen arsitektur FormSpec AI yang lebih lengkap.*

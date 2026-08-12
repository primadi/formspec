# Behind the Screen: Sharing Practical AI Implementations in Software Dev

> Materi sesi sharing internal — cara pakai AI (Copilot/Claude Code/DeepSeek, dkk) secara
> praktis dalam pekerjaan software development sehari-hari, ditutup dengan demo **FormSpec**
> sebagai jawaban konkret atas celah yang masih ada di era AI-assisted development hari ini.
>
> Format: presentasi + live demo. Audiens: developer yang sudah pakai AI tool tapi ingin
> lebih sistematis (bukan sekadar "tanya lalu tempel").

## Alokasi Waktu (total ~110 menit — sesuaikan dengan durasi sesi aktual)

| Bagian | Estimasi | Catatan pacing |
|---|---|---|
| 1 — AI Mindset & Anatomi Tools | 15 menit | Interaktif, boleh molor kalau diskusi hidup — ini pembuka, jangan buru-buru |
| 2 — Memilih & Mengendalikan Otak AI | 10 menit | Padat tabel, cepat kalau audiens sudah familiar |
| 3 — Memperluas Indra AI (MCP, Skills) | 25 menit | Anatomi MCP (primitif, local vs remote) + Agent Skills sebagai standar terbuka (format, lokasi folder, koleksi referensi) |
| 4 — Showcase Praktek Nyata | 20 menit | Termasuk live demo `@plan → @agent` — sisakan buffer 5 menit kalau demo meleset |
| 5 — Repository-Driven Context & SOP | 10 menit | Bisa dipercepat, isinya ringkas |
| 6 — Masa Depan Software Dev + Demo FormSpec | 30 menit | Bagian terberat — 10 menit paparan celah + objection-handling, 20 menit demo |

Kalau waktu sesi lebih pendek, pangkas dari Bagian 2 dan 5 dulu (paling mudah dipercepat tanpa
kehilangan inti pesan) — jangan potong dari Bagian 4 dan 6 (demo adalah inti nilai sesi ini).

---

## Bagian 1 — AI Mindset & Anatomi Tools (The Philosophy)

**Tujuan:** membuka sesi dengan santai, menyamakan persepsi, dan memvalidasi kebiasaan unik
masing-masing peserta — sebelum masuk ke tools, selaraskan dulu cara berpikirnya.

### 1.1 Subjektivitas Pengalaman AI

Tidak ada satu "cara yang benar" untuk chat dengan AI. Beberapa orang menulis prompt panjang
dan runtut, yang lain cukup satu baris santai campur bahasa daerah/Indonesia-Inggris — dua-duanya
valid selama idenya sampai.

- Jangan meniru gaya prompting orang lain mentah-mentah (screenshot prompt viral, template
  "prompt engineering" generik) kalau itu bukan cara alami Anda berpikir.
- Tidak perlu memaksakan Bahasa Inggris baku. Model modern (Claude, GPT, DeepSeek) sudah cukup
  baik memahami campuran Indonesia-Inggris/istilah teknis — memaksakan grammar malah memecah
  fokus dari isi masalah ke bentuk kalimat.
- Yang penting: **konteks lengkap** dan **maksud jelas**, bukan kerapian bahasa.

### 1.2 AI Sebagai Cermin

AI merespons dengan pola pikir yang mirip dengan cara instruksi diberikan:

- Instruksi sistematis, bertahap, dengan kriteria jelas → jawaban terstruktur, mudah diverifikasi.
- Instruksi samar ("benerin dong") → AI menebak-nebak, hasil sering meleset dari maksud asli.
- Instruksi yang menyertakan *constraint* eksplisit ("jangan ubah skema database", "pertahankan
  gaya penamaan yang ada") → AI menghormati batas itu, karena diberi tahu batasnya di awal, bukan
  ditegur setelah menyimpang.

Implikasi praktis: kalau AI sering "ngawur", cek dulu instruksi sendiri — apakah sudah cukup
eksplisit soal *apa yang dilarang*, bukan cuma *apa yang diminta*.

### 1.3 Manajemen Memori (Context Window)

Context window itu working memory, bukan long-term memory — semakin panjang percakapan, semakin
besar risiko AI "nyasar" ke keputusan lama yang sudah tidak relevan atau overfit ke detail yang
seharusnya sudah selesai dibahas.

Trik praktis kapan menekan **New Chat / Reset Context**:

| Sinyal | Aksi |
|---|---|
| Sudah pindah topik/fitur yang tidak berkaitan dengan chat sebelumnya | Reset — jangan tarik-tarik histori lama yang tidak relevan |
| AI mulai mengulang saran yang sudah ditolak, atau "lupa" keputusan yang baru saja disepakati | Reset — tanda context sudah terlalu penuh/berisik |
| Task besar selesai, mau lanjut ke task berikutnya yang independen | Reset, mulai fresh dengan ringkasan singkat sebagai starting context |
| Masih dalam satu alur debugging/implementasi yang sama | Pertahankan — histori itu justru dibutuhkan |

Aturan sederhana: **satu context window = satu unit kerja yang koheren**. Kalau sudah mulai
terasa "berat" atau responsnya melenceng, itu tanda untuk mulai ulang, bukan untuk menambah
instruksi koreksi di atas tumpukan context yang sudah kotor.

---

## Bagian 2 — Memilih & Mengendalikan "Otak" AI (Model & Effort)

**Tujuan:** membedah efisiensi performa vs biaya berdasarkan setup harian.

### 2.1 DeepSeek Flash vs Pro — kapan pindah gigi

| | **Flash** | **Pro** |
|---|---|---|
| Cocok untuk | Masalah lokal/terisolasi, 1–3 file | Masalah arsitektural, lintas banyak file/modul |
| Contoh kasus | Perbaiki satu fungsi, tambah validasi field, styling komponen | Redesain alur data lintas service, migrasi skema, refactor besar yang menyentuh banyak lapisan |
| Kecepatan | Cepat, iterasi murah | Lebih lambat, tapi reasoning lebih dalam |
| Biaya efektif | Rendah — cocok untuk iterasi cepat berulang | Lebih mahal — dipakai selektif, bukan default |

Rule of thumb: **mulai dari Flash**. Naik ke Pro begitu Anda menyadari sedang menjelaskan
konteks lintas-file yang panjang berulang kali ke Flash tanpa hasil yang presisi — itu sinyal
masalahnya sudah arsitektural, bukan lokal.

### 2.2 Mengatur Pedal Gas: Reasoning Effort

| Level | Kapan dipakai |
|---|---|
| **None/Min** | Pertanyaan faktual cepat, format ulang teks, task mekanis yang tidak butuh penalaran mendalam |
| **High** | Default harian — implementasi fitur, debugging, review kode |
| **Max** | Masalah yang benar-benar sulit dilacak (bug intermiten, keputusan desain dengan banyak trade-off), atau saat sudah dua kali salah dengan effort lebih rendah |

**Sweet spot harian:** kombinasi **Flash/Pro + High** — cukup dalam untuk kebanyakan pekerjaan
tanpa menunggu lama atau membakar biaya berlebih. `Max` disimpan untuk kasus yang benar-benar
butuh, bukan default settingan.

---

## Bagian 3 — Memperluas Indra AI (MCP, Skills, & Ecosystem)

**Tujuan:** menunjukkan cara membuat AI bisa "melihat" dan "menyentuh" infrastruktur riil, bukan
cuma menulis kode dalam ruang hampa.

### 3.1 Tiga Mode Agent Bawaan

| Mode | Kapan dipakai |
|---|---|
| `@ask` | Tanya-jawab, eksplorasi kode, tidak ada perubahan file — untuk memahami sebelum bertindak |
| `@agent` | Eksekusi langsung untuk tugas yang sudah jelas scope-nya — implementasi, fix bug yang sudah terdiagnosis |
| `@plan` | Diskusi arsitektur sebelum eksekusi — untuk perubahan besar/berisiko yang butuh kesepakatan dulu |

Navigasi praktis: masalah rumit atau berisiko tinggi **selalu** mulai dari `@plan`, bukan
langsung `@agent` — biaya berhenti sejenak untuk menyamakan pemahaman jauh lebih murah daripada
biaya membongkar hasil eksekusi yang salah arah.

### 3.2 Menghubungkan AI ke Dunia Nyata (MCP & Ekosistem)

- **Database Access (MCP)** — AI query langsung ke skema database lokal, bukan menebak-nebak
  nama kolom dari ingatan atau dari kode lama yang mungkin sudah berubah. Verifikasi realita,
  bukan asumsi.
- **Browser Testing (Playwright, built-in di Copilot terbaru)** — AI menguji hasil kerjanya
  sendiri di browser sungguhan: klik, isi form, screenshot — bukan cuma "seharusnya jalan".
- **Framework-specific Tools** (mis. Laravel Boost dan sejenisnya) — ekstensi yang memberi AI
  pemahaman langsung terhadap konvensi & environment kerja spesifik framework, alih-alih AI
  menerka dari pola generik.

Benang merah ketiganya: **AI yang punya akses ke kenyataan (data, browser, environment) jauh
lebih dipercaya daripada AI yang hanya menebak dari teks.**

### 3.3 Anatomi MCP: Standar, Primitif, & Ekosistem CLI

**MCP (Model Context Protocol)** awalnya dirilis Anthropic (November 2024) sebagai spesifikasi
terbuka untuk menyambungkan AI model ke tools dan data eksternal — sejak itu diadopsi vendor lain
(OpenAI, Google DeepMind, dkk) dan berkembang jadi standar lintas-vendor, bukan fitur eksklusif
satu produk. Analogi yang sering dipakai: **MCP itu "USB-C untuk AI tools"** — satu protokol
yang sama dipakai model manapun untuk bicara ke tool/data manapun, tanpa integrasi custom per
pasangan model-tool.

**Dua mode deployment server MCP:**

| | **MCP Local (stdio)** | **MCP Server (remote/hosted)** |
|---|---|---|
| Transport | stdin/stdout — server jalan sebagai child process di mesin sendiri | HTTP/SSE (Streamable HTTP) — server jalan sebagai service terpisah |
| Cocok untuk | Akses resource lokal (filesystem, DB lokal), workflow personal/dev | Tool dipakai bersama tim/organisasi, servicenya sudah disediakan vendor SaaS |
| Auth | Biasanya tidak perlu — proses lokal, dipercaya penuh | Perlu token/OAuth eksplisit (server tidak tahu siapa yang connect kalau tidak diverifikasi) |
| Contoh | `npx -y @modelcontextprotocol/server-filesystem`, server DB lokal | GitHub MCP (`api.githubcopilot.com/mcp/`), Notion MCP, Linear MCP, Slack MCP |

Rule of thumb: **local untuk eksperimen/dev pribadi, remote untuk kapabilitas yang perlu
governance dan dipakai bersama** (kredensial terpusat, audit, bisa dicabut aksesnya tanpa
mengubah kode di laptop siapapun).

**Tiga primitif inti MCP** (didefinisikan dari sisi server, konsisten di semua implementasi):

| Primitif | Siapa yang kontrol | Fungsi |
|---|---|---|
| **Tools** | Model — AI yang memutuskan kapan dan dengan parameter apa memanggilnya | Aksi/fungsi nyata: `create_issue`, `search_repository`, `send_message` |
| **Resources** | Aplikasi/klien — biasanya klien yang memilih resource mana yang dilampirkan ke context | Data terekspos untuk dibaca: isi file, baris database, dokumen — bukan aksi |
| **Prompts** | User — dipicu eksplisit oleh pengguna, bukan otomatis oleh model | Template instruksi siap pakai yang disediakan server, semacam slash-command bawaan |

Tiga primitif ini yang membuat MCP lebih dari sekadar "function calling" — ada pembagian jelas
kapan sesuatu adalah aksi yang diputuskan AI (tools), data yang dilampirkan aplikasi (resources),
dan starting point yang dipilih manusia (prompts).

**Posisi Skills dan Context relatif terhadap MCP** — ketiganya sering tertukar, padahal beda lapis:

- **MCP** = jalur **akses** ke kapabilitas/data eksternal yang sebelumnya AI tidak punya sama
  sekali (tools nyata, sistem nyata di luar teks).
- **Skills** = paket **pengetahuan/prosedur** yang di-load on-demand — bukan akses baru, tapi cara
  kerja/instruksi yang reusable. Satu skill = satu folder dengan `SKILL.md`; deskripsi singkatnya
  selalu ada di context, badan lengkapnya baru dibaca AI kalau tugasnya relevan.
- **Context (window)** = "meja kerja" tempat semuanya — riwayat chat, skema tools dari MCP yang
  aktif, isi skill yang ter-load — hidup sementara (lihat [[1.3]]). Makin banyak MCP tools/skills
  yang dimuat sekaligus, makin besar tekanan ke context; makanya ada teknik seperti *tool search*
  (baru memuat skema tool detail kalau relevan, bukan semua di awal) supaya context tidak
  kebanjiran definisi yang jarang dipakai.

Analogi ringkas untuk audiens: **MCP kasih tangan & mata baru (akses ke dunia nyata), Skills kasih
buku panduan yang dibuka pas dibutuhkan (keahlian), context ya meja kerjanya sendiri** — terbatas,
harus dikelola aktif (reset/kompresi, sudah dibahas di Bagian 1.3), bukan ditumpuk terus.

### 3.4 Agent Skills: Format & Standar Terbuka

**Skill juga sudah jadi standar terbuka resmi, bukan sekadar konvensi satu vendor.** Kronologi
singkatnya: format `SKILL.md` lahir di Claude Code, lalu **18 Desember 2025 Anthropic merilis
Agent Skills sebagai open standard** — spec + SDK dipublikasikan di `agentskills.io` untuk
diadopsi platform manapun. Standar ini sekarang di-*steward* oleh **Agentic AI Foundation** —
badan yang sama yang mengoordinasikan MCP dan AGENTS.md, di bawah payung Linux Foundation, dengan
platinum member **AWS, Anthropic, Block, Google, Microsoft, OpenAI**. Per hari ini tercatat 40+
tool sudah adopsi resmi: Claude Code, Cursor, Windsurf, VS Code/Copilot, Gemini CLI, OpenAI Codex,
Goose, Roo Code, Amp, Letta, Laravel Boost, dan lainnya.

**Format minimum** (`SKILL.md` per spec resmi):

| Field | Wajib? | Aturan |
|---|---|---|
| `name` | Ya | Maks 64 karakter, lowercase + angka + hyphen saja, harus sama dengan nama folder induk |
| `description` | Ya | Maks 1024 karakter — jelaskan apa DAN kapan dipakai, ini yang jadi index di context |
| `license`, `compatibility`, `metadata`, `allowed-tools` | Tidak | Opsional — kebanyakan skill tidak perlu ini |

Badan file di bawah frontmatter bebas formatnya (markdown biasa), plus folder pendukung opsional
`scripts/`, `references/`, `assets/` yang di-load belakangan sesuai kebutuhan (progressive disclosure).

**Lokasi folder tetap beda per tool** (masing-masing punya folder native sendiri, bukan semua
pindah ke satu folder bersama):

| Tool | Folder native | Catatan |
|---|---|---|
| Claude Code | `.claude/skills/` | Folder pertama/asal — paling banyak berisi skill publik existing |
| Cursor | `.cursor/skills/` | |
| Windsurf | `.windsurf/skills/` | |
| OpenCode | `.opencode/skills/` | Juga baca `.claude/skills/` & `.agents/skills/` sebagai fallback |
| GitHub Copilot | `.github/skills/` | Juga baca `.claude/skills/` & `.agents/skills/` sebagai fallback |
| — netral, tanpa vendor | `.agents/skills/` | Lokasi bersama yang mulai muncul untuk interop lintas-tool |

**Koleksi skill siap pakai untuk programming** (untuk demo/referensi, bukan sekadar dibuat sendiri):

| Sumber | Isi |
|---|---|
| [firecrawl.dev/blog/best-claude-code-skills](https://www.firecrawl.dev/blog/best-claude-code-skills) | Kurasi blog, fokus Claude Code |
| [github.com/VoltAgent/awesome-agent-skills](https://github.com/VoltAgent/awesome-agent-skills) | 1000+ skill, termasuk yang resmi dari tim dev asli: Anthropic, Google Labs, Vercel, Stripe, Cloudflare, Netlify, Sentry, dll |
| [github.com/addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) | "Production-grade engineering skills" — standar kualitas tinggi, fokus programming |
| [agentskills.io](https://agentskills.io) | Rumah spec resmi + Client Showcase (daftar lengkap tool yang adopsi) |

**Ekosistem CLI tools — siap pakai vs multi-model:**

| Tool | Vendor lock-in | Karakter |
|---|---|---|
| **Claude Code** | Anthropic | Coding agent interaktif di terminal, siap pakai, MCP-native |
| **`ant` CLI** | Anthropic | CLI umum untuk API Anthropic — kelola agents/sessions/vaults langsung dari terminal, cocok untuk automation & CI, bukan cuma coding |
| **OpenCode** | Vendor-neutral (open source) | Coding agent terminal yang mendukung banyak provider/model (Claude, OpenAI, dll) — pola interaksi mirip Claude Code tapi tidak terkunci satu vendor |
| **Vercel AI SDK** (`ai` package) | Vendor-neutral (library, bukan CLI) | Abstraksi unified untuk memanggil banyak provider LLM dari kode TypeScript/Next.js — satu API untuk streaming, tool-calling, dan text generation lintas provider |

Pesan untuk audiens: pilihan CLI/library bukan soal "mana yang terbaik", tapi soal **berapa besar
kamu mau terkunci ke satu vendor** — Claude Code dan `ant` optimal kalau tim sudah all-in Claude;
OpenCode/Vercel AI SDK relevan kalau tim perlu fleksibilitas ganti model/provider tanpa menulis
ulang integrasi.

---

## Bagian 4 — Showcase Praktek Nyata (The Blueprint Workflow)

**Tujuan:** demo inti — bagaimana mengarahkan AI mengubah puluhan file dan ribuan baris kode
secara presisi, pada pekerjaan nyata (bukan toy example).

### 4.1 The Collaboration Step

1. **Mulai di mode `@plan`** (global atau tag `@plan`) untuk masalah rumit — biarkan AI
   membedah masalah dan mengusulkan langkah, bukan langsung menulis kode.
2. **Debat dan koreksi bersama** — tolak asumsi yang salah, tambahkan constraint yang belum
   disebut, sampai rencana benar-benar disetujui berdua.
3. **Tekan Start Implementation** — biarkan `@agent` mengeksekusi rencana yang sudah disepakati
   secara massal, tanpa AI perlu menebak ulang arah di tengah jalan.

Kenapa urutan ini penting: kesepakatan di tahap rencana adalah *guardrail* murah — jauh lebih
murah daripada mengoreksi eksekusi yang sudah terlanjur menyentuh puluhan file.

### 4.2 Live Demo

Skenario: modifikasi pada sistem berjalan (legacy/brownfield project yang sedang dikelola saat
ini) — menunjukkan `@plan` → debat → eksekusi `@agent` end-to-end pada masalah nyata, bukan
skenario yang disiapkan.

---

## Bagian 5 — Repository-Driven Context & SOP (The Conclusion)

**Tujuan:** menunjukkan cara mengunci pengetahuan proyek di dalam repositori itu sendiri, supaya
kualitas kerja AI tidak bergantung pada siapa yang kebetulan sedang chat.

**Prinsip: Semua AI yang mengeksekusi, kita yang mengarahkan.**

| Mekanisme | Fungsi |
|---|---|
| Folder `docs/` | Menyuruh AI me-review kode dan menulis struktur folder + alur kerja sebagai kompas tim — bukan pengetahuan yang cuma ada di kepala satu orang |
| Folder `plan/` & Todo | Komitmen checklist pengerjaan tertulis, supaya kerja AI tidak melebar dari scope yang disepakati di tahap `@plan` |
| Custom Skills | Mengunci pola kode/fungsi yang berulang jadi reusable, dipanggil otomatis alih-alih dijelaskan ulang setiap kali |
| `.copilot-instructions` / `CLAUDE.md` | "Pagar gaib" SOP arsitektur — dibaca AI sejak awal percakapan, jadi kepatuhan terhadap konvensi tim bukan opsional |

Efek gabungannya: proyek yang didokumentasikan dan diberi pagar dengan baik membuat **AI baru
(context baru, sesi baru, bahkan tool baru) langsung produktif** tanpa perlu re-briefing manual
setiap kali — pengetahuan ada di repo, bukan di riwayat chat yang hilang begitu direset.

---

## Bagian 6 — Masa Depan Software Development dengan AI

**Tujuan:** membedah keterbatasan AI saat ini, lalu menunjukkan bagaimana **FormSpec** — proyek yang
sedang dikembangkan — secara konkret menjawab celah tersebut.

### A. Apa yang Kurang Lengkap dari Era AI Saat Ini?

1. **Ketiadaan Guardrails (Pagar Pengaman)** — tanpa batasan arsitektur yang ketat, AI menulis
   kode liar yang melebar ke mana-mana (bloated code), merusak konsistensi sistem. Instruksi
   teks (`CLAUDE.md`, `.copilot-instructions`) membantu, tapi itu *social contract* yang bisa
   dilupakan atau di-skip AI — bukan batasan yang dipaksakan oleh sistem itu sendiri.
2. **Ketiadaan Infrastruktur & Akses yang Jelas** — AI pintar menulis logika kode, tapi bingung
   menyusun topologi jaringan, mengonfigurasi hypervisor (Proxmox/LXC), atau mengatur hak akses
   yang aman. Tidak ada model deklaratif yang seragam untuk "di mana dan bagaimana ini berjalan".
3. **Kompleksitas Deployment & Maintenance** — AI tidak tahu cara menghadapi perbedaan
   environment (dev vs prod), prosedur disaster recovery, otomatisasi backup, pemeliharaan
   jangka panjang. Setiap tim biasanya reinvent pipeline deployment-nya sendiri.
4. **Belum Ada Standarisasi Bisnis Vertikal** — tidak ada service standar tepercaya dan
   ready-to-use untuk kebutuhan industri riil (QRIS, payment gateway, akuntansi, inventory,
   dll), sehingga AI selalu reinventing the wheel dari nol setiap proyek baru.

### B. Antisipasi Keberatan: "Ini kan Low-Code/No-Code Lagi?"

Begitu audiens melihat "spec YAML → CRUD otomatis", reaksi wajar developer berpengalaman adalah
skeptis — sudah banyak yang kapok dengan low-code generation di masa lalu. Jawab ini **sebelum**
masuk demo, bukan sesudah:

- **Bedanya bukan "declarative vs kode"** — Frappe/ERPNext (`DocType`) sudah lebih dulu melakukan
  ini dan terbukti production-proven 15+ tahun. Perbedaan FormSpec ada di **di mana kontrak itu
  hidup**: DocType Frappe tersimpan sebagai row di database (`tabDocType`), Entity FormSpec tersimpan
  sebagai **file YAML di disk**.
- Konsekuensinya konkret: file YAML **git-diffable, reviewable di PR, dan dibaca/ditulis AI
  secara natural** — sama seperti AI sudah terbiasa dengan manifest K8s/Docker Compose/CI config.
  DocType di database butuh tooling ekspor/impor khusus (`bench export-doc`) sebelum bisa masuk
  alur git/PR, dan AI perlu API call atau akses DB langsung untuk membacanya.
- FormSpec juga tidak mengunci ke satu bahasa/ORM — logic bisa Go native, Starlark script, atau
  sidecar bahasa apapun; akses data pakai raw SQL (`ctx.db`), bukan ORM abstraction yang
  membatasi query kompleks.
- Kejujuran perlu disampaikan juga: FormSpec **masih MVP**, belum punya modul bisnis siap pakai
  sebanyak ERPNext (30+ modul). Argumennya bukan "FormSpec sudah lebih lengkap", tapi "FormSpec
  dirancang supaya AI-assisted development bekerja lebih presisi di atasnya sejak awal."

- Rujukan: `docs/comparison/formspec-vs-frappe.md` §4 (YAML di disk vs DocType di DB) dan §6
  (kapan pilih yang mana) — kutip langsung dari situ kalau ada pertanyaan lanjutan di sesi.

### C. Live Demo: FormSpec Menjawab Keempat Celah Ini

**FormSpec** adalah platform *spec-driven* untuk aplikasi bisnis: aplikasi dideklarasikan sebagai
kumpulan spec YAML (`workspace → app → module → kind`), diinterpretasikan saat runtime oleh satu
engine (`formspec`) dan sepasang implementasi resmi (renderer). Prinsip intinya:

> **Spec adalah kontrak; renderer adalah implementasi kontrak itu.**

Berikut pemetaan langsung celah di atas ke bagaimana FormSpec dibangun:

#### 1 → Guardrails yang dipaksakan sistem, bukan sekadar instruksi teks

AI (atau developer manapun) tidak menulis SQL migration, route REST, atau state-machine secara
bebas — semua itu **derivasi otomatis dari spec YAML** (`kind: Entity`, field, relasi, action,
event, natural key). AI hanya boleh mengubah kontrak di dalam skema yang sudah didefinisikan;
migration adalah *structural-diff* dari spec lama ke baru, bukan skrip bebas yang bisa lolos
review. Errornya sendiri terstandarisasi (`FORMSPEC.*` di `docs/spec/backend/error-glossary.yaml`),
bukan pesan ad-hoc yang beda-beda tiap developer/AI menulisnya.

- Rujukan: `docs/spec/backend/01-core-basic.md` (Document/Entity/action sebagai kontrak),
  `docs/architecture/08-repo-structure.md` (`pkg/spec/` = realisasi kode dari kontrak spec).

#### 2 → Infrastruktur & akses dideklarasikan, bukan dikonfigurasi manual

Satu binary engine (`formspec`) melayani **semua bahasa** (Go, PHP, Python, Ruby, Java, .NET,
TypeScript, Rust) — app code berjalan sebagai child process yang berbicara ke engine lewat Unix
socket via SDK tipis (`lib-formspec-*`). Tidak ada topologi berbeda per bahasa yang harus dipahami
AI atau developer. Di sisi cluster, `formspec-operator` (K8s CRD controller) dan `formspec-ctl`
(`--mode=region/cluster`) membuat Workspace/Datastore/ResourceClaim sebagai objek deklaratif —
developer memilih region + tier (`ClusterClass: premium/standard/economy`), tidak perlu tahu
hypervisor atau topologi jaringan fisik di baliknya.

- Rujukan: `docs/architecture/01-architecture-overview.md` §1–§2 (topology + component
  inventory), `docs/runtimes/04-formspec-sidecar.md` (protokol lintas bahasa).

#### 3 → Satu pipeline deployment, generic image, dev = prod secara desain

`formspec dev` dan `formspec serve` adalah **binary yang sama**, beda mode — bukan dua sistem berbeda
yang bisa divergen. Deployment mengikuti satu pipeline generik (`formspec apply` → deploy → run)
untuk semua App, semua bahasa. HA/failover dan reconciliation K8s ditangani `formspec-operator`
secara otomatis, bukan runbook manual per tim.

- Rujukan: `docs/guides/how-to-run.md` (dev vs prod, satu command), `docs/architecture/03-deployment-flow.md`,
  `docs/architecture/05-failover.md`, `docs/architecture/06-k8s-operator.md`.

#### 4 → Modul vertikal bisnis siap pakai, bukan reinvent per proyek

`verticals/` berisi App bisnis nyata yang independen dan siap diinstal — `billing`, `inventory`,
`gl` (general ledger), `company` — masing-masing modul lengkap dengan entity, state machine,
UI derivatif, dan integrator lintas modul (mis. `sales-gl-integrator`) sebagai App terpisah,
bukan kode yang menyatu erat seperti kebanyakan ERP existing. Distribusi modul (termasuk modul
vendor pihak ketiga closed-source) sudah punya jalur kontraktual sendiri lewat
`kind: Marketplace` + trust tier — tanpa mengorbankan prinsip "spec YAML tetap terbuka dan
terbaca" (`docs/spec/platform/07-marketplace.md`).

- Rujukan: `docs/architecture/07-vertical-modules.md` (taksonomi modul, perbandingan ERPNext),
  `docs/comparison/formspec-vs-frappe.md` (posisi FormSpec vs ekosistem existing terdekat).

#### Alur Demo yang Disarankan

**Pilihan spec untuk live demo** — dua opsi, pilih sesuai risiko yang bisa ditoleransi:

| Opsi | Command | Kapan pakai |
|---|---|---|
| **Aman (default)** | `go run ./cmd/formspec/ dev --spec examples/Clinic-UI-Showcase/spec --dsn "sqlite:.formspec/clinic.db" --addr :8080 --force --dev-ui` | Sudah jadi contoh resmi yang terdokumentasi di `docs/guides/how-to-run.md` — paling kecil kemungkinan gagal di depan audiens |
| **Naratif bisnis lebih kuat** | `go run ./cmd/formspec/ dev --spec verticals/billing/spec --dsn "sqlite:.formspec/billing.db" --addr :8080 --dev --force --dev-ui` | Kalau ingin cerita "modul vertikal siap pakai" terasa lebih konkret — **wajib gladi bersih dulu** sebelum sesi, karena vertical module masih berkembang dan belum sekokoh example app |

Kalau tidak sempat gladi bersih penuh sebelum sesi, pakai opsi Aman. Siapkan juga screenshot/rekaman
singkat sebagai fallback kalau demo tetap macet di tempat — jangan mengandalkan live demo 100% tanpa cadangan.

1. Tunjukkan `docs/architecture/08-repo-structure.md` — peta repo, kontrak vs renderer.
2. Jalankan command sesuai pilihan di atas — satu proses, tanpa setup infra manual.
3. Buka admin panel (`http://localhost:5173/default/_admin`) — tunjukkan CRUD, form, dan tabel yang **otomatis diturunkan dari spec**, belum ada satu baris React/SQL ditulis manual.
4. Minta AI (Claude Code) menambah satu field baru ke `kind: Entity` di spec YAML — tunjukkan
   efeknya otomatis merambat ke form, tabel, dan REST API tanpa AI perlu menyentuh migration
   SQL atau kode frontend. **Inilah guardrail nyata**: AI bekerja di dalam kontrak spec, bukan
   bebas menulis infrastruktur dari nol.
5. Tutup dengan `docs/comparison/formspec-vs-frappe.md` — posisikan FormSpec bukan sebagai "framework
   AI generatif lainnya", tapi sebagai lapisan kontrak yang membuat kerja AI (dan manusia)
   otomatis terpagari, portable lintas bahasa/infrastruktur, dan tidak reinventing modul bisnis
   yang sudah selesai dipikirkan orang lain.

### Penutup

AI hari ini sudah sangat baik menulis *logika*. Yang masih kurang adalah **pagar** (guardrails),
**peta infrastruktur**, **jalur deployment**, dan **modul bisnis siap pakai** — empat hal yang
tidak selesai dengan prompt yang lebih baik, tapi dengan *sistem* yang secara desain memaksakan
kontrak, menyederhanakan topologi, menyatukan pipeline, dan menyediakan blok bisnis yang sudah
teruji. FormSpec adalah taruhan bahwa AI-assisted development akan jauh lebih produktif ketika AI
diberi rel yang jelas untuk berlari, bukan lapangan kosong tanpa batas.

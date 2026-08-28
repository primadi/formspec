# FormSpec AI

**Status:** Design — belum diimplementasikan (lihat §5)
**License:** Creative Commons CC0

FormSpec AI adalah lapisan AI opsional di atas platform FormSpec. Produk pertamanya
adalah **`formspec consult`** — asisten yang berperan sebagai **konsultan bisnis**
(aktif bertanya soal tujuan aplikasi, mengusulkan alur sistem, berdiskusi dengan
business owner dalam bahasa awam) sekaligus **penulis spec FormSpec yang akurat**
(tidak mengarang nama field/kind/aturan yang tidak ada).

Dua kapabilitas itu mudah gagal kalau digabung tanpa struktur: hasilnya AI yang
lancar mengobrol tapi spec-nya asal, atau sebaliknya. FormSpec AI memisahkan
keduanya secara eksplisit dengan lapisan **grounding** (fakta diambil lewat
pemanggilan tool nyata, bukan ingatan model) dan **validasi wajib** di sisi
server yang independen dari perilaku LLM — penerapan prinsip _safety via
structure, not documentation_ yang dipegang di seluruh arsitektur FormSpec.

## 1. Komponen

| Komponen              | Wujud                                                                                                                              | Peran                                                                                                                                                 |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `formspec consult`    | Subcommand di binary `formspec` (Go)                                                                                               | Client mandiri: REPL, tool-use loop, kelola sesi, render diff — [`02-formspec-consult.md`](02-formspec-consult.md)                                    |
| `formspec-local-mcp`  | Subcommand `formspec mcp-serve` (Go, stdio) — bukan binary terpisah                                                                | Grounding "tentang project saya": workspace manifest, modul terinstal, validasi, apply draft — [`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) |
| `formspec-remote-mcp` | Server Streamable HTTP, hosted FormSpec Cloud                                                                                      | Grounding "tentang ekosistem FormSpec": industry template, module registry — [`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md)                 |
| LLM Provider Layer    | `openai-go` di balik interface `llm.Provider` internal                                                                             | BYOK multi-provider, tool-use loop, capability bar — [`05-llm-provider-layer.md`](05-llm-provider-layer.md)                                           |
| FormSpec Skill        | File YAML frontmatter + Markdown, dibundel bersama instalasi `formspec`                                                            | Pengetahuan prosedural authoring (cara memakai Spec), dibaca on-demand — [`06-formspec-skill.md`](06-formspec-skill.md)                               |
| `ai_index`            | Blok opsional di manifest Module ([`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §2) | Metadata discovery supaya AI bisa mengusulkan reuse module vertikal — [`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §3                     |

Cara komponen-komponen ini tersusun (empat lapisan, dua artifact, satu jalur
integrasi MCP) dikontrakkan di [`01-architecture.md`](01-architecture.md).

## 2. Fitur yang Didukung

### 2.1 Target rilis pertama (M1)

| #   | Fitur                              | Keterangan                                                                                                                                                                                                   |
| --- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | **Konsultasi discovery**           | AI aktif bertanya (probing questions) sebelum menulis apa pun; bahasa awam default, teknis kalau developer jelas menanyakan hal teknis — adaptif alami, tanpa mode switch business-owner/developer eksplisit |
| 2   | **Industry template**              | Pattern bisnis terkurasi (appointment-based-service, dst.) dengan pertanyaan pemandu dan kandidat entity — discovery tidak mulai dari kosong                                                                 |
| 3   | **Workspace awareness**            | AI membaca App manifest, daftar modul terinstal (`modules/`+`vendors/`+`formspec.lock`), dan detail spec project yang sedang dikerjakan — otomatis di awal sesi                                              |
| 4   | **Spec authoring tervalidasi**     | Draft YAML ditulis ke folder sesi dan **selalu** melewati validation gate server-side — kualitas hasil tidak bergantung kedisiplinan LLM                                                                     |
| 5   | **Diff, apply, undo**              | Draft di-diff terhadap project asli (spec-ke-spec, unified diff biasa); developer accept/reject per file; setiap apply di-backup untuk undo satu langkah                                                     |
| 6   | **Rekomendasi reuse module**       | AI mengusulkan "pakai module X" dari `ai_index` modul terinstal maupun registry publik — bukan reinvent dari nol                                                                                             |
| 7   | **FormSpec Skill**                 | Pengetahuan authoring per topik (entity, form, extension, vendoring) dibaca on-demand saat topik percakapan cocok                                                                                            |
| 8   | **Kontrol server dev**             | Restart/status/stop `formspec dev` dari dalam sesi — restart menjalankan validasi dulu, menolak kalau spec invalid                                                                                           |
| 9   | **BYOK multi-provider**            | Developer membawa API key sendiri (Anthropic, OpenAI, DeepSeek, dst.); FormSpec tidak jadi reseller AI                                                                                                       |
| 10  | **Attach ke client MCP eksternal** | `formspec mcp-serve` bisa dipakai langsung dari Claude Code/Cursor/VS Code — bonus reuse, bukan prasyarat; proteksi validation gate berlaku sama                                                             |
| 11  | **Sesi persisten**                 | Transcript penuh + discovery summary + draft tersimpan di `.formspec/consult/{session}/`; riwayat panjang dikompresi terstruktur, keputusan final tetap utuh                                                 |

### 2.2 Jangka menengah / masa depan

| Fitur                                         | Catatan                                                                                                                                                                                                               |
| --------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **FormSpec Studio (Lite)**                    | Upgrade GUI di atas fondasi yang sama — setelah CLI terbukti                                                                                                                                                          |
| **Community template & community `ai_index`** | Menunggu prasyarat untrusted-input terpenuhi ([`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §3.1)                                                                                                          |
| **MCP Resources & Prompts**                   | Subscribe perubahan `formspec.yaml` di tengah sesi; persona konsultan sebagai MCP prompt (mis. `/formspec-consultant`) untuk client eksternal — belum dievaluasi, seluruh desain saat ini memakai primitif Tools saja |
| **Slot model kedua ("fast model")**           | Model ringan untuk tahap tertentu — ditambah kalau ada sinyal nyata biaya inference jadi masalah, bukan dioptimasi di depan                                                                                           |
| **Cache lokal hasil registry search**         | Kurangi round-trip ke `formspec-remote-mcp` per sesi — masih pertanyaan terbuka                                                                                                                                       |

### 2.3 Non-Goals

- **FormSpec bukan reseller AI** — cost inference sepenuhnya ditanggung developer (BYOK); tidak ada klaim "semua LLM setara" (lihat minimum capability bar, [`05-llm-provider-layer.md`](05-llm-provider-layer.md) §2).
- **Tidak ada validasi data runtime** dalam sesi consult — `validate_spec` structural/statis saja; pemeriksaan yang butuh `formspec-server` + DB jalan bukan urusan sesi consult.
- **Tidak ada AI di dalam MCP tool** untuk retrieval — reasoning tetap di LLM konsultan di luar tool; tool hanya mengembalikan data/top-K kandidat ([`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §2).
- **Tidak menulis langsung ke `vendors/`** — guard read-only ditegakkan di tool, bukan konvensi; jalur yang benar adalah Entity Extension atau shadow copy ([`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) §4).

## 3. Prinsip Desain

1. **Grounding lewat tool, bukan ingatan model.** AI memanggil tool MCP untuk membaca schema kind, workspace manifest, dan katalog module sebelum menulis YAML. Ini juga menjaga posisi netral-vendor FormSpec: grounding hidup di MCP server, bukan di model tertentu.
2. **Validasi wajib di server.** Tool penulisan draft menjalankan validasi sebagai bagian dari perilakunya sendiri (`propose_spec_file`), bukan langkah terpisah yang berharap LLM memanggilnya — proteksi berlaku sama untuk client built-in maupun eksternal.
3. **Data sovereignty.** Spec bisnis klien (harga, struktur komisi) tidak pernah keluar mesin developer hanya untuk divalidasi — `formspec-local-mcp` selalu lokal via stdio, bukan server yang di-tunnel ke internet.
4. **Kepemilikan data menentukan arsitektur.** Dua MCP server dipisah berdasarkan milik siapa datanya ("project saya" vs "ekosistem FormSpec"), bukan berdasarkan ukuran.
5. **Satu implementasi, satu jalur.** `validate_spec` memakai package yang sama dengan boot `formspec-server`; semua client (built-in dan eksternal) memakai jalur MCP yang sama — tidak ada jalur pintas yang bisa diam-diam divergen.
6. **Manfaatkan open source dulu.** SDK resmi per provider (`openai-go`) untuk provider layer, MCP Go SDK resmi untuk client/server, OS keyring untuk kredensial, git untuk versioning lanjutan — bukan reinvent.

## 4. Peta Dokumen

| Dokumen                                                                      | Isi                                                                                    |
| ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| [`01-architecture.md`](01-architecture.md)                                   | Empat lapisan, dua artifact (Go + TS), tool-use loop, strategi context injection       |
| [`02-formspec-consult.md`](02-formspec-consult.md)                           | Client: alur konsultasi, sesi, diff/apply/undo, attach client eksternal                |
| [`03-formspec-local-mcp.md`](03-formspec-local-mcp.md)                       | Katalog tool lokal, validation gate, guard `vendors/`, kontrol server dev              |
| [`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md)                     | Industry template, module registry search, `ai_index`, risiko untrusted input          |
| [`05-llm-provider-layer.md`](05-llm-provider-layer.md)                       | openai-go, BYOK, capability bar, penyimpanan kredensial, ekonomi token                 |
| [`06-formspec-skill.md`](06-formspec-skill.md)                               | Format skill, granularitas, versioning, mekanisme relevansi                            |
| [`../cli-tools/05-formspec-consult.md`](../cli-tools/05-formspec-consult.md) | Referensi verb CLI (`formspec consult`, `formspec consult diff`, `formspec mcp-serve`) |

Detail diskusi dan alternatif yang ditolak ada di catatan kerja
[`../technical-notes/FormSpec-Technical-Note-FormSpec-AI-Consult.md`](../technical-notes/FormSpec-Technical-Note-FormSpec-AI-Consult.md)
(arsip, bukan kontrak).

## 5. Status Implementasi

Seluruh section ini **target desain, belum diimplementasikan** — `formspec-consult`,
`formspec mcp-serve`, kedua MCP server, dan FormSpec Skill belum ada di codebase.
Rencana pengerjaan: [`../plan/todo.md`](../plan/todo.md) Fase 10. Pertanyaan
terbuka per komponen dicatat di bagian akhir masing-masing dokumen.

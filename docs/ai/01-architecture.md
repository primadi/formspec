# Arsitektur FormSpec AI

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> Dokumen ini mengontrakkan susunan komponen FormSpec AI: empat lapisan terpisah,
> dua artifact binary, tool-use loop, dan strategi context injection. Detail
> per komponen ada di dokumen 02–06.

---

## 1. Empat Lapisan Terpisah

```
┌──────────────────────────────────────────┐
│ 1. Client (formspec-consult, TS)             │  ← REPL, kelola sesi, tampilkan diff
├──────────────────────────────────────────┤
│ 2. LLM Provider Layer (Vercel AI SDK)     │  ← BYOK, ganti-ganti provider
├──────────────────────────────────────────┤
│ 3a. formspec-local-mcp (stdio, Go)           │  ← Grounding: workspace, project ini
│ 3b. formspec-remote-mcp (HTTP, FormSpec Cloud)  │  ← Grounding: template & registry ekosistem
├──────────────────────────────────────────┤
│ 4. Validation Gate (server-side, wajib)   │  ← Safety net, independen dari LLM
└──────────────────────────────────────────┘
```

Prinsip pemisahan: kelemahan satu lapisan (mis. LLM yang kurang reliabel
mengikuti instruksi) tidak boleh merembet ke lapisan lain. Lapisan 3 dipecah dua
berdasarkan **kepemilikan data** — data project (lokal) dan data ekosistem
(bersama) punya lifecycle dan trust boundary berbeda, jadi dipisah server juga.
Lapisan 4 hidup di server MCP, bukan di client, supaya proteksinya berlaku sama
untuk client apa pun ([`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) §2).

## 2. Dua Artifact, Dua Bahasa, Satu Protokol

```
formspec                     (Go — CLI utama: dev, apply, generate, module install; tidak berubah)
  └─ formspec mcp-serve      subcommand baru: expose formspec-local-mcp lewat stdio —
                          pembungkus tipis atas formspec-core, BUKAN binary terpisah

formspec-consult             (TypeScript + Vercel AI SDK, di-compile jadi binary standalone
                          via `bun build --compile`)
  ├─ spawn `formspec mcp-serve` sebagai child process (stdio, satu mesin)
  ├─ tool-use loop (ToolLoopAgent, Vercel AI SDK)
  └─ panggil LLM API langsung lewat provider adapter (BYOK)
```

**Kenapa dua bahasa.** Tooling multi-provider LLM + MCP jauh lebih matang di
ekosistem TypeScript (Vercel AI SDK: tool-use loop first-class, adapter 25+
provider, MCP client bawaan — [`05-llm-provider-layer.md`](05-llm-provider-layer.md))
dibanding Go hari ini. `formspec-consult` adalah satu-satunya bagian berbahasa
TypeScript; kontributor yang bekerja di `formspec-core`/`formspec-server`/module
system tidak pernah perlu menyentuhnya. Keduanya terhubung lewat MCP stdio yang
bahasa-agnostik by design — bukan monolith satu bahasa, tapi juga bukan
pencampuran kode dalam satu binary.

**Kenapa `formspec mcp-serve` bukan binary terpisah.** Ia cuma pembungkus tipis
atas `formspec-core` — sama-sama Go, tidak butuh build/release pipeline sendiri.
Client MCP eksternal (VS Code/Claude Code/Cursor) dan `formspec-consult` (built-in)
menjalankan command yang identik sebagai child process via stdio — **satu
implementasi server, dua cara pakai**.

**Kenapa client mandiri, bukan menumpang client cloud.** Spec bisnis ada di
folder lokal (`modules/`, `vendors/`, `formspec.lock`) dan `formspec-local-mcp` perlu
membacanya langsung dari disk. Kalau jalur utamanya lewat client cloud/remote
(yang hanya mendukung konektor MCP remote via HTTP publik), server lokal harus
di-tunnel ke internet — spec bisnis klien (harga, struktur komisi, dst.) sempat
lewat jaringan publik hanya untuk divalidasi. Itu bertentangan dengan prinsip
data sovereignty yang dipegang di tempat lain (BYOK, pemisahan App Owner/
Workspace Owner). Attach ke client MCP eksternal tetap didukung sebagai bonus
reuse — opsional, bukan prasyarat.

## 3. Tool-Use Loop — LLM Tidak Pernah Memanggil MCP Langsung

LLM tidak punya kemampuan eksekusi/jaringan sendiri — ia model teks.
`formspec-consult` adalah perantara wajib di setiap putaran (secara internal
ditangani `ToolLoopAgent`, tapi mekanismenya tetap):

```
1. formspec-consult kirim [riwayat percakapan] + [daftar tool, JSON Schema] ke LLM API
2. LLM balas blok terstruktur "tool_use" (nama tool + input parameter)
   — ini CUMA teks/JSON, belum ada eksekusi apa pun
3. formspec-consult BACA tool_use itu, lalu DIA SENDIRI melakukan panggilan MCP
   nyata ke formspec-local-mcp (stdio) / formspec-remote-mcp (HTTP)
4. Server MCP eksekusi (wrapper tipis atas formspec-core), kembalikan hasil
5. formspec-consult bungkus hasil jadi "tool_result", kirim balik ke LLM API
   sebagai giliran berikutnya
6. LLM lanjut — bisa minta tool lagi (ulang dari langkah 2),
   atau langsung memberi jawaban teks final ke developer
```

Satu giliran percakapan bisa berisi beberapa siklus `tool_use`/`tool_result`
berturut-turut sebelum LLM akhirnya menjawab — itu normal.

**Dua budget token yang berbeda.** Karena API LLM stateless, setiap panggilan
memang menyertakan ulang seluruh riwayat + daftar skema tool. Tapi **deklarasi
tool** (nama + skema JSON parameter) tetap kecil dan konstan berapa pun besar
data di baliknya (`list_skills` tetap satu skema kecil, mau ada 5 atau 500
skill), sementara **isi hasil panggilan tool** tidak pernah dideklarasikan di
depan — hanya diambil on-demand. Inilah alasan konkret pola "semua via MCP
tool" (§5) menyelesaikan masalah skala: yang tumbuh besar tidak pernah masuk ke
bagian yang wajib diulang tiap panggilan API. Ditambah prompt caching provider
(daftar tool + system prompt identik antar turn di-cache prefix-nya), biaya
pengulangan ini di praktiknya jauh lebih murah daripada kelihatannya.

## 4. MCP Sebagai Satu-Satunya Jalur Integrasi

Setiap tool MCP adalah **pembungkus terstruktur di atas `formspec-core` yang sudah
ada — tidak membawa logic baru**. Beda tool MCP dengan "AI shell out ke CLI
`formspec validate`" bukan soal di mana logic-nya jalan (sama-sama lokal), tapi
soal kontrak antarmuka: tool MCP punya JSON Schema input/output terstruktur
yang langsung dipakai tool-calling API, sementara shell out membuat AI menyusun
string command bebas dan menafsirkan stdout/stderr teks bebas — jauh lebih
rawan salah.

Konsekuensinya: **`formspec-consult` sendiri juga lewat MCP** untuk kedua server,
bukan memanggil `formspec-core` langsung in-process meski sama-sama buatan FormSpec.
Hanya ada satu jalur integrasi ("bagaimana AI mengakses kapabilitas FormSpec")
yang dipakai baik built-in client maupun client eksternal — jalur pintas
tersendiri untuk built-in client adalah risiko divergensi. Overhead serialisasi
stdio/HTTP diabaikan karena tool dipanggil beberapa kali per sesi, bukan hot
path performa tinggi.

## 5. Strategi Context Injection: On-Demand, Bukan Dump Statis

Mengirim seluruh Core Basic + Extended Spec sebagai context statis di setiap
sesi ditolak — dump penuh menurunkan precision begitu dokumen makin panjang,
rawan basi begitu Spec naik versi, dan dibayar di setiap pesan sepanjang sesi
terlepas relevan atau tidak. Satu pola untuk semua katalog (skill, template,
module): **query on-demand via MCP tool**.

```
Auto-invoke deterministik (dipicu client, sekali di awal sesi — tetap tool call
biasa, cuma dipicu otomatis oleh kode client, bukan inisiatif LLM):
  - read_workspace_manifest() + list_installed_modules()   [formspec-local-mcp]
  - list_skills()                                          [formspec-local-mcp]

On-Demand (dipanggil LLM sendiri, kapan pun dibutuhkan di tengah percakapan):
  - list_kind_schemas("Entity")        [formspec-local-mcp]   → schema penuh, saat authoring
  - list_business_templates()          [formspec-remote-mcp]  → seluruh daftar (katalog kecil)
  - search_modules_registry(query)     [formspec-remote-mcp]  → top-K narrowing (katalog besar)
  - get_module_detail(name)            [formspec-remote-mcp]  → ai_index penuh
  - read_skill(name)                   [formspec-local-mcp]   → isi skill penuh, saat topik cocok
  - validate_spec(yaml)                [formspec-local-mcp]   → hasil validasi
```

Auto-invoke di awal sesi tidak melanggar prinsip on-demand: workspace manifest
dan daftar skill terbatas by nature (satu project, katalog terkurasi — bukan
katalog global yang bisa tumbuh tak terbatas) dan nyaris selalu relevan. Pola
pemicu deterministik dari kode client — bukan bergantung AI ingat memanggil —
adalah safety-net yang sama dengan validation gate: perilaku penting tidak
digantungkan pada kedisiplinan model.

## 6. Kompresi Riwayat Sesi Panjang

Sesi discovery bisa berpuluh-puluh turn. Mekanisme:

1. Transcript penuh selalu tersimpan di `.formspec/consult/{session}/transcript.md`
   ([`02-formspec-consult.md`](02-formspec-consult.md) §3) — kompresi tidak pernah
   menghapus data di disk.
2. Token usage dipantau per turn; kompresi dipicu mendekati ambang context
   window (kisaran 70–80%, ambang pasti per model masih terbuka).
3. Saat kompresi, turn lama didistilasi jadi ringkasan terstruktur (fakta/
   keputusan yang sudah disepakati — format serupa `discovery-summary.md`),
   bukan dialog eksploratif mentah. Turn lama diganti ringkasan di context
   aktif; transcript penuh tetap di disk.
4. Tool baca-balik ke transcript penuh tersedia untuk detail spesifik yang tidak
   tertangkap ringkasan — tidak berharap ringkasan sempurna dari awal.

Prinsip: yang dikompresi adalah proses eksploratif; keputusan final tetap utuh.
Perhatian teknis: API provider mewajibkan setiap blok `tool_use` punya pasangan
`tool_result` valid — kompresi tidak boleh memotong turn di tengah pasangan itu,
atau riwayat jadi tidak valid untuk dikirim ulang.

## 7. Referensi

| Dokumen | Isi |
|---|---|
| [`02-formspec-consult.md`](02-formspec-consult.md) | Client: alur konsultasi, sesi, diff/apply |
| [`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) | Tool lokal + validation gate (lapisan 3a & 4) |
| [`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) | Tool ekosistem (lapisan 3b) |
| [`05-llm-provider-layer.md`](05-llm-provider-layer.md) | Lapisan 2 — Vercel AI SDK, BYOK |
| [`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) | `modules/`/`vendors/`/`formspec.lock` yang dibaca `formspec-local-mcp` |

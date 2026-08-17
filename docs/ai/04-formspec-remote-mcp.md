# `formspec-remote-mcp` — Grounding Ekosistem FormSpec

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> MCP server terpusat (Streamable HTTP, hosted FormSpec Cloud). Grounding "tentang
> ekosistem FormSpec": industry template dan module registry publik — data bersama
> lintas developer, dikurasi FormSpec, berpotensi multi-kontributor nanti. Dipisah
> dari [`formspec-local-mcp`](03-formspec-local-mcp.md) berdasarkan kepemilikan data,
> bukan ukuran ([`01-architecture.md`](01-architecture.md) §1).

---

## 1. Industry Template — Discovery Tidak Mulai dari Kosong

Template pattern bisnis (bukan spec lengkap) mempercepat discovery dengan
pertanyaan pemandu:

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
  - Customer    [characteristic: master]
  - Staff       [characteristic: master]
  - Service     [characteristic: master]
  - Appointment [characteristic: transaction]
  - Sale        [characteristic: transaction]
```

Template adalah meta-knowledge untuk discovery — **bukan** module yang
di-install ke aplikasi, bukan bagian mekanisme `modules:`/`vendors/`. Ia hidup
di server ini (bukan lokal) karena sekelas dengan module registry: dikurasi
FormSpec sekarang, berpotensi multi-kontributor nanti — arsitekturnya disamakan
sejak awal supaya tidak perlu migrasi saat community template dibuka. Seluruh
template awal ditulis FormSpec sendiri.

## 2. Katalog Kecil vs Besar — Dua Pola Tool, Disengaja

| Tool | Katalog | Pola |
|---|---|---|
| `list_business_templates()` | Kecil, terkurasi | Kembalikan **seluruh daftar** (nama+deskripsi+alias) dalam satu response — tanpa embedding, tanpa AI di dalam tool. LLM konsultan yang sudah jalan di sesi itu yang mencocokkan, memanfaatkan kemampuan multilingual-nya gratis ("tukang potong rambut" → barbershop) |
| `search_modules_registry(query)` | Besar, terus tumbuh | Perlu **narrowing** (vector similarity, pgvector) sebelum kembalikan top-K kandidat berperingkat — katalog tidak muat dikirim utuh berapa pun besar context LLM |
| `get_module_detail(name)` | — | Detail satu module, termasuk `ai_index` penuh (§3) |

**Reasoning "module mana yang cocok" selalu di LLM konsultan di luar tool** —
yang sudah membaca konteks Discovery/Proposal — bukan LLM di dalam tool server.
AI di dalam MCP tool untuk retrieval ditolak sebagai pola default: tidak
menghilangkan kebutuhan narrowing di skala besar (context limit berlaku untuk
LLM manapun), menduplikasi biaya inference (2× panggilan LLM per pencarian),
dan merusak netralitas provider (tool jadi butuh API key/model sendiri,
independen dari pilihan client). Kalau katalog kecil tumbuh besar, migrasinya
mulus ke pola narrowing yang sama — bukan re-arsitektur.

**Multilingual.** pgvector sendiri agnostik bahasa — kemampuan lintas-bahasa
bergantung model embedding. Kandidat: Voyage AI (keluarga `voyage-3`
multilingual) atau BGE-M3 (open source, bisa self-host). Karena jargon domain
(mis. "buku besar" / "general ledger") berisiko kurang presisi kalau hanya
mengandalkan kedekatan semantik, `ai_index` menyediakan field `aliases:` yang
diisi eksplisit oleh publisher dan ikut di-embed bersama deskripsi — hybrid
semantic + sinonim eksplisit, bukan berharap model menebak sinonim
lintas-bahasa.

## 3. `ai_index` — Module Index untuk AI

Supaya AI mengusulkan "pakai module X" alih-alih reinvent, Module boleh
mendeklarasikan blok opsional `ai_index` di manifestnya
([`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) §2):

```yaml
# modules/payment-gateway-xendit/module.yaml
spec:
  ai_index:
    category: payment
    features: [charge, refund, webhook_callback, virtual_account]
    aliases: [pembayaran online, payment gateway, va]   # ikut di-embed (§2)
    integration_pattern: |
      depends: [{module: payment-gateway-xendit}]  # akses lewat service call
    skills_for_ai: |
      Pakai module ini kalau bisnis butuh terima pembayaran online. Integrasi
      umum: buat Transaction yang refer ke charge service module ini, listen
      event payment.completed untuk update status Sale lokal.
```

Dua jalur baca, sesuai kepemilikan data:

- **Modul terinstal** (`vendors/` project ini) → `list_installed_modules()` di
  `formspec-local-mcp` mengembalikan index-nya.
- **Modul registry publik yang belum di-install** →
  `search_modules_registry`/`get_module_detail` di server ini.

### 3.1 `skills_for_ai` Adalah Untrusted Input

`skills_for_ai` adalah teks dari vendor pihak ketiga. Begitu trust tier
`community` dibuka ([`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md)
§2), teks ini jadi data tidak terpercaya yang masuk ke sesi AI — berisiko
prompt injection: deskripsi module dibuat menyesatkan supaya AI "disuruh"
merekomendasikan module tertentu, atau menyisipkan instruksi tersembunyi yang
dibaca AI sebagai perintah, bukan deskripsi.

**Prasyarat wajib sebelum community module dibuka:** `skills_for_ai` dari tier
non-`official` diperlakukan sebagai **data untuk dibaca AI, bukan instruksi
untuk diikuti**. Mitigasi struktural yang sudah built-in: teks vendor hanya
masuk context kalau `get_module_detail` benar-benar dipanggil untuk module itu
(on-demand, [`01-architecture.md`](01-architecture.md) §5) — tidak pernah
didorong penuh ke semua sesi; dan `list_installed_modules()` hanya mengekstrak
field metadata modul vendor, tidak men-dump isi spec-nya sebagai teks bebas.
Selama seluruh index masih first-party (`official`), risiko ini belum berlaku.

## 4. Pertanyaan Terbuka

- Autentikasi dan rate-limit `formspec-remote-mcp` — perlu API key FormSpec Cloud
  terpisah dari BYOK LLM, atau akses baca/search bebas?
- Apakah hasil pencarian boleh di-cache sementara di sisi lokal (per sesi)
  untuk mengurangi round-trip jaringan berulang?
- Governance `ai_index` — siapa yang berhak mengisi `skills_for_ai`: vendor
  sendiri saat publish, atau ada proses review terpisah, terutama untuk trust
  tier `verified` di bawah `official`?

## 5. Referensi

| Dokumen | Isi |
|---|---|
| [`01-architecture.md`](01-architecture.md) | Pemisahan dua server berdasarkan kepemilikan data; strategi on-demand |
| [`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) | Server saudara — jalur baca `ai_index` modul terinstal |
| [`../spec/platform/02-workspace-app-module.md`](../spec/platform/02-workspace-app-module.md) | Manifest Module tempat `ai_index` dideklarasikan |
| [`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md) | Trust tier — dasar analisis risiko §3.1 |

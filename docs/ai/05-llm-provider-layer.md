# LLM Provider Layer — Vercel AI SDK & BYOK

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> Lapisan kedua arsitektur Forma AI ([`01-architecture.md`](01-architecture.md)
> §1): bagaimana `forma-consult` memanggil LLM — multi-provider lewat Vercel AI
> SDK, kredensial BYOK milik developer, dan batas kemampuan minimum model.

---

## 1. Kenapa Vercel AI SDK

`forma-consult` memakai **Vercel AI SDK** (TypeScript) sebagai fondasi provider
layer — bukan agentic framework besar, bukan tulis sendiri dari nol:

- **`ToolLoopAgent`** — siklus kirim → cek `tool_use` → eksekusi → kembalikan
  `tool_result` ([`01-architecture.md`](01-architecture.md) §3) tersedia sebagai
  abstraksi first-class; `forma-consult` tidak menulis loop-nya manual.
- **Provider adapter 25+ provider** — format tool-call di API mentah berbeda
  per provider dan ini alasan konkret lapisan ini ada, bukan teoretis:
  Anthropic memakai content-block (`type: "tool_use"`, `input` objek JSON
  native); OpenAI memakai `message.tool_calls[].function.arguments` dengan
  `arguments` berupa **string JSON** yang harus di-parse; DeepSeek sengaja
  kompatibel format OpenAI; Gemini beda lagi (`functionCall` di dalam `parts`).
  Normalisasi lintas-provider ditangani adapter SDK yang sudah matang, bukan
  ditulis manual per provider.
- **MCP client bawaan** — koneksi ke `forma mcp-serve` (stdio) dan
  `forma-remote-mcp` (Streamable HTTP) tanpa implementasi protokol sendiri.

Ini penerapan prinsip "manfaatkan open source dulu" — kebutuhan spesifik
komponen ini (multi-provider + MCP + tool loop) paling matang di ekosistem
TypeScript hari ini; itulah alasan `forma-consult` berbahasa TypeScript
sementara seluruh platform tetap Go ([`01-architecture.md`](01-architecture.md)
§2). `forma-local-mcp` (Go) tidak perlu berubah per provider — protokol MCP
sendiri sudah bahasa/provider-agnostik by design.

Forma Skill **bukan** urusan lapisan ini: skill tidak pernah jadi konsep di
level API provider mana pun — murni konvensi client-side, dikirim sebagai teks
biasa lewat `tool_result`, tanpa field khusus yang perlu dipahami provider
([`06-forma-skill.md`](06-forma-skill.md) §3). Tidak perlu adapter per provider
untuk skill, beda dari kasus format tool-call di atas.

## 2. BYOK & Minimum Capability Bar

**BYOK** — developer membawa API key sendiri; cost inference sepenuhnya
ditanggung developer. Forma tidak menjadi reseller AI. Konsisten dengan prinsip
BYOK untuk data sovereignty di bagian lain platform.

**Tidak semua LLM setara — tidak ada klaim itu.** Tiga alasan:

1. **Reliabilitas tool-calling** berbeda jauh antar model — model kecil/murah
   sering skip memanggil tool atau salah format argumen, terutama di percakapan
   panjang (discovery bisa 20+ turn).
2. **Instruction-following sesi panjang** — consultant behavior (bertanya dulu,
   jangan lompat ke YAML) butuh model yang konsisten mengikuti system prompt
   sampai turn ke-20+, bukan cuma turn pertama.
3. **Dukungan tool-use** — tidak semua model (terutama lokal/open-weight lama)
   mendukungnya.

Kebijakan: **minimum capability bar** (lolos test tool-calling + context window
minimum) + daftar provider yang sudah divalidasi Forma. Model yang gagal bar
ditolak dengan pesan jelas, bukan dibiarkan menghasilkan sesi berkualitas
buruk. Benchmark konkret bar ini masih pertanyaan terbuka (§4).

**Default satu model untuk semua tahap** (Discovery, Proposal, draft-writing).
Slot model kedua ("fast model" untuk tahap ringan) bisa ditambah nanti kalau
ada sinyal nyata biaya inference jadi masalah — bukan dioptimasi di depan tanpa
data pemakaian.

## 3. Penyimpanan API Key

Dua tingkat cek yang praktis gratis — tanpa fallback file terenkripsi
(kompleksitas kripto: key derivation, penyimpanan passphrase — tidak sepadan
untuk rilis pertama):

```
1. OS keyring (zalando/go-keyring)   → kasus normal, desktop
2. Environment variable              → kasus headless/CI
3. (tidak ada apa pun)               → error jelas: minta set env var
                                       atau jalankan di desktop
```

Dibungkus interface kecil (`CredentialStore`) — bukan menambah kompleksitas
sekarang, tapi menjaga kalau kebutuhan enterprise nyata muncul (Vault, dst.),
penggantian implementasi tidak menyentuh titik pemanggilan di banyak tempat.
Pilihan `zalando/go-keyring` (bukan alternatif dengan backend lebih banyak):
dirawat aktif, dan backend tambahan belum relevan tanpa sinyal permintaan
nyata — defer non-essential complexity.

## 4. Ekonomi Token

Dua budget yang berbeda ([`01-architecture.md`](01-architecture.md) §3):
**deklarasi tool** (nama + skema JSON) kecil dan konstan, wajib diulang tiap
panggilan API; **isi data hasil tool** hanya masuk context saat benar-benar
dipanggil. Katalog yang tumbuh (skill, template, module) tidak pernah membebani
bagian yang diulang — itulah alasan pola on-demand dipilih. **Prompt caching**
provider (prefix system prompt + daftar tool yang identik antar turn di-cache)
membuat biaya/latensi pengulangan riwayat jauh lebih murah di praktik daripada
di atas kertas. Untuk sesi yang sangat panjang, kompresi riwayat terstruktur
berlaku ([`01-architecture.md`](01-architecture.md) §6).

## 5. Pertanyaan Terbuka

- Minimum capability bar konkret — benchmark tool-calling seperti apa, context
  window minimum berapa token?
- Ambang pasti kompresi riwayat per model (context window berbeda-beda), dan
  penanganan pasangan `tool_use`/`tool_result` agar riwayat terkompresi tetap
  valid dikirim ulang ([`01-architecture.md`](01-architecture.md) §6).

## 6. Referensi

| Dokumen | Isi |
|---|---|
| [`01-architecture.md`](01-architecture.md) | Tool-use loop yang dijalankan lapisan ini; dua budget token |
| [`02-forma-consult.md`](02-forma-consult.md) | Client yang menggunakan lapisan ini |
| [`06-forma-skill.md`](06-forma-skill.md) | Kenapa skill tidak butuh dukungan provider |

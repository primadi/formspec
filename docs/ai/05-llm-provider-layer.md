# LLM Provider Layer — openai-go & BYOK

**Status:** Draft — diimplementasikan di `internal/consult/llm` (todo 10.2.3)

> Lapisan kedua arsitektur FormSpec AI ([`01-architecture.md`](01-architecture.md)
> §1): bagaimana `formspec consult` memanggil LLM — multi-provider lewat SDK
> resmi, kredensial BYOK milik developer, dan batas kemampuan minimum model.

---

## 1. Fondasi: SDK Resmi, Interface Internal

> **Keputusan 2026-08-27**: fondasi provider layer adalah **`openai-go` (SDK
> resmi OpenAI)** di balik interface internal `llm.Provider` — bukan Vercel AI
> SDK (TypeScript) seperti rancangan awal, bukan juga framework agentic
> (`langchaingo` ditolak: API tidak stabil; `go-ai`/digitallysavvy dievaluasi
> dan ditolak: bus factor, klaim paritas 1:1 dengan Vercel SDK sulit
> dipertahankan, permukaan dependency besar). Tipe SDK tidak pernah bocor ke
> logic consult — swap SDK adalah perubahan lokal.

- **Satu wire format menutup target provider** — DeepSeek, GLM/Zhipu, dan
  gateway OpenAI-compatible lain diakses lewat `openai-go` + base URL
  override. Normalisasi 25+ format provider berbeda tidak pernah terpakai
  karena capability bar (§2) hanya mengizinkan provider tervalidasi.
- **Tool-use loop ditulis sendiri** (~100 baris di `internal/consult`) —
  kirim → cek tool_call → eksekusi via MCP → kembalikan tool_result → ulang.
  Satu wire format = satu loop; testable dengan mock provider.
- **MCP client memakai `modelcontextprotocol/go-sdk`** (resmi, sama dengan
  server) — bukan MCP client dari framework pihak ketiga.
- **Jalur eskalasi murah**: provider non-OpenAI-compatible (mis. Anthropic
  native untuk extended thinking/prompt caching) ditambah sebagai adapter
  kedua (`anthropic-sdk-go`, juga Stainless-generated) di balik interface
  `Provider` yang sama.

FormSpec Skill **bukan** urusan lapisan ini: skill tidak pernah jadi konsep di
level API provider mana pun — murni konvensi client-side, dikirim sebagai teks
biasa lewat `tool_result`, tanpa field khusus yang perlu dipahami provider
([`06-formspec-skill.md`](06-formspec-skill.md) §3). Tidak perlu adapter per provider
untuk skill, beda dari kasus format tool-call di atas.

## 2. BYOK & Minimum Capability Bar

**BYOK** — developer membawa API key sendiri; cost inference sepenuhnya
ditanggung developer. FormSpec tidak menjadi reseller AI. Konsisten dengan prinsip
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
minimum) + daftar provider yang sudah divalidasi FormSpec. Model yang gagal bar
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

| Dokumen                                            | Isi                                                         |
| -------------------------------------------------- | ----------------------------------------------------------- |
| [`01-architecture.md`](01-architecture.md)         | Tool-use loop yang dijalankan lapisan ini; dua budget token |
| [`02-formspec-consult.md`](02-formspec-consult.md) | Client yang menggunakan lapisan ini                         |
| [`06-formspec-skill.md`](06-formspec-skill.md)     | Kenapa skill tidak butuh dukungan provider                  |

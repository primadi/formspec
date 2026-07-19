# Forma Skill — Pengetahuan Prosedural On-Demand

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> Forma Skill adalah paket pengetahuan prosedural untuk AI: *cara memakai* Spec
> Forma dengan benar (urutan konsultasi, kapan validasi, aturan authoring per
> kind) — pelengkap Spec yang berisi fakta/schema. Dibundel bersama instalasi
> `forma`, dibaca lewat `list_skills()`/`read_skill()` di
> [`forma-local-mcp`](03-forma-local-mcp.md).

---

## 1. Kenapa Bukan Dump Spec Penuh

Dua kebutuhan yang berbeda — Skill dan Spec saling melengkapi, bukan saling
menggantikan:

| Kebutuhan | Kenapa bukan spec mentah |
|---|---|
| **Procedural/behavioral** — cara menjadi konsultan yang baik, kapan memanggil `validate_spec`, urutan Discovery → Proposal → Draft | Bukan fakta yang ada di Spec — ini instruksi cara *memakai* Spec. Spec tidak mengajari AI cara berdiskusi. |
| **Factual/schema** — nama field persis, aturan `characteristic`, whitelist per kind | Diambil on-demand lewat `list_kind_schemas()` ([`03-forma-local-mcp.md`](03-forma-local-mcp.md) §1). Dump penuh sebagai context statis menurunkan precision begitu dokumen makin panjang, dan basi begitu Spec naik versi. |

## 2. Format

YAML frontmatter + Markdown body:

```yaml
---
name: entity-authoring
description: >
  Gunakan skill ini saat membuat atau mengubah Entity kind — field types,
  characteristic (master/transaction/reference), natural_key, state_machine.
  Trigger: percakapan menyebut "tambah field", "buat entity baru", "state machine".
applies_to_kind: [Entity]
min_core_spec_version: "0.1.9"
---

# Entity Authoring

... isi lengkap: aturan detail, contoh, edge case ...
```

- **Granularitas per kebutuhan** — skill "entity-authoring", "form-layout",
  "entity-extension-authoring", "module-vendoring" terpisah, bukan satu skill
  besar "Forma Consultant". Footprint awal tetap kecil; behavior authoring
  tidak ter-load kalau percakapan baru sampai tahap Discovery.
- **`min_core_spec_version`** — menandai skill terikat versi Core Spec
  tertentu; dipakai mendeteksi skill basi kalau Core Spec naik versi tapi skill
  belum diupdate. Dicek terhadap Core Spec yang dimuat lokal — salah satu
  alasan skill hidup di `forma-local-mcp` (ikut siklus rilis `forma`), bukan di
  server remote.
- **Output `read_skill` adalah Markdown mentah** — input tool tetap wajib skema
  JSON (`name: string`), tapi isinya tidak dibungkus JSON terstruktur: skill
  dirancang dibaca LLM sebagai instruksi bahasa natural, bukan di-parse program
  deterministik (beda dari spec Entity/Form yang wajib mengikuti skema YAML).

## 3. Mekanisme Relevansi

Alur dua tahap, index kecil dulu:

1. **Saat sesi mulai** — client memanggil `list_skills()` otomatis
   (deterministik, bareng workspace manifest —
   [`01-architecture.md`](01-architecture.md) §5). AI hanya menerima `name` +
   `description` semua skill.
2. **Saat topik cocok** — begitu percakapan cocok dengan salah satu
   `description`, AI membaca isi lengkap lewat `read_skill(name)`.

Penilaian "skill mana yang cocok" dilakukan **LLM konsultan yang sama, di turn
yang sama** — bukan sesi/LLM klasifikasi terpisah (menduplikasi biaya inference
tanpa manfaat; deskripsi skill cukup pendek untuk dinilai langsung, katalognya
kecil-terkurasi karena 100% Forma-authored). Tanpa embedding, tanpa nested LLM
call — pola katalog-kecil yang sama dengan
[`04-forma-remote-mcp.md`](04-forma-remote-mcp.md) §2.

**Pemicu tidak bergantung inisiatif LLM semata** (risiko lupa di model
lemah/sesi panjang): selain auto-invoke `list_skills()` di awal sesi, re-cek
skill yang relevan otomatis menjadi bagian composite tool `propose_spec_file`
sebelum draft ditulis — pola safety-net yang sama dengan validation gate
([`03-forma-local-mcp.md`](03-forma-local-mcp.md) §2).

**Bukan standar provider.** Skill tidak pernah jadi konsep di level API LLM
mana pun — murni konvensi client-side: index/isi dikirim sebagai teks biasa
lewat `tool_result`. Tidak butuh adapter per provider
([`05-llm-provider-layer.md`](05-llm-provider-layer.md) §1).

## 4. Pertanyaan Terbuka

- Versioning mekanis — `min_core_spec_version` di frontmatter cukup untuk
  deteksi basi, atau perlu CI yang menjalankan validasi otomatis setiap Core
  Spec rilis versi baru?
- Siapa penulis skill pertama (entity-authoring, form-layout,
  entity-extension-authoring, module-vendoring) — tim Forma langsung, atau
  didistilasi dari catatan kerja yang sudah ada?

## 5. Referensi

| Dokumen | Isi |
|---|---|
| [`01-architecture.md`](01-architecture.md) | Strategi context injection yang memuat skill on-demand |
| [`03-forma-local-mcp.md`](03-forma-local-mcp.md) | Tool `list_skills`/`read_skill`; composite `propose_spec_file` |
| [`04-forma-remote-mcp.md`](04-forma-remote-mcp.md) | Pola katalog kecil vs besar yang sama |

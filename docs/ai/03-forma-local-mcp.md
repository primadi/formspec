# `formspec-local-mcp` — Grounding Project Lokal

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> MCP server lokal, diekspos lewat subcommand `formspec mcp-serve` (stdio).
> Grounding "tentang project saya": workspace manifest, modul terinstal,
> validasi spec, apply draft, kontrol server dev. Selalu lokal — spec bisnis
> klien tidak pernah keluar mesin developer
> ([`01-architecture.md`](01-architecture.md) §2).

---

## 1. Katalog Tool

| Tool | Fungsi |
|---|---|
| `list_kind_schemas(kind)` | Schema resmi tiap kind (Entity, Form, dst.) — sumber kebenaran authoring, bukan ingatan model |
| `read_workspace_manifest()` | App manifest: modules aktif, Menu, Auth, Theme |
| `list_installed_modules()` | Modul di `modules/`+`vendors/`, status aktif/nonaktif dari `formspec.lock` ([`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §6), termasuk `ai_index` modul terinstal |
| `read_module_spec(module, kind, name)` | Detail satu spec |
| `propose_spec_file(path, content)` | Tulis draft ke sesi + validasi otomatis (§2) |
| `apply_draft(session, file)` | Pindahkan draft yang di-accept ke lokasi asli; guard `vendors/` (§4); auto-backup ke `undo/` ([`02-formspec-consult.md`](02-formspec-consult.md) §4) |
| `validate_spec(yaml)` | Validasi statis — reuse package boot `formspec-server` (§3) |
| `check_naming_conflict(name)` | Deteksi bentrok nama module/entity |
| `restart_server()` / `get_server_status()` / `stop_server()` | Kontrol proses `formspec dev` lokal (§5) |
| `list_skills()` / `read_skill(name)` | Index dan isi FormSpec Skill ([`06-formspec-skill.md`](06-formspec-skill.md)) |

Setiap tool adalah pembungkus terstruktur di atas `formspec-core` — tidak membawa
logic baru ([`01-architecture.md`](01-architecture.md) §4).

## 2. Validation Gate — Perilaku Tool, Bukan Kedisiplinan Pemanggil

Client yang dipakai bisa eksternal (Claude Code/Cursor — di luar kontrol FormSpec),
jadi validasi **tidak boleh** bergantung pada CLI/LLM yang disiplin
memanggilnya. Solusinya bukan tool "tulis file" mentah, tapi tool composite:

```
propose_spec_file(path, content)
  → server tulis ke .formspec/consult/{session}/draft/{path}
  → server jalankan validate_spec secara internal
  → return { written: true, validation: {...} } ke LLM
```

Validasi adalah bagian dari *apa yang dilakukan tool* — mitigasi utama untuk
model lemah/berhalusinasi atau client yang tidak disiplin: kualitas hasil akhir
tidak bergantung pada kedisiplinan LLM/client mengikuti instruksi. Pemicu
deterministik yang sama juga dipakai untuk re-cek skill relevan sebelum draft
ditulis ([`06-formspec-skill.md`](06-formspec-skill.md) §3).

## 3. `validate_spec` — Satu Package, Tiga Pemanggil

`validate_spec` **bukan** logic terpisah yang ditulis khusus untuk MCP. Dua
implementasi "apa itu spec valid" akan diam-diam divergen — spec yang lolos
saat consult tapi gagal saat `formspec-server` betulan boot adalah kelas bug
paling susah dilacak. Satu package Go internal (`formspec-core`):

```
formspec-server              → boot betulan, load spec, serve traffic
formspec apply --dry-run     → CLI, load spec, validasi, TANPA serve traffic
formspec-local-mcp           → fungsi validasi yang SAMA, in-process
  (tool validate_spec)      (bukan HTTP call ke server jalan, bukan reimplementasi)
```

**Scope: structural, bukan runtime data.** Yang dicek: schema, referensi
`depends`/Entity Extension/shadow-copy valid, bentrok nama. Di luar scope:
validasi yang butuh instance `formspec-server` + DB jalan (mis. "apakah
`natural_key` ini sudah dipakai row lain di produksi").

**Pengecualian kecil terhadap "semua offline":** verifikasi signature/trust-tier
vendor module ([`../spec/platform/07-marketplace.md`](../spec/platform/07-marketplace.md)
§2) berpotensi butuh cek revocation list ke registry pusat. Itu jalur terpisah
yang boleh online dan harus eksplisit — tidak diam-diam ikut terjadi saat
`validate_spec` dipanggil.

## 4. Guard Read-Only `vendors/`

Semua tool tulis (khususnya `apply_draft`) **menolak eksplisit** kalau target
path ada di bawah `vendors/` — ditegakkan di kode tool, bukan konvensi
dokumentasi. Kalau AI/developer mencoba memodifikasi konten vendor, tool error
dan mengarahkan ke mekanisme yang benar:

- Field/validasi tambahan → **Entity Extension**
  ([`../spec/backend/03-entity-extension.md`](../spec/backend/03-entity-extension.md)).
- Kustomisasi presentasi (layout Form, caption, urutan) → **shadow copy**
  ([`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §6.4).

Konsisten dengan `vendors/` yang read-only by design (integritas
checksum/signature, jalur update versi aman).

## 5. Kontrol Server Dev

`restart_server()` / `get_server_status()` / `stop_server()` mengelola proses
`formspec dev` lokal dari dalam sesi. `restart_server()` adalah composite yang
sama polanya dengan §2: jalankan `validate_spec` dulu, tolak restart kalau
invalid — sekaligus memberi sinyal runtime tambahan di luar scope structural §3
(boot yang benar-benar gagal ketahuan di sini). Berjalan tanpa konfirmasi
interaktif di mode dev — proses lokal, risiko rendah, gampang diulang.

**Terbuka:** bagaimana log server ditangkap dan disampaikan balik ke sesi AI
kalau boot gagal — streaming penuh, atau ringkasan error saja?

## 6. Referensi

| Dokumen | Isi |
|---|---|
| [`01-architecture.md`](01-architecture.md) | Posisi server ini di empat lapisan; MCP sebagai satu-satunya jalur |
| [`02-formspec-consult.md`](02-formspec-consult.md) | Client yang memanggil tool ini; sesi, diff, undo |
| [`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) | Server saudara untuk data ekosistem |
| [`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) | Struktur `modules/`/`vendors/`/`overrides/`/`formspec.lock` yang dibaca tool di sini |

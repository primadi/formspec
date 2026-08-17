# `formspec-consult` — Client Konsultasi

**Status:** Design — belum diimplementasikan
**License:** Creative Commons CC0

> Client mandiri FormSpec AI: REPL yang menjalankan tool-use loop, mengelola sesi
> konsultasi, dan merender diff. Berjalan 100% lokal tanpa bergantung aplikasi
> AI lain. Referensi verb CLI ada di
> [`../cli-tools/05-formspec-consult.md`](../cli-tools/05-formspec-consult.md).

---

## 1. Alur Konsultasi: Discovery → Proposal → Draft

Tiga tahap yang ditegakkan lewat system prompt + FormSpec Skill
([`06-formspec-skill.md`](06-formspec-skill.md)) — AI tidak boleh lompat ke YAML
sebelum kebutuhan bisnis jelas:

1. **Discovery** — AI bertanya aktif soal tujuan aplikasi, alur kerja, dan
   aturan bisnis, dipandu probing questions dari industry template
   ([`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §1) bila pattern-nya
   dikenali. Hasilnya dirangkum ke `discovery-summary.md` dalam bahasa awam
   untuk dikonfirmasi business owner.
2. **Proposal** — AI mengusulkan alur sistem dan komposisi module: entity apa
   saja, lifecycle-nya, module vertikal mana yang bisa di-reuse alih-alih
   dibangun ulang ([`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §3).
3. **Draft** — AI menulis spec YAML lewat `propose_spec_file` — setiap draft
   otomatis melewati validation gate ([`03-formspec-local-mcp.md`](03-formspec-local-mcp.md)
   §2), lalu direview developer lewat diff (§4).

**Tanpa deteksi role eksplisit.** Batas business owner vs developer tidak jelas
dan sering campur dalam satu sesi — AI cukup adaptif secara alami: bahasa awam
default, teknis kalau lawan bicara jelas menanyakan hal teknis. Tidak ada mode
switch.

## 2. Workspace Awareness

Saat sesi mulai, client memanggil otomatis (deterministik, bukan inisiatif LLM
— [`01-architecture.md`](01-architecture.md) §5):

- `read_workspace_manifest()` — App manifest: modules aktif, Menu, Auth, Theme.
- `list_installed_modules()` — gabungan `modules/` + `vendors/` + status
  aktif/nonaktif dari `formspec.lock`
  ([`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) §6).
- `list_skills()` — index FormSpec Skill (name + description saja).

Pertanyaan seperti "sistem apa yang sedang dibuat?" dijawab LLM konsultan
dengan mensintesis hasil tool-tool itu — tidak ada tool "describe_workspace"
dengan narasi baked-in. Untuk modul di `vendors/` (berpotensi community-tier),
tool hanya mengekstrak field metadata (nama, versi, sumber, deskripsi) — tidak
men-dump mentah isi spec vendor sebagai teks bebas ke context
([`04-formspec-remote-mcp.md`](04-formspec-remote-mcp.md) §3.1).

## 3. Penyimpanan Sesi

```
project/
  .formspec/consult/
    2026-07-18-barbershop-initial/
      transcript.md          # log percakapan penuh, ditulis real-time
      discovery-summary.md   # ringkasan alur, bahasa awam, untuk konfirmasi owner
      draft/
        modules/barbershop/service.resource.yaml
        modules/barbershop/visit.resource.yaml
      undo/                  # backup file yang ditimpa apply_draft (§4)
```

`draft/` mencerminkan struktur folder project asli — `modules/` untuk module
baru, atau `overrides/`/target Entity Extension kalau draft menyentuh vendor
module ([`../spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md)
§6.4, [`../spec/backend/03-entity-extension.md`](../spec/backend/03-entity-extension.md)).

## 4. Diff, Apply, Undo

- **`formspec consult diff`** — bandingkan `draft/` vs `modules/`/`vendors/` yang
  sebenarnya. Karena tidak ada tahap compile (spec FormSpec *adalah*
  implementasinya), diff yang relevan adalah **spec-ke-spec**: unified diff
  biasa atas YAML, tanpa mekanisme diff khusus.
- **Accept/reject per file** — accept memindahkan file dari `draft/` ke lokasi
  asli lewat `apply_draft` ([`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) §1).
- **Undo satu langkah** — sebelum `apply_draft` menimpa file, versi lama
  di-backup ke `undo/` (copy file sederhana, bukan git). Inilah yang membuat
  `apply_draft` tidak butuh konfirmasi blocking: diff tetap ditampilkan untuk
  visibilitas, tapi kesalahan trivial cukup di-undo — konfirmasi dan safety-net
  saling menggantikan, tidak perlu dua-duanya. Ini **bukan** sistem versioning
  penuh; kalau kebutuhan multi-snapshot/navigasi bebas muncul, reuse git lewat
  ref/branch terisolasi (tanpa mengotori commit history developer), bukan
  reinvent version control.
- **Operasi lifecycle server dev** (start/restart/stop) berjalan tanpa
  konfirmasi — proses lokal, risiko rendah, gampang diulang.

## 5. Attach ke Client MCP Eksternal (Opsional)

```
formspec consult
  → default: built-in client, 100% lokal, tidak butuh aplikasi lain
  → opsional: attach `formspec mcp-serve` ke client MCP yang sudah terpasang
    (Claude Code, Cursor, VS Code, OpenCode, dst.)
```

Kedua cara menjalankan command server yang identik (`formspec mcp-serve`) — satu
implementasi, dua cara pakai. Proteksi validation gate berlaku sama karena
hidup di server, bukan di client ([`03-formspec-local-mcp.md`](03-formspec-local-mcp.md)
§2). Catatan: dukungan primitif MCP resources/prompts lebih tidak merata antar
client eksternal dibanding tools yang nyaris universal — perlu dicek per client
kalau fitur itu nanti dipakai.

## 6. Pertanyaan Terbuka

- Format persis `discovery-summary.md`/proposal — diagram Mermaid? Naratif
  murni?
- UX render diff di terminal — cukup unified diff teks, atau perlu TUI?
- Apakah sesi `.formspec/consult/` di-commit ke git (riwayat diskusi ikut
  ter-audit) atau di-gitignore (scratch, bukan artifact permanen)?
- Retensi `undo/` — dihapus otomatis setelah sesi selesai, atau dibiarkan
  sampai dibersihkan manual?
- Apakah `formspec consult diff` mengarahkan draft yang menyentuh vendor module
  secara otomatis ke `overrides/`/Entity Extension yang sesuai, atau developer
  memindahkan manual?

## 7. Referensi

| Dokumen | Isi |
|---|---|
| [`01-architecture.md`](01-architecture.md) | Tool-use loop yang dijalankan client ini; kompresi riwayat sesi |
| [`03-formspec-local-mcp.md`](03-formspec-local-mcp.md) | Tool yang dipanggil client — termasuk `apply_draft`, guard `vendors/` |
| [`05-llm-provider-layer.md`](05-llm-provider-layer.md) | Cara client memanggil LLM (BYOK, Vercel AI SDK) |
| [`../cli-tools/05-formspec-consult.md`](../cli-tools/05-formspec-consult.md) | Referensi verb CLI |

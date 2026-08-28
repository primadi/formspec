# `formspec consult` — Referensi CLI

**Status:** Design — belum diimplementasikan (lihat §5)
**License:** Creative Commons CC0

> Referensi verb untuk lapisan AI FormSpec. **Arsitektur lengkap FormSpec AI —
> `formspec-consult`, `formspec-local-mcp`, `formspec-remote-mcp`, provider layer,
> FormSpec Skill — dikontrakkan di [`docs/ai/`](../ai/README.md)**; dokumen ini
> hanya permukaan CLI-nya, konsisten dengan folder ini sebagai referensi
> verb-per-verb.

---

## 1. Dua Artifact

| Artifact             | Wujud                                                        | Peran                                                                                                               |
| -------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- |
| `formspec consult`   | Subcommand di binary `formspec` (Go)                         | Client konsultasi: REPL, tool-use loop, sesi, diff — [`../ai/02-formspec-consult.md`](../ai/02-formspec-consult.md) |
| `formspec mcp-serve` | Subcommand di binary `formspec` (Go) — bukan binary terpisah | Expose `formspec-local-mcp` lewat stdio — [`../ai/03-formspec-local-mcp.md`](../ai/03-formspec-local-mcp.md)        |

`formspec consult` bukan CLI inti ketiga ([`README.md`](README.md)) — ia lapisan
opsional; `formspec` (CLI utama Go) tidak berubah.

## 2. `formspec consult`

```bash
formspec consult
# Mulai/lanjutkan sesi konsultasi — REPL built-in, 100% lokal.
# Spawn `formspec mcp-serve` sebagai child process (stdio).
# Butuh API key LLM milik developer (BYOK) — dari OS keyring atau env var
# (../ai/05-llm-provider-layer.md §3).
```

Sesi tersimpan di `.formspec/consult/{session}/` — `transcript.md`,
`discovery-summary.md`, `draft/`, `undo/`
([`../ai/02-formspec-consult.md`](../ai/02-formspec-consult.md) §3).

## 3. `formspec consult diff`

```bash
formspec consult diff
# Bandingkan draft/ sesi vs modules//vendors/ project asli — unified diff
# biasa (spec-ke-spec; tidak ada tahap compile). Accept/reject per file;
# accept memindahkan file ke lokasi asli dengan auto-backup untuk undo.
```

Lihat [`../ai/02-formspec-consult.md`](../ai/02-formspec-consult.md) §4.

## 4. `formspec mcp-serve`

```bash
formspec mcp-serve
# Jalankan formspec-local-mcp lewat stdio — dipakai formspec-consult built-in,
# dan bisa di-attach langsung ke client MCP eksternal (Claude Code, Cursor,
# VS Code): satu implementasi server, dua cara pakai.
```

Katalog tool (read workspace/module, `propose_spec_file` + validation gate,
`apply_draft` + guard `vendors/`, kontrol server dev, FormSpec Skill):
[`../ai/03-formspec-local-mcp.md`](../ai/03-formspec-local-mcp.md).

## 5. Status Implementasi Hari Ini

Belum ada yang diimplementasikan — `formspec-consult`, `formspec mcp-serve`, kedua
MCP server, dan FormSpec Skill adalah target desain. Rencana pengerjaan:
[`../plan/todo.md`](../plan/todo.md) Fase 10; pertanyaan terbuka per komponen
ada di bagian akhir masing-masing dokumen [`docs/ai/`](../ai/README.md).

## 6. Referensi

| Dokumen                                                | Isi                                                             |
| ------------------------------------------------------ | --------------------------------------------------------------- |
| [`../ai/README.md`](../ai/README.md)                   | FormSpec AI: komponen, fitur yang didukung, prinsip desain      |
| [`../ai/01-architecture.md`](../ai/01-architecture.md) | Empat lapisan, dua artifact, tool-use loop                      |
| [`02-formspec-cli.md`](02-formspec-cli.md)             | CLI `formspec` utama — `mcp-serve` akan jadi subcommand di sini |

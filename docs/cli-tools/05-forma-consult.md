# `forma consult` — Referensi CLI

**Status:** Design — belum diimplementasikan (lihat §5)
**License:** Creative Commons CC0

> Referensi verb untuk lapisan AI Forma. **Arsitektur lengkap Forma AI —
> `forma-consult`, `forma-local-mcp`, `forma-remote-mcp`, provider layer,
> Forma Skill — dikontrakkan di [`docs/ai/`](../ai/README.md)**; dokumen ini
> hanya permukaan CLI-nya, konsisten dengan folder ini sebagai referensi
> verb-per-verb.

---

## 1. Dua Artifact

| Artifact | Wujud | Peran |
|---|---|---|
| `forma-consult` | Binary standalone (TypeScript + Vercel AI SDK, compile via `bun build --compile`) | Client konsultasi: REPL, tool-use loop, sesi, diff — [`../ai/02-forma-consult.md`](../ai/02-forma-consult.md) |
| `forma mcp-serve` | Subcommand di binary `forma` (Go) — bukan binary terpisah | Expose `forma-local-mcp` lewat stdio — [`../ai/03-forma-local-mcp.md`](../ai/03-forma-local-mcp.md) |

`forma consult` bukan CLI inti ketiga ([`README.md`](README.md)) — ia lapisan
opsional; `forma` (CLI utama Go) tidak berubah.

## 2. `forma consult`

```bash
forma consult
# Mulai/lanjutkan sesi konsultasi — REPL built-in, 100% lokal.
# Spawn `forma mcp-serve` sebagai child process (stdio).
# Butuh API key LLM milik developer (BYOK) — dari OS keyring atau env var
# (../ai/05-llm-provider-layer.md §3).
```

Sesi tersimpan di `.forma/consult/{session}/` — `transcript.md`,
`discovery-summary.md`, `draft/`, `undo/`
([`../ai/02-forma-consult.md`](../ai/02-forma-consult.md) §3).

## 3. `forma consult diff`

```bash
forma consult diff
# Bandingkan draft/ sesi vs modules//vendors/ project asli — unified diff
# biasa (spec-ke-spec; tidak ada tahap compile). Accept/reject per file;
# accept memindahkan file ke lokasi asli dengan auto-backup untuk undo.
```

Lihat [`../ai/02-forma-consult.md`](../ai/02-forma-consult.md) §4.

## 4. `forma mcp-serve`

```bash
forma mcp-serve
# Jalankan forma-local-mcp lewat stdio — dipakai forma-consult built-in,
# dan bisa di-attach langsung ke client MCP eksternal (Claude Code, Cursor,
# VS Code): satu implementasi server, dua cara pakai.
```

Katalog tool (read workspace/module, `propose_spec_file` + validation gate,
`apply_draft` + guard `vendors/`, kontrol server dev, Forma Skill):
[`../ai/03-forma-local-mcp.md`](../ai/03-forma-local-mcp.md).

## 5. Status Implementasi Hari Ini

Belum ada yang diimplementasikan — `forma-consult`, `forma mcp-serve`, kedua
MCP server, dan Forma Skill adalah target desain. Rencana pengerjaan:
[`../plan/todo.md`](../plan/todo.md) Fase 10; pertanyaan terbuka per komponen
ada di bagian akhir masing-masing dokumen [`docs/ai/`](../ai/README.md).

## 6. Referensi

| Dokumen | Isi |
|---|---|
| [`../ai/README.md`](../ai/README.md) | Forma AI: komponen, fitur yang didukung, prinsip desain |
| [`../ai/01-architecture.md`](../ai/01-architecture.md) | Empat lapisan, dua artifact, tool-use loop |
| [`02-forma-cli.md`](02-forma-cli.md) | CLI `forma` utama — `mcp-serve` akan jadi subcommand di sini |

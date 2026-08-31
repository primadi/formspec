# 2026-08-27-007 — Fase 10.3/10.4/10.6/10.7: lengkapi alur consult

## Apa yang diubah

Menuntaskan sisa Fase 10 kecuali 10.5 (remote MCP) dan item yang menunggu
Fase 13:

- **10.3.1/10.3.2 ✅** — validation gate composite + scope structural sudah
  terimplementasi di 10.1; todo ditandai dengan verifikasi bahwa client
  built-in memakai jalur MCP yang sama dengan client eksternal.
  **10.3.3 ⏸️** deferred ke Fase 13 (belum ada signature vendor).
- **10.4.1 ✅** — `discovery-summary.md`: perintah REPL `/summary` meminta
  model merangkum discovery dalam bahasa awam → ditulis ke file untuk
  konfirmasi business owner (`Session.WriteDiscoverySummary`).
- **10.4.3 ✅** — accept/reject per file: `/apply <path>` (satu file),
  `/apply` (semua), `/reject <path>` (buang draft, spec tree tak tersentuh).
- **10.6.1 ✅** — parser skill membaca `applies_to_kind` +
  `min_core_spec_version` (06 §2).
- **10.6.2 ✅** — 4 skill baru di `ai_skills/`: `entity-authoring`,
  `form-layout`, `entity-extension-authoring`, `module-vendoring` — isi
  diground ke docs/spec (field types, characteristic, FormSpecExpr, guard
  vendors/, trust tier).
- **10.6.4 ✅** — re-cek skill relevan deterministik di `propose_spec_file`:
  parse kind draft → match `applies_to_kind` → `relevant_skills` dikembalikan
  dalam hasil tool; model membaca via `read_skill` sebelum lanjut.
- **10.7.1/10.7.2 ✅** — sudah di 10.1 (undo backup + guard vendors/), todo
  ditandai.

## File terdampak

- Baru: `ai_skills/{entity-authoring,form-layout,entity-extension-authoring,
  module-vendoring}/SKILL.md`, `cmd/formspec/skills_test.go`
- Ubah: `internal/consult/repl.go` (/summary, /apply per-file, /reject),
  `internal/consult/session.go` (WriteDiscoverySummary),
  `cmd/formspec/mcpserve.go` (skillMeta fields),
  `cmd/formspec/mcp_consult.go` (relevantSkillsFor),
  `ai_skills/README.md`, `internal/consult/consult_test.go`

## Referensi

- Plan: `docs/plan/fase10-consult-client.md`, `docs/plan/fase10-local-mcp.md`
- Spec: `docs/ai/02-formspec-consult.md`, `docs/ai/06-formspec-skill.md`

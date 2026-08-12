# Agent-Assisted App Development (tanpa MCP)

Panduan untuk AI agent (mis. VS Code Copilot) yang membantu membangun aplikasi
FormSpec. Mekanisme ini **tidak bergantung pada MCP server** — agent bekerja
langsung di project memakai CLI `formspec` + file yang di-scaffold oleh
`formspec init`.

> Arsitektur MCP (`formspec-consult`, `formspec-local-mcp`, `formspec-remote-mcp`)
> adalah **desain yang ditunda** (`docs/ai/`, todo.md Fase 10). Konten skill
> dan guide ini sengaja dibuat MCP-agnostic sehingga bisa dipakai ulang tanpa
> penulisan ulang saat arsitektur itu landing.

## Bagaimana Ini Bekerja

`formspec init` menyiapkan project agar agent bisa memandu pembuatan aplikasi:

| Artifact | Peran |
|---|---|
| `.agents/skills/*/SKILL.md` | 4 skill: workflow orkestrator, katalog kind, navigasi spec, validasi |
| `.github/copilot-instructions.md` | Instruksi agent: workflow 4 fase + aturan validasi |
| `schemas/` + `.vscode/settings.json` | Autocomplete + validasi YAML di editor |
| `formspec validate` | Validation gate (engine + JSON Schema) |

## Workflow 4 Fase

Agent mengikuti fase berurutan dan **tidak boleh melompat ke YAML** sebelum
kebutuhan bisnis dikonfirmasi:

```
Discovery ──→ Proposal ──→ Draft ──→ Iterate
```

| Fase | Output | Gate |
|---|---|---|
| Discovery | `docs/overview.md` | Persetujuan user |
| Proposal | `docs/architecture.md` + `docs/domain-model.md` | Persetujuan user |
| Draft | `spec/modules/**/*.yaml` + `spec/apps/*.yaml` | `formspec validate` = 0 problem |
| Iterate | changelog + update file | `formspec validate` + consistency check |

## Deteksi Fase

Fase ditentukan oleh **state percakapan**, bukan keberadaan file. Tiga sinyal
berurutan:

1. **Tipe request user** — "buat aplikasi baru" → Discovery; "tambah fitur /
   ubah aturan" → Iterate.
2. **Gate konfirmasi** — fase maju hanya lewat persetujuan eksplisit user di
   percakapan. `docs/overview.md` ada tapi belum disetujui = masih Discovery.
3. **Artifact workspace** — petunjuk orientasi saja, bukan state machine
   (docs bisa basi / ditulis ad-hoc).

Saat ragu: tanya user. Detail lengkap ada di skill `formspec-app-workflow`
section "Phase Detection".

## Tool Map Tanpa MCP

| Kebutuhan | Ekivalen tanpa MCP |
|---|---|
| Baca manifest / modul terinstal | Baca `spec/apps/*.yaml` + `spec/modules/*/module.yaml` |
| Index + isi skill | `/skills` di Copilot; baca `.agents/skills/<name>/SKILL.md` |
| Schema kind | `schemas/kinds/<Kind>.schema.json` |
| Validasi | `formspec validate --spec spec` (harus exit 0) |
| Tulis draft | Tulis YAML langsung ke `spec/...`, lalu validasi |
| Apply / review | `git diff` + `git status`; commit per file |
| Cek bentrok nama | `formspec validate` + grep `spec/` |
| Kontrol dev | `formspec dev` di terminal |

## Validation Gate

- Jalankan `formspec validate --spec spec` **setelah setiap penulisan signifikan**.
- Target: `0 problem(s) found`.
- Kalau binary basi: `make build`, atau
  `go run ./cmd/formspec validate --spec <path>`.
- Error diklasifikasi `engine:` vs `schema:` dan diperbaiki lewat katalog
  perbaikan di skill `schema-validation`.

## Contoh Referensi

[`examples/cafe/`](../../examples/cafe/) adalah aplikasi referensi yang
dibangun lewat alur ini — pola `docs/` + `spec/` + `.agents/skills/` yang
sama dengan `examples/arisan/`.

## Lingkup & Batas

- Tidak ada CLI command baru — murni skill + instructions + `formspec validate`.
- Template industri (probing questions per pola bisnis) ditunda — menunggu
  desain MCP `formspec-remote-mcp`.
- MCP (`docs/ai/`, Fase 10) di-defer; skill tetap kompatibel.

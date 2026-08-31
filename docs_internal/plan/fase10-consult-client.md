# Fase 10.2: `formspec consult` — Client Konsultasi (Go)

> Referensi spec: `docs/ai/02-formspec-consult.md` (alur Discovery → Proposal →
> Draft, sesi, diff), `docs/ai/05-llm-provider-layer.md` (BYOK, capability bar,
> credential), `docs/ai/01-architecture.md` §3 (tool-use loop), §5 (auto-invoke),
> §6 (kompresi riwayat), `docs/cli-tools/05-formspec-consult.md` (verb CLI).

## Keputusan Desain (deviasi dari spec — dicatat, docs/ai akan direvisi)

1. **Bahasa Go, subcommand `formspec consult`** — bukan TypeScript + Vercel AI
   SDK (`docs/ai/01` §2, `05` §1). Alasan: satu binary, tanpa toolchain
   bun/node, MCP Go SDK resmi (`modelcontextprotocol/go-sdk`) punya client +
   server yang sama matangnya, `zalando/go-keyring` adalah library Go, dan
   seluruh tim sudah Go. `docs/ai/01` §2 dan `05` §1 direvisi menyusul.
2. **LLM SDK: `openai-go` (SDK resmi OpenAI)** di balik interface internal
   `llm.Provider` — bukan framework agentic (`langchaingo` ditolak: API tidak
   stabil; `go-ai`/digitallysavvy dievaluasi dan ditolak: bus factor, paritas
   1:1 dengan Vercel SDK sulit dipertahankan, permukaan dependency besar).
   Target provider: **DeepSeek v4 Flash & GLM 5.3 Flash via gateway
   OpenAI-compatible** (base URL override) — wire format OpenAI menutup
   semuanya. Tipe SDK tidak bocor keluar dari package `llm`.
3. **Tool loop ditulis sendiri** (~100–150 baris) di `internal/consult` —
   satu wire format = satu loop; testable dengan mock provider tanpa API nyata.
4. **MCP boundary tetap dipertahankan**: client spawn `formspec mcp-serve`
   sebagai child process (stdio) via `CommandTransport` — jalur identik dengan
   client MCP eksternal, sehingga validation gate & guard `vendors/` berlaku
   sama (01 §4, 03 §2). Bukan in-process direct call.
5. **Option picker langsung di REPL** (keputusan user): model sering
   menyajikan opsi A/B/C — REPL mendeteksi blok opsi, user memilih satu
   huruf, seleksi di-inject eksplisit ke riwayat. Detail di §Option Picker.
6. **History konsultasi disimpan ke file untuk review** (keputusan user):
   `transcript.md` ditulis incremental per turn — sesuai 10.4.1. Detail di
   §Transcript.

## Struktur File

| File                                 | Isi                                                                                                |
| ------------------------------------ | -------------------------------------------------------------------------------------------------- |
| `internal/consult/llm/provider.go`   | Interface `Provider` (`GenerateText`/`StreamText` + tools), tipe `Message`/`ToolCall`/`ToolResult` |
| `internal/consult/llm/openai.go`     | Adapter `openai-go` — base URL override (DeepSeek/GLM/OpenCode gateway), konversi tool schema      |
| `internal/consult/llm/credential.go` | `CredentialStore`: OS keyring (`zalando/go-keyring`) → env var → error jelas (05 §3)               |
| `internal/consult/mcp.go`            | Wrapper MCP client: spawn `formspec mcp-serve`, `ListTools`, `CallTool`                            |
| `internal/consult/loop.go`           | Tool-use loop: kirim → cek tool_call → eksekusi via MCP → append tool_result → ulang (01 §3)       |
| `internal/consult/session.go`        | Sesi: id, transcript writer, riwayat in-memory, kompresi (10.2.7)                                  |
| `internal/consult/options.go`        | Deteksi blok opsi `A) … B) …` + parsing                                                            |
| `internal/consult/repl.go`           | REPL: prompt, render balasan, option picker, perintah `/diff`, `/apply`, `/status`, `/quit`        |
| `internal/consult/prompt.go`         | System prompt consultant (Discovery → Proposal → Draft, 02 §1) + konvensi format opsi              |
| `cmd/formspec/consult.go`            | Subcommand `formspec consult` + `formspec consult diff` (10.4.2)                                   |

## Option Picker (keputusan user)

- **Konvensi system prompt**: saat menyajikan pilihan, model memformat
  `A) …` / `B) …` di awal baris (satu opsi satu baris, berurutan).
- **Deteksi di REPL**: balasan terakhir discan — blok ≥2 baris berpola
  `^[A-Z]\)` berurutan → tampilkan prompt `Pilih A–C (atau ketik bebas):`.
- **Seleksi di-inject eksplisit** sebagai user message:
  `[memilih: B) <teks lengkap opsi>]` — teks lengkap disertakan supaya
  konteks tidak hilang saat kompresi riwayat (10.2.7).
- **Fallback selalu ada**: ketik bebas tetap bisa; parsing opsi adalah
  nice-to-have, bukan jalur wajib — tidak bergantung disiplin model
  (prinsip yang sama dengan validation gate 03 §2).
- **Tidak dipaksa via structured output / pseudo-tool** — model lemah sering
  skip tool call atau salah format; konvensi teks + deteksi client lebih tahan.

## Transcript (keputusan user — 10.4.1 sebagian)

- Path: `.formspec/consult/{session}/transcript.md` — **ditulis incremental
  (append per turn), tidak di-buffer di memori** — aman terhadap crash.
- Format Markdown human-readable untuk review:
  - Header: session id, provider/model, waktu mulai, project path.
  - Per turn: `## Turn N — user|assistant|tool` + isi; tool call dicatat
    ringkas (nama + argumen ter-truncate + status), bukan dump penuh.
- Review: file dibaca langsung, atau `formspec consult --resume <session>`
  melanjutkan sesi (riwayat di-rebuild dari transcript).
- `discovery-summary.md` menyusul (fase Discovery selesai — 02 §1).

## Credential (10.2.4, 05 §3)

Urutan sesuai spec: **OS keyring** (`zalando/go-keyring`, service
`formspec-consult`) → **env var** (`FORMSPEC_LLM_API_KEY`, fallback
`OPENAI_API_KEY`) → error jelas yang memandu set keduanya. Flag
`--api-key-env` untuk override nama env var (headless/CI).

## Auto-invoke (10.2.6, 01 §5)

Saat sesi mulai (baru maupun resume), client memanggil deterministik:
`read_workspace_manifest` + `list_installed_modules` + `list_skills` — hasil
di-inject sebagai context awal (bukan inisiatif LLM). Skill index hanya
name+description; isi dibaca model via `read_skill` saat topik cocok.

## Kompresi Riwayat (10.2.7, 01 §6)

- Trigger: riwayat melewati ambang (estimasi token / jumlah turn).
- Turn lama didistilasi jadi ringkasan terstruktur (panggilan provider
  terpisah); pasangan `tool_use`/`tool_result` dijaga utuh; seleksi opsi
  dan keputusan bisnis dipertahankan sebagai fakta.
- Transcript penuh **tidak pernah dipotong** — file di disk adalah sumber
  kebenaran untuk review.

## Capability Bar (05 §2)

Validated-provider list di `internal/consult/llm`: DeepSeek, GLM (OpenCode
gateway), OpenAI. Model di luar daftar tetap bisa dipakai dengan flag
eksplisit `--allow-unvalidated` (warning, bukan blok) — bar formal
(benchmark tool-calling) menyusul setelah ada data pemakaian.

## Testing

- Unit: `options.go` (parsing blok opsi, edge case), `session.go` (transcript
  append + resume rebuild), `loop.go` (mock provider: urutan tool call,
  tool_result pairing, max-steps guard), `llm/openai.go` (httptest server
  OpenAI-compatible: tool_calls parsing, base URL override).
- Integration: REPL headless (stdin pipe) terhadap mock provider + MCP server
  nyata di temp dir.
- E2E manual: sesi nyata terhadap `examples/cafe` via GLM/DeepSeek gateway.

## Out of Scope (batch ini)

- `discovery-summary.md` auto-generate (menyusul setelah alur Discovery stabil).
- 10.5 remote MCP, 10.6 skill baru, benchmark capability bar formal.
- Streaming render per-token (v1: render per-blok setelah selesai; streaming
  ditambahkan belakangan — SDK sudah mendukung).

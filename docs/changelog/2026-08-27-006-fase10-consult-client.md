# 2026-08-27-006 — Fase 10.2: `formspec consult` client (Go)

## Apa yang diubah

Implementasi client konsultasi AI sebagai subcommand Go `formspec consult`
(todo 10.2.1–10.2.8) — **deviasi keputusan dari spec** (`docs/ai/01` §2, `05` §1):
client dalam Go, bukan TypeScript + Vercel AI SDK. Kedua docs direvisi
mencatat keputusan ini.

- **`internal/consult/llm`** — LLM Provider Layer: interface `Provider`
  (tipe SDK tidak bocor), adapter `openai-go` (base URL override menutup
  DeepSeek/GLM/gateway OpenAI-compatible), `CredentialStore` keyring → env
  → error jelas (05 §3).
- **`internal/consult`** — tool-use loop ditulis sendiri (~100 baris, testable
  via executor seam + mock provider); MCP client spawn `formspec mcp-serve`
  via `modelcontextprotocol/go-sdk` `CommandTransport` (boundary identik
  dengan client eksternal); session dengan **transcript.md incremental per
  turn** (keputusan user: history wajib tersimpan ke file untuk review) +
  `Resume` rebuild dari transcript; **option picker A/B/C langsung di REPL**
  (keputusan user: deteksi blok opsi → pilih satu huruf → seleksi di-inject
  eksplisit dengan teks lengkap, fallback teks bebas); unified diff draft vs
  spec tree (10.4.2); system prompt consultant (Discovery → Proposal → Draft).
- **`cmd/formspec/consult.go`** — subcommand `consult` (+ `consult diff`):
  validated-provider list (deepseek/glm/openai) dengan capability bar,
  auto-invoke deterministik `read_workspace_manifest` +
  `list_installed_modules` + `list_skills` di awal sesi (10.2.6).

## Keputusan teknis

- `openai-go` dipilih di atas `go-ai` (digitallysavvy) dan `langchaingo` —
  SDK resmi, bus factor rendah, satu wire format menutup semua target
  provider; capability bar berarti adapter 25+ provider tidak pernah terpakai.
- Boundary MCP dipertahankan (bukan in-process call) supaya validation gate
  dan guard `vendors/` berlaku sama untuk client built-in maupun eksternal.
- Transcript di-append per turn (bukan buffer memori) — aman crash; tool
  traffic dicatat ringkas untuk review manusia.

## File terdampak

- Baru: `internal/consult/{llm/{provider,openai,credential}.go, mcp.go,
loop.go, session.go, options.go, repl.go, diff.go}` + test
  (`consult_test.go`, `loop_test.go`, `llm/openai_test.go`),
  `cmd/formspec/consult.go`
- Ubah: `cmd/formspec/main.go` (subcommand), `docs/ai/01-architecture.md` §2,
  `docs/ai/05-llm-provider-layer.md` §1, `docs/ai/README.md`,
  `docs/cli-tools/05-formspec-consult.md`, `go.mod`/`go.sum`
  (`openai-go`, `go-keyring`)

## Referensi

- Plan: `docs/plan/fase10-consult-client.md`
- Spec: `docs/ai/02-formspec-consult.md`, `docs/ai/05-llm-provider-layer.md`

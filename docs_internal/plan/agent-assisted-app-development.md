# Plan — AI-Agent-Assisted App Development (tanpa MCP)

**Tanggal:** 2026-08-11 · **Status:** Done (implementasi)
**Referensi:** `docs/guides/agent-assisted-app-development.md`, `ai_skills/formspec-app-workflow/SKILL.md`

## Masalah

FormSpec adalah aplikasi untuk membangun aplikasi berbasis spec. Proses pembuatan
aplikasi ingin dibantu AI agent, **tanpa MCP server** (`formspec-consult`,
`formspec-local-mcp`, `formspec-remote-mcp` — semuanya masih desain di `docs/ai/`,
todo.md Fase 10, belum diimplementasikan). Mekanisme yang benar-benar ada:

- `ai_skills/*/SKILL.md` di-embed ke binary CLI (`embed_skills.go`), ditulis
  `formspec init` ke `.agents/skills/` di project baru.
- `.github/copilot-instructions.md` + `schemas/` + `.vscode/settings.json`
  juga di-scaffold oleh `formspec init`.
- `formspec validate` berfungsi penuh (engine + JSON Schema) — validation gate
  yang tersedia hari ini.

Gap: skill `formspec-app-workflow` belum menjelaskan (a) cara menyimpulkan fase
dari state percakapan, dan (b) tool map untuk bekerja tanpa MCP. Instruksi
yang ditulis `formspec init` belum mereferensikan workflow 4 fase. Belum ada
contoh aplikasi yang membuktikan alur ini jalan.

## Solusi

1. **Skill layer** — tambah section "Phase Detection" (tiga sinyal: tipe
   request user → gate konfirmasi → artifact workspace sebagai petunjuk) dan
   "No-MCP Tool Map" (peta kebutuhan workflow → ekivalen tanpa MCP; konten
   MCP-agnostic agar reuse saat Fase 10 landing).
2. **Scaffolding** — tulis ulang `makeCopilotInstructions` di `cmd/formspec/init.go`
   agar mereferensikan workflow 4 fase + `formspec validate` sebagai gate; update
   output "Next steps"; tambah `init_test.go` yang memverifikasi scaffolding.
3. **Dokumentasi** — `docs/guides/agent-assisted-app-development.md` (kontrak
   workflow, deteksi fase, tool map, validation gate) + daftar di
   `docs/guides/README.md`.
4. **Contoh referensi** — `examples/cafe/` dibangun lewat alur ini:
   `formspec init` (scaffold) → docs (overview/architecture/domain-model) →
   spec (App + 3 modul: cafe-master, cafe-order, cafe-report) → `formspec validate`
   = 0 problem.

## File terdampak

| File | Aksi |
|---|---|
| `ai_skills/formspec-app-workflow/SKILL.md` | Ubah — tambah Phase Detection + No-MCP Tool Map |
| `cmd/formspec/init.go` | Ubah — tulis ulang `makeCopilotInstructions` + Next steps |
| `cmd/formspec/init_test.go` | Baru — assert scaffolding (skills, schemas, instructions) |
| `docs/guides/agent-assisted-app-development.md` | Baru — kontrak guide |
| `docs/guides/README.md` | Ubah — index entry |
| `examples/cafe/**` | Baru — contoh referensi (docs + spec + scaffold) |
| `examples/README.md` | Ubah — daftar cafe |
| `docs/plan/todo.md` | Ubah — task row + catatan MCP deferred |
| `docs/changelog/2026-08-11-001-agent-assisted-app-development.md` | Baru |

## Verifikasi

1. `make build` sukses — skill ter-embed compile ke binary.
2. `go test ./cmd/formspec/...` lolos (20 test, termasuk `init_test.go`).
3. `formspec validate --spec examples/cafe/spec` → `16 manifest(s) validated, 0 problem(s) found`.
4. `go test ./internal/manifest/ -run TestExamplesLoadAndValidate` lolos — engine
   menerima contoh cafe (deep validation termasuk child relation).
5. `formspec init` di temp dir menghasilkan 4 skill + copilot-instructions yang
   mereferensikan workflow (di-cover `init_test.go`).

## Keputusan

- Tidak ada CLI command baru — murni skill + instructions + `formspec validate`.
- MCP (`docs/ai/`, Fase 10) di-defer eksplisit; konten skill MCP-agnostic.
- Template industri (probing questions per pola bisnis) ditunda — menunggu MCP.
- `examples/cafe` menyertakan `.github/copilot-instructions.md` (parity dengan
  project hasil `formspec init`; `arisan` tidak punya).
- Guide ditaruh di `docs/guides/` (bukan `docs/ai/` yang merupakan home desain
  MCP yang di-defer).

# 2026-08-11-001-agent-assisted-app-development

**Lapis**: AI Skill + CLI scaffolding + Docs + Contoh
**Referensi**: `docs/plan/agent-assisted-app-development.md`

## Perubahan

Perkuat mekanisme **tanpa-MCP** untuk proses pembuatan aplikasi FormSpec yang
dibantu AI agent, memakai yang sudah ada: `ai_skills/` (di-embed CLI, ditulis
`formspec init` ke `.agents/skills/`), `.github/copilot-instructions.md`, dan
`formspec validate` sebagai validation gate. Arsitektur MCP (`docs/ai/`, todo.md
Fase 10) tetap di-defer — konten skill dibuat MCP-agnostic agar reuse nanti.

## Isi

1. **Skill** — `formspec-app-workflow` ditambah section **Phase Detection** (tiga
   sinyal: tipe request user → gate konfirmasi → artifact workspace sebagai
   petunjuk, bukan state machine file) dan **No-MCP Tool Map** (peta kebutuhan
   workflow → ekivalen tanpa MCP, mis. `formspec validate` ganti `validate_spec`,
   tulis file langsung ganti `propose_spec_file`, `git diff` ganti `apply_draft`).
2. **Scaffolding** — `makeCopilotInstructions` di `cmd/formspec/init.go` ditulis
   ulang: workflow 4 fase, daftar 4 skill, `formspec validate --spec spec` sebagai
   gate, aturan "validate setelah setiap penulisan". Output "Next steps" ikut
   diupdate. `init_test.go` baru memverifikasi scaffolding (skills + schemas +
   copilot-instructions).
3. **Docs** — `docs/guides/agent-assisted-app-development.md` (kontrak workflow,
   deteksi fase, tool map, validation gate) terdaftar di `docs/guides/README.md`.
4. **Contoh** — `examples/cafe/` (App kafe): scaffold `formspec init`, docs
   (overview/architecture/domain-model), spec 3 modul (`cafe-master` master,
   `cafe-order` transaction dengan inline child `items` + relation, `cafe-report`
   dashboard/widgets/report). `formspec validate` = 16 manifest, 0 problem.

## File terdampak

- `ai_skills/formspec-app-workflow/SKILL.md` — tambah 2 section
- `cmd/formspec/init.go` — tulis ulang copilot-instructions + Next steps
- `cmd/formspec/init_test.go` — baru
- `docs/guides/agent-assisted-app-development.md` — baru; `docs/guides/README.md` — index
- `examples/cafe/**` — baru (16 manifest + docs + scaffold)
- `examples/README.md` — daftar cafe
- `docs/plan/todo.md` — task row + catatan MCP deferred
- `docs/plan/agent-assisted-app-development.md` — baru (plan)

## Verifikasi

- `make build` ✅ · `go test ./cmd/formspec/...` ✅ (20 test)
- `formspec validate --spec examples/cafe/spec` → 0 problem ✅
- `go test ./internal/manifest/ -run TestExamplesLoadAndValidate` ✅

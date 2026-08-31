# Fase 10.1: `formspec-local-mcp` — Tool Server (`formspec mcp-serve`)

> Referensi spec: `docs/ai/03-formspec-local-mcp.md` (katalog tool, validation
> gate, guard vendors/, kontrol server), `docs/ai/06-formspec-skill.md`
> (list_skills/read_skill), `docs/ai/01-architecture.md` §2 (lokal saja).

## Scope (todo 10.1.3–10.1.7 + tool grounding pendukung)

| Tool                                                         | Spec    | File                          | Status |
| ------------------------------------------------------------ | ------- | ----------------------------- | ------ |
| `list_kind_schemas(kind)`                                    | 03 §1   | `cmd/formspec/mcpserve.go`    | ✅     |
| `read_workspace_manifest()`                                  | 03 §1   | `cmd/formspec/mcpserve.go`    | ✅     |
| `list_installed_modules()`                                   | 03 §1   | `cmd/formspec/mcpserve.go`    | ✅     |
| `read_module_spec(module, kind, name)`                       | 03 §1   | `cmd/formspec/mcpserve.go`    | ✅     |
| `propose_spec_file(session, path, content)`                  | 03 §2   | `cmd/formspec/mcp_consult.go` | ✅     |
| `apply_draft(session, file)`                                 | 03 §4   | `cmd/formspec/mcp_consult.go` | ✅     |
| `validate_spec(yaml)`                                        | 03 §3   | `cmd/formspec/mcp_consult.go` | ✅     |
| `check_naming_conflict(name)`                                | 03 §1   | `cmd/formspec/mcp_consult.go` | ✅     |
| `restart_server()` / `get_server_status()` / `stop_server()` | 03 §5   | `cmd/formspec/mcp_consult.go` | ✅     |
| `list_skills()` / `read_skill(name)`                         | 06 §2–3 | `cmd/formspec/mcpserve.go`    | ✅     |

## Keputusan Teknis

1. **SDK resmi MCP** — `github.com/modelcontextprotocol/go-sdk` v1.7.0, stdio
   transport (`formspec mcp-serve` dijalankan sebagai child process oleh
   client MCP). Semua log ke stderr — stdout adalah kanal protokol.
2. **Validation gate = satu package, tiga pemanggil** (03 §3): `validateSpecTree`
   di `cmd/formspec/mcp_consult.go` memakai `internal/manifest` (engine loader)
   - `kindSchemaCompiler` (JSON Schema layer) + honesty scan — package yang sama
     dengan `formspec validate`. Bukan reimplementasi. Scope structural saja.
3. **`propose_spec_file` composite** (03 §2): tulis draft ke
   `.formspec/consult/{session}/draft/{path}` → validasi otomatis dengan
   _overlay_ draft ke salinan spec tree (cross-file reference tercek) →
   return `{written, validation}`. LLM tidak bisa skip validasi.
4. **Guard `vendors/` ditegakkan di kode** (03 §4, 10.7.2): semua tool tulis
   (`propose_spec_file`, `apply_draft`) menolak path di bawah `vendors/`
   (spec-relative dan project-root), dengan pesan yang mengarahkan ke Entity
   Extension / shadow copy.
5. **`apply_draft` auto-backup** (02 §4, 10.7.1): file asli disalin ke
   `.formspec/consult/{session}/undo/{path}` sebelum ditimpa.
6. **Kontrol server dev** (03 §5): PID file `.formspec/dev.pid` (sudah ditulis
   `formspec dev`). `restart_server` = validate dulu (tolak kalau invalid) →
   stop → spawn `formspec dev` detached (log → `.formspec/consult/server.log`)
   → poll `/health`. `get_server_status` = PID alive + GET `/health`.
7. **Skill** (06 §2): dibaca dari `AISkillsFS` (embed `ai_skills/*/SKILL.md`,
   sudah ada). `list_skills` parse frontmatter (name, description);
   `read_skill` return Markdown mentah (06 §2 — bukan JSON terstruktur).
8. **Path draft = relatif terhadap spec dir** (`--spec`, default `spec`) —
   sesuai realita loader hari ini; guard `vendors/` juga cek project-root
   untuk layout §6.1 yang akan datang.

## Out of scope (deferred)

- `formspec.lock` parsing penuh (Fase 13 — Module Vendoring belum ada);
  `list_installed_modules` scan `modules/`+`vendors/` + baca lock best-effort.
- 10.2 client `formspec-consult`, 10.5 remote MCP, 10.6 skill baru.

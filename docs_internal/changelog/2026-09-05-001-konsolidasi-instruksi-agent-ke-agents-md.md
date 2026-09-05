# 2026-09-05-001-konsolidasi-instruksi-agent-ke-agents-md.md

**Apa:** Mengkonsolidasi instruksi agent dari tiga file menjadi satu `AGENTS.md` di root.

**Kenapa:** `CLAUDE.md` (Claude Code) dan `.github/copilot-instructions.md` (Copilot) berisi konten yang saling melengkapi tapi terpisah, dan bagian RTK di keduanya redundan karena rewrite sudah ditangani hook (`.github/hooks/rtk-rewrite.json` untuk Copilot, `rtk hook claude` untuk Claude Code). Dokumentasi resmi VS Code merekomendasikan hanya satu file instruksi (`AGENTS.md` ATAU `copilot-instructions.md`), dan `AGENTS.md` adalah open standard yang dibaca oleh semua agent (Copilot, Claude Code, Cursor, dll). Konsolidasi menghilangkan duplikasi token di konteks.

**File terkena dampak:**

- `AGENTS.md` — dibuat: gabungan framework guide (dari `copilot-instructions.md`) + konvensi repo (dari `CLAUDE.md`) + bagian RTK ringkas
- `CLAUDE.md` — dihapus
- `.github/copilot-instructions.md` — dihapus
- `.github/agents/todo-runner.agent.md` — referensi di-update dari `copilot-instructions.md` ke `AGENTS.md`

**Catatan:** `cmd/formspec/init.go` dan `docs/guides/agent-assisted-app-development.md` TIDAK diubah — keduanya mendeskripsikan output `formspec init` untuk project baru, yang tetap menulis `AGENTS.md` + pointer tipis `copilot-instructions.md` (per desain changelog `2026-09-04-004`).

**Referensi:** `docs_internal/plan/agent-assisted-app-development.md`, changelog `2026-09-04-004-init-agents-md.md`

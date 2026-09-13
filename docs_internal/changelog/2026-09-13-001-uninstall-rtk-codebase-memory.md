# 2026-09-13-001 — Uninstall `rtk` & `codebase-memory-mcp`

**Apa:** Menghapus dua tooling pihak ketiga yang sebelumnya di-bake ke
devcontainer dan di-inject ke sesi agent: **`rtk`** (Rust Token Killer,
`github.com/rtk-ai/rtk`) dan **`codebase-memory-mcp`**
(`DeusData/codebase-memory-mcp`, MCP knowledge-graph server).

**Kenapa:** Keduanya tidak lagi diinginkan — `rtk` me-rewrite perintah shell
lewat hook (opaque, sulit di-debug) dan `codebase-memory-mcp` memaksa agent
memakai graph MCP sebelum grep/read, plus mengonsumsi 293 MB binary + index.

## Perubahan

**Repo:**

- `.devcontainer/Dockerfile` — hapus blok `RUN curl ... rtk-ai/rtk/install.sh`
  (dan komentarnya). `USER vscode` tetap ada karena blok install opencode CLI
  masih membutuhkannya.
- `.devcontainer/devcontainer.json` — `postCreateCommand` kembali menjadi hanya
  `npm install`; `rtk init -g --auto-patch`/`--opencode` dan rantai
  install/config/index `codebase-memory-mcp` dihapus.

Integrasi level repo lainnya (`AGENTS.md`, `.github/agents/todo-runner.agent.md`,
`.github/hooks/rtk-rewrite.json`, `.github/skills/codebase-memory-id/`,
`.rtk/filters.toml`, `.vscode/mcp.json`) sudah dihapus di worktree yang sama
sebelum entry ini.

**Artefak terinstal (persisten, host home via bind-mount `/workspaces`):**

- `rtk`: binary `~/.local/bin/rtk`, `~/.config/rtk/`, `~/.local/share/rtk/`,
  `~/.config/opencode/plugins/rtk.ts`, `~/.claude/RTK.md`, `~/.claude/CLAUDE.md`
  (isinya hanya `@RTK.md`), dan hook `rtk hook claude` + permission `rtk *` di
  `~/.claude/settings.json`.
- `codebase-memory-mcp`: binary `~/.local/bin/codebase-memory-mcp` (293 MB),
  installer `~/.local/bin/install.sh`, cache/index `~/.cache/codebase-memory-mcp/`,
  log `~/.cache/claude-cli-nodejs/*/mcp-logs-codebase-memory-mcp/`, hook
  `~/.claude/hooks/cbm-{code-discovery-gate,session-reminder,subagent-reminder}`,
  skill `~/.claude/skills/codebase-memory/` + `~/.config/opencode/skills/codebase-memory/`,
  agent `codebase-memory{,-scout,-auditor}.md` (Claude + opencode), plugin
  `~/.config/opencode/plugins/cbm-augment.ts`, blok AGENTS.md opencode, dan
  registrasi MCP di `~/.claude.json` + `~/.config/opencode/opencode.jsonc`.
- `~/.claude/settings.json` di-restore dari `settings.json.bak` (state pra-rtk:
  permission `go build`/`npm install` + `theme: dark`), lalu `.bak` dihapus.
- Sisa state ephemeral di `/home/vscode` (`~/.config/rtk`, `~/.local/share/rtk`,
  `~/.config/opencode/plugins/rtk.ts`, `~/.claude/CLAUDE.md`) ikut dihapus.

## File terdampak

- `.devcontainer/Dockerfile`, `.devcontainer/devcontainer.json`
- `docs_internal/plan/uninstall-rtk-codebase-memory.md` (plan baru)
- `docs_internal/plan/todo.md` (catatan + `Last Updated`)

## Verifikasi

`command -v rtk` dan `command -v codebase-memory-mcp` → not found; `find` atas
`~/.local`, `~/.config`, `~/.cache`, `~/.claude` tidak menemukan sisa
`*rtk*`/`*codebase-memory*`/`*cbm-*`.

## Referensi

- Plan: `docs_internal/plan/uninstall-rtk-codebase-memory.md`
- Entry pemasangan: `docs_internal/changelog/2026-07-31-003-bake-rtk-ke-devcontainer-dockerfile.md`
- Catatan: `docs_internal/plan/*.md` yang menyebut `rtk go test ...` dibiarkan
  apa adanya (catatan historis).

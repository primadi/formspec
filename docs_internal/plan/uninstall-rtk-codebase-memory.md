# Plan — Uninstall `rtk` & `codebase-memory-mcp`

**Status:** ✅ selesai · **Tanggal:** 2026-09-13 · **Effort:** small

## Tujuan

Hapus dua tooling pihak ketiga yang sebelumnya di-bake ke environment agar
tidak lagi terpasang maupun ter-inject ke sesi agent:

1. **`rtk`** (Rust Token Killer, `github.com/rtk-ai/rtk`) — CLI proxy yang
   me-rewrite perintah shell via hook.
2. **`codebase-memory-mcp`** (`DeusData/codebase-memory-mcp`) — MCP server
   knowledge-graph + hook/skill/agent pendampingnya.

## Catatan penting: state sudah sebagian dibersihkan

Integrasi level repo **sudah dihapus** di worktree (belum di-commit) oleh sesi
sebelumnya:

- `AGENTS.md` — blok `<!-- rtk-instructions v2 -->` dihapus
- `.github/agents/todo-runner.agent.md` — `rtk go test`/`rtk vitest` → perintah polos
- `.github/hooks/rtk-rewrite.json` — dihapus
- `.github/skills/codebase-memory-id/SKILL.md` — dihapus
- `.rtk/filters.toml` — dihapus
- `.vscode/mcp.json` — server `codebase-memory` dihapus

Sisa pekerjaan = file `.devcontainer/*` + artefak terinstal.

## Mount layout (hasil investigasi)

`compose.yaml` mem-bind-mount `../..` (home host) → `/workspaces`. Jadi:

| Path                                                                                     | Sifat                     | Isi                                     |
| ---------------------------------------------------------------------------------------- | ------------------------- | --------------------------------------- |
| `/workspaces/.local`, `/workspaces/.config`, `/workspaces/.claude`, `/workspaces/.cache` | **persisten** (host)      | binary, config, hook, skill, agent, MCP |
| `/home/vscode/.local`, `/home/vscode/.config`, `/home/vscode/.claude`                    | **ephemeral** (container) | sisa `rtk init -g`                      |

`HOME=/home/vscode`, tapi `~/.local/bin` kosong → `rtk` exit 127.

## Perubahan

### A. File repo

| File                              | Aksi                                                                                                                      |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `.devcontainer/Dockerfile`        | Hapus blok `RUN curl ... rtk-ai/rtk install.sh`                                                                           |
| `.devcontainer/devcontainer.json` | `postCreateCommand`: hapus `rtk init -g ...` dan rantai install/config/index `codebase-memory-mcp`; sisakan `npm install` |

### B. Artefak `rtk` (persisten)

- `/workspaces/.local/bin/rtk` (10 MB)
- `/workspaces/.config/rtk/` (`config.toml`, `filters.toml`)
- `/workspaces/.local/share/rtk/` (`history.db`, `.device_salt`, dst.)
- `/workspaces/.config/opencode/plugins/rtk.ts`
- `/workspaces/.claude/RTK.md` + `/workspaces/.claude/CLAUDE.md` (isinya hanya `@RTK.md`)
- Hook di `/workspaces/.claude/settings.json` (`rtk hook claude` + permission `rtk *`);

  `settings.json.bak` (backup buatan `rtk init --auto-patch`) dipakai untuk
  restore state pra-rtk, lalu dihapus.

### C. Artefak `codebase-memory-mcp` (persisten)

- `/workspaces/.local/bin/codebase-memory-mcp` (293 MB)
- `/workspaces/.cache/codebase-memory-mcp/` (+ `_config.db`, index `.db`)
- `/workspaces/.cache/claude-cli-nodejs/*/mcp-logs-codebase-memory-mcp/`
- `/workspaces/.claude/hooks/cbm-code-discovery-gate`, `cbm-session-reminder`,
  `cbm-subagent-reminder` + entri hook-nya di `settings.json`
- `/workspaces/.claude/skills/codebase-memory/`, `/workspaces/.claude/agents/codebase-memory{,-scout,-auditor}.md`
- `/workspaces/.config/opencode/skills/codebase-memory/`, `/workspaces/.config/opencode/agents/codebase-memory{,-scout,-auditor}.md`
- Blok `<!-- codebase-memory-mcp:start -->…:end -->` di `/workspaces/.config/opencode/AGENTS.md`
- Registrasi MCP: `mcpServers.codebase-memory-mcp` di `/workspaces/.claude.json`
  dan `mcp.codebase-memory-mcp` di `/workspaces/.config/opencode/opencode.jsonc`

### D. Artefak ephemeral (`/home/vscode`)

- `~/.config/rtk/`, `~/.local/share/rtk/`, `~/.config/opencode/plugins/rtk.ts`,
  dan `~/.claude/CLAUDE.md` (`@RTK.md`)

## Yang TIDAK diubah

- `docs_internal/plan/*.md` & `docs_internal/changelog/2026-07-31-003-*.md` —
  catatan historis; menyebut `rtk go test` sebagai perintah saat itu.
- opencode CLI install di Dockerfile (tool terpisah, tidak diminta hapus).
- Config `sevima-mcp` di `opencode.jsonc` (tidak terkait).
- `memories/`, `~/.claude/skills/formspec-*`, skill FormSpec di repo.

## Dependensi & urutan

1. Edit file repo (A) — tidak bergantung apa pun.
2. Bersihkan hook/config JSON (B, C) **sebelum** menghapus binary, agar config
   tidak menunjuk ke path yang sudah hilang.
3. Hapus binary & cache.
4. Changelog + todo.
5. Verifikasi: `command -v rtk`, `command -v codebase-memory-mcp`, grep repo.

## Referensi

- `docs_internal/changelog/2026-07-31-003-bake-rtk-ke-devcontainer-dockerfile.md`
- `.devcontainer/Dockerfile`, `.devcontainer/devcontainer.json`, `.devcontainer/compose.yaml`

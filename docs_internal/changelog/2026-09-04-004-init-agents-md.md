# 2026-09-04-004 — `formspec init` menulis AGENTS.md, copilot-instructions jadi pointer

## Apa

`formspec init` kini menulis instruksi agent ke **`AGENTS.md`** (standar
tool-agnostic — dibaca Copilot, Codex, Cursor, Gemini CLI, dll), dan
`.github/copilot-instructions.md` hanya berisi pointer tipis ke `AGENTS.md`
(hanya ditulis jika belum ada, untuk versi Copilot lama yang belum membaca
AGENTS.md).

## Kenapa

`.github/copilot-instructions.md` spesifik Copilot; `AGENTS.md` membuat
scaffold vendor-neutral, konsisten dengan `.agents/skills/` yang sudah
tool-agnostic.

## File terkena dampak

- `cmd/formspec/init.go` — `makeCopilotInstructions` → `makeAgentsInstructions`,
  write `AGENTS.md`, pointer `copilot-instructions.md`, output teks di-update.
- `cmd/formspec/init_test.go` — assert `AGENTS.md` + kontennya.

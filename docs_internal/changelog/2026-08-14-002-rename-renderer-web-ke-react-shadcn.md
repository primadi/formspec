# Changelog 2026-08-14-002 — Rename `renderers/web` → `renderers/react-shadcn`

**Apa:** Folder frontend renderer di-rename dari `renderers/web/` menjadi
`renderers/react-shadcn/`, selaras dengan nilai `stack_family` (`react-shadcn`)
di spec Renderer. Semua referensi path di live code (path-lookup Go
`findWebDir`/`findWebDist`, komentar), build config (Makefile, devcontainer,
.gitignore), skill/AI config, dan docs aktif diperbarui. Referensi `web/` yang
stale (sisa restructure 0.3) di docs aktif ikut dibersihkan; folder `web/`
kosong di root dihapus.

**Kenapa:** Nama `web/` generik tidak menjelaskan stack-nya. `react-shadcn`
konsisten dengan konvensi nilai `stack_family` dan docs identity
`shadcn-shell` (`docs/renderers/shadcn-shell/` tetap — ada preseden
`jsonb-persist` ↔ `jsonbpersist`).

**File terdampak:**
- Rename: `renderers/web/` → `renderers/react-shadcn/` (+ `package.json` /
  `package-lock.json` `name` → `react-shadcn`)
- Go: `cmd/formspec/{dev.go,dev_vite.go,main.go}`, `internal/api/{handler.go,
  router.go,handler_update_test.go}`, `internal/ui/{meta.go,registry.go}`,
  `resource/formspec.go`, `examples/reference-app/main.go`
- Build: `Makefile`, `.devcontainer/devcontainer.json`, `.gitignore`
- Skill/AI: `.github/skills/forma-frontend/SKILL.md`,
  `.github/copilot-instructions.md`, `.claude/settings.json`
- Docs aktif: `docs/architecture/08-repo-structure.md`,
  `docs/renderers/{realtime.md,shadcn-shell/*}`,
  `docs/cli-tools/{01-formspec-dev,02-formspec-cli,03-formspec-generate}.md`,
  `docs/guides/how-to-run.md`, `docs/README.md`,
  `examples/Clinic-UI-Showcase/{README.md,how-to-run.md}`

**Tidak diubah (scope exclusion):** file historis `docs/changelog/`,
`docs/plan/`, `docs_old/`, `reff_docs/`; public API `--web-dir` / `WebDir` /
`web-dir` (tetap, bukan path). Build output (`dist/`, `node_modules/`)
diregenerasi, tidak dipindah manual.

**Referensi:** `docs/plan/rename-formspec.md` (pola scope exclusion),
keputusan user di session (nama `react-shadcn`, docs aktif saja, bersihkan
ref stale).

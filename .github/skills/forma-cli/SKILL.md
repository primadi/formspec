---
name: forma-cli
description: "Use when: working on Forma CLI — cmd/forma/, commands (apply, dev, validate, generate, etc.), flags, subcommand dispatch, or CLI documentation. Provides command status, implementation priority, and key design rules."
---

# Forma CLI Skill

Context for AI coding agents working on the Forma CLI (`cmd/forma/`).

## Key paths
- `cmd/forma/main.go` — Entry point, subcommand dispatch
- `cmd/forma/apply.go` — `forma apply`
- `cmd/forma/dev.go` — `forma dev`
- `cmd/forma/dev_vite.go` — Vite HMR integration
- `cmd/forma/dev_runtime.go` — Sidecar runtime setup
- `cmd/forma/dev_config.go` — Dev configuration
- `cmd/forma/generate.go` — `forma generate`
- `cmd/forma/generate_*.go` — Sidecar app scaffolders

## Command Status
| Command | Status |
|---|---|
| `apply` | Partial ✅ |
| `dev` | Partial ✅ |
| `generate --lang typescript` | Implemented ✅ |
| `validate`, `check`, `new`, `diff`, `get`, `describe`, `delete`, `migrate`, `repl`, `seed`, `backup`, `restore`, `logs` | Stub ⏳ |
| `promote`, `archive`, `saga`, `module`, `sign`, `script`, `freeze`, `rollback`, `lock`, `workspace` | Stub, deferred ⏸️ |

## Implementation priority (per docs/cli-tools/01-forma-cli.md)
1. `validate` — CI gate, high value
2. `new` — simple scaffold, quick DX win
3. `dev` — meaningful after pipeline fixed
4. `generate` — depends on pkg/spec stability
5. Everything else

## Key design rules
- Single binary: `cmd/forma` (not separate binaries per verb)
- `//go:embed dist/*` embeds SPA for single-process serving
- `--dev-ui` spawns Vite HMR for frontend development
- Runtime auto-detect from project files (go.mod, package.json, etc.)
- Config file: `forma-app.yaml`

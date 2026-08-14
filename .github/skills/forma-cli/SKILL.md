---
name: formspec-cli
description: "Use when: working on FormSpec CLI — cmd/formspec/, commands (apply, dev, validate, generate, etc.), flags, subcommand dispatch, or CLI documentation. Provides command status, implementation priority, and key design rules."
---

# FormSpec CLI Skill

Context for AI coding agents working on the FormSpec CLI (`cmd/formspec/`).

## Key paths

- `cmd/formspec/main.go` — Entry point, subcommand dispatch
- `cmd/formspec/apply.go` — `formspec apply`
- `cmd/formspec/dev.go` — `formspec dev`
- `cmd/formspec/dev_vite.go` — Vite HMR integration
- `cmd/formspec/dev_runtime.go` — Sidecar runtime setup
- `cmd/formspec/dev_config.go` — Dev configuration
- `cmd/formspec/generate.go` — `formspec generate`
- `cmd/formspec/generate_*.go` — Sidecar app scaffolders
- `cmd/formspec/validate.go` — `formspec validate` (version-routed schema layer)
- `cmd/formspec/schema.go` — `formspec schema` (fetch/update/list/clear)
- `cmd/formspec/schema_registry.go` — registry URL resolution (env > config > default)
- `internal/schemaregistry/` — registry client + local schema cache

## Command Status

| Command                                                                                                                 | Status            |
| ----------------------------------------------------------------------------------------------------------------------- | ----------------- |
| `apply`                                                                                                                 | Partial ✅        |
| `dev`                                                                                                                   | Partial ✅        |
| `generate --lang typescript`                                                                                            | Implemented ✅    |
| `validate`, `check`, `new`, `diff`, `get`, `describe`, `delete`, `migrate`, `repl`, `seed`, `backup`, `restore`, `logs` | Stub ⏳           |
| `schema` (fetch/update/list/clear)                                                                                      | Implemented ✅    |
| `promote`, `archive`, `saga`, `module`, `sign`, `script`, `freeze`, `rollback`, `lock`, `workspace`                     | Stub, deferred ⏸️ |

## Implementation priority (per docs/cli-tools/01-formspec-cli.md)

1. `validate` — CI gate, high value
2. `new` — simple scaffold, quick DX win
3. `dev` — meaningful after pipeline fixed
4. `generate` — depends on pkg/spec stability
5. Everything else

## Key design rules

- Single binary: `cmd/formspec` (not separate binaries per verb)
- `//go:embed dist/*` embeds SPA for single-process serving
- `--dev-ui` spawns Vite HMR for frontend development
- Runtime auto-detect from project files (go.mod, package.json, etc.)
- Config file: `formspec-app.yaml`
- **JSON Schema is NOT embedded** — `validate`/`init`/`schema` fetch from the
  registry (default `https://schemas.formspec.dev`), cached at
  `os.UserCacheDir()/formspec/schemas/<version>`; spec version comes from each
  manifest's `apiVersion` (`formspec.dev/v1`); see `internal/schemaregistry/`

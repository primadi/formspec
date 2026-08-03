---
name: schema-validation
description: Validate Forma YAML manifests against the schema contract + engine loader, then repair every error to canonical form. Use when a generated or hand-written spec fails validation, when the user reports YAML/schema errors, after generating manifests with a skill (generate → validate → fix), or before `forma apply`/`forma dev`. Runs `forma validate`, interprets engine vs schema failures, applies the canonical fix, and re-validates until clean.
metadata:
  version: "1.0"
  source: cmd/forma/validate.go + schemas/
---

# Forma Schema Validation & Repair

Validate every Forma manifest against the contract, fix what fails, and
re-validate until zero problems. Ground truth is the engine loader
(`internal/manifest`) plus the JSON Schema contract (`schemas/`), which is
generated from `pkg/spec` via `make generate-schema`.

## Core Loop (never skip steps)

```
1. RUN    forma validate --spec <dir> [--schema <dir>] [--no-schema]
2. READ   classify each FAIL as engine: vs schema:
3. REPAIR apply the canonical fix (Repair Catalog below)
4. RE-RUN until 0 problem(s) found
5. DONE   confirm exit code 0 and report the fixed files
```

Run from the project root (cwd with `spec/`) with the built binary, or from
the forma repo via `go run ./cmd/forma validate --spec <path>`. If the binary
is stale (says "not implemented yet"), rebuild first: `make build`.

The schema source is auto-detected — no flag needed: a `schemas/` folder next
to the spec dir (`<spec>/../schemas`) is preferred, then `./schemas` (cwd),
then the schema embedded in the binary. The output's first line shows which
one was used (`schema: <dir> (local)` or `schema: embedded`). Pass
`--schema <dir>` to force a specific schema dir.

## Reading the Output

Each manifest is reported per layer:

- `engine: ...` — the manifest would break `forma dev`/`forma apply`
  (YAML parse error, or deep Entity validation). **Hard error — fix first.**
- `schema: ...` — violates the JSON Schema contract for that kind
  (App, Module, Form, Workflow, Table, ...) that the loader does not yet
  deep-validate. **Contract error — fix to canonical form.**

Exit code: `0` = clean, `1` = at least one problem.

## Repair Catalog

Most failures map to a small set of canonical fixes. Always check the schema
(`schemas/kinds/<Kind>.schema.json`) or the spec docs before inventing syntax.

| Error / pattern | Canonical fix |
|---|---|
| `expose: all` / `expose: read` / `expose: none` | `expose` is an **array** of `{type, actions}`. Remove it entirely for UI-only, or use `expose: [{type: rest, actions: [list, find, create, update, delete]}]` (see `docs/spec/backend/01-core-basic.md` §8.4) |
| `lifecycle: {doc_status: true}` | `lifecycle` is a **string** enum (`two_step_autosave`/`two_step_manual`/`plain_crud`). The built-in `doc_status` lifecycle is default-on → delete the block, or set a string value |
| `type: relation, target: X` | `target` is silently ignored (dangling relation). Use `type: relation` + `relation: {type: belongs_to, resource: <module.entity>}` |
| `type: child, target: X` (referencing another entity) | `child` is an *embedded* inline collection, not a reference. For a separate entity use `relation: {type: belongs_to, resource: <module.entity>}` |
| Missing `spec.version` on Entity | Every Entity requires `spec.version: v1` |
| `spec.version` missing on App | App requires `version`, `vendor`, `root_url` (plus optional `modules` + `menu`) |
| Module `depends_on: [...]` | Use `depends: [{module: X}]` — `depends_on` is not a valid key |
| Workflow with `states:`/`transitions:`/`guards:`/`condition:` | States/transitions live on the Entity (`state_machine`: `field`, `initial`, `states`, `transitions` with `via`). A `kind: Workflow` only declares `entity` + `on: {transition: {from, to}}` + `steps` + `on_reject` + `escalation` |
| Unknown property "X" / additionalProperties | Remove it or check the kind schema for the correct key name |
| `field.type: money` with `currency:` | `currency` is not a Field property; `money` is a plain field type |

## Known Schema-vs-Engine Gaps

Some Go types accept a scalar shorthand via custom `UnmarshalYAML`, but the
generated schema only expresses the object form. The schema layer is stricter
than the engine here — use the **object form** to pass validation:

- `guard: "expr"` → `guard: {expression: "expr", message: "..."}`
- `render: drawer` → `render: {mode: drawer}`

Do NOT "fix" these by relaxing the schema unless that is a deliberate
`pkg/spec` change — the object form is the canonical contract.

## Verification

- Re-run `forma validate --spec <dir>` → expect `0 problem(s) found`.
- Optionally run `go test ./internal/manifest/ -run TestExamplesLoadAndValidate`
  (from the forma repo) to confirm the engine accepts every example/vertical.
- Report which files changed and why (one line each).

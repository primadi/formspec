# {{projectName}} — AGENTS.md

## Project Overview

This is a **FormSpec** application. FormSpec is a spec-first, declarative ecosystem
for business applications. YAML manifests (apiVersion/kind/metadata/spec) are
the single source of truth for API, UI, permissions, state machines, and events.

## How to Build a FormSpec App — 4 Phases

Follow the **formspec-app-workflow** skill (in `.agents/skills/`) when
creating or changing this app. It orchestrates the full lifecycle:

1. **Discovery** — ask business questions in plain language, output
   `docs/overview.md`, get user approval
2. **Proposal** — map to modules + entities (characteristics, state machines),
   output `docs/architecture.md` + `docs/domain-model.md`, get approval
3. **Draft** — write YAML manifests in `spec/`, validate after every write
4. **Iterate** — classify changes, update docs top-down, write a changelog entry

**Never jump straight to YAML.** Confirm with the user between phases. Which
phase you are in is decided by the user's request + their approvals — not by
which files happen to exist.

## App Shape — decide before you build

Every `kind: App` has two orthogonal axes. **Confirm both with the user**
before writing the App manifest (see the **formspec-kinds** skill → App shape):

| Axis           | Values                                                    | Default       |
| -------------- | --------------------------------------------------------- | ------------- |
| `access`       | `private` (login required) · `public` (anonymous landing) | `private`     |
| `app_renderer` | `sidebar-nav` · `topnav` · `no-nav`                       | `sidebar-nav` |

Ask whether the app is a single **private** app (back-office), a single
**public** app (landing — pair with `kind: Listing`), or **two apps** (public
portal + private admin sharing modules at different `root_url`).

## Skills Loaded

This project includes AI skills in `.agents/skills/`. In Copilot Chat,
use `/skills` to see them:

- **formspec-app-workflow** — Full lifecycle orchestrator (Discovery → Proposal → Draft → Iterate)
- **formspec-kinds** — Complete catalog of all 34 FormSpec resource kinds
- **formspec-spec-structure** — Navigate the FormSpec spec docs
- **schema-validation** — Run `formspec validate`, classify errors, repair manifests

## Reference Docs

Authoritative FormSpec documentation lives at <https://docs.formspec.dev/>:

- **Kind reference** — <https://docs.formspec.dev/kind/> — one page per kind
  (attributes, when to use, examples, gotchas)
- **Contracts** — <https://docs.formspec.dev/spec/> — platform / backend / frontend specs
- **Guides** — <https://docs.formspec.dev/guides/>

## Stack

| Layer     | Technology                                     |
| --------- | ---------------------------------------------- |
| Backend   | Go, module github.com/primadi/formspec         |
| Frontend  | React + TypeScript + Vite + shadcn/ui          |
| Database  | PostgreSQL (production) / SQLite (dev)         |
| Scripting | Starlark (sandboxed, editable via admin panel) |
| Manifest  | YAML (apiVersion/kind/metadata/spec)           |

## Key Commands

| Command                         | Purpose                                                |
| ------------------------------- | ------------------------------------------------------ |
| `formspec validate --spec spec` | **Validation gate** — must report `0 problem(s) found` |
| `formspec dev`                  | Start development server (API + UI)                    |
| `formspec generate`             | Generate typed TypeScript client from Entity manifests |

**Rule: validate after every significant write.** Run
`formspec validate --spec spec` whenever you add or change a manifest, and
fix errors (use the schema-validation skill) before moving on.

## Conventions

1. **Manifest first** — always write YAML spec before implementation
2. **Module granularity** — one Module = one business bounded context
3. **Entity characteristics** — master (stable data), transaction (append-heavy),
   reference (read-only seed), summary (system-managed projection)
4. **Permissions** — permission = resource + action, never hardcode role names
5. **Use ctx.\* primitives** — ctx.db, ctx.cache, ctx.lock, ctx.queue,
   ctx.pubsub, ctx.storage — never raw SQL
6. **Derived by default** — Entity auto-generates CRUD API + Table + Forms + Page
   (95% of the time, `kind: Entity` is enough — no UI overrides needed)

## Project Layout

```
{{projectName}}/
  AGENTS.md              # Agent instructions (tool-agnostic)
  formspec-app.yaml      # CLI config (NOT a kind: Config manifest)
  spec/                  # All YAML manifests
    apps/                # kind: App manifests
    modules/             # kind: Module -> Entity, Page, Form, etc.
  .agents/skills/        # AI skills for coding agents
```

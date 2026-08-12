# crc-management — Copilot Instructions

## Project Overview

This is a **FormSpec** application. FormSpec is a spec-first, declarative ecosystem
for business applications. YAML manifests (apiVersion/kind/metadata/spec) are
the single source of truth for API, UI, permissions, state machines, and events.

## Skills Loaded

This project includes AI skills in `.agents/skills/`:

- **formspec-spec-structure** — Navigate the FormSpec spec docs
- **formspec-kinds** — Complete catalog of all FormSpec resource kinds

Use `/skills` in Copilot Chat to verify they are discovered. These skills give
the agent domain-specific knowledge about FormSpec kinds, manifest formats, and
spec documentation.

## Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, module github.com/primadi/formspec |
| Frontend | React + TypeScript + Vite + shadcn/ui |
| Database | PostgreSQL (production) / SQLite (dev) |
| Scripting | Starlark (sandboxed, editable via admin panel) |
| Manifest | YAML (apiVersion/kind/metadata/spec) |

## Key Commands

| Command | Purpose |
|---------|---------|
| `formspec dev` | Start development server (API + UI) |
| `formspec apply` | Register YAML manifests |
| `formspec generate` | Generate typed TypeScript client from Entity manifests |

## Conventions

1. **Manifest first** — always write YAML spec before implementation
2. **Module granularity** — one Module = one business bounded context
3. **Entity characteristics** — master (stable data), transaction (append-heavy),
   reference (read-only seed), summary (system-managed projection)
4. **Permissions** — permission = resource + action, never hardcode role names
5. **Use ctx.* primitives** — ctx.db, ctx.cache, ctx.lock, ctx.queue,
   ctx.pubsub, ctx.storage — never raw SQL
6. **Derived by default** — Entity auto-generates CRUD API + Table + Forms + Page

## Project Layout

```
crc-management/
  formspec-app.yaml       # CLI config (NOT a kind: Config manifest)
  spec/                # All YAML manifests
    apps/              # kind: App manifests
    modules/           # kind: Module -> Entity, Page, Form, etc.
  app/                 # Optional sidecar (only with --with-sidecar)
  .agents/skills/      # AI skills for Copilot
```

## Creating a FormSpec App

When asked to create a FormSpec app:

1. **Identify the business domain** — what entities are needed?
2. **Choose Entity characteristics** — master, transaction, reference, or summary
3. **Write YAML manifests** in spec/modules/<module>/
4. **Organize by characteristic folders** — master/, transaction/, reference/, summary/
5. **Start with Entity** — 95% of cases, the answer is kind: Entity
6. **Let derivation handle the rest** — Forms, Tables, Pages are auto-generated

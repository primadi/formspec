---
name: formspec-spec-structure
description: Navigates the FormSpec specification documentation structure. Use when the user asks where to find spec docs, how the spec folder is organized, which spec file covers a topic (entity, kinds, frontend, backend, platform), how to read the FormSpec docs, or what the FormSpec project layout should be. Covers the 3 sub-folders (platform/, backend/, frontend/), doc status lifecycle (Outline→Draft→Final), and the contract-vs-renderer principle.
metadata:
  version: "1.0"
  source: docs/spec/
---

# FormSpec Spec Structure

Context for AI coding agents working on FormSpec apps — where to find the
authoritative specification for every concept.

## Core Principle

> **Spec adalah kontrak; renderer adalah implementasi.**

One spec, many renderers. The spec is written once and is
implementation-agnostic. Implementations (frontend shell, persist backend)
can be swapped or added without changing the spec — as long as the seam was
designed from the first implementation.

Consequence: when you need to know how something works, consult the **spec**,
not a specific renderer's code.

## Three Sub-Folders

All spec documents live under `docs/spec/` and are divided into three
sub-folders, each with its own `README.md` as the entry point:

| Folder | Contract for | Start here |
|---------|-------------|------------|
| `platform/` | Both sides: workspace/app/module model, kind system, control plane, plane protocol, datastore, marketplace, project layout | `platform/README.md` |
| `backend/` | Engine & any PersistBackend: entity model, actions, lifecycle, extension, storage interface, field types, script runtime | `backend/README.md` |
| `frontend/` | Any Shell & visual renderer: 4-tier hierarchy, VisualSpecKind, Renderer, Spec Resolution API, kind catalog, FormSpecExpr | `frontend/README.md` |

## Platform Docs (Cross-Cutting)

| File | Covers |
|------|--------|
| `01-overview.md` | What FormSpec is, contract-vs-renderer, personas, scope |
| `02-workspace-app-module.md` | Ownership model: workspace → app → module; App as module curation; menu; reference qualifiers |
| `03-kind-system.md` | Kind taxonomy, meta-kinds, kind→plane mapping |
| `04-control-plane.md` | Control plane contract |
| `05-plane-protocol.md` | Inter-plane protocol |
| `06-datastore.md` | Datastore kind and connection lifecycle |
| `07-marketplace.md` | Module & renderer distribution, trust tiers |
| `08-project-layout.md` | **Project folder structure** — THE reference for scaffolding. Covers `spec/`, `app/`, `formspec-app.yaml`, module organization by entity characteristic |
| `09-observability.md` | Logging, metrics, tracing, health vocabulary |
| `10-deployment-operations.md` | Deployment pipeline, rollback, canary, promotion, DR/HA |

## Backend Docs (Data & Behavior)

| File | Covers |
|------|--------|
| `01-core-basic.md` | Entity (characteristic, lifecycle doc_status, fields, child vs relation), Service, Config, Migration, Subscription, expose, permission model, REST API |
| `02-core-extended.md` | Workflow (multi-approver), Api, Webhook, Mockup, Integrator, async actions, validation levels 4-6, hooks, query builder, rate limiting, audit trail |
| `03-entity-extension.md` | Entity extension by other modules; clean uninstall contract |
| `04-persist-backend.md` | PersistBackend interface — storage seam equivalent to Shell |
| `05-field-types.md` | Normative field type catalog, `money` type, validation vocabulary, tree/hierarchy support |
| `06-script-runtime.md` | Script handler API — `execute` entrypoint, `resource` object, cross-entity access, `ok`/`fail` return contract, native `ref` resolution |
| `error-glossary.yaml` | Canonical `FORMSPEC.*` error codes |

## Frontend Docs (Visual)

| File | Covers |
|------|--------|
| `01-visual-hierarchy.md` | Four-tier hierarchy: Shell → App renderer → Page renderer → Component renderer; `stack_family` rules; new shell policy |
| `02-visual-spec-kind.md` | Meta-kind VisualSpecKind: tier, schema, renderer_contract, slot system |
| `03-renderer-kind.md` | Kind Renderer: implements, stack_family, trust_tier, registry, conformance |
| `04-spec-resolution-api.md` | Runtime seam shell ↔ engine; must be backend-agnostic |
| `05-app-kinds.md` | App-tier catalog: sidebar-nav, topnav, landing-page |
| `06-page-kinds.md` | Page-tier catalog: data-entry, table, master-detail, kanban, calendar, wizard, dashboard, report, timeline, approval-inbox, notification-center, custom page |
| `07-component-kinds.md` | Component-tier catalog: input (incl. fileinput, richtext), widget, slot filling, asset |
| `08-formspec-expr.md` | Client-side expression grammar (visible_when, readonly_when, required_when, compute) — applies to all shells |

## Doc Status Lifecycle

Each spec document has a status in its header:

```
Outline → Draft → Final
```

| Status | Meaning | Agent behavior |
|--------|---------|---------------|
| `Outline` | Scope skeleton only; content is aspirational | **DO NOT treat as source of truth.** Code behavior follows `docs_old/spec/` until successor reaches ≥ Draft |
| `Draft` | Complete content, open to revision | Can be used as reference; note that details may still change |
| `Final` | Binding; changes require version bump | Authoritative source of truth |

## How to Read the Spec

1. **Start at the README** of the relevant sub-folder — it has a table of contents
2. **Follow cross-references** — documents link to each other with relative paths
3. **Check the status** — Draft vs Final tells you how binding the content is
4. **Use the kinds catalog** (`formspec-kinds` skill) to find which kind maps to which spec file
5. **For project layout**: always consult `platform/08-project-layout.md` — it defines the standard folder structure

## Gotchas

- **`docs/spec/` is authoritative; `docs_old/` and `reff_docs/` are historical archives (read-only).** Never cite `docs_old/` or `reff_docs/` as a current contract.
- **`formspec-app.yaml` is dev/serve config for the CLI — NOT a `kind: Config` manifest.** It points the engine to spec path, DSN, and runtime settings.
- **The spec is storage-agnostic.** SQL examples live in renderer documentation (`docs/renderers/`), not in the spec.
- **Entity characteristics are mutually exclusive.** An Entity can be `master`, `transaction`, `reference`, or `summary` — not a combination.

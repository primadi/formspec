---
name: forma-backend
description: "Use when: working on Forma Go backend code — Entity, Service, PersistBackend, Starlark, API, auth, manifest loading, or any package under internal/ renderers/jsonbpersist/ or pkg/spec/. Provides project structure, kind system, implementation types, and key design rules."
---

# Forma Backend Skill

Context for AI coding agents working on the Forma backend (Go, Starlark, YAML).

## Key paths
- `pkg/spec/` — Go types that ARE the contract (entity.go, frontend.go, resources.go, datastore.go)
- `renderers/jsonbpersist/` — PersistBackend renderer (db, datastore, crud, ddl, migrate, outbox, extension)
- `internal/api/` — HTTP router, handlers, meta API, middleware, WebSocket hub
- `internal/action/` — Action dispatcher (native, script, sidecar, hooks, events)
- `internal/starlark/` — Sandboxed Starlark runtime (executor, context, resource, primitives)
- `internal/entity/` — Entity registry, state machine engine
- `internal/auth/` — Authentication (JWT, permission checker, dev identity)
- `internal/events/` — Event hub, outbox worker
- `internal/permission/` — Permission registry, validator, UsesEnforcement
- `internal/manifest/` — YAML loader, parser, validator (30 kinds in KnownKinds)
- `internal/app/` — App resolution (ResolvedApp, menu tree)
- `internal/ui/` — UI registry for frontend kinds, meta response builders
- `internal/validation/` — Cross-field validation (after/before/exists)
- `cmd/forma/` — CLI binary (apply, dev, generate, dev_vite, dev_runtime)

## Kind system
- `kind: Entity` — stateful resource (was "Document") with characteristic: master|transaction|reference|summary
- `kind: Service` — stateless computation
- `kind: Config` — module configuration (read via ctx.config)
- `kind: Subscription` — cross-module event reaction
- `kind: Migration` — DDL-only structural changes
- `kind: Workflow` — approval-based state machine transitions
- `kind: Api` — external API surface override
- `kind: Webhook` — verified inbound endpoints
- `kind: Integrator` — cross-module bridge

## Implementation types
- `impl.native` — Go (ref: "{Type}.{Method}")
- `impl.script` — inline Starlark
- `impl.script_ref` — named script (ref: "module/script-name")
- `impl.compiled` — WASM (deferred)
- `impl.sidecar` — external process (deferred)

## Key design rules
- All mutations MUST be in a transaction (mutation + outbox + counter atomic)
- PK MUST be UUID v7
- Filter operators: eq, neq, gt, gte, lt, lte, between, in, nin, like, ilike, null, notnull
- `spec.expose` = deny-by-default; `/_ui/entity/` always available
- Permission = resource + action, never hardcoded role names
- Use `ctx.*` for all infrastructure (ctx.db, ctx.cache, ctx.lock, ctx.queue, ctx.pubsub, ctx.storage)
- Error codes: FORMA.{DOMAIN}.{REASON}

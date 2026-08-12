---
title: FormSpec vs PocketBase
description: Comparing FormSpec's spec-first ecosystem with PocketBase — the minimalist Go backend that inspired "One Definition, Many Protocols"
date: 2026-07-06
---

# FormSpec vs PocketBase

> **FormSpec** is a spec-first ecosystem for building business applications in Go. **PocketBase** is a minimalist Go backend with embedded SQLite, auto-generated CRUD API, admin UI, and realtime subscriptions — all from a single binary.

PocketBase is one of **FormSpec's six explicit inspirations**. FormSpec directly adopted PocketBase's principle of *"One Definition, Many Protocols"* — from a single resource definition, you automatically get HTTP endpoints, admin panel UI, API docs, and generated types. Understanding this comparison reveals the DNA they share and where FormSpec extends beyond PocketBase's scope.

---

## 1. Overview

### FormSpec
A complete ecosystem for business applications. YAML manifests define entities, state machines, actions, permissions, and UI events. Two-process architecture (Control + Resource Planes). Five implementation types (Go native, Starlark, compiled, sidecar). PostgreSQL + Valkey + MinIO. Module marketplace with pricing and governance.

### PocketBase
A minimalist Go backend. One binary with embedded SQLite. Auto-generates REST API, admin dashboard, and realtime subscriptions from database schema. Auth (email/oauth) and file storage are built-in. Extensible via Go plugins (hooks) or JavaScript (via Goja runtime). Designed for small to medium projects.

---

## 2. Philosophy

| | FormSpec | PocketBase |
|---|---|---|
| **Paradigm** | Spec-first, declarative (YAML files define everything before code) | Schema-first (define collections, then optionally extend with code) |
| **Source of truth** | YAML manifests on disk (git-friendly) | Admin UI or JSON import/export (collections defined in UI or imported) |
| **Core principle** | "One Definition, Many Protocols" — YAML → API + UI + docs + types | Same! One collection → API + admin UI + realtime + types |
| **Target app size** | Large business apps (ERP, POS, inventory, billing) | Small to medium apps (API backend, internal tools, MVPs) |
| **Database** | PostgreSQL (production), SQLite (dev) | SQLite (embedded, always) |

### What FormSpec took from PocketBase

From the Foundation Document: *"The principle of 'One Definition, Many Protocols' — from a single resource definition, you automatically get HTTP endpoints, WebSocket handlers, admin panel UI, API docs, and generated types. Auth required by default; anonymous access must be explicitly declared. DX benchmark: `formspec dev` — one command, everything runs."*

| PocketBase Feature | FormSpec Equivalent | Notes |
|---|---|---|
| Auto-generated CRUD API | ✅ Auto-generated REST API | Both derive endpoints from definitions |
| Admin dashboard UI | ✅ Auto-generated admin panel | Derived from Entity manifests |
| Realtime subscriptions | ✅ Realtime via WebSocket | Same convention (`entity:{module}.{name}`) |
| Auth required by default | ✅ Security by default | Auth mandatory, anonymous explicit |
| File storage | ✅ `ctx.storage` (MinIO/S3) | PocketBase uses local FS + S3 |
| Go plugin hooks | ✅ `native` impl type + events | Both extensible in Go |
| JS scripting (Goja) | ✅ Starlark scripting (`script_ref`) | Both offer sandboxed scripting |

---

## 3. Feature Comparison

| Dimension | FormSpec | PocketBase |
|---|---|---|
| **Paradigm** | Spec-first (YAML on disk) | Schema-first (admin UI or JSON) |
| **Backend language** | Go (native) + Starlark (script) + sidecar (any) | Go (core) + JS (Goja runtime for hooks) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA — 12 UI kinds) | Admin UI (limited customization) + Bring-Your-Own-Frontend |
| **State Machine** | ✅ Built-in — states, transitions, guards in YAML | ❌ Not available — implement in application code |
| **Idempotency** | ✅ Enforced by framework | ❌ Not available — implement manually |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once delivery | ❌ Not available — PocketBase has realtime, not reliable event delivery |
| **Multi-tenancy** | ✅ Workspace model — automatic, tenancy-blind apps | ❌ Not built-in — DIY with collections or separate instances |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` per action | ⚠️ Collection-level rules (simple, but limited for complex RBAC) |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing for all releases | ❌ Not applicable |
| **Audit Trail** | ✅ Write-once immutable audit log | ❌ Not available — implement manually |
| **Database** | PostgreSQL (prod) + SQLite (dev). `ctx.db` — raw SQL, no ORM. | SQLite (always, embedded). Max ~10-50 GB practical limit. |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — versioned, rollback, admin-panel-editable | ✅ JS (Goja) — hooks on requests, not full sandboxed scripting |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ Go + JS only |
| **Built-in Admin Panel** | ✅ Derived from Entity manifests (full customization via UI kinds) | ✅ Basic admin UI (auto-generated from collections) |
| **File Storage** | ✅ `ctx.storage` (MinIO/S3) | ✅ Built-in (local FS + S3) |
| **Auth / Auth providers** | Built-in with JWT (extensible) | ✅ Email/password, OAuth2 (GitHub, Google, etc.), JWT |
| **Realtime** | ✅ WebSocket push via `ctx.pubsub` | ✅ Server-Sent Events (SSE) — lightweight, built-in |
| **Database Migration** | ✅ Automatic from YAML (idempotent, checksummed) | ✅ Auto-migration from schema changes (limited to SQLite) |
| **Ecosystem / Marketplace** | Module registry with pricing models | ❌ No marketplace (SDK + examples only) |
| **Deployment Model** | Self-host (single binary, Docker, K8s) + FormSpec Cloud | Single binary (embedded SQLite) — dead simple deploy |
| **Production Scale** | Enterprise (PostgreSQL + Valkey + MinIO) | Small-to-medium (SQLite limits) |
| **Binary size** | ~15-30 MB | ~30 MB (embedded SQLite + admin UI) |
| **Learning Curve** | Medium — YAML + Go + Starlark + 2-plane architecture | Low — one binary, one command, collections defined in UI |
| **Open Source** | FSL (source available → Apache 2.0 after 2 years). Spec is CC0. | ✅ MIT (fully open source) |

---

## 4. The Core Difference: Scope

PocketBase is intentionally **minimalist**. The creator's philosophy is that most features beyond the core should live in application code, not the framework. This makes PocketBase:
- **Easy to learn** — one binary, one command, simple concepts
- **Easy to deploy** — SQLite means no database server
- **Limited for complex apps** — no state machine, no multi-tenancy, no outbox, no governance

FormSpec is intentionally **comprehensive**. The philosophy is that enterprise patterns should be built into the framework because:
- Most developers don't implement them correctly (idempotency, outbox, locking)
- Governance is impossible to retrofit
- Multi-tenancy is hard to add later

### When PocketBase beats FormSpec

| Scenario | Why PocketBase wins |
|---|---|
| **Simple API backend** | PocketBase: one binary, one command, done. FormSpec: overkill. |
| **MVP / prototype** | Collections in admin UI → API in 5 minutes. FormSpec needs YAML files + setup. |
| **Small internal tool** | SQLite is sufficient, no infra management. FormSpec's PostgreSQL requirement is unnecessary. |
| **Learning Go** | PocketBase is simpler to understand and extend. |
| **Single-tenant app** | Multi-tenancy not needed — PocketBase is simpler. |

### When FormSpec beats PocketBase

| Scenario | Why FormSpec wins |
|---|---|
| **Multi-tenant SaaS** | FormSpec's workspace model is built-in. PocketBase needs DIY or separate instances. |
| **Business app with state machine** | FormSpec has built-in state machine. PocketBase needs custom code. |
| **High-reliability system** | FormSpec has idempotency, outbox, audit trail. PocketBase has none. |
| **Large dataset (>50 GB)** | FormSpec uses PostgreSQL. PocketBase is limited by SQLite. |
| **Compliance / governance** | FormSpec has Control Plane with policy, signing, audit. PocketBase has nothing. |
| **Polyglot team** | FormSpec supports sidecar containers. PocketBase is Go + JS. |
| **Multi-region deployment** | FormSpec supports PostgreSQL streaming. PocketBase is single-node SQLite. |

---

## 5. Conclusion

PocketBase and FormSpec share the same **core insight**: one definition should generate many surfaces automatically. But they target different scales:

```
PocketBase:      Small/Medium ──► SQLite, one binary, simple auth, realtime
                     │
                     │ (grow beyond SQLite limits)
                     ▼
FormSpec:          Medium/Large ──► PostgreSQL, two planes, governance, marketplace
```

**PocketBase is ideal for:** APIs, small internal tools, MVPs, prototypes, learning Go, single-tenant apps.

**FormSpec is ideal for:** Multi-tenant business applications (ERP, POS, inventory, billing, healthcare) that need state machines, reliable events, audit trails, and governance.

> If PocketBase is the **Go backend equivalent of SQLite** (simple, embedded, great for small projects), FormSpec is the **Go backend equivalent of PostgreSQL** (heavier, more features, built for scale). Both are valid — use the right tool for the job.

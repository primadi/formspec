---
title: Forma vs Supabase
description: Comparing Forma's spec-first application framework with Supabase's Backend-as-a-Service approach
date: 2026-07-06
---

# Forma vs Supabase

> **Forma** is a spec-first ecosystem for building business applications in Go. **Supabase** is an open-source Backend-as-a-Service (BaaS) that provides a managed PostgreSQL database, authentication, realtime subscriptions, storage, and edge functions — all accessible via client SDKs.

These two projects approach the same problem (building applications faster) from opposite directions: Forma is a **framework you run** (self-hosted or cloud); Supabase is a **service you connect to** (managed infrastructure with client libraries).

---

## 1. Overview

### Forma
A spec-first, declarative framework where YAML manifests define entities, state machines, permissions, and UI. Runs as a Go binary (self-hosted or cloud). Business logic via Go native, Starlark scripting, or sidecar. Two-process architecture with governance Control Plane. PostgreSQL + Valkey + MinIO.

### Supabase
An open-source Firebase alternative. Provides managed PostgreSQL (with Row-Level Security), Auth (built-in providers + JWT), Realtime (broadcast/presence/postgres changes), Storage (S3-compatible), and Edge Functions (Deno/TypeScript). Accessed via client SDKs that call the API directly. Self-hostable, but primarily used as a cloud service.

---

## 2. Philosophy

| | Forma | Supabase |
|---|---|---|
| **Approach** | Framework — you run it, it runs your logic | BaaS — you connect to it, your client talks to it |
| **Where logic lives** | Server-side (Go, Starlark, sidecar) | Client-side (JS/TS SDK) or Edge Functions (Deno) |
| **API generation** | From YAML manifests — structured, declarative | From database schema (Postgres introspection) |
| **Authorization** | Server-enforced `required_permission` | PostgreSQL Row-Level Security (RLS policies) |
| **Source of truth** | YAML manifests (durable, git-versioned) | Database schema + client code |
| **Self-hosted** | ✅ Yes — single binary or Docker | ✅ Yes (Docker Compose, self-hosted Supabase) |
| **Target user** | Go developers building business applications | Full-stack JS developers, startups, mobile apps |

---

## 3. Architecture

```
Forma:                              Supabase:
┌──────────────────────┐           ┌──────────────────────┐
│  Your YAML Manifests │           │  Your Client App     │
│  (kind: Entity, ...) │           │  (React, Flutter,    │
└──────────┬───────────┘           │   Swift, etc.)       │
           │ loads                 └──────────┬───────────┘
┌──────────┴───────────┐                      │
│  forma-resource       │              ┌───────┴───────┐
│  (Go binary)          │              │  Supabase      │
│                       │              │  Cloud         │
│  Entity Engine        │              │               │
│  CRUD API             │              │  Postgres      │
│  State Machine        │              │  + RLS         │
│  Events · Actions     │              │  + Auth        │
│  Admin Panel          │              │  + Realtime    │
│                       │              │  + Storage     │
│  PostgreSQL           │              │  + Edge Fns    │
│  Valkey + MinIO       │              └───────────────┘
└──────────────────────┘
```

**Key differences:**
- Forma runs your business logic server-side. Supabase expects business logic in **client code** or **edge functions** (with their limitations).
- Forma enforces permissions server-side via `required_permission`. Supabase uses **PostgreSQL Row-Level Security** — powerful but requires writing SQL policies.
- Forma generates API from YAML. Supabase generates API from **database introspection** (schema-based).

---

## 4. Feature Comparison

| Dimension | Forma | Supabase |
|---|---|---|
| **Paradigm** | Spec-first application framework | Backend-as-a-Service |
| **Backend language** | Go (native) + Starlark (script) + sidecar (any) | Client-side (JS/TS, Dart, Swift, Kotlin) + Edge Functions (Deno/TypeScript) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | Any — client talks to Supabase SDK directly |
| **State Machine** | ✅ Built-in — define in YAML | ❌ Not available — implement client-side or in Edge Functions |
| **Idempotency** | ✅ Enforced by framework | ❌ Not available — implement manually in Edge Functions or client |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once delivery | ❌ Not available — use Realtime (no delivery guarantee) + manual outbox |
| **Multi-tenancy** | ✅ Workspace model — automatic, tenancy-blind apps | ❌ DIY — RLS policies with `tenant_id` column + separate schemas |
| **Permission Model** | ✅ Declarative `required_permission` + `uses`, server-enforced | ⚠️ Row-Level Security (RLS) — powerful SQL-based, but must be written and maintained per-table |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing | ❌ Not available |
| **Audit Trail** | ✅ Write-once immutable audit log | ❌ Not available — use PostgreSQL audit extensions manually |
| **Database** | PostgreSQL (managed or self-hosted) via `ctx.db` — raw SQL | PostgreSQL (managed) — direct access via SQL or client SDK |
| **Realtime** | WebSocket via `ctx.pubsub` — permission-filtered | ✅ Built-in Realtime (broadcast, presence, Postgres changes) |
| **File Storage** | ✅ `ctx.storage` (MinIO/S3) | ✅ Supabase Storage (S3-compatible) |
| **Authentication** | Built-in JWT (extensible providers) | ✅ Built-in (email/password, OAuth2, phone, anonymous, multi-factor) |
| **Edge Functions** | ❌ Not needed (server-side runtime handles all logic) | ✅ Edge Functions (Deno/TypeScript, global edge deployment) |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — runtime-editable | ❌ Not available — edge functions require redeploy |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ Edge Functions run TypeScript only |
| **Built-in Admin Panel** | ✅ Auto-generated from Entity manifests | ✅ Supabase Studio (table browser, SQL editor, schema designer) |
| **Auto-generated API** | ✅ REST API from YAML manifests | ✅ REST + GraphQL API from database schema (via PostgREST) |
| **Local Development** | `forma dev` (one command, Docker Compose) | ✅ Supabase CLI (`supabase start` — local Docker stack) |
| **Hosting** | Self-host + Forma Cloud | Supabase Cloud (managed) + self-host option |
| **Pricing** | FSL (free self-hosted) + Forma Cloud (paid tiers) | Free tier (generous) + paid plans (usage-based) |
| **Open Source** | FSL (source available → Apache 2.0 after 2 years). Spec is CC0. | ✅ Apache 2.0 (fully open source) |
| **Learning Curve** | Medium — YAML + Go + Starlark + 2-plane architecture | Low-Medium — if you know JS/SQL, Supabase is easy to start |

---

## 5. The Fundamental Difference: Where Logic Lives

### Supabase: Client-Side / Edge Logic

```
Client App ──► Supabase SDK ──► Supabase API
                │                    │
                │ User Auth          │ RLS (database-level)
                │ Realtime           │
                │ File Upload        │
                └────────────────────┘
                Logic lives here:    Logic lives here:
                Client-side (JS)     SQL policies (RLS)
                Edge Functions       Database triggers
```

- Business logic often ends up in **client code** (duplicated, insecure) or **Edge Functions** (limited runtime, cold starts).
- RLS policies are powerful but become **complex to maintain** as permission rules grow.
- No built-in patterns for **state machines, idempotency, outbox, or locking** — you must build these yourself.

### Forma: Server-Side Logic

```
Client App ──► forma-resource (Go binary)
                │
                │ Entity Engine (state machine, idempotency, outbox)
                │ Action execution (Go native / Starlark / sidecar)
                │ Permission enforcement (`required_permission`)
                │
                ▼
              PostgreSQL
```

- All business logic runs **server-side** — consistent, auditable, secure.
- Patterns like idempotency, outbox, and state machine are **enforced by the framework**.
- No client-side logic duplication.

---

## 6. When to Choose Which

### Choose Forma when:
- You are building a **business application** with complex server-side logic (state machines, approval workflows, multi-step transactions).
- You need **enterprise patterns by default** — idempotency, outbox, locking, audit trail.
- You prefer **server-side logic** over client-side or edge functions.
- **Governance matters** — you need deployment policies, artifact signing, and immutable audit.
- You want **Go** for backend logic.
- You want **self-hosted, air-gapped** deployment.
- Your permission model is complex (role hierarchies, delegated admin, cross-app grants).

### Choose Supabase when:
- You are building a **real-time application**, mobile app backend, or startup MVP.
- You prefer **client-side logic** with a managed backend.
- You want **RLS** (Row-Level Security) — database-level permissions are sufficient for your needs.
- You are deeply invested in the **JavaScript/TypeScript ecosystem**.
- You need **generous free tier** for prototyping.
- You want **proven realtime** (broadcast, presence, database changes).
- Your team is comfortable with **SQL** for permission policies.

---

## 7. Conclusion

Forma and Supabase approach the same goal from different directions:

| Supabase's approach | Forma's approach |
|---|---|
| Managed BaaS (you connect to it) | Self-hosted framework (you run it) |
| Client-side logic + Edge Functions | Server-side logic (Go, Starlark, sidecar) |
| RLS policies in SQL | Declarative `required_permission` in YAML |
| Schema-first (introspect DB) | Spec-first (YAML generates schema) |
| Startups, mobile, real-time apps | Enterprise business applications |
| No enterprise patterns built-in | Enterprise patterns by default |

**They are not mutually exclusive.** A forward-looking architecture could use **Forma for the backend business logic** (entity engine, state machine, events) and **Supabase for the client-facing realtime layer** (realtime subscriptions, storage, auth providers) — though this duplicates some functionality.

> If Supabase is the "managed backend for frontend developers," Forma is the "self-hosted framework for backend engineers building complex business systems." Both are valuable — the choice depends on where your logic lives and how much infrastructure you want to own.

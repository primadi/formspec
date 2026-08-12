---
title: FormSpec vs Custom Application (From Scratch)
description: Comparing FormSpec's spec-first approach against building business applications entirely from scratch — the true competitor
date: 2026-07-06
---

# FormSpec vs Custom Application (From Scratch)

> **FormSpec** is a spec-first ecosystem for building business applications. **"Custom app"** means building the same application using general-purpose tools (Go, PostgreSQL, React, Redis) without a business application framework — the approach most teams use today.

This is arguably the **most important comparison** in this directory. While FormSpec can be compared to Vercel, Spring Boot, Laravel, Frappe, PocketBase, Supabase, or Budibase — **none of those are FormSpec's real competition.** The real competitor is the default choice: building from scratch, because that's what most developers do.

---

## 1. The Problem FormSpec Solves

Building a business application from scratch requires solving the same set of problems **every single time**. Most teams discover these problems one by one — usually after production incidents.

### The hidden checklist

| Problem | How teams discover it | Consequence when missed |
|---|---|---|
| **Idempotency** | Payment webhook sent twice → double invoice | Financial loss |
| **Race condition** | Two cashiers deduct stock simultaneously | Inventory inconsistency |
| **Sequential numbering** | Concurrent requests get the same invoice number | Legal/compliance issue |
| **Outbox pattern** | Event lost on crash | Missing downstream updates |
| **Multi-tenancy** | Data leak between customers | Compliance violation |
| **Tenant isolation** | Cross-tenant access returns 403 (leaks existence) | Security vulnerability |
| **Audit trail** | "Who changed what and when?" needs retrofitting | Compliance failure |
| **Migration safety** | Deploy fails because migration conflicts | Production downtime |
| **Permission enforcement** | Unauthorized action discovered post-deploy | Security incident |
| **Natural key format** | Invoice numbers with gaps after rollback | Audit finding |

**FormSpec solves all of these by design.** Building from scratch means solving each one manually — or discovering them later.

---

## 2. What You Must Build Yourself

Here is the minimum checklist for a production-ready business application without FormSpec:

### Database Layer
- [ ] Connection pooling
- [ ] Migration runner (idempotent, checksummed)
- [ ] DDL generation from your models
- [ ] Transaction management
- [ ] Soft delete or hard delete
- [ ] Optimistic concurrency (CAS)

### API Layer
- [ ] HTTP router with middleware chaining
- [ ] Request validation
- [ ] Response envelope format
- [ ] Error handling (structured errors)
- [ ] Request ID tracking
- [ ] CORS
- [ ] Rate limiting (optional but recommended)

### Security
- [ ] Authentication (JWT or session)
- [ ] Password hashing + rotation
- [ ] Authorization middleware
- [ ] Role-based access control
- [ ] Permission checking per action
- [ ] Cross-tenant isolation enforcement

### Business Logic Patterns
- [ ] Idempotency store (persistent, with expiry)
- [ ] Distributed locking (`ctx.lock`)
- [ ] Natural key counter (yearly/monthly/daily)
- [ ] State machine engine (transitions + guards)
- [ ] Outbox table + worker (at-least-once delivery)
- [ ] Event system (publish/subscribe)
- [ ] Audit trail (write-once, immutable)

### Frontend
- [ ] CRUD pages for each entity
- [ ] Form validation
- [ ] Table with pagination, sorting, filtering
- [ ] Admin panel
- [ ] Menu/navigation
- [ ] Permission-aware UI (hide/show elements)

### Operations
- [ ] Structured logging with context (tenant, user, request ID)
- [ ] Health check endpoints
- [ ] Metrics / observability
- [ ] Backup/restore strategy
- [ ] Graceful shutdown
- [ ] Configuration management

### Estimated effort: **3–6 months** for a single developer to build a production-quality foundation, **before writing any business logic.**

---

## 3. Feature Comparison

| Dimension | FormSpec | Custom App (From Scratch) |
|---|---|---|
| **Paradigm** | Spec-first — write YAML, get API + UI + docs | Build everything manually |
| **Backend language** | Go (native) + Starlark + sidecar | You choose (Go, Python, Node, Java, etc.) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | You build it (React, Vue, Angular, or none) |
| **State Machine** | ✅ Built-in | ❌ DIY — research, design, implement, test |
| **Idempotency** | ✅ Enforced by framework | ❌ DIY — idempotency store + middleware + cleanup |
| **Outbox / Reliable Events** | ✅ Built-in (outbox table + worker) | ❌ DIY — outbox table, worker, retry logic, idempotency |
| **Multi-tenancy** | ✅ Workspace model — automatic, tenancy-blind apps | ❌ DIY — tenant resolution, isolation strategy, query filtering |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` | ❌ DIY — RBAC tables, middleware, enforcement |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego | ❌ DIY — policy engine integration, deployment gates |
| **Artifact Signing** | ✅ Ed25519 signing | ❌ Probably won't build this |
| **Audit Trail** | ✅ Write-once immutable audit log | ❌ DIY — audit tables, write-once enforcement, query |
| **Database Abstraction** | `ctx.db` — raw SQL, module-scoped, tenant-isolated | ✅ You choose — ORM, query builder, or raw SQL |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — runtime-editable | ❌ DIY — integrate Lua, JS, or WASM runtime |
| **Polyglot Logic** | ✅ Sidecar container | ✅ You choose — build microservices, not sidecar |
| **Built-in Admin Panel** | ✅ Auto-generated from Entity manifests | ❌ DIY — build a separate admin UI |
| **Entity → DDL generation** | ✅ Automatic | ❌ DIY — ORM auto-migrate or manual migration files |
| **Natural Key Counter** | ✅ Built-in (yearly/monthly/daily/never) | ❌ DIY — sequence tables with proper locking |
| **Distributed Lock** | ✅ `ctx.lock` | ❌ DIY — Redis Redlock, PostgreSQL advisory locks, etc. |
| **File Storage** | ✅ `ctx.storage` (MinIO/S3) | ✅ You choose — S3 SDK, local FS, etc. |
| **Module Ecosystem** | ✅ Marketplace with pricing | ❌ Not applicable — libraries via package manager |
| **Time to MVP** | **Days to weeks** (YAML → API + admin panel) | **Months** (infrastructure + foundation before business logic) |
| **Time to production** | **Weeks to months** (add business logic + custom UI) | **6–18 months** (foundation + business logic + testing) |
| **Maintenance burden** | Low — framework handles patterns | High — you own every line of infrastructure code |
| **Flexibility** | Medium — constrained by framework conventions | Maximum — no constraints, full control |
| **Learning Curve** | Medium — YAML + Go + Starlark + 2-plane architecture | Steep — everything from database to deployment |

---

## 4. Cost Analysis

### Cost of Building FormSpec's Features Yourself

| Feature | Estimated Build Time | Maintenance Cost/Year | Risk if Wrong |
|---|---|---|---|
| Idempotency store | 2-4 weeks | Low-Medium | Financial loss |
| Outbox + worker | 3-6 weeks | Medium | Data inconsistency |
| State machine engine | 3-6 weeks | Medium | Business logic bugs |
| Multi-tenancy | 4-8 weeks | High | Data leak (compliance) |
| Permission system | 3-6 weeks | Medium | Security incident |
| Audit trail | 2-4 weeks | Low-Medium | Compliance failure |
| Natural key counter | 1-2 weeks | Low | Invoice gaps |
| Admin panel (CRUD) | 6-12 weeks | Medium | Developer productivity |
| Auto-DDL migration | 3-6 weeks | Medium | Downtime |
| **Total (conservative)** | **~6-9 months** | **High** | **Various** |

Compare with FormSpec: **These features are done. Tested. Integrated.** You only build business logic.

### Team Size Implications

| Phase | Custom App | FormSpec |
|---|---|---|
| **Foundation** (infrastructure + patterns) | 1-3 developers, 3-6 months | 0 developers, 0 months (built-in) |
| **Business logic** | 1-2 developers ongoing | 1-2 developers ongoing |
| **Frontend CRUD** | 1 developer, 2-4 months per entity | 0 developers (auto-generated) |
| **Maintenance** | 1 developer ongoing (infrastructure) | Minimal (framework handles infra) |

---

## 5. When to Choose Which

### Choose FormSpec when:
- You are building a **business application** that would benefit from enterprise patterns (idempotency, outbox, state machine, multi-tenancy).
- You want to **ship faster** — days to working API+admin panel instead of months.
- You don't want to **build and maintain** infrastructure code (migrations, auth, permissions, audit).
- You value **structural guarantees** — the framework ensures patterns are correct.
- **Governance matters** — you need policies, signing, audit trails.
- Your team is **small** — FormSpec multiplies a small team's output.

### Choose custom app when:
- You need **maximum flexibility** — FormSpec's conventions don't fit your use case.
- You are building something that is **not a business application** (real-time game server, IoT platform, data pipeline, ML inference).
- You have a **large team** with dedicated infrastructure engineers.
- You have **very specific compliance requirements** that FormSpec's abstractions can't satisfy.
- You want **no framework dependency** — full control over every dependency and pattern.
- You are **already invested** in a specific stack (e.g., Python/Django, Node/Express) and migration cost exceeds benefits.

---

## 6. Conclusion

> **The biggest competitor to FormSpec is not another framework. It is the default choice: building from scratch, because that's what developers know how to do.**

The trade-off is clear:

| Custom App | FormSpec |
|---|---|
| Maximum flexibility | Convention with guardrails |
| You own every line (control) | You own business logic only |
| 6-18 months to production | Weeks to months to production |
| High maintenance burden | Low maintenance burden |
| Patterns are DIY (often wrong) | Patterns are built-in (correct by default) |
| Full freedom | 80% conventional, 20% custom (via `asset`) |

FormSpec's value proposition is simple: **the features that take most teams 6-12 months to build, debug, and harden are provided out of the box — tested, documented, and integrated.** You start writing business logic on day one, not infrastructure code.

> If you are building a business application and your alternative is "we'll build it from scratch because there's no framework that fits" — **FormSpec is the framework designed for exactly that situation.**

---
title: Forma vs Vercel (Next.js + v0)
description: Comparing Forma's spec-first business application ecosystem with Vercel's frontend deployment platform and AI codegen
date: 2026-07-06
---

# Forma vs Vercel

> **Forma** is a spec-first ecosystem for building business applications. **Vercel** is a cloud platform for frontend deployment, centered around Next.js and the `v0` AI codegen tool.

These two projects operate in **fundamentally different domains** — Forma targets backend-heavy business applications (ERP, POS, inventory, billing), while Vercel targets frontend and general web application deployment. This comparison exists because both address "building apps fast" — but from opposite directions.

---

## 1. Overview

### Forma
A complete, spec-first ecosystem for building **business applications** (multi-user transactional systems with domain rules). YAML manifests are the single source of truth for APIs, admin panel, frontend, documentation, state machines, permissions, and events. Built in Go, with business logic via Go (native), Starlark (sandboxed scripting), or any language (sidecar pattern).

### Vercel
A cloud platform for **frontend deployment and hosting**, centered on Next.js (React framework). Offers edge functions, serverless compute, static hosting, and the `v0` AI code generation tool. Acquired the `v0.dev` AI coding platform to enable generating full-stack applications from natural language prompts.

---

## 2. Philosophy

| | Forma | Vercel |
|---|---|---|
| **How to build** | Write YAML spec → framework generates API, UI, docs, types. Contract before implementation. | Write code (or prompt `v0`) → deploy to edge. Code-first, AI-assisted. |
| **Source of truth** | The YAML manifest. Structural guarantees are enforced by the framework. | The code in your repository. No framework-level guarantees beyond what you write. |
| **Target user** | Go developers building business software (ERP, POS, inventory, clinic, school, HRM). | Frontend and full-stack developers, especially those using Next.js and React. |
| **"App" means** | A multi-user transactional system with domain rules, state machines, and permissions. | A web application — static site, e-commerce, SaaS dashboard, blog, or API. |

---

## 3. Architecture

```
Forma:                              Vercel:
┌──────────────────────┐           ┌──────────────────────┐
│  Control Plane        │           │  Edge Network (300+) │
│  (forma-control)      │           │                      │
│  Policy · Signing     │           │  Static assets       │
│  Audit · Governance   │           │  Edge Functions      │
└──────────┬───────────┘           │  Serverless Fns      │
           │ mTLS                  │  ISR / SSR           │
┌──────────┴───────────┐           └──────────┬───────────┘
│  Resource Plane       │                      │
│  (forma-resource)     │              ┌───────┴───────┐
│                      │              │  Your Code    │
│  Entity Engine       │              │  (Next.js)    │
│  State Machine       │              └───────────────┘
│  Events · Actions    │
│  CRUD API · Admin    │
└──────────────────────┘
```

**Key differences:**
- Forma has **two processes** (Control + Resource) even in development — governance is built-in from day one.
- Vercel has **300+ edge locations** for low-latency static/edge delivery, but all dynamic logic (serverless functions, database queries) runs in a centralized region.
- Forma is **self-hostable** (single binary, Docker, or Forma Cloud). Vercel is **cloud-only**.

---

## 4. Feature Comparison

| Dimension | Forma | Vercel |
|---|---|---|
| **Paradigm** | Spec-first, declarative | Code-first (or AI-prompted), deploy-centric |
| **Backend language** | Go (native) + Starlark (script) + sidecar (any language) | JavaScript / TypeScript (serverless, edge functions) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | Next.js (React Server Components, App Router, SSR, ISR) |
| **State Machine** | ✅ Built-in — define states, transitions, guards in YAML | ❌ Not available — use XState or build manually |
| **Idempotency** | ✅ Enforced by framework (Idempotency-Key header) | ❌ Must be implemented manually |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once delivery via outbox table | ❌ Not available — use queues (RabbitMQ, AWS SQS) manually |
| **Multi-tenancy** | ✅ Workspace model — automatic tenant isolation, apps are tenancy-blind | ❌ Not available — implement with Clerk, Auth0, or manual logic |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` per action | ❌ DIY — NextAuth middleware, RBAC library, or manual |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego — deployment policy, approval, signing | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing for all module releases | ❌ Not available |
| **Audit Trail** | ✅ Write-once, immutable audit log | ❌ DIY — use external audit service |
| **Database Abstraction** | `ctx.db` — raw SQL, module-scoped, tenant-isolated | Prisma / Drizzle / raw — you choose and manage |
| **Scripting / Hot Reload** | Starlark (`script_ref`) — editable from admin panel, versioned, rollback | ❌ Not available (code changes require redeploy) |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) via Unix socket | ❌ JavaScript / TypeScript only |
| **Global Latency** | Via global proxy (Cloudflare, AWS Global Accelerator) — network-layer optimization | 300+ edge locations + Edge Functions — compute-layer optimization |
| **Preview Deployments** | Manual via `forma apply` GitOps | ✅ Automatic per-PR staging + database branching |
| **Ecosystem / Marketplace** | Module registry with pricing models (free, one-time, subscription, per-seat, per-call) | Vercel Templates + Integrations (marketplace of 3rd-party services) |
| **Hosting** | Self-host (single binary, Docker, K8s) + Forma Cloud (managed) | Cloud-only (Vercel platform) |
| **Open Source** | ✅ FSL (source available, auto-converts to Apache 2.0 after 2 years). Spec is CC0. | ❌ Closed source platform (Next.js is MIT) |
| **Learning Curve** | Medium — YAML, Go, Starlark concepts | Low — if you know React/Next.js |

---

## 5. When to Choose Which

### Choose Forma when:
- You are building a **business application** with domain rules, state machines, and multi-user workflows (ERP, POS, inventory, billing, clinic/school management).
- You need **enterprise patterns by default** — idempotency, outbox, distributed locking, tenant isolation — without researching and implementing each one.
- **Governance matters** — you need deployment policies, artifact signing, immutable audit trails, and approval workflows.
- You want **self-hosted or air-gapped deployment** (government, military, banking).
- You prefer **Go performance** for backend logic but want the option to use other languages via sidecar.

### Choose Vercel when:
- You are building a **public-facing website, e-commerce store, blog, or marketing site**.
- You want **instant preview deployments** for every pull request.
- You need **global edge delivery** for static assets and SSR with minimal latency.
- You are deeply invested in the **React / Next.js ecosystem**.
- Your application is **read-heavy or stateless** — edge functions and CDN caching are sufficient.
- You want **zero-ops deployment** — `git push` and it's live.

---

## 6. Conclusion

**Forma and Vercel are not direct competitors.** They solve different problems:

| This is a Forma app | This is a Vercel app |
|---|---|
| Multi-branch POS system | Marketing website |
| Inventory management | Documentation site |
| Billing/invoicing system | E-commerce storefront |
| HRM / payroll system | SaaS landing page + blog |
| Healthcare management | API playground / demo |
| School management system | Portfolio / personal site |

Vercel is unmatched for **frontend DX, preview deployments, and global CDN**. Forma is unmatched for **business application structure, enterprise patterns, and governance**. A forward-looking architecture could use **Forma as the backend** (API + business logic + admin panel) and **Vercel for the customer-facing frontend** — the best of both worlds.

> **The real competitor to Forma is not Vercel — it is building business applications from scratch without a framework, which is what most teams still do today.**

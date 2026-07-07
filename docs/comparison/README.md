# Forma: Comparison with Alternative Platforms

> **Last Updated:** 2026-07-06
> **Audience:** Developers, architects, and decision-makers evaluating Forma against other frameworks and platforms.

This directory provides structured comparisons between **Forma** — the spec-first, declarative ecosystem for building business applications in Go — and alternative approaches to building business software.

Each document follows a consistent format: overview, philosophy, architecture, feature comparison table, decision guidance, and conclusion.

---

## Master Comparison Matrix

| Dimension | [Forma](./forma-vs-vercel.md) | [Spring Boot](./forma-vs-springboot.md) | [Laravel](./forma-vs-laravel.md) | [Frappe/ERPNext](./forma-vs-frappe.md) | [PocketBase](./forma-vs-pocketbase.md) | [Custom App](./forma-vs-custom-app.md) | [Supabase](./forma-vs-supabase.md) | [Budibase/NocoDB](./forma-vs-budibase.md) |
|---|---|---|---|---|---|---|---|---|
| **Paradigm** | Spec-first, declarative | Code-first, imperative | Code-first, convention | Code-first, declarative DocType | Code-first, minimalist | Code-first (DIY) | BaaS, managed | Visual low-code |
| **Language** | Go (native + Starlark + sidecar) | Java / Kotlin | PHP | Python | Go | Any | Any (client-side) | JS (internal) |
| **Frontend** | Manifest-driven renderer (YAML → React) | Any (Thymeleaf, React, Angular) | Blade + Inertia + Livewire | Frappe JS + Jinja templating | Admin panel only | You build it | Any (JS client SDK) | Built-in visual builder |
| **State Machine** | ✅ Built-in (YAML) | ❌ DIY (Spring Statemachine) | ❌ DIY | ✅ Built-in (Workflow) | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY |
| **Idempotency** | ✅ Enforced by framework | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY |
| **Outbox / Reliable Events** | ✅ Built-in (at-least-once) | ❌ DIY (Debezium) | ❌ DIY (Laravel Pulse?) | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY (via PG notify) | ❌ DIY |
| **Multi-tenancy** | ✅ Workspace model (built-in) | ❌ DIY | ❌ DIY (multi-tenant package) | ✅ Built-in (site-based) | ❌ DIY | ❌ DIY | ❌ DIY (via RLS) | ❌ DIY |
| **Permission Model** | ✅ Declarative (`required_permission`) | ❌ DIY (Spring Security) | ❌ DIY (Gates/Policies) | ✅ Built-in (Role-Permission) | ❌ Basic (collection rules) | ❌ DIY | ✅ Row-Level Security | ❌ Basic |
| **Governance/Policy** | ✅ Control Plane + OPA | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None |
| **Database** | Raw SQL via `ctx.db` (no ORM) | JPA / Hibernate (ORM) | Eloquent (ORM) | Frappe ORM | SQLite (built-in) | Any (you choose) | Postgres (managed) | Built-in (SQLite/Postgres) |
| **Scripting** | Starlark (sandboxed) | ❌ None | ❌ None (PHP is code) | Python (full) | ❌ None | ❌ None | ❌ None (edge functions) | ❌ Limited (JS) |
| **Polyglot Logic** | ✅ Sidecar pattern (PHP/Python/Node/Java) | ❌ JVM-only | ❌ PHP-only | ❌ Python-only | ❌ Go-only | ✅ Any | ❌ JS/TS only | ❌ JS only |
| **Ecosystem / Marketplace** | ✅ Module registry + pricing models | ❌ Maven/Gradle (libs) | ❌ Packagist (libs) | ✅ Frappe Marketplace | ❌ None | ❌ None | ✅ Supabase Integrations | ❌ Limited templates |
| **Hosting** | Self-host + Forma Cloud | Self-host (any) | Self-host + Laravel Forge | Self-host + Frappe Cloud | Self-host (single binary) | Self-host (any) | Cloud-only | Self-host + Cloud |
| **Learning Curve** | Medium (YAML + Go + Starlark) | High (Java ecosystem) | Low (PHP) | Medium (Python + Frappe framework) | Low (Go, minimal) | High (everything DIY) | Low (JS, managed) | Low (visual) |
| **Ideal For** | Business apps (ERP, POS, inventory, billing) | Large enterprise systems | Web apps, SaaS, MVPs | ERP, CRM, business management | Small APIs, prototypes, internal tools | Any (with full control) | Real-time apps, mobile backends | Internal tools, CRUD apps |

---

## Documents

| # | File | Comparand | Key Angle |
|---|---|---|---|
| 1 | [`forma-vs-vercel.md`](./forma-vs-vercel.md) | **Vercel** (Next.js + v0) | Spec-first vs AI-codegen; business application framework vs frontend deployment platform |
| 2 | [`forma-vs-springboot.md`](./forma-vs-springboot.md) | **Spring Boot** (Java) | Declarative YAML vs imperative annotations; Go vs JVM ecosystem |
| 3 | [`forma-vs-laravel.md`](./forma-vs-laravel.md) | **Laravel** (PHP) | "Laravel of Go" — where Forma was inspired and where they diverge |
| 4 | [`forma-vs-frappe.md`](./forma-vs-frappe.md) | **Frappe/ERPNext** (Python) | The most similar — DocType vs Entity; both open ecosystems for business apps |
| 5 | [`forma-vs-pocketbase.md`](./forma-vs-pocketbase.md) | **PocketBase** (Go) | "One definition, many protocols" — DNA they share and where Forma extends beyond |
| 6 | [`forma-vs-custom-app.md`](./forma-vs-custom-app.md) | **Custom App** (from scratch) | The cost of building all enterprise patterns (idempotency, outbox, lock, multi-tenancy) manually |
| 7 | [`forma-vs-supabase.md`](./forma-vs-supabase.md) | **Supabase** | Backend-as-a-Service vs application framework; managed infrastructure vs self-hosted |
| 8 | [`forma-vs-budibase.md`](./forma-vs-budibase.md) | **Budibase / NocoDB** | Visual low-code vs spec-first declarative; drag-and-drop vs YAML-as-source-of-truth |

---

## How to Read These Comparisons

Each document is self-contained and assumes no prior knowledge of Forma. They are designed to help you answer:

- **What problems does this solve that my current stack doesn't?**
- **What trade-offs am I making by choosing Forma over X?**
- **Is Forma mature enough for my use case?**
- **Which architectural patterns are built-in vs DIY?**

For a complete understanding of Forma itself, start with the [Forma Overview](../spec/01-overview.md).

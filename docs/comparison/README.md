# FormSpec: Comparison with Alternative Platforms

> **Last Updated:** 2026-09-02
> **Audience:** Developers, architects, and decision-makers evaluating FormSpec against other frameworks and platforms.

This directory provides structured comparisons between **FormSpec** — the spec-first, declarative ecosystem for building business applications in Go — and alternative approaches to building business software.

Each document follows a consistent format: overview, philosophy, architecture, feature comparison table, decision guidance, and conclusion.

---

## Master Comparison Matrix

| Dimension | [FormSpec](./formspec-vs-vercel.md) | [Spring Boot](./formspec-vs-springboot.md) | [Laravel](./formspec-vs-laravel.md) | [Frappe/ERPNext](./formspec-vs-frappe.md) | [PocketBase](./formspec-vs-pocketbase.md) | [Custom App](./formspec-vs-custom-app.md) | [Supabase](./formspec-vs-supabase.md) | [Budibase/NocoDB](./formspec-vs-budibase.md) | [AI App Builder](./formspec-vs-ai-app-builder.md) |
|---|---|---|---|---|---|---|---|---|---|
| **Paradigm** | Spec-first, declarative | Code-first, imperative | Code-first, convention | Code-first, declarative DocType | Code-first, minimalist | Code-first (DIY) | BaaS, managed | Visual low-code | Prompt-first, AI-generated code |
| **Language** | Go (native + Starlark + sidecar) | Java / Kotlin | PHP | Python | Go | Any | Any (client-side) | JS (internal) | Opaque (vendor stack) |
| **Frontend** | Manifest-driven renderer (YAML → React) | Any (Thymeleaf, React, Angular) | Blade + Inertia + Livewire | Frappe JS + Jinja templating | Admin panel only | You build it | Any (JS client SDK) | Built-in visual builder | AI-generated per prompt |
| **State Machine** | ✅ Built-in (YAML) | ❌ DIY (Spring Statemachine) | ❌ DIY | ✅ Built-in (Workflow) | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ⚠️ Not disclosed |
| **Idempotency** | ✅ Enforced by framework | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY | ⚠️ Not disclosed |
| **Outbox / Reliable Events** | ✅ Built-in (at-least-once) | ❌ DIY (Debezium) | ❌ DIY (Laravel Pulse?) | ❌ DIY | ❌ DIY | ❌ DIY | ❌ DIY (via PG notify) | ❌ DIY | ⚠️ Not disclosed |
| **Multi-tenancy** | ✅ Workspace model (built-in) | ❌ DIY | ❌ DIY (multi-tenant package) | ✅ Built-in (site-based) | ❌ DIY | ❌ DIY | ❌ DIY (via RLS) | ❌ DIY | ⚠️ Feature-level claim, isolation strategy undisclosed |
| **Permission Model** | ✅ Declarative (`required_permission`) | ❌ DIY (Spring Security) | ❌ DIY (Gates/Policies) | ✅ Built-in (Role-Permission) | ❌ Basic (collection rules) | ❌ DIY | ✅ Row-Level Security | ❌ Basic | ⚠️ RBAC advertised, enforcement consistency undisclosed |
| **Governance/Policy** | ✅ Control Plane + OPA | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None | ❌ None disclosed |
| **Database** | Raw SQL via `ctx.db` (no ORM) | JPA / Hibernate (ORM) | Eloquent (ORM) | Frappe ORM | SQLite (built-in) | Any (you choose) | Postgres (managed) | Built-in (SQLite/Postgres) | Managed, opaque |
| **Scripting** | Starlark (sandboxed) | ❌ None | ❌ None (PHP is code) | Python (full) | ❌ None | ❌ None | ❌ None (edge functions) | ❌ Limited (JS) | N/A (regenerate via prompt) |
| **Polyglot Logic** | ✅ Sidecar pattern (PHP/Python/Node/Java) | ❌ JVM-only | ❌ PHP-only | ❌ Python-only | ❌ Go-only | ✅ Any | ❌ JS/TS only | ❌ JS only | ❌ Single opaque stack |
| **Ecosystem / Marketplace** | ✅ Module registry + pricing models | ❌ Maven/Gradle (libs) | ❌ Packagist (libs) | ✅ Frappe Marketplace | ❌ None | ❌ None | ✅ Supabase Integrations | ❌ Limited templates | ❌ None (closed platform) |
| **Hosting** | Self-host + FormSpec Cloud | Self-host (any) | Self-host + Laravel Forge | Self-host + Frappe Cloud | Self-host (single binary) | Self-host (any) | Cloud-only | Self-host + Cloud | Cloud-only, managed/serverless |
| **Learning Curve** | Medium (YAML + Go + Starlark) | High (Java ecosystem) | Low (PHP) | Medium (Python + Frappe framework) | Low (Go, minimal) | High (everything DIY) | Low (JS, managed) | Low (visual) | Very Low (natural language) |
| **Ideal For** | Business apps (ERP, POS, inventory, billing) | Large enterprise systems | Web apps, SaaS, MVPs | ERP, CRM, business management | Small APIs, prototypes, internal tools | Any (with full control) | Real-time apps, mobile backends | Internal tools, CRUD apps | Non-technical founders, rapid idea validation |

---

## Documents

| # | File | Comparand | Key Angle |
|---|---|---|---|
| 1 | [`formspec-vs-vercel.md`](./formspec-vs-vercel.md) | **Vercel** (Next.js + v0) | Spec-first vs AI-codegen; business application framework vs frontend deployment platform |
| 2 | [`formspec-vs-springboot.md`](./formspec-vs-springboot.md) | **Spring Boot** (Java) | Declarative YAML vs imperative annotations; Go vs JVM ecosystem |
| 3 | [`formspec-vs-laravel.md`](./formspec-vs-laravel.md) | **Laravel** (PHP) | "Laravel of Go" — where FormSpec was inspired and where they diverge |
| 4 | [`formspec-vs-frappe.md`](./formspec-vs-frappe.md) | **Frappe/ERPNext** (Python) | The most similar — DocType vs Entity; both open ecosystems for business apps |
| 5 | [`formspec-vs-pocketbase.md`](./formspec-vs-pocketbase.md) | **PocketBase** (Go) | "One definition, many protocols" — DNA they share and where FormSpec extends beyond |
| 6 | [`formspec-vs-custom-app.md`](./formspec-vs-custom-app.md) | **Custom App** (from scratch) | The cost of building all enterprise patterns (idempotency, outbox, lock, multi-tenancy) manually |
| 7 | [`formspec-vs-supabase.md`](./formspec-vs-supabase.md) | **Supabase** | Backend-as-a-Service vs application framework; managed infrastructure vs self-hosted |
| 8 | [`formspec-vs-budibase.md`](./formspec-vs-budibase.md) | **Budibase / NocoDB** | Visual low-code vs spec-first declarative; drag-and-drop vs YAML-as-source-of-truth |
| 9 | [`formspec-vs-ai-app-builder.md`](./formspec-vs-ai-app-builder.md) | **AI App Builder** (Hercules and similar prompt-to-app tools) | Where correctness lives once a human stops writing the implementation — a hardened shared interpreter vs a bespoke codebase regenerated per prompt |

---

## How to Read These Comparisons

Each document is self-contained and assumes no prior knowledge of FormSpec. They are designed to help you answer:

- **What problems does this solve that my current stack doesn't?**
- **What trade-offs am I making by choosing FormSpec over X?**
- **Is FormSpec mature enough for my use case?**
- **Which architectural patterns are built-in vs DIY?**

For a complete understanding of FormSpec itself, start with the [FormSpec Overview](../spec/platform/01-overview.md).

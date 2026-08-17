---
title: FormSpec vs Laravel
description: Comparing FormSpec — the "Laravel of Go" — with Laravel itself. What they share, where they diverge, and why FormSpec is not just a clone.
date: 2026-07-06
---

# FormSpec vs Laravel

> **FormSpec** is a spec-first ecosystem for building business applications in Go. **Laravel** is the most popular PHP framework for web applications, known for elegant syntax, convention over configuration, and a complete ecosystem.
> 
> **Positioning:** _"If Laravel made PHP delightful for web apps, FormSpec makes Go practical for business software — but FormSpec goes further: spec-first, governance control plane, scripting runtime, and a fully declarative surface."_

This comparison is especially important because FormSpec **explicitly cites Laravel as a key inspiration** (one of six sources). Understanding what FormSpec shares with Laravel — and where it deliberately diverges — reveals FormSpec's core identity.

---

## 1. Overview

### FormSpec
A spec-first, declarative ecosystem where YAML manifests are the single source of truth for APIs, UI, documentation, state machines, permissions, and events. Built in Go with business logic via Go native, Starlark scripting, or sidecar. Includes a governance Control Plane (policy, signing, audit) and a marketplace for distributing modules.

### Laravel
The dominant PHP web framework. Elegant syntax (`Route::get()`, `DB::table()`, `Mail::send()`), convention over configuration, and a comprehensive ecosystem (Eloquent ORM, Blade templating, Artisan CLI, Livewire, Vapor, Forge). Powers a huge portion of the modern web.

---

## 2. Philosophy

| | FormSpec | Laravel |
|---|---|---|
| **Source of truth** | YAML manifest (declarative, spec-first) | Code (PHP files, routes, migrations, blade templates) |
| **How to define an entity** | Write `kind: Entity` YAML → framework generates schema, API, UI, docs, types | Write migration + Eloquent model + controller + routes + views (5 files for basic CRUD) |
| **Business logic** | Go native, Starlark script, or sidecar | PHP (custom classes, jobs, events, listeners) |
| **"One definition, many protocols"** | ✅ Yes — YAML → API + UI + docs + types + permissions | ❌ No — each surface is a separate file (controller, view, route, migration) |
| **Governance** | Built-in Control Plane | None (rely on external tools) |
| **Framework first vs spec first** | Spec-first (standard before implementation) | Framework-first (Laravel IS the standard) |

### What FormSpec took from Laravel

From the Foundation Document: *"Ecosystem completeness, DX, and business model — Laravel proved that a framework wins because of the layers around it (Horizon, Pulse, Filament, Forge), not the runtime alone."*

| Laravel Feature | FormSpec Equivalent | Notes |
|---|---|---|
| Horizon (queue monitoring) | `formspec.observe` | Observability dashboard, built with FormSpec's own Dashboard/Widget kinds |
| Pulse (application monitoring) | `formspec.observe` | Same dashboard — metrics, jobs, audit |
| Filament (admin panel) | Admin panel | Derived automatically from Entity manifests (zero setup) |
| Forge / Vapor (deployment) | FormSpec Cloud | Managed hosting with tiered resource plans |
| Cashier (billing) | Marketplace + Ledger | Module revenue sharing, verifiable metering |
| Telescope (debugging) | `formspec repl` | Interactive Starlark console with `ctx.*` access |
| Socialite (social auth) | Official modules | `formspec/mail`, `formspec/notify`, `formspec/scheduler` — built on primitives |
| Artisan CLI | `formspec` CLI | `formspec dev`, `formspec apply`, `formspec generate`, `formspec validate` |

---

## 3. Feature Comparison

| Dimension | FormSpec | Laravel |
|---|---|---|
| **Paradigm** | Spec-first, declarative | Code-first, convention-over-configuration |
| **Backend language** | Go (compiled, low resource usage) | PHP (interpreted, shared-nothing architecture) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | Blade (templating), Livewire (server-side reactivity), Inertia (SPA with Vue/React) |
| **State Machine** | ✅ Built-in — define states, transitions, guards in YAML | ❌ DIY — use packages like spatie/laravel-state-machine or build manually |
| **Idempotency** | ✅ Enforced by framework (built-in idempotency store) | ❌ DIY — implement manually or use a package |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once via outbox table + worker | ❌ DIY — implement Transactional Outbox manually, or use queues (Laravel has queues, not outbox) |
| **Multi-tenancy** | ✅ Workspace model — automatic, apps are tenancy-blind | ❌ DIY — use packages like stancl/tenancy or spatie/laravel-multitenancy |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` per action | ❌ DIY — Laravel Gates/Policies (manual, per-action) |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego — deployment policy, approval, signing | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing for all module releases | ❌ Not available |
| **Audit Trail** | ✅ Write-once immutable audit log | ❌ DIY — use packages like spatie/laravel-activitylog |
| **Database Abstraction** | `ctx.db` — raw SQL, module-scoped, tenant-isolated. **No ORM.** | Eloquent ORM (Active Record pattern) + Query Builder + raw SQL |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — editable from admin panel, versioned, rollback | ❌ PHP is the only language (no sandboxed scripting runtime) |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ PHP-only |
| **Built-in Admin Panel** | ✅ Instant — derived from Entity manifests with zero config | ❌ Not built-in — use Filament, Nova (paid), or build custom |
| **Preview Deployments** | Via GitOps (`formspec apply`) | Via Laravel Vapor or Forge |
| **Ecosystem / Marketplace** | Module registry + pricing models | Packagist (composer packages, no pricing models) |
| **Hosting** | Self-host (single binary, Docker, K8s) + FormSpec Cloud | Self-host + Laravel Forge (server management) + Vapor (serverless) |
| **Performance** | High — compiled Go, minimal memory | Moderate — PHP (interpreted, each request boots framework) |
| **Memory per request** | ~5-15 MB | ~30-80 MB (framework bootstrap) |
| **Learning Curve** | Medium — YAML + Go + Starlark | Low — PHP is beginner-friendly, Laravel documentation is excellent |
| **Open Source** | FSL (source available → Apache 2.0 after 2 years). Spec is CC0. | ✅ MIT (fully open source) |

---

## 4. The Fundamental Divergence

Both FormSpec and Laravel aim to **make developers productive faster**. But they chose fundamentally different paths:

### Laravel's bet: "Framework-first, code authoring"
> Write PHP code with elegant syntax. Conventions and helpers make common tasks (routing, DB, auth, mail) simple. The framework accelerates code writing.

**Result:** You still write code for every surface — controller, model, migration, view, route, test. Each surface can be customized independently. Flexible, but no structural guarantees.

### FormSpec's bet: "Spec-first, contract-driven development"
> Write YAML specs that describe what the application does. The framework generates all surfaces from the contract. You only write code for the unique business logic.

**Result:** One source of truth for API, UI, docs, types, permissions, state machine. Structural guarantees are enforced (idempotency, tenant isolation, outbox). Less flexibility for non-standard cases (solved via `asset` escape hatch and `sidecar`).

### Concrete example: Adding a field to an entity

**Laravel:**
1. Create migration: `php artisan make:migration add_email_to_customers_table`
2. Write migration SQL/Blueprint
3. Run `php artisan migrate`
4. Update Eloquent model with `$fillable`
5. Update form request with validation rules
6. Update blade/Livewire view to show field
7. Update controller if custom logic
8. Update API resource if exposing

**FormSpec:**
1. Add field to YAML:
   ```yaml
   fields:
     email:
       type: email
       required: true
       unique: true
   ```
2. Restart dev server (or hot-reload). Migration runs automatically, API updates, admin panel shows field, validation applies.

---

## 5. When to Choose Which

### Choose FormSpec when:
- You are building a **business application** that needs structural guarantees (idempotency, multi-tenancy, outbox, state machine).
- You want **one source of truth** that generates all surfaces automatically.
- **Governance matters** — deployment policies, artifact signing, audit trails are requirements.
- You prefer **Go** (performance, type safety, low resource usage) and want to use PHP/Python/Node only for specific logic via sidecar.
- You want a **self-hosted, air-gap capable** solution.
- You value **raw SQL** over ORM.

### Choose Laravel when:
- You are building a **general web application** (SaaS, marketplace, blog, content site) where Laravel's conventions map well.
- You work in **PHP** and the PHP ecosystem is your strength.
- You need **maximum flexibility** — custom routes, custom views, custom everything.
- You want **proven ecosystem maturity** — Laravel has been production-ready for over a decade.
- **Hiring is a priority** — Laravel developers are widely available.
- You want a **battle-tested ORM** (Eloquent).

---

## 6. Conclusion

FormSpec is **not** "Laravel in Go." That comparison is a communication bridge — it helps Laravel developers understand what FormSpec aims to do. But the two frameworks diverge on fundamental choices:

| Laravel's choice | FormSpec's choice |
|---|---|
| Framework-first | Spec-first |
| Code is the source of truth | YAML is the source of truth |
| Each surface is a separate file | One definition → all surfaces |
| Flexible (no structural guarantees) | Rigid where it matters (enforced patterns) |
| PHP-only | Go + Starlark + sidecar |
| No governance layer | Built-in Control Plane |
| ORM (Eloquent) | Raw SQL (`ctx.db`) |

FormSpec takes Laravel's **ecosystem completeness** and **developer experience** as inspiration, but builds on a **different architectural foundation**: spec-first, declarative, with governance built-in from day one.

> If Laravel is the "Framework of the People" (accessible, elegant, proven), FormSpec aims to be the "Spec of the Enterprise" — what you reach for when structural guarantees, governance, and reliability matter as much as developer speed.

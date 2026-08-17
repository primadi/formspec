---
title: FormSpec vs Frappe (ERPNext)
description: Comparing FormSpec's spec-first ecosystem with Frappe/ERPNext — the most similar project, and the inspiration for Entity and Module concepts
date: 2026-07-06
---

# FormSpec vs Frappe (ERPNext)

> **FormSpec** is a spec-first ecosystem for building business applications in Go. **Frappe** (with ERPNext) is a full-stack low-code framework in Python, best known for powering the ERPNext ecosystem.

Of all the comparisons in this directory, **this is the most important one.** Frappe is the closest existing project to FormSpec — and FormSpec's `kind: Entity` is directly inspired by Frappe's `DocType`. Understanding the similarities and differences reveals whether FormSpec is "just another Frappe" or something fundamentally new.

---

## 1. Overview

### FormSpec
Spec-first ecosystem for business applications in Go. YAML manifests define entities, state machines, actions, permissions, and UI. Includes a governance Control Plane, six `ctx.*` primitives, five implementation types (native Go, Starlark, compiled, sidecar), and a marketplace. Two-process architecture (Control + Resource planes).

### Frappe / ERPNext
A full-stack low-code framework in Python. **DocType** is the central concept — a declarative definition (stored in the database, not YAML) that generates CRUD, forms, and reports. ERPNext is the flagship application built on Frappe — a complete ERP system with 30+ modules (accounting, HR, inventory, manufacturing, CRM, etc.). Frappe also provides a web framework (Jinja templates + JS), workflow engine, permissions system, and a marketplace.

---

## 2. Philosophy

| | FormSpec | Frappe |
|---|---|---|
| **Paradigm** | Spec-first, declarative (YAML files) | Low-code (DocType defined in DB, Python code for logic) |
| **Source of truth** | YAML files on disk (git-friendly, AI-friendly) | Database (DocType JSON stored in `tabDocType`) |
| **Entity definition format** | `kind: Entity` YAML — fields, state machine, actions, events, permissions in one file | DocType — fields, permissions, workflows defined via UI or `frappe` CLI, stored in DB |
| **Language for business logic** | Go (native), Starlark (script), sidecar (any) | Python (server-side scripts, hooks, overrides) |
| **Governance** | Built-in Control Plane (OPA/Rego, signing, audit) | None — rely on Frappe's permission system |
| **Marketplace** | Module registry with pricing & metering | Frappe Marketplace (apps, themes, paid listings) |

### What FormSpec took from Frappe

From the Foundation Document: *"How to define business entities, workflows, and modules declaratively. DocType → `kind: Entity`. Vertical modules (accounting, HRM, inventory) as first-class ecosystem citizens."*

| Frappe Concept | FormSpec Equivalent | Notes |
|---|---|---|
| DocType | `kind: Entity` | Core data definition — fields, child tables, naming series |
| Workflow | `state_machine` (in Entity) + `kind: Workflow` | State transitions with guards, approval steps |
| Module | `kind: Module` | Packaging unit, permission namespace |
| Role-Permission | `required_permission` + `uses` | Explicit, auditable permission model |
| Report Builder | `kind: Report` | Parameterized, exportable reports |
| Print Format | `kind: Print` | Multi-target output (PDF, thermal, dot matrix) |
| Form layout (sections, columns) | `kind: Form` UI hints in spec | Design-time layout locking (modal/drawer/page) |
| Frappe Marketplace | Module Registry + pricing models | Verified Badge, consent footprint, verifiable metering |
| DocType editor (UI) | Visual editor in admin panel | Writes YAML to files, not hidden DB — git remains source of truth |

---

## 3. Feature Comparison

| Dimension | FormSpec | Frappe / ERPNext |
|---|---|---|
| **Paradigm** | Spec-first (YAML files on disk) | Low-code (DocType defined in DB, Python code) |
| **Backend language** | Go (compiled) + Starlark script + sidecar (any language) | Python (interpreted) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA) | Frappe JS + Jinja templating (server-rendered + SPA hybrid) |
| **State Machine** | ✅ Built-in — states, transitions, guards in YAML | ✅ Built-in — Workflow with states, transitions, approval roles |
| **Idempotency** | ✅ Enforced by framework (built-in store) | ❌ Not available — implement manually |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once delivery | ⚠️ Partially — Frappe Events, but no built-in outbox pattern |
| **Multi-tenancy** | ✅ Workspace model — automatic, apps are tenancy-blind | ✅ Site-based multi-tenancy (single vs multiple sites) |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` | ✅ Role-Permission system with Role Profile |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing for all releases | ❌ Not available |
| **Audit Trail** | ✅ Write-once immutable audit log | ✅ Frappe has Activity Log / Document Timeline |
| **Database** | `ctx.db` — raw SQL, no ORM | Frappe ORM (abstraction over MariaDB/Postgres) |
| **Scripting / Hot Reload** | ✅ Starlark (`script_ref`) — editable from admin panel, versioned, rollback | ✅ Python scripts (Server Script doctype) — editable from UI, but Python is not sandboxed |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ Python-only |
| **Built-in Admin Panel** | ✅ Instant — derived from Entity manifests | ✅ Form/List views auto-generated from DocType |
| **Built-in ERP Modules** | ❌ Not included (modules are marketplace items) | ✅ ERPNext ships with 30+ ready modules (accounting, HR, inventory, CRM, manufacturing, etc.) |
| **Ecosystem / Marketplace** | Module registry with pricing models (free, subscription, per-seat, per-call) + verifiable metering | Frappe Marketplace — apps, themes, paid listings |
| **Hosting** | Self-host (single binary, Docker, K8s) + FormSpec Cloud | Self-host + Frappe Cloud (managed) |
| **Performance** | High — compiled Go, low resource usage | Moderate — Python (interpreted, GIL-bound for CPU tasks) |
| **Memory footprint** | ~10-50 MB | ~100-300 MB (Python + MariaDB) |
| **Maturity** | New (MVP in development) | Mature (15+ years, production-proven at scale) |
| **Open Source** | FSL (source available → Apache 2.0 after 2 years). Spec is CC0. | ✅ GPL v3 (fully open source) |
| **Learning Curve** | Medium — YAML + Go + Starlark | Medium — Python + Frappe framework + Jinja templating |

---

## 4. The Key Difference: YAML Files on Disk vs DocType in DB

This single design decision has profound implications:

### Frappe: DocType in the Database
- Define entities via the browser UI → stored in `tabDocType`
- Changes are **database rows**, not files
- Git workflow requires `bench export-doc` / `bench import-doc`
- AI tools cannot easily read/edit — they need API calls or direct DB access
- Merging changes requires custom tooling

### FormSpec: YAML Files on Disk
- Define entities as `kind: Entity` YAML files in your project
- Changes are **git-diffable, mergeable, reviewable**
- AI tools read/edit YAML naturally — it's the same format as K8s, Docker Compose, CI configs
- `formspec validate` runs on every PR — structural errors caught before deploy
- Visual editor (planned) writes YAML to files → git remains the source of truth

**This makes FormSpec fundamentally more compatible with modern GitOps and AI-assisted development workflows.**

---

## 5. The ERPNext Advantage

Frappe/ERPNext has something FormSpec cannot match today: **ERPNext itself.** It includes 30+ ready-to-use business modules:

- Accounting (GL, AR, AP, budgeting, cost centers)
- Human Resources (employees, payroll, leave, attendance)
- Inventory (warehouse, stock, serial/batch)
- CRM (leads, opportunities, customer management)
- Manufacturing (BOM, work orders, production planning)
- Selling & Buying (quotations, sales/purchase orders)
- Projects (tasks, timesheets, billing)
- Assets, Healthcare, Education, Non-Profit, and more

FormSpec will have a **marketplace** where such modules can be built and sold — but they don't exist yet. Building a complete ERP on FormSpec is possible but requires developing each module.

---

## 6. When to Choose Which

### Choose FormSpec when:
- You want a **modern, GitOps-native** workflow — YAML on disk, diff in PRs, validate in CI.
- **AI-assisted development** is a priority — YAML files are trivially read/written by AI.
- You prefer **Go** for performance, type safety, and low resource usage.
- You need **built-in governance** — deployment policies, artifact signing, audit trails.
- You want **raw SQL control** without an ORM layer.
- You need **polyglot business logic** — different languages for different modules.
- You need **air-gapped, self-hosted** deployment with minimal dependencies.

### Choose Frappe/ERPNext when:
- You need a **complete, production-ready ERP** today — accounting, HR, inventory, CRM, manufacturing.
- You prefer **Python** and the Python ecosystem.
- You want a **proven, mature platform** (15+ years of production use).
- You need **ready-made business modules** that work out of the box.
- You want the **Frappe ecosystem** — hundreds of community apps on the marketplace.
- You prefer **GPL v3** license (FormSpec uses FSL).

---

## 7. Conclusion

Frappe/ERPNext is the **closest existing project** to FormSpec — and in many ways, the most important comparison. Here's the honest assessment:

| Frappe/ERPNext is better today because: | FormSpec will eventually be better because: |
|---|---|
| Has ERPNext — 30+ ready modules | GitOps-native (YAML files on disk) |
| Mature (15+ years) | Designed for AI-assisted development |
| GPL v3 (fully open source) | Built-in governance (Control Plane) |
| Hundreds of community apps | Multi-language sidecar support |
| Battle-tested at scale | Go performance + low resource usage |
| | Raw SQL (no ORM overhead) |
| | Closed-set primitives (auditable) |

**If you need ERP functionality today, choose Frappe/ERPNext.** It's production-proven and feature-complete.

**If you are building a new business application framework and want modern GitOps, AI-native workflows, Go performance, and governance by default — FormSpec is the architecture designed for that future.**

> FormSpec does not aim to be "Frappe in Go." It aims to be what Frappe would look like if it were designed today: YAML-native, GitOps-ready, AI-friendly, with governance and polyglot execution built in from the start.

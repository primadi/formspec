---
title: Forma vs Budibase / NocoDB
description: Comparing Forma's spec-first declarative approach with visual low-code platforms for building business applications
date: 2026-07-06
---

# Forma vs Budibase / NocoDB

> **Forma** is a spec-first, declarative ecosystem for building business applications in Go. **Budibase** and **NocoDB** are open-source low-code platforms that let you build business apps through a visual interface — drag-and-drop UI builder, spreadsheet-like data modeling, and point-and-click automation.

This comparison examines Forma against the **visual low-code category** represented by Budibase and NocoDB — both popular open-source platforms for building internal tools and business applications without writing traditional code.

---

## 1. Overview

### Forma
A spec-first ecosystem where YAML manifests define entities, state machines, permissions, and UI. The frontend is a **manifest-driven renderer** — a React SPA that reads UI manifests at runtime and renders 12 kinds of UI pages (Table, Form, Dashboard, Kanban, Wizard, etc.). Business logic in Go, Starlark, or sidecar. Two-process architecture with governance Control Plane.

### Budibase
An open-source low-code platform for building business applications. Visual UI builder (drag-and-drop components), built-in database (internal or connect to external), automations (trigger → action workflows), and role-based access control. Apps are hosted on Budibase Cloud or self-hosted via Docker.

### NocoDB
An open-source Airtable alternative that turns any SQL database into a smart spreadsheet. Connect to existing MySQL/Postgres/MariaDB/MSSQL/SQLite, get a spreadsheet-like UI, generate REST API, and build automations. More focused on **database management as a spreadsheet** than full application building.

---

## 2. Philosophy

| | Forma | Budibase | NocoDB |
|---|---|---|---|
| **Primary interface** | YAML files (code, git-friendly) | Visual drag-and-drop UI builder | Spreadsheet-like grid (Airtable style) |
| **Who builds the app** | Developers (Go + YAML) | Citizen developers + IT teams | Database admins + business users |
| **Source of truth** | YAML manifests on disk | Budibase internal DB | The database itself |
| **Git / version control** | ✅ Native (YAML files, git-diffable) | ⚠️ Export/import JSON (not native) | ⚠️ Export/import CSV/JSON |
| **Target user** | Go developers building scalable business apps | Non-developers building internal tools | Anyone managing data with a spreadsheet UI |
| **Flexibility limit** | Very high (custom code via sidecar/asset) | Medium (component library + JS actions) | Low (spreadsheet + automations) |

---

## 3. Feature Comparison

| Dimension | Forma | Budibase | NocoDB |
|---|---|---|---|
| **Paradigm** | Spec-first (YAML files) | Visual low-code (drag-and-drop) | Spreadsheet-first (Airtable-like) |
| **Backend language** | Go (native) + Starlark + sidecar (any) | Internal (Node.js) + JavaScript in automations | Internal (Node.js) |
| **Frontend approach** | Manifest-driven renderer (YAML → React SPA — 12 UI kinds) | Visual builder — drag and drop components | Spreadsheet grid + form views |
| **State Machine** | ✅ Built-in — declare in YAML | ❌ DIY — use automations (limited) | ❌ Not available |
| **Idempotency** | ✅ Enforced by framework | ❌ Not available | ❌ Not available |
| **Outbox / Reliable Events** | ✅ Built-in at-least-once delivery | ❌ Not available | ❌ Not available |
| **Multi-tenancy** | ✅ Workspace model — automatic, tenancy-blind apps | ❌ DIY — separate instances or manual configuration | ❌ Not built-in |
| **Permission Model** | ✅ Declarative `required_permission` + `uses` (server-enforced) | ✅ Role-based access control (builder-defined) | ⚠️ Basic (view/edit per table, no row-level) |
| **Governance / Policy** | ✅ Control Plane with OPA/Rego | ❌ Not available | ❌ Not available |
| **Artifact Signing** | ✅ Ed25519 signing | ❌ Not available | ❌ Not available |
| **Audit Trail** | ✅ Write-once immutable audit log | ⚠️ Basic audit logs | ⚠️ Basic change history |
| **Database** | PostgreSQL + Valkey + MinIO — raw SQL via `ctx.db` | Built-in (CouchDB/internal) or connect to Postgres/MySQL/SQL Server | **Any SQL** (MySQL, Postgres, MSSQL, SQLite, MariaDB) — connects to your existing database |
| **API Generation** | ✅ REST API from YAML (deny-by-default exposure) | ✅ REST API auto-generated | ✅ REST API + Swagger docs |
| **Scripting / Automation** | ✅ Starlark (`script_ref`) — versioned, runtime-editable, rollback | ✅ JavaScript automations (trigger → action) + custom components | ⚠️ Webhooks + formulas (limited) |
| **Polyglot Logic** | ✅ Sidecar container (PHP, Python, Node, Java) | ❌ JavaScript only | ❌ JavaScript only |
| **Built-in UI Components** | 12 UI kinds + shadcn/ui components + 80/20 hybrid low-code (custom component escape hatch) | ✅ Rich component library (30+ components — tables, forms, charts, maps, etc.) | ✅ Spreadsheet grid + form + gallery + Kanban views |
| **UI Customization** | High — full React component via `asset` escape hatch | Medium — component props + CSS variables + custom JS | Low — layout options limited to what grid/form/gallery/Kanban offer |
| **Hosting** | Self-host (single binary, Docker, K8s) + Forma Cloud | Self-host (Docker, K8s) + Budibase Cloud | Self-host (Docker) + NocoDB Cloud |
| **Open Source** | FSL (source available → Apache 2.0 after 2 years). Spec is CC0. | ✅ AGPL v3 + Budibase EE (paid) | ✅ AGPL v3 + NocoDB EE (paid) |
| **Learning Curve** | Medium — YAML + Go + Starlark | Low — visual interface, no code needed | Low — spreadsheet UI, familiar to business users |
| **Target Skill Level** | Professional developers | Citizen developers + IT pros | Business users + database admins |

---

## 4. The Fundamental Difference: File vs Visual

### The Forma approach: YAML is the source of truth

```
Write YAML ──► git commit ──► CI validate ──► deploy
    │                              │
    │ └── Entity definitions       │ └── forma validate
    │ └── State machine            │ └── schema diff
    │ └── Permissions              │ └── permission audit
    │ └── UI kinds                 │
    └──────────────────────────────┘
```

**Advantages:**
- Fully git-native — diff, review, merge, rollback
- AI tools can read/write YAML trivially
- CI can validate structure before deploy
- No vendor lock-in (YAML is portable)

**Disadvantages:**
- Requires developer skills
- Cannot visualize layout without deploying
- Edit → review → deploy cycle is slower than drag-and-drop

### The Budibase/NocoDB approach: Visual is the source of truth

```
Drag-and-drop ──► Auto-save ──► Publish
    │
    │ └── Layout components on canvas
    │ └── Configure data sources
    │ └── Set permissions visually
    │ └── Add automations
    └────────────────────────────┘
```

**Advantages:**
- Instant visual feedback
- Non-developers can build working apps
- Quick prototyping
- No YAML/config file knowledge needed

**Disadvantages:**
- Git is not native (export/import workaround)
- Difficult to review structural changes
- Limited flexibility for complex business logic
- Scaling limitations for large enterprise apps

---

## 5. The 80/20 Rule — Differently Applied

Both Forma and Budibase/NocoDB acknowledge that ~80% of UI is patterned and ~20% is custom. But they draw the line differently:

| | Forma | Budibase / NocoDB |
|---|---|---|
| **Patterned 80%** | YAML-defined — 12 UI kinds (Table, Form, Dashboard, Kanban, Wizard, Report, etc.) | Visual builder — drag-and-drop components (table, form, chart, map, etc.) |
| **Custom 20%** | Full React component via `asset` escape hatch | JavaScript actions + custom component plugin |
| **Who handles the 80%** | Developer writes YAML | Non-developer drags components |
| **Who handles the 20%** | Developer writes React component | Developer writes JS |
| **Reviewability** | YAML is git-diffable | Visual changes need screenshots |
| **AI readiness** | YAML is AI-native | Visual is not AI-native |

---

## 6. When to Choose Which

### Choose Forma when:
- You are a **development team** building business applications with **complex server-side logic**.
- You need **enterprise patterns** (idempotency, outbox, state machine, audit trail) enforced by the framework.
- **Git workflows** are essential — code review, CI validation, rollback.
- You want **Go** for backend logic with polyglot options.
- You need **governance** (deployment policy, artifact signing, audit).
- You are building **large-scale or compliance-sensitive** applications.

### Choose Budibase when:
- You are a **non-developer or citizen developer** building internal tools.
- You need **visual feedback** — drag and drop to see what it looks like.
- You want **quick internal tools** without a development team.
- Your business logic is **simple CRUD + automations**.
- You don't need advanced enterprise patterns.

### Choose NocoDB when:
- You need a **spreadsheet interface for an existing database**.
- You want to replace **Airtable** with a self-hosted, open-source alternative.
- Your use case is **data management with a friendly UI** rather than application building.
- You need to **connect to an existing SQL database** and provide a business-user-friendly interface.

---

## 7. Conclusion

Forma, Budibase, and NocoDB exist on a spectrum of **abstraction vs control**:

```
                     Less Control / More Abstraction
                     ─────────────────────────────────►

NocoDB:     Spreadsheet UI for databases
              ↓
Budibase:   Visual low-code platform
              ↓
Forma:      Spec-first application framework

                     ◄─────────────────────────────────
                     More Control / Less Abstraction
```

| | NocoDB | Budibase | Forma |
|---|---|---|---|
| **User** | Business user | Citizen developer | Professional developer |
| **Interface** | Spreadsheet | Drag-and-drop | YAML files |
| **Best for** | Managing data | Internal tools | Enterprise applications |
| **Complex logic** | ❌ Limited | ⚠️ Automations only | ✅ Full programming |
| **Enterprise patterns** | ❌ None | ❌ None | ✅ Built-in |
| **Git / DevOps** | ❌ Not native | ❌ Not native | ✅ Native |

**Forma is not a competitor to Budibase or NocoDB for their target users.** They serve different audiences:

- **NocoDB users** want a spreadsheet — Forma is overkill.
- **Budibase users** want to avoid coding — Forma requires coding.
- **Forma users** want structural guarantees and governance — visual tools can't provide that.

> If Budibase is **Excel with an API**, and NocoDB is **Airtable for your own database**, Forma is **PostgreSQL with compile-time guarantees** — more powerful, more rigid, and designed for professional developers building systems that need to be right.

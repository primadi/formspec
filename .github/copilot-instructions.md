# FormSpec Framework — Agent Instructions

## Project Overview

FormSpec adalah **spec-first, declarative ecosystem untuk business applications di Go**.
YAML manifest (`apiVersion`/`kind`/`metadata`/`spec`) adalah single source of truth untuk API, UI, permissions, state machines, dan events.

Tiga tipe file: `yaml` (deskripsi), `script` (Starlark logic), `asset` (static/custom UI).

---

## Architecture

- **Dua proses selalu**: `formspec-control` (control plane — governance, policy, keys) + `formspec-resource` (resource plane — business logic, API, rendering)
- **Go** untuk performance-critical logic (`native`/`compiled`)
- **Starlark** untuk sandboxed scripting (bisa di-edit via admin panel tanpa redeploy)
- **Sidecar pattern** untuk polyglot (PHP, Python, Node, Java)
- Enam `ctx.*` primitives (closed set, tidak bisa ditambah): `ctx.db`, `ctx.cache`, `ctx.lock`, `ctx.queue`, `ctx.pubsub`, `ctx.storage`

---

## Stack

| Layer | Teknologi |
|---|---|
| Backend | Go 1.26, module `github.com/primadi/formspec` |
| YAML | `gopkg.in/yaml.v3` |
| Frontend | React 19 + TypeScript 6 + Vite 8 |
| UI | shadcn/ui (Nova preset) + Tailwind CSS v4 |
| Routing | react-router-dom |
| State | zustand |
| Form | react-hook-form + zod |
| HTTP client | ky |
| Table | @tanstack/react-table |
| DnD | @dnd-kit/core + @dnd-kit/sortable |
| Icons | lucide-react |
| Toast | sonner |
| Database | PostgreSQL |
| Object Storage | MinIO |

---

## Directory Structure

```
cmd/
  formspec/               # Developer CLI
  formspec-ctl/           # Control Plane binary
  formspec-operator/      # Kubernetes Operator
internal/
  api/                 # API handlers
  auth/                # Authentication
  entity/              # Entity engine
  events/              # Event system
  manifest/            # YAML manifest loader & validator
  service/             # Service definitions
  starlark/            # Starlark runtime
  tenant/              # Tenant isolation
pkg/spec/              # Go spec types (entity.go, frontend.go, resources.go, spec.go)
renderers/
  jsonbpersist/        # PersistBackend renderer (JSONB strategy, Postgres+SQLite)
  web/                 # Frontend: React + TypeScript + Vite (shadcn-shell)
  web/src/
    App.tsx            # Root dengan BrowserRouter
    main.tsx           # Entry point
    index.css          # Tailwind + shadcn CSS variables
    renderer/          # Manifest-driven renderer engine
      kinds/           #   Renderers per Kind (Page, Form, Table, etc.)
      expr/            #   FormSpecExpr AST interpreter
      components/      #   Base component library
    api/               # Generated typed API client
    generated/         # Generated TS types dari `make generate`
    hooks/             # Custom React hooks (useRealtime, useManifest, dll)
    lib/               # Utilities (cn, dll)
    components/ui/     # shadcn/ui components (button, dll)
examples/
  Customer/
  General-Ledger/
  Inventory/
  Order-to-Cash/
  Midtrans-Payment-Gateway/
reff_docs/             # Reference docs, drafts, contoh lama
```

---

## Manifest Format (YAML)

Semua YAML manifest mengikuti format K8s-style:

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Entity               # PascalCase, huruf besar di awal
metadata:
  name: invoice            # kebab-case, unique per (kind, module)
  module: billing          # owning module
  description: "..."       # recommended untuk AI readability
  labels: {}
  annotations: {}
spec:
  # kind-specific body
```

### Resource Kinds — Backend

| Kind | Concern |
|---|---|
| `App` | Root project manifest, unit deployment |
| `Module` | Package of code + manifests |
| `Entity` | Stateful data — CRUD, state machine, events, permissions |
| `Service` | Stateless computation |
| `Config` | Module configuration |
| `Migration` | Data migration |
| `Subscription` | Event subscription |
| `Workflow` | Approval attached to state machine (Extended) |
| `Api` | API exposure override (Extended) |
| `Webhook` | Verified inbound endpoints (Extended) |
| `Environment` | Deployment target (Control Plane) |
| `Policy` | Governance rules (Control Plane) |

### Resource Kinds — Frontend

| Kind | Concern |
|---|---|
| `Page` | Route + UI composition (blocks, tabs, or full component) |
| `Form` | Input/edit layout per entity |
| `Table` | List/browse per entity |
| `Dashboard` | Widget canvas |
| `Widget` | Single dashboard widget |
| `Report` | Parameterized tabular report |
| `Wizard` | Multi-step business process |
| `Kanban` | Drag-and-drop status board |
| `Timeline` | Chronological event journal |
| `Calendar` | Calendar view for date-based entity data |
| `Listing` | Public catalog (paired with landing-page App kind) |
| `ApprovalInbox` | Pending approval task queue |
| `NotificationCenter` | In-app notification feed |
| `Print` | Printable document |
| `Theme` | Look & feel (CSS variables) |

### Entity Characteristics

| Characteristic | Sifat |
|---|---|
| `master` | Stable data — categories, products, customers |
| `transaction` | Append-heavy, time-partitioned — orders, invoices, journal entries |
| `reference` | Read-only seed data — provinces, tax rates, chart of accounts |
| `summary` | System-managed projection — no CUD via API |

---

## Frontend Architecture

- **Manifest-driven renderer**: SPA membaca UI manifests via meta API (`/api/v1/_meta/ui`) dan merender di runtime
- **Dua surfaces, satu renderer**: Admin panel (`/_admin`) derived dari Entity manifests; App UI (`/app`) via frontend kinds
- **Derived by default**: Setiap Entity auto-generate Table, Forms (create/edit), detail Page, dan Menu entries
- **Design-time layout locking**: Modal/Drawer/SeparatePage ditentukan di manifest, tidak bisa di-switch di runtime
- **Hybrid Low-Code**: ~80% patterned UI via YAML, ~20% via `asset` custom component (JS/TS)
- **FormSpecExpr**: Starlark expression subset untuk `visible_when`, `readonly_when`, `required_when`, `compute`

### shadcn/ui Components

- Semua komponen styled di `src/components/ui/`
- Import via alias `@/components/ui/...`
- Utility `cn()` di `src/lib/utils.ts` via `@/lib/utils`
- Icons via `lucide-react`

---

## Business Logic Conventions

1. **Manifest first** — selalu tulis YAML spec sebelum implementasi
2. **Business logic** di `impl/` dalam module (Go file)
3. **Gunakan `ctx.*`** untuk semua infrastruktur — jangan SQL/Raw langsung
4. **Entity defines**: `fields`, `state_machine` (states + transitions + guards), `actions`, `events`, dan `permissions`
5. **Service defines**: `inputs`, `outputs`, dan `handler` (no state)
6. **Permission = resource + action**, never hardcoded role names dalam YAML
7. **Error handling**: kembalikan error ke framework, jangan `log.Fatal` atau `os.Exit`
8. **Module granularity**: module bisa di-publish ke marketplace. Satu repo bisa punya banyak module

---

## Testing

| Type | Command |
|---|---|
| Backend unit tests | `go test ./...` atau `make test` |
| Backend verbose | `make test-verbose` |
| Frontend | `cd web && vitest` |
| YAML validation | `formspec validate` |

---

## Key Commands

```makefile
make build       # Build all Go binaries
make dev         # Run dev server (formspec-resource --dev)
make test        # Run Go tests
make generate    # Generate TypeScript types from spec -> web/src/generated
make web-dev     # Run frontend dev server (Vite)
make web-build   # Build frontend for production
make web-deps    # Install frontend npm dependencies
make lint        # Run golangci-lint
make clean       # Clean build artifacts
```

---

## Key Reference Files

| Path | Isi |
|---|---|
| `docs/spec/01-overview.md` | Visi, arsitektur, prinsip, 6 sources of inspiration |
| `docs/spec/02-core-basic.md` | Core spec — minimal implementation yang harus dipenuhi |
| `docs/spec/03-core-extended.md` | Extended kinds — Workflow, Api, Webhook |
| `docs/spec/04-control-plane.md` | Control Plane — Environment, Policy, signing, audit |
| `docs/spec/05-frontend.md` | Frontend spec — 12 UI kinds, renderer contract, FormSpecExpr |
| `docs/spec/06-plane-protocol.md` | Plane Protocol — komunikasi Control ↔ Resource |
| `docs/spec/07-marketplace.md` | Marketplace spec — pricing, metering, licensing |
| `docs/spec/10-entity-extension.md` | Entity Extension — add fields to owned entities |
| `docs/spec/11-reference.md` | Glossary & semua design decisions (D1–D48) |
| `reff_docs/FormSpec-Foundation-Document-v2.0.md` | Foundation doc — latar belakang, keputusan fundamental |
| `reff_docs/FormSpec-Technical-Note-DX-dan-Entity-Extension.md` | Technical note — DX dan entity extension |
| `pkg/spec/entity.go` | Go struct untuk Entity manifest |
| `pkg/spec/frontend.go` | Go struct untuk frontend kinds |
| `pkg/spec/spec.go` | Enum dan shared types |
| `internal/manifest/loader.go` | Manifest loader — load, parse, validate YAML |

---

## Design Principles (dari Overview & Foundation)

1. **Everything is a Resource** — `Entity` (stateful) atau `Service` (stateless)
2. **One Definition, Many Protocols** — satu YAML → API, UI, docs, types
3. **Convention over Configuration** — sensible defaults, override hanya yang perlu
4. **Security by Default** — auth mandatory, tenant isolation otomatis, cross-tenant → 404
5. **Contract Before Implementation** — YAML dulu, `impl` kedua
6. **Three File Types Only** — yaml, script, asset. Tidak ada file `.env`, route files, migration files manual
7. **Closed Primitives** — 6 `ctx.*`, tidak bisa ditambah
8. **Location Transparency** — caller tidak pernah tahu di mana resource berjalan
9. **Derived by Default** — Entity auto-generate admin panel, Table, Forms, Menu
10. **Hybrid Low-Code** — 80% YAML, 20% custom component (`asset`)

---

## Workflow Discipline

Setiap perubahan code wajib mengikuti alur kerja berikut. Urutan ini **tidak bisa dilompati**.

### 1. Plan Before Code

Sebelum menulis implementasi dari todo, **buat rencana teknis terlebih dahulu** di `docs/plan/`. Rencana harus mencakup:

- File apa saja yang akan dibuat/diubah
- Dependensi antar task
- Estimasi level of effort (small / medium / large)
- Referensi ke spec document yang relevan di `docs/spec/`

Rencana bisa berupa file Markdown baru (misal `docs/plan/entity-engine-plan.md`) atau section di file plan yang sudah ada.

### 2. Changelog

Setiap kali ada perubahan code, **catat di `docs/changelog/`**:

- Buat file dengan format `YYYY-MM-DD-NNN-<deskripsi-singkat>.md`  
  — `NNN` adalah 3-digit sequence number (001, 002, …) yang **reset setiap hari**  
  — Contoh: `2026-07-08-001-integrasi-document-model-spec.md`
- Isi: apa yang diubah, kenapa diubah, file yang terkena dampak, dan referensi ke todo/plan
- Format ringkas, maksimal 1–2 paragraf per entry
- Urutan `NNN` harus sesuai kronologi perubahan — file dengan NNN lebih kecil terjadi lebih dulu

### 3. Todo Management

File utama: `docs/plan/todo.md`.

| Aksi | Aturan |
|---|---|
| Task selesai | Tandai ✅ dan catat timestamp selesai |
| Butuh catatan | Tambahkan komentar inline di bawah task |
| Pekerjaan ditunda (deferred) | Tambahkan task baru dengan status ⏸️ dan alasan penundaan |
| Task baru ditemukan | Tambahkan ke fase yang sesuai, jangan hapus task lama |
| Update `Last Updated` | Selalu update tanggal di header todo.md |

### 4. Code → Plan Traceability

Setiap perubahan code **harus mereferensi `docs/plan/`**:

- Commit message menyebutkan plan file terkait
- PR description (jika ada) mencantumkan link ke plan
- Komentar di code (jika komplex) menyebutkan section plan

### 5. Implementation Notes

Jika selama implementasi ada keputusan teknis, trade-off, atau konteks yang perlu diingat:

- Tulis di `docs/implementation/<topik>.md`
- Jangan menimpa file yang sudah ada; tambahkan section baru atau buat file baru
- Contoh: `docs/implementation/api-layer.md`, `docs/implementation/database-layer.md`

### 6. Audit & Gap Resolution

Saat melakukan audit (membandingkan code terhadap spec atau todo):

| Langkah | Output |
|---|---|
| Jalankan audit | Simpan hasil di `docs/audit/<topik>-<tanggal>.md` |
| Identifikasi gap | List gap dengan severity (critical / major / minor) |
| Selesaikan gap | Implementasi perbaikan sesuai workflow di atas |
| Gap selesai | Update changelog, dan update dokumen terkait (spec, plan, todo) jika dibutuhkan |
| Gap tidak bisa diselesaikan | Catat sebagai deferred todo dengan alasan |

<!-- rtk-instructions v2 -->
# RTK — Token-Optimized CLI

**rtk** is a CLI proxy that filters and compresses command outputs, saving 60-90% tokens.

## Rule

Always prefix shell commands with `rtk`:

```bash
# Instead of:              Use:
git status                 rtk git status
git log -10                rtk git log -10
cargo test                 rtk cargo test
docker ps                  rtk docker ps
kubectl get pods           rtk kubectl get pods
```

## Meta commands (use directly)

```bash
rtk gain              # Token savings dashboard
rtk gain --history    # Per-command savings history
rtk discover          # Find missed rtk opportunities
rtk proxy <cmd>       # Run raw (no filtering) but track usage
```
<!-- /rtk-instructions -->
# Forma Frontend Renderer — `web/` + Meta API

**Category:** Implementation Documentation
**Status:** Backend prasyarat (Fase 4.B1–B5) ✅ selesai 2026-07-11 — UIRegistry (`internal/ui`), Meta API, sort/filter list API, static SPA serving semuanya live & teruji. Frontend renderer (Fase 4.F) belum dimulai.
**Packages:** `web/` (React renderer), `internal/ui` (Meta API + UI registry), `internal/api` (extensions)
**Implements:** Frontend Spec v0.5.0 (`docs/spec/05-frontend.md`), Admin Surfaces §4 (`docs/architecture/02-admin-surfaces.md`), D10, D17, D20, D33, D38

> Dokumen ini adalah desain komprehensif + rencana implementasi untuk **manifest-driven renderer**: frontend yang otomatis menghasilkan page, form (modal/drawer/separate page), table, sidebar menu, dashboard, dst. langsung dari spec YAML — tanpa menulis kode frontend untuk 80% kebutuhan UI bisnis.

---

## 1. Goal

Satu renderer SPA yang:

1. **Zero-manifest**: setiap `kind: Document` yang di-load otomatis mendapat list Table, create/edit Form, detail Page, dan entri sidebar Menu per module — tanpa satu pun YAML frontend (D17, benchmark PocketBase/D10). Ini surface `/_admin`.
2. **Override via kinds**: 12 UI kinds (`Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Timeline, Menu, Print, Theme`) meng-override default tersebut untuk surface `/app`.
3. **Permission-driven**: elemen UI (menu item, tombol aksi, kolom aksi) otomatis disembunyikan jika caller tidak punya permission action yang mendasarinya (§1.4 spec).
4. **Interpreted, bukan generated**: renderer membaca manifest lewat Meta API saat runtime. Tidak ada codegen UI, tidak ada build step per app.

Non-goal (fase ini): `forma/ops`, `forma/console` (closed-source, arsitektur terpisah), unmanaged client (Flutter/native).

---

## 2. Current State (audit 2026-07-11)

### 2.1 Yang sudah ada

| Aset | Lokasi | Kondisi |
|---|---|---|
| **12 UI kind structs** | `pkg/spec/frontend.go` (231 lines) | Lengkap ter-typed (`PageSpec`, `FormSpec`, `TableSpec`, `DashboardSpec`, `WidgetSpec`, `ReportSpec`, `WizardSpec`, `KanbanSpec`, `TimelineSpec`, `MenuSpec`, `PrintSpec`) — **tapi tidak pernah di-parse atau dikonsumsi** oleh kode manapun. `ThemeSpec` belum ada meski `KindTheme` terdaftar. |
| **Kind registration** | `internal/manifest/loader.go:215-218` | Semua frontend kinds ada di `KnownKinds` — YAML lolos validasi, tapi hanya sebagai `RawSpec` generik |
| **Entity schema lengkap** | `pkg/spec/entity.go` | `Field` (name, type, required, enum_values, rules, relation, child, computed), `Action`, `StateMachine`, `Characteristic`, `Expose` — semua bahan derivasi UI tersedia |
| **REST API** | `internal/api/` | Route `/{ws}/api/v1/{module}/{plural}` untuk list/find/create/update/delete + custom action `POST .../{id}/{action}`; envelope `SingleResponse`/`ListResponse`/`ErrorResponse`; permission middleware per route |
| **Auth** | `internal/auth/` | Dev mode (identity sintetis `perms: ["*"]`) + prod JWT Bearer (HS256/RS256/ES256). Stateless, tanpa cookie |
| **Contoh YAML UI nyata** | `examples/Clinic-UI-Showcase/` (dibuat sebagai fixture Fase 4 — coverage penuh 12 kinds + derived), `examples/Order-to-Cash/spec/modules/billing/{pages,tables,menus,forms,widgets}/` | Material uji utama renderer |
| **Web scaffold** | `web/` | React 19, Vite 8, TS 6, Tailwind 4, shadcn (style `base-nova`), react-router-dom 7, TanStack Table, react-hook-form + zod, dnd-kit, zustand, ky, sonner, lucide. `src/` masih kosong (satu route `<h1>Forma</h1>`) |

### 2.2 Gap backend yang menjadi prasyarat

> **Update 2026-07-11:** G1–G5 dan G7 **sudah ditutup** oleh Fase 4.B (lihat §7). Yang tersisa hanya G6 (realtime WebSocket — milik Fase 3.5). Tabel di bawah dipertahankan sebagai catatan audit awal.

| # | Gap | Bukti |
|---|---|---|
| G1 | **Meta API `/_meta/ui` belum ada** — tidak ada endpoint yang menyajikan manifest UI maupun entity schema ke browser | grep `_meta` di `internal/` kosong; `todo.md` 4.1 ⏳ |
| G2 | **Frontend kinds tidak di-parse** — tidak ada `RawSpecToPageSpec()` dkk.; tidak ada registry UI | `internal/manifest/loader.go:239-247` hanya `RawSpecToEntitySpec` |
| G3 | **Tidak ada endpoint identity/permissions** — SPA tidak bisa tahu permission efektif caller | grep `/me`, `whoami` kosong |
| G4 | **`sort` & `filters` tidak ter-wire ke HTTP** — `db.ListParams` sudah dukung `Sort` + `Filters` (eq/neq/gt/gte/lt/lte/like/in/nin), tapi `HandleList` hanya membaca `page`, `per_page`, `search` | `internal/api/handler.go:52-64` vs `internal/db/crud.go:568-591` |
| G5 | **Server tidak menyajikan static file/SPA** — hanya API + `/health` | `resource/forma.go` |
| G6 | **Realtime WebSocket belum ada** (Fase 3.5) — `realtime: true` di Table/Kanban/Dashboard/Timeline harus degradasi anggun ke polling/refetch dulu | tidak ada ws server di codebase |
| G7 | **`ThemeSpec` belum ada** di `pkg/spec/frontend.go` | grep kosong |

---

## 3. Architecture

```
                    ┌──────────────────────── Browser ─────────────────────────┐
                    │  Renderer SPA (web/)                                     │
                    │                                                          │
                    │  ┌ App Shell ─────────────────────────────────────────┐  │
                    │  │ Sidebar Menu (auto/Menu kind) · Topbar · Breadcrumb│  │
                    │  │ Router (derived + Page routes) · Overlay Manager   │  │
                    │  └────────────────────────────────────────────────────┘  │
                    │  ┌ Kind Renderers ────────────────────────────────────┐  │
                    │  │ Page Form Table Dashboard Widget Report Wizard     │  │
                    │  │ Kanban Timeline Menu Print Theme                   │  │
                    │  └────────────────────────────────────────────────────┘  │
                    │  ┌ Engines ───────────────────────────────────────────┐  │
                    │  │ Derivation Engine (Entity → default UI)            │  │
                    │  │ FormaExpr Interpreter · Permission Gate            │  │
                    │  │ forma client: api / meta / navigate / ui / theme   │  │
                    │  └────────────────────────────────────────────────────┘  │
                    └───────────────┬──────────────────────────┬───────────────┘
                                    │ GET /{ws}/api/v1/_meta/* │ /{ws}/api/v1/{module}/{plural}...
                    ┌───────────────▼──────────────────────────▼───────────────┐
                    │  forma-resource (Go)                                     │
                    │  internal/ui  ── UIRegistry + Meta API handlers (BARU)   │
                    │  internal/api ── REST CRUD + custom actions (ADA)        │
                    │  internal/entity ── EntityRegistry (ADA)                 │
                    │  static: serve web/dist dengan SPA fallback (BARU)       │
                    └──────────────────────────────────────────────────────────┘
```

Prinsip desain (dari spec, mengikat implementasi):

- **Interpreted, not generated** (§1.1) — manifest dibaca runtime; edit YAML → UI berubah tanpa build.
- **Two surfaces, one renderer** (§1.2) — `/_admin` (100% derived) dan `/app` (composed) memakai komponen renderer yang sama; `/_admin` hanyalah "app tanpa manifest UI".
- **Design-time layout locking** (§1.6) — modal vs drawer vs separate page ditentukan `Form.render` di manifest, bukan runtime.
- **Closed vocabulary** (D33) — kebutuhan UI baru tidak menambah sintaks YAML; jadi custom component (§7 spec, fase belakangan).
- **Client checks are never security** — FormaExpr & validasi client hanya UX; otoritas tetap server.

---

## 4. Backend Workstream

### 4.1 `internal/ui` — UI Registry (G2)

Package baru yang mem-parse frontend kinds dari `RawSpec` menjadi struct `pkg/spec/frontend.go` dan mengindeksnya:

```go
type Registry struct {
    pages      map[string]*spec.PageManifest      // key: metadata.name
    forms      map[string]*spec.FormManifest
    tables     map[string]*spec.TableManifest
    menus      map[string]*spec.MenuManifest
    dashboards map[string]*spec.DashboardManifest
    // ... widget, report, wizard, kanban, timeline, print, theme
}
```

Tugas:
- `RawSpecTo{Page,Form,Table,...}Spec()` di `internal/manifest` (pola sama dengan `RawSpecToEntitySpec`).
- Tambah `ThemeSpec` ke `pkg/spec/frontend.go` (G7) sesuai §10 spec (tokens, stylesheet, widgets).
- **Cross-validation saat load** (ini nilai jual `forma validate` untuk UI):
  - `Form.entity` / `Table.entity` / `Kanban.entity` / `Timeline.entity` harus entity terdaftar.
  - Setiap `Form.fields[].name` / `Table.columns[].field` harus ada di entity (dukung dot-path `customer.name` via relation).
  - Setiap `row_actions` / form `actions` harus action yang ada & tidak disabled.
  - `Page.blocks[].{form,table}.ref` harus manifest terdaftar; `route` unik per app.
  - `Kanban.columns[].value` ⊆ nilai state machine / enum `status_field`.
- Registry diisi pada `LoadEntities()` yang sama (hot-reload ikut atomic swap yang ada).

### 4.2 Meta API (G1, G3)

Endpoint read-only, same-origin, di bawah prefix workspace, ETag + `304` (manifest jarang berubah; hash snapshot deployment bisa dipakai sebagai ETag):

| Endpoint | Isi | Catatan |
|---|---|---|
| `GET /{ws}/api/v1/_meta/ui` | Bundle lengkap: semua manifest UI (per kind) **yang lolos filter permission caller** + daftar entity schema ringkas | Payload utama renderer; satu round-trip saat boot |
| `GET /{ws}/api/v1/_meta/entities/{module}/{name}` | Satu entity schema penuh (fields, rules, enum, relations, child, state machine, actions + permission string per action) | Untuk lazy-load form berat |
| `GET /{ws}/api/v1/_meta/me` | `{ user_id, workspace, roles, permissions: [...] }` — identity + permission efektif | Sumber Permission Gate client; di dev mode berisi `["*"]` |

Bentuk entity schema yang dikirim (subset `DocumentSpec`, JSON):

```jsonc
{
  "module": "billing", "name": "order", "plural": "orders",
  "characteristic": "transaction",
  "label_field": "number",              // heuristik: natural key → name → id
  "fields": [
    { "name": "customer_id", "type": "relation", "required": true,
      "relation": { "resource": "billing.customer", "type": "belongs_to" } },
    { "name": "status", "type": "enum", "enum_values": ["draft","confirmed","paid"] },
    { "name": "items", "type": "child",
      "child": { "fields": [ /* nested */ ] } },
    { "name": "total", "type": "decimal", "rules": [{ "name": "min", "value": 0 }] }
  ],
  "state_machine": { "field": "status", "initial": "draft",
    "transitions": [{ "from": ["draft"], "to": "confirmed", "via": "confirm" }] },
  "actions": [
    { "name": "confirm", "permission": "billing.orders.confirm", "has_params": false }
  ],
  "lifecycle": "two_step_autosave"       // dihitung server dari status action `submit` (§1.7 spec)
}
```

Keputusan: **derivasi UI dilakukan di client** (bagian §5.3), server hanya mengirim schema + manifest mentah. Alasan: menjaga Meta API stabil/kecil, dan logika derivasi butuh iterasi cepat di sisi renderer.

Filter permission server-side pada `/_meta/ui`: manifest yang seluruh backing action-nya tidak dimiliki caller tidak dikirim (defense in depth; client tetap punya Permission Gate sendiri untuk granularitas tombol).

### 4.3 Wire `sort` + `filter` ke `HandleList` (G4)

Query param → `db.ListParams` yang sudah ada:

```
GET /{ws}/api/v1/billing/orders?sort=-created_at&status=confirmed&total[gte]=100&page=2
```

- `sort=field` / `sort=-field` → `ListParams.Sort`.
- `{field}={v}` → `Filters[field] = {Op: eq}`; `{field}[{op}]={v}` untuk `neq|gt|gte|lt|lte|like|in|nin` (`in` = comma-separated).
- Validasi nama field terhadap entity spec → `422 VALIDATION_ERROR` jika tak dikenal (hindari probing kolom).
- Tanpa ini, `TableSpec.filters` dan `default_sort` (sudah dipakai di Order-to-Cash) tidak berfungsi.

### 4.4 Static SPA serving (G5)

- `resource/forma.go`: mount `web/dist` (via `--web-dir` flag; opsi `go:embed` menyusul) pada `/{ws}/_admin/*` dan `/{ws}/app/*` dengan **SPA fallback** (path tak dikenal → `index.html`).
- API tetap presiden di `/{ws}/api/v1/*`; tidak ada konflik karena prefix berbeda.
- Dev workflow: Vite dev server (`npm run dev`) dengan proxy `/{ws}/api` → `:8080`; produksi: file statis dari Go.

### 4.5 Yang TIDAK dikerjakan di workstream backend fase ini

- **Realtime push (G6)** — milik Fase 3.5. Renderer mendesain `realtime: true` sebagai *interface* (subscription manager) dengan implementasi awal polling/refetch-on-focus; WebSocket di-drop-in kemudian.
- **Page-scoped routing / BFF (4.7, IMP-3)** — proposal yang berkonflik dengan path REST normatif §16; renderer memakai REST API publik biasa dulu. Materialisasi permission per page (D38/IMP-4) menyusul setelah pola BFF diputuskan.
- **Export async job** (Report §5 spec) — butuh job runtime Core §17; export fase awal = CSV client-side.

---

## 5. Frontend Workstream (`web/src`)

### 5.1 Struktur project

```
web/src/
├── main.tsx / App.tsx           # bootstrap: load meta bundle → render shell
├── lib/
│   ├── api/client.ts            # ky wrapper: envelope, error, auth header, 401 handling
│   ├── api/meta.ts              # /_meta/ui, /_meta/me, /_meta/entities fetchers
│   └── formaexpr/               # lexer.ts, parser.ts, eval.ts (+ tests)
├── types/manifest.ts            # TS mirror dari pkg/spec/frontend.go + entity schema
├── stores/                      # zustand: session (me), meta (manifests), prefs
├── engine/
│   ├── derive.ts                # Entity schema → default TableSpec/FormSpec/PageSpec/Menu
│   ├── permissions.ts           # can(action) → boolean; wildcard match (port auth.go)
│   ├── lifecycle.ts             # pola §1.7: plain CRUD / 2-step / 1-step → tombol
│   └── registry.tsx             # kind → React renderer lookup
├── shell/
│   ├── AppShell.tsx             # sidebar + topbar + breadcrumb + <Outlet/>
│   ├── Sidebar.tsx              # menu tree (derived/Menu kind), permission-filtered
│   ├── router.tsx               # route table dinamis dari meta bundle
│   └── OverlayHost.tsx          # modal/drawer dikendalikan query string
├── kinds/
│   ├── page/  form/  table/  dashboard/  widget/  report/
│   ├── wizard/  kanban/  timeline/  menu/  print/  theme/
├── widgets/                     # field widgets: text, number, date, enum-select,
│                                # relation-picker, child-grid, badge, currency, ...
└── components/ui/               # shadcn primitives (button, dialog, sheet, table, ...)
```

### 5.2 Boot sequence

1. Parse URL → `{workspace}` + surface (`/_admin` atau `/app`).
2. Fetch paralel `/_meta/me` + `/_meta/ui` → isi stores. (401 → layar login token/JWT; dev mode langsung lolos.)
3. Bangun **route table**:
   - Setiap `kind: Page` → route dari `spec.route`.
   - Setiap `Wizard`/`Kanban`/`Timeline` → `/wizard/:name`, `/kanban/:name`, `/timeline/:name`.
   - **Derived routes** per entity exposed (surface `/_admin`): `/{module}/{plural}` (list), `/{module}/{plural}/:id` (detail), `/{module}/{plural}/:id/edit` & `/new` (bila form `render: separate_page`).
4. Render `AppShell` dengan sidebar + `<RouterProvider>`.

### 5.3 Derivation Engine (D17) — jantung "otomatis dari spec"

`derive.ts` mengubah entity schema menjadi manifest default in-memory — **tipe outputnya sama persis dengan manifest hasil YAML**, sehingga kind renderer tidak tahu bedanya derived vs authored:

| Derivasi | Aturan |
|---|---|
| **Table** | Kolom = field non-child, urut deklarasi, maks ~8 kolom pertama (sisanya di detail); `enum`/`doc_status` → badge; `decimal` dengan rules currency-like → format currency; `relation` → tampilkan `label_field` entity target; `datetime` → relative + tooltip absolut. `search: true` bila ada field string. Default sort `-created_at`. Row actions = view/edit/delete + custom actions (permission-gated) |
| **Form** | Section tunggal; semua field editable non-computed; widget per `FieldType` (lihat §5.6); `required`/`rules` → zod schema; `immutable` → readonly saat mode edit; `child` → child-grid; `relation` → relation-picker (search via list API target). `render`: `modal` bila ≤5 field, `drawer` bila 6–12, `separate_page` bila >12 atau punya child table (heuristik §1.6 spec) |
| **Detail Page** | Field grid readonly + child tables + **tombol transisi state machine** dari state saat ini (§ lifecycle) + audit info |
| **Menu** | Satu grup per module (label = module name), item per entity exposed dengan permission `list`, urut alfabetis. `characteristic: reference` → group "Settings"; `summary` → tidak dapat menu (hanya dipakai widget) |
| **Lifecycle buttons** | Dari `lifecycle` server (§4.2): `plain_crud` → Save saja; `two_step_autosave` → auto-save debounced + tombol Submit; `two_step_manual` → Save Draft + Submit; `one_step` → tombol create-submit. `characteristic: reference` → tanpa New/Delete (Configuration pattern §3 spec) |

Override resolution: `authored manifest (nama sama / entity sama) > derived default`. Ada di satu fungsi `resolveTable(entity)` / `resolveForm(entity, mode)` supaya presedensinya teruji unit.

### 5.4 Navigasi & kontainer (page, dialog, sidebar — inti request)

- **Sidebar Menu**: merge `kind: Menu` manifests (sorted `order`) + derived module menus untuk entity yang belum tercakup menu manapun. Item disembunyikan bila: (a) permission backing page tidak dimiliki (§1.4), (b) `when:` FormaExpr false. Nested items → collapsible group. Icon = lucide by name.
- **Modal & Drawer** (dialog): `Form.render: modal|drawer` dirender `OverlayHost` yang dikendalikan **query string** (`?action=edit&id=123&form=order-quick-edit`) sesuai §1.6 — back button menutup overlay, list di belakang tetap hidup, URL shareable. shadcn `Dialog` untuk modal, `Sheet` untuk drawer.
- **Separate page**: route sendiri + breadcrumb; back = `navigate(-1)` dengan fallback ke list.
- **Tabs Page** (§3 spec): `Page.tabs` → shadcn `Tabs`, tab aktif di query string (`?tab=general`), tiap tab lazy-render dan permission-checked mandiri.
- **Guard**: masuk route yang permission-nya tidak dimiliki → layar 403 (bukan crash); route tak dikenal → 404 shell.

### 5.5 Kind renderers (urutan prioritas)

| Kind | Fase | Catatan implementasi |
|---|---|---|
| `Table` | **F3** | TanStack Table headless + shadcn table. Server-side pagination/sort/filter/search (butuh G4). Row actions dengan `confirm_msg` → confirm dialog. Bulk actions fase belakangan |
| `Form` | **F3** | react-hook-form + zod resolver dari rules; mode create/edit/view; auto-save debounced utk lifecycle default; CAS: kirim `version`, tangani 409 → toast + refetch |
| `Page` | **F3** | Komposisi blocks (form/table/widget/html) grid `layout.columns`; varian tabs; `component:` block = placeholder "custom component belum didukung" sampai F6 |
| `Menu` | **F3** | Lihat §5.4 |
| `Dashboard`/`Widget` | **F4** | stat + chart (line/bar) via komponen chart ringan; sumber = entity summary via list API; `customizable` (user layout, dnd-kit) fase belakangan |
| `Wizard` | **F4** | Stepper; step state di `?step=N`; `depends_on` refetch dropdown; final submit = satu custom action; `allow_partial_save` menyusul (butuh draft server) |
| `Kanban` | **F4** | dnd-kit; drag = PATCH `status_field` optimistic, 409 → snap back; kolom dari manifest, validasi transisi dari state machine |
| `Timeline` | **F4** | Infinite scroll cursor `created_at`; group by date; read-only enforced |
| `Report` | **F5** | Param form → filtered list + group/totals client-side; export CSV client-side dulu (async job menyusul) |
| `Print` | **F5** | `format: html` saja dulu (client `window.print()` + `@page` CSS); pdf/thermal/dotmatrix butuh pipeline server |
| `Theme` | **F5** | Token → CSS custom properties di `:root`; stylesheet & widget skin menyusul |

### 5.6 Field widget library

Pemetaan `FieldType` → widget (dipakai derived & authored form):

`string`→Input/Textarea(rules max_length>120) · `integer`/`decimal`→NumberInput (precision-aware) · `boolean`→Switch · `enum`→Select (badge di table) · `date`/`datetime`→DatePicker (react-day-picker) · `uuid`→readonly mono · `json`→CodeEditor ringan · `relation`→RelationPicker (combobox + search, fetch list entity target, tampilkan `label_field`) · `child`→ChildGrid (inline editable rows, add/remove, sequence) · `computed`→readonly hasil FormaExpr `compute`.

Semua widget themeable (token) dan menerima `readonly`/`required`/`visible` hasil evaluasi FormaExpr.

### 5.7 FormaExpr interpreter (§6 spec)

- Subset ekspresi Starlark: literal, `fields.x` / `user.*` / param ref, perbandingan, `and/or/not`, aritmetika, `len`, `sum`, list comprehension. **Tanpa** def/loop/import/`ctx`.
- Implementasi: lexer + Pratt parser + tree-walking evaluator murni TypeScript (~300–400 LOC), tanpa dependency, tanpa `eval()`.
- Dipakai untuk: `visible_when`, `readonly_when`, `required_when`, `compute`, interpolasi `title` (`"Order {order.number}"`), `Menu.when`.
- Kontrak: gagal parse → log warning + fallback aman (visible=true, readonly=false, required=apa kata schema); `forma validate` kelak menolak di sisi CLI.
- Test suite tabel-driven — grammar ini dishare dengan guard server, jadi test case bisa dicontoh dari test Starlark Go.

### 5.8 Permission Gate (§1.4)

- Port `Identity.HasPermission` (wildcard `*`, `module.entity.*`) dari `internal/auth/auth.go` ke `engine/permissions.ts` dengan test paritas.
- `can("billing.orders.update")` dipakai: item menu, tombol row/bulk action, tombol submit form, drag Kanban, tombol transisi state.
- Ingat: ini UX saja — server tetap menolak 403; UI menangani 403 dengan toast, bukan crash.

### 5.9 API client

- `ky` instance per workspace: prefix `/{ws}/api/v1`, `Authorization: Bearer` dari session store, unwrap envelope (`data`/`meta`/`links`), map `ErrorResponse` → typed `FormaApiError` (dipakai form untuk field-level error dari `VALIDATION_ERROR.details`).
- List helper: params `{page, perPage, search, sort, filters}` → query string format §4.3.
- Mutation helper: sertakan `version` (CAS); `Idempotency-Key` header untuk custom action.
- **Subscription manager**: `subscribe(entity, cb)` — implementasi v1 = refetch on window focus + interval polling opsional; kontrak API-nya sudah bentuk final supaya WebSocket (Fase 3.5) tinggal mengganti transport.

### 5.10 Testing

| Layer | Alat | Fokus |
|---|---|---|
| FormaExpr | vitest, table-driven | grammar penuh, edge cases, rejection non-expression |
| Derivation engine | vitest | entity schema fixture (order, customer, config) → snapshot manifest derived; presedensi override |
| Permission gate | vitest | paritas dengan test Go `auth_test.go` |
| Kind renderers | testing-library + jsdom | render dari manifest fixture Order-to-Cash; interaksi (sort, filter, submit, 409, confirm dialog) |
| E2E smoke | menyusul (Playwright) | boot → sidebar → list → create via modal → submit → detail; jalan lawan `forma-resource` + Order-to-Cash |

Fixture utama: **`examples/Clinic-UI-Showcase`** — dibuat khusus untuk meng-exercise semua fitur renderer (12 kinds, semua pola lifecycle, derived-by-default, semua tipe field; lihat matriks coverage di README-nya — terverifikasi load bersih: 10 entities, 45 routes). Fixture kedua: `examples/Order-to-Cash` (Page/Table/Menu/Widget nyata dengan business logic lengkap).

---

## 6. TypeScript type strategy

- `types/manifest.ts` ditulis manual sekali, mirror `pkg/spec/frontend.go` + bentuk entity schema §4.2. Field YAML `snake_case` dipertahankan (manifest adalah wire format).
- Codegen types dari Go (Core §27, `forma generate`) adalah milik Fase 6 — jangan diblok oleh itu; kontraknya kecil dan stabil.
- Satu file → drift mudah diaudit; tambahkan test JSON round-trip terhadap fixture hasil marshal Go (golden files di `internal/ui/testdata/`).

---

## 7. Implementation Plan

> Checklist granular hidup di `docs/plan/todo.md` Fase 4. Fase di bawah berurutan namun B (backend) dan F (frontend) bisa paralel setelah F1 memakai fixture JSON statis sebelum Meta API siap.

### Fase 4.B — Backend prasyarat (Go)

| # | Task | Acceptance |
|---|---|---|
| B1 | `internal/manifest`: `RawSpecTo*Spec` untuk 12 kinds + `ThemeSpec` baru | YAML Order-to-Cash ter-parse typed, error posisi jelas |
| B2 | `internal/ui`: UIRegistry + cross-validation (entity/field/action/route/ref) | load Order-to-Cash tanpa error; manifest rusak tertolak dengan pesan berguna |
| B3 | Meta API: `/_meta/ui`, `/_meta/me`, `/_meta/entities/...` + ETag + permission filter | curl kembali bundle JSON sesuai §4.2 |
| B4 | Wire `sort`/`filters` ke `HandleList` | query §4.3 berfungsi + 422 utk field tak dikenal + tests |
| B5 | Static serving `web/dist` + SPA fallback + `--web-dir` | buka `/{ws}/_admin` menyajikan SPA |

### Fase 4.F1 — Fondasi renderer

API client + meta client, stores, types, FormaExpr interpreter, permission gate. **Milestone: unit tests hijau tanpa UI.**

### Fase 4.F2 — App shell & navigasi

AppShell, Sidebar (derived + Menu kind), router dinamis, OverlayHost (modal/drawer via query string), breadcrumb, 403/404, login token screen. **Milestone: navigasi Order-to-Cash terlihat, halaman masih placeholder.**

### Fase 4.F3 — Derived CRUD (PocketBase benchmark, D10) ⭐

Derivation engine, Table renderer, Form renderer (3 mode render, auto-save, CAS, zod), Detail page + tombol state machine, field widget library inti, lifecycle patterns §1.7. **Milestone: app dengan NOL manifest frontend mendapat `/_admin` CRUD lengkap — kriteria sukses terpenting Fase 4.**

### Fase 4.F4 — Override kinds & extended kinds

Page (blocks/tabs), Table/Form override resolution, Menu authored, Dashboard/Widget, Wizard, Kanban, Timeline. **Milestone: seluruh YAML UI Order-to-Cash ter-render sesuai manifest.**

### Fase 4.F5 — Pelengkap

Report (+CSV export), Print (html), Theme tokens, empty states, skeleton loading, download/upload tray minimal. **Milestone: spec §5, §9, §10 tercakup subset yang tak butuh server pipeline.**

### Fase 4.F6 — Escape hatch (bisa digeser)

Component contract (`mount/unmount`, injected `forma` client), `needs:` enforcement client-side, headless form engine (`forma.form`), embed-as-library (`<FormaPage/>`). Bergantung kebutuhan nyata; jangan menahan F3–F5.

### Blocked / menunggu fase lain

| Item | Menunggu |
|---|---|
| `realtime: true` transport WebSocket | Fase 3.5 (subscription manager sudah siap ganti transport) |
| Report/Table export async + download tray penuh | Job runtime (Core §17) |
| Print pdf/thermal/dotmatrix | Server rendering pipeline |
| Page-scoped BFF routing + permission materialization D38 | Keputusan arsitektur 4.7 (IMP-3 belum direkonsiliasi dengan §16) |
| Codegen TS types resmi | Fase 6.4 |

---

## 8. Open Questions

1. **Login UI prod**: JWT diterbitkan siapa? (forma/console? IdP eksternal?) Fase ini: input token manual + dev mode. Perlu keputusan sebelum GA.
2. **`/_admin` vs `/app` pemisahan**: satu bundle dengan dua entry route (rekomendasi — satu renderer, beda sumber manifest) atau dua build? Desain ini mengasumsikan satu bundle.
3. **Ikon**: spec memakai nama bebas (`icon: receipt`); kita kunci ke nama lucide + fallback default.
4. **`label_field` heuristik** (natural key → `name` → `id`) — perlu diangkat ke spec sebagai field eksplisit di Document? Diusulkan ke spec sebagai `display_field`.
5. **i18n**: label derived dari nama field (`created_at` → "Created At") — cukup title-case dulu; strategi i18n belum di-spec.

---

*Last updated: 2026-07-11*

# 2026-08-10-007-kind-docs-narrative-and-examples

**Lapis**: Docs (`docs/kind/` + `pkg/spec`)
**Referensi**: `docs/plan/kind-reference-docs.md` §P2, §P3

## Perubahan

### P2 — Narrative semua 33 kind

Isi section narasi (`Kapan Memakai`, `Contoh Manifest`, `Gotchas`) untuk
seluruh 33 file `docs/kind/`, lengkap dengan cross-reference ke `docs/spec/`
dan `ai_skills/formspec-kinds`. Contoh YAML mengikuti pola yang sudah terbukti di
`examples/` (arisan, clinic) dan kontrak `docs/spec/`.

### P3 — Enrichment `@schema {example}` di `pkg/spec`

Tambah annotation `// @schema {example: ...}` di field utama spec struct untuk
memperkaya kolom "Contoh" di tabel atribut:

- `resources.go`: Service, Module, App, Renderer, PersistBackend, Config, Subscription, Migration, Workflow, Api, Webhook, Integrator, Mockup, KindDefinition
- `entity.go`: EntitySpec (`version`, `plural`, `lifecycle`, `display_field`)
- `frontend.go`: Page, Form, Table, Dashboard, Widget, Report, Wizard, Kanban, Print, Timeline, Calendar, Listing
- `control.go`: Environment
- `datastore.go`: Datastore
- `spec.go`: Characteristic, ImplType

JSON Schema (`schemas/`) ikut mendapat `example` di properti terkait (bukan
hanya docs/kind) — dua output sinkron dari satu sumber.

## File terdampak

- `docs/kind/**` (narrative + tabel diperkaya)
- `pkg/spec/{resources,entity,frontend,control,datastore,spec}.go` (annotations)

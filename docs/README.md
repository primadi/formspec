# Dokumentasi Forma

Forma adalah platform spec-driven: aplikasi dideklarasikan sebagai kumpulan spec
YAML (workspace → app → module → kinds), lalu diinterpretasikan saat runtime oleh
engine dan renderer resmi. Prinsip yang mengikat seluruh dokumentasi ini:

> **Spec adalah kontrak; renderer adalah implementasi kontrak itu.**

Karena itu dokumentasi terbagi dua poros utama — [`spec/`](spec/README.md)
(kontrak, normatif) dan [`renderers/`](renderers/README.md) (implementasi resmi,
deskriptif) — ditambah section pendukung.

## Peta Dokumen

| Section | Isi | Sifat |
|---|---|---|
| [`spec/platform/`](spec/platform/) | Kontrak lintas sisi: overview, workspace/app/module, kind system, control plane, plane protocol, datastore, marketplace | Normatif, semver |
| [`spec/backend/`](spec/backend/) | Kontrak data & perilaku: core basic/extended, entity extension, **PersistBackend** | Normatif, semver |
| [`spec/frontend/`](spec/frontend/) | Kontrak visual: hirarki Shell→App→Page→Component, **VisualSpecKind**, **Renderer**, **Spec Resolution API**, katalog kind per tier, FormaExpr | Normatif, semver |
| [`renderers/shadcn-shell/`](renderers/shadcn-shell/) | Frontend renderer resmi (React + shadcn/ui) | Deskriptif, dated |
| [`renderers/jsonb-persist/`](renderers/jsonb-persist/) | Backend renderer resmi (PersistBackend hybrid JSONB, Postgres/SQLite) | Deskriptif, dated |
| [`architecture/`](architecture/) | Topologi deployment, HA/failover, K8s operator, admin surfaces, struktur repo | Deskriptif |
| [`runtimes/`](runtimes/) | Internals per komponen runtime: forma-ctl, forma-resource, forma-operator, forma-sidecar, engine API layer | Deskriptif |
| [`cli-tools/`](cli-tools/) | Referensi CLI: forma, forma-ctl, forma dev, forma generate, forma consult | Deskriptif |
| [`ai/`](ai/README.md) | Forma AI: `forma-consult`, `forma-local-mcp`/`forma-remote-mcp`, LLM provider layer (Vercel AI SDK, BYOK), Forma Skill | Deskriptif — design, belum diimplementasikan |
| [`guides/`](guides/) | Cara menjalankan, tutorial Order-to-Cash, panduan menulis renderer/shell/persist-backend | Tutorial |
| [`reference/`](reference/) | Glossary istilah kanonik | Referensi |
| [`comparison/`](comparison/) | Forma dibandingkan platform lain | Referensi |

## Jalur Baca per Persona

**App developer** (membangun aplikasi di atas Forma):
`spec/platform/01-overview` → `spec/platform/02-workspace-app-module` →
`spec/backend/01-core-basic` → `spec/frontend/06-page-kinds` →
`guides/order-to-cash-tutorial`.

**Renderer/Shell author** (menambah renderer visual atau persist backend baru):
`spec/frontend/01-visual-hierarchy` → `02-visual-spec-kind` → `03-renderer-kind` →
`04-spec-resolution-api` → `guides/authoring-a-page-renderer` (atau
`spec/backend/04-persist-backend` → `guides/authoring-a-persist-backend`).

**Platform operator** (menjalankan Forma untuk banyak workspace):
`architecture/01-architecture-overview` → `runtimes/` → `cli-tools/04-forma-ctl`.

**Framework contributor** (mengubah kode Forma sendiri — `cmd/`, `internal/`,
`pkg/`, `web/`, `sdk/`): `architecture/08-repo-structure` → dokumen `spec/`
atau `renderers/` yang relevan dengan area yang disentuh.

## Konvensi

- Direktori tanpa nomor; file berprefix dua digit sesuai urutan baca; `README.md`
  per direktori adalah indeksnya.
- Dokumen kontrak (`spec/`) memakai header `Version` (semver) + `Status`
  (Outline → Draft → Final). Dokumen implementasi (`renderers/`, `runtimes/`)
  memakai `Updated:` (tanggal).
- Bahasa dokumen: Indonesia; nama kind, field, dan istilah teknis tetap English.
- Seluruh isi ditulis present-tense — dokumen ini menjelaskan Forma sebagaimana
  adanya, bukan sejarah perubahannya.

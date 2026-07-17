# Spec Backend — Kontrak Data & Perilaku

Kontrak yang wajib dipenuhi engine dan **PersistBackend** manapun: model dokumen,
action, lifecycle, extension, dan interface penyimpanan. Seluruh kontrak di sini
storage-agnostic — detail bagaimana backend resmi (hybrid JSONB, jalan di atas
Postgres maupun SQLite) memenuhinya ada di
[`../../renderers/jsonb-persist/`](../../renderers/jsonb-persist/).

| Dokumen | Cakupan |
|---|---|
| [01-core-basic.md](01-core-basic.md) | Document/Entity, field, relasi, action, event, natural key, migration sebagai kontrak structural-diff, config & global settings |
| [02-core-extended.md](02-core-extended.md) | Lifecycle, workflow (multi-approver), subscription, webhook, integrator, `kind: Api` (override permukaan external), async action & job tracking, validation levels 4-6, hook spec, query builder, rate limiting, `ctx.secrets`, period closing & backdating, archiving & retention, audit trail bisnis |
| [03-entity-extension.md](03-entity-extension.md) | Extension entity oleh module lain; kontrak uninstall bersih |
| [04-persist-backend.md](04-persist-backend.md) | Interface PersistBackend — seam penyimpanan setara Shell |
| [05-field-types.md](05-field-types.md) | Katalog tipe field normatif, tipe `money`, kosakata validasi, dukungan tree/hierarki |
| [06-script-runtime.md](06-script-runtime.md) | API penulisan handler script — entrypoint `execute`, objek `resource`, query dari script, akses lintas-entity, kontrak return `ok`/`fail`, resolusi `ref` native |
| [error-glossary.yaml](error-glossary.yaml) | Kode error kanonik `FORMA.*` |

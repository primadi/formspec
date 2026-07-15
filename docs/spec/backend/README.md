# Spec Backend — Kontrak Data & Perilaku

Kontrak yang wajib dipenuhi engine dan **PersistBackend** manapun: model dokumen,
action, lifecycle, extension, dan interface penyimpanan. Seluruh kontrak di sini
storage-agnostic — detail bagaimana backend resmi (Postgres hybrid JSONB)
memenuhinya ada di [`../../renderers/persist-postgres/`](../../renderers/persist-postgres/).

| Dokumen | Cakupan |
|---|---|
| [01-core-basic.md](01-core-basic.md) | Document/Entity, field, relasi, action, event, natural key, migration sebagai kontrak structural-diff |
| [02-core-extended.md](02-core-extended.md) | Lifecycle, workflow, subscription, webhook, integrator |
| [03-entity-extension.md](03-entity-extension.md) | Extension entity oleh module lain; kontrak uninstall bersih |
| [04-persist-backend.md](04-persist-backend.md) | Interface PersistBackend — seam penyimpanan setara Shell |
| [error-glossary.yaml](error-glossary.yaml) | Kode error kanonik `FORMA.*` |

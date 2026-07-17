# jsonb-persist — Backend Renderer Resmi

**Updated:** 2026-07-15 · Status: Outline

PersistBackend resmi Forma: strategi hybrid JSONB, jalan di atas Postgres
maupun SQLite (nama renderer ini soal *strategi skema*, bukan engine SQL —
lihat [02-schema-strategies.md](02-schema-strategies.md)). Memenuhi kontrak
[`spec/backend/04-persist-backend.md`](../../spec/backend/04-persist-backend.md).
Kode: `internal/db/`, `internal/datastore/`.

| Dokumen | Cakupan |
|---|---|
| [01-architecture.md](01-architecture.md) | Bagaimana hybrid JSONB menjawab kontrak PersistBackend |
| [02-schema-strategies.md](02-schema-strategies.md) | Strategi skema pluggable: hybrid JSONB vs fully-relational; DDL table/field/index |
| [03-migration-engine.md](03-migration-engine.md) | Penerjemahan structural diff → SQL |
| [04-query-and-keys.md](04-query-and-keys.md) | Translasi filter operator, natural-key counter, idempotency, dialek `ctx.db` |

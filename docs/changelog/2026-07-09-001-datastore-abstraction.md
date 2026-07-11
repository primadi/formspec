# 2026-07-09-001 — Two-Level Named Datastore Abstraction

**Apa:** Memperkenalkan `kind: Datastore` di Control Plane sebagai abstraksi infrastruktur backend.
Semua definisi datastore berasal dari Control Plane; Resource Plane hanya menerima
datastore yang diotorisasi via Plane Protocol snapshot.

**Kenapa:** Sebelumnya developer langsung terekspos ke teknologi backend (SQLite/Postgres,
Redis/Valkey) melalui `ctx.db`, `ctx.cache`, dll. Dengan named datastore, developer cukup
mereferensi nama (`'primary-db'`, `'session-cache'`) tanpa perlu tahu backend apa.

**File yang terdampak:**

| File | Tipe | Keterangan |
|---|---|---|
| `docs/spec/00-kind-plane-mapping.md` | NEW | Pemetaan kanonik kind → plane |
| `docs/spec/12-datastore.md` | NEW | Spec lengkap kind: Datastore |
| `pkg/spec/datastore.go` | NEW | Go types: DatastoreSpec, AccessSpec, PermissionSpec |
| `internal/datastore/registry.go` | NEW | Registry: RegisterNamed, Resolve, ResolveDefault |
| `internal/datastore/factory.go` | NEW | ConnectionFactory + 9 driver implementasi |
| `internal/datastore/resolver.go` | NEW | Convenience resolver: DB(), DBNamed(), dll |
| `internal/datastore/connection.go` | NEW | ConnectionPool wrapper |
| `internal/datastore/filter.go` | NEW | FilterMatch, PermissionCheck, matchScope |
| `internal/datastore/filter_test.go` | NEW | 12 test untuk filter & permission |
| `internal/datastore/registry_test.go` | NEW | 7 test untuk registry |
| `internal/starlark/primitive.go` | NEW | primitiveHandle + .named() support |
| `pkg/spec/spec.go` | EDIT | +KindDatastore di Control Plane kinds & IsValidKind |
| `pkg/spec/entity.go` | EDIT | +Datastores field di UsesDecl |
| `internal/starlark/context.go` | EDIT | +7 primitive attributes, +SetDatastoreResolver |
| `internal/manifest/loader.go` | EDIT | +Datastore di KnownKinds |
| `docs/spec/README.md` | EDIT | +00-kind-plane-mapping, +12-datastore di diagram & reading path |
| `docs/spec/01-overview.md` | EDIT | +Datastore di §8 Resource Kinds table |
| `docs/spec/04-control-plane.md` | EDIT | +Datastore di §2 Kinds table |

**Design decisions:** D51–D59 (lihat `docs/spec/12-datastore.md` §7 dan `docs/spec/11-reference.md`)

**Referensi:** `docs/plan/todo.md` — Phase 0–7 selesai, Phase 8–10 deferred.

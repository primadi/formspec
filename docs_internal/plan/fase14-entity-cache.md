# Plan — Fase 14: Framework-Level Entity Cache (Read-Through)

**Status**: planned · **Keputusan desain** (disepakati 2026-08-29):
(1) opt-in eksplisit per entity, (2) list caching skip, (3) invalidasi
multi-instance v1 (in-process + TTL) dan v2 (Redis pub/sub broadcast),
(4) Fase terpisah — cache berlaku umum, bukan registry saja.

## Masalah

`HandleFind` selalu `store.GetByID` ke DB. Registry (portal katalog, CLI
lookup) read-heavy; pola yang sama berlaku untuk entity master/reference di
app mana pun. `ctx.cache()` ada tapi hanya bisa dipakai manual dari script —
generic CRUD tidak melewati cache.

## Desain

### 1. Lingkup v1: read-through di `GetByID` saja

```
HandleFind → cache.Get(key) → hit: record mentah → sanitize per-user → response
                            → miss: store.GetByID → cache.Set(TTL) → ↑
HandleCreate/Update/Delete → DB write (CAS tetap) → cache.Delete(key)
```

- List **tidak** di-cache (kombinasi filter arbitrer → salah-invalidasi).
- Cache menyimpan **record mentah** (`db.EntityRecord`, JSON) — `sanitize()`
  (field security) tetap jalan per-request, tidak pernah di-cache.
- Write path tidak berubah — CAS version check tetap ke DB.

### 2. Konfigurasi: opt-in eksplisit per entity

```yaml
spec:
  cache:
    ttl: 300s # absent = off (default)
```

- `pkg/spec`: `CacheSpec{ TTL string }` + field `Cache *CacheSpec` di
  `EntitySpec` + validasi (parse duration, min 1s, max 1h).
- Tidak ada auto-on by characteristic — correctness by default.

### 3. Key & backend

- Key: `{workspace}:{module}:{entity}:id:{id}` — tenant di dalam key,
  cross-tenant 404 semantics tetap.
- Backend: interface yang sama dengan primitive KV (KVGetter/KVSetter/
  KVDeleter). Resolver seam di HandlerFactory: module bound ke datastore
  `serves: [cache]` → backend itu (Redis); tidak bound → shared in-memory.

### 4. Invalidasi

| Tingkat | Mekanisme                                                                                                  | Cakupan                                                          |
| ------- | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| v1      | delete key di create/update/delete (in-process) + TTL                                                      | single-instance konsisten; multi-instance staleness ≤ TTL        |
| v2      | Redis pub/sub channel `formspec:cache:invalidate` — mutator publish key, semua instance subscribe & delete | multi-instance konsisten penuh; aktif hanya saat backend rediskv |

v2 memakai klien Redis yang sudah ada di `rediskv` — tanpa outbox subscriber
baru. `NotifyMutation` (SSE) tidak disentuh.

## Implementasi

| File                                              | Isi                                                                                 |
| ------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `pkg/spec/entity.go`                              | `CacheSpec` + validasi                                                              |
| `internal/api/entitycache.go`                     | key builder, read-through helper, invalidasi, pub/sub broadcast (v2)                |
| `internal/api/handler.go`                         | wiring HandleFind + HandleCreate/Update/Delete                                      |
| `internal/api/router.go` / `resource/formspec.go` | resolver seam backend KV                                                            |
| `renderers/jsonb-persist/datastore/rediskv/`      | pub/sub invalidate helper                                                           |
| Tests                                             | hit/miss/TTL, invalidasi, sanitize-not-cached, CAS unaffected, dua-instance (Redis) |

## Adoptasi & Docs

- Registry: `cache: {ttl: 300s}` di entity `module`, `vendor`, `module-version`.
- Docs: `docs/spec/backend/01-core-basic.md` (section baru §cache) +
  `docs/registry/01-concepts.md` (catatan performa).

## Out of scope

- List/query caching, cache warming, negative caching (404), LRU local layer.

# 2026-08-29-003 — Fase 14: framework-level entity cache (read-through find-by-id)

## Apa yang diubah

Cache read-through di level framework (bukan registry-specific) — keputusan
desain disepakati: opt-in eksplisit per entity, list tidak di-cache,
invalidasi v1 (in-process) + v2 (Redis pub/sub broadcast), Fase terpisah.
Plan: `docs/plan/fase14-entity-cache.md`.

- **14.1 Spec** — `pkg/spec/entity.go`: `CacheSpec{ttl}` (1s–1h, parse
  duration) + field `Cache *EntitySpec.Cache` + validasi di
  `ValidateEntitySpec`; `CacheSpec` masuk shared defs generator schema.
- **14.2 Cache layer** — `internal/api/entitycache.go`: `CacheKV` (kontrak
  sama dengan primitive KV — memory.KV/rediskv.KV plug-in langsung), key
  `{ws}:{module}:{entity}:id:{id}`, encoding via struct `cachedRecord`
  eksplisit (EntityRecord punya MarshalJSON flat TANPA unmarshal kebalikan —
  round-trip via struct polos), read-through di `HandleFind` (cache menyimpan
  record MENTAH; sanitize tetap per-request), invalidasi di update/delete/
  submit/deactivate-reactivate/workflow-transition.
- **14.3 Backend resolver** — `resource/formspec.go`: module bound ke
  datastore `serves: [cache]` → backend itu (Redis); tidak bound → shared
  in-memory KV. `RouterBuilder.SetEntityCache` + `HandlerFactory.SetEntityCache`.
- **14.4 Multi-instance (v2)** — `rediskv.KV`: subscription goroutine ke
  channel `formspec:cache:invalidate` (receive → delete lokal, tanpa
  re-broadcast → tanpa loop) + `BroadcastInvalidate` (delete + publish);
  `api.CacheInvalidator` optional interface — backend tanpa broadcast
  fallback ke local delete (staleness ≤ TTL).
- **14.5 Tests** — 6 test api (hit/miss, invalidasi update/delete, non-opted
  tidak ter-cache, tenant key isolation, TTL validation) + 2 test rediskv
  (set/get/ttl/delete + broadcast dua-instance terhadap Valkey nyata).
- **14.6 Adoptasi + docs** — `cache: {ttl: 300s}` di entity registry
  `module`/`vendor`/`module-version`; docs normatif
  `docs/spec/backend/01-core-basic.md` §10 baru; catatan performa di
  `docs/registry/01-concepts.md`.

## Verifikasi

- `go build ./...` hijau; test api+spec 153 pass; rediskv 2 pass (Valkey
  dev container nyata — termasuk skenario dua instance).
- E2E smoke: find1 populate → DB diubah langsung → find2 masih nilai lama
  (dari cache) → update via API → find3 fresh (invalidasi bekerja).
- `formspec validate --spec registry/spec --schema schemas` — 11 manifest
  0 problem. Catatan: `validate` default memakai schema dari cache registry
  online (`~/.cache/formspec/schemas/v1`) — schema lokal baru perlu
  di-publish dulu (`make publish-schemas`) agar validasi default lolos.

## Pelajaran (bug yang hampir terjadi)

- Edit multi-titik di handler.go dengan anchor whitespace-longgar sempat
  menghapus baris `if rec != nil {` / `RunAfterPhase` dan menimpa variabel
  `actionName` — tertangkap oleh `git diff` review sebelum commit; dipulihkan
  via python byte-exact replace. Pelajaran: untuk edit berulang di file besar,
  verifikasi diff per titik, jangan andalkan flexible-whitespace match.

## Referensi

- Plan: `docs/plan/fase14-entity-cache.md` · todo Fase 14
- Keputusan user: opt-in eksplisit, list skip, v1+v2, Fase terpisah

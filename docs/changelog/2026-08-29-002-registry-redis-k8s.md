# 2026-08-29-002 — Plan C batch 2: driver Redis/Valkey ctx.cache + deploy K8s

## Apa yang diubah

Batch kedua Plan C (todo 13.5.6):

- **Backend Redis KV** — `renderers/jsonb-persist/datastore/rediskv/redis.go`:
  `KV` (Get/Set/Delete) mengimplementasikan kontrak KVGetter/KVSetter/
  KVDeleter starlark dengan semantik memory.KV (nilai JSON-encoded, miss =
  `(nil, nil)`, TTL 0 = tanpa expiry). Namespace key `formspec:<datastore>:`
  untuk isolasi antar app/datastore.
- **Resolver cloud driver** — `resource/datastoreregistry.go`: driver
  `valkey`/`redis` kini **resolve** untuk primitive `cache`/`kvstore`
  (sebelumnya fail loudly). DB/lock/queue/pubsub di Redis = backend terpisah,
  belum di-wire. Driver s3/minio/nats tetap unsupported.
- **Deploy artifacts** — `registry/deploy/`: `Dockerfile` (multi-stage →
  distroless/static non-root, spec ter-embed), `k8s/deployment.yaml`
  (Secret + Deployment **3 replica** rolling maxUnavailable 0 + probes
  `/health` + Service + Ingress TLS `registry.formspec.dev` body-size 64m),
  `k8s/datastore-valkey.yaml` (kind: Datastore valkey serves cache/kvstore),
  `README.md` (arsitektur + catatan multi-instance).

## Verifikasi

- `go build ./...` hijau; `go test ./resource/... ./renderers/jsonb-persist/
datastore/...` — **89 pass** (termasuk test integrasi baru `rediskv`:
  set/get roundtrip, TTL expiry, delete — terhadap Valkey dev container
  `valkey:6379`, skip otomatis tanpa server, pola `stream/redis_test.go`).
- Test lama `TestDatastoreRegistry_UnsupportedDriver` diupdate: valkey kini
  resolve (error = koneksi, bukan unsupported).
- `formspec validate --spec registry/spec` — 11 manifest, 0 problem.

## Catatan

- Cache-aside wiring di module registry (baca metadata via ctx.cache +
  invalidasi saat publish) menyusul — saat ini driver siap, pemakaian masih
  manual via script.
- Rate limiter per-IP masih in-memory per-pod (enforcement per-instance);
  shared limiter menyusul.

## Referensi

- `registry/deploy/README.md` — arsitektur deploy
- Plan: session plan "FormSpec Registry — 3 Plan Terpisah" (Plan C batch 2)

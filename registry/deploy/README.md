# Registry Deploy — K8s (vanilla)

Production deployment artifacts untuk `cmd/formspec-registry` (Plan C batch 2).

## Isi

| File                                                     | Peran                                                                                                                                                                      |
| -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [`Dockerfile`](Dockerfile)                               | Multi-stage build → distroless/static, non-root. Spec ter-embed di binary.                                                                                                 |
| [`k8s/deployment.yaml`](k8s/deployment.yaml)             | Secret (dev HMAC) + Deployment **3 replica** (rolling, maxUnavailable 0) + probes `/health` + Service + Ingress TLS (`registry.formspec.dev`, body-size 64m untuk tarball) |
| [`k8s/datastore-valkey.yaml`](k8s/datastore-valkey.yaml) | `kind: Datastore` driver valkey `serves: [cache, kvstore]` — backend `ctx.cache`                                                                                           |

## Deploy

```bash
docker build -t formspec-registry:latest -f registry/deploy/Dockerfile .
kubectl create namespace formspec
# JWT keys (production: asymmetric RS256/ES256 — HMAC hanya dev):
kubectl create secret generic registry-jwt-keys -n formspec \
  --from-file=jwt.pub=keys/jwt.pub
# Datastore valkey: gabungkan ke spec tree (lihat komentar file) atau embed
# sebelum build image.
kubectl apply -f registry/deploy/k8s/deployment.yaml
```

## Arsitektur

```
Ingress (TLS, registry.formspec.dev)
  └─ Service :80
       └─ Deployment ×3 (stateless, probes /health)
            ├─ Postgres (state: entities, versions, sessions) — HA per platform/10 §6
            ├─ Valkey  (ctx.cache: MRU module metadata, namespace formspec:<ds>:)
            └─ Object storage (tarball immutable — CDN-friendly)
```

## Catatan

- **Multi-instance**: session state di Postgres (entity-backed) — aman untuk
  replika ganda. Rate limiter per-IP masih in-memory per-pod (enforcement
  per-instance); shared limiter menyusul.
- **Metrics**: `:9102/metrics` per pod (Prometheus scrape via Service port metrics).
- **Cache invalidasi**: publish versi baru menulis ke Postgres; cache TTL
  menutup window staleness (invalidasi aktif menyusul bersama cache-aside
  wiring di module registry).

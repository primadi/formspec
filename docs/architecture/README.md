# Forma Architecture

**Status:** Draft
**License:** Creative Commons CC0

> Dokumen arsitektur menjelaskan **bagaimana** Forma berjalan di production — topology deployment, resource registration, failover, K8s operator, dan admin surfaces. Dokumen ini bersifat **deskriptif/penjelasan**, bukan normatif. Spec normatif tetap di `docs/spec/`.
>
> Untuk fitur, desain internal, dan API tiap komponen runtime secara individual (Forma Control, Forma Resource, Forma Operator, Forma Sidecar), lihat **[`docs/runtimes/`](../runtimes/README.md)**. Untuk referensi CLI (`forma`, `forma-ctl`), lihat **[`docs/cli-tools/`](../cli-tools/README.md)**.

---

## Document Map

```
01-architecture-overview.md   ← Mulai dari sini
02-admin-surfaces.md          ← Tiga admin UI + pemiliknya
03-deployment-flow.md         ← Pipeline: forma apply → deploy → run
04-resource-registration.md   ← Server, DB, Valkey lifecycle
05-failover.md                ← HA, auto-failover, recovery
06-k8s-operator.md            ← Forma Operator, CRD, ClusterClass
```

---

## Reading Paths

### 🏗️ App Developer

| Order | Document |
|---|---|
| 1 | [`01-architecture-overview.md`](./01-architecture-overview.md) — Where does my app run? |
| 2 | [`03-deployment-flow.md`](./03-deployment-flow.md) — How does `forma apply` work in production? |

### 🖥️ Platform Operator

| Order | Document |
|---|---|
| 1 | [`01-architecture-overview.md`](./01-architecture-overview.md) — Full topology |
| 2 | [`04-resource-registration.md`](./04-resource-registration.md) — Server, DB, Valkey registration |
| 3 | [`05-failover.md`](./05-failover.md) — HA and auto-failover |
| 4 | [`06-k8s-operator.md`](./06-k8s-operator.md) — Forma Operator + ClusterClass |

### 👤 Workspace Owner

| Order | Document |
|---|---|
| 1 | [`01-architecture-overview.md`](./01-architecture-overview.md) — Regions, ClusterClass, tiers |
| 2 | [`02-admin-surfaces.md`](./02-admin-surfaces.md) — forma/console features |

---

## Architecture Decisions Index

Semua keputusan desain arsitektur tercatat sebagai D-ARCH-1 sampai D-ARCH-31 di [`01-architecture-overview.md`](./01-architecture-overview.md#12-architecture-decisions). Component inventory ada di [§2](./01-architecture-overview.md#2-component-inventory). Deployment model (satu pipeline, generic image) ada di [§3](./01-architecture-overview.md#3-deployment-model--satu-pipeline-generic-image).

---

## Relationship to Spec Documents

| Architecture Doc | Related Spec |
|---|---|
| `01-architecture-overview.md` | `01-overview.md` §5–§6, `04-control-plane.md` |
| `02-admin-surfaces.md` | `05-frontend.md` §1.2, `Forma-Technical-Note-Katalog-Aplikasi.md` |
| `03-deployment-flow.md` | `06-plane-protocol.md` §0 |
| `04-resource-registration.md` | `12-datastore.md`, `04-control-plane.md` |
| `05-failover.md` | `06-plane-protocol.md` §1 |
| `06-k8s-operator.md` | `00-kind-plane-mapping.md` |

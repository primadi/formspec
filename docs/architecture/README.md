# FormSpec Architecture

**Status:** Draft
**License:** Creative Commons CC0

> Dokumen arsitektur menjelaskan **bagaimana** FormSpec berjalan di production — topology deployment, resource registration, failover, K8s operator, dan admin surfaces. Dokumen ini bersifat **deskriptif/penjelasan**, bukan normatif. Spec normatif tetap di `docs/spec/`.
>
> Untuk fitur, desain internal, dan API tiap komponen runtime secara individual (FormSpec Control, FormSpec Resource, FormSpec Operator, FormSpec Sidecar), lihat **[`docs/runtimes/`](../runtimes/README.md)**. Untuk referensi CLI (`formspec`, `formspec-ctl`), lihat **[`docs/cli-tools/`](../cli-tools/README.md)**.

---

## Document Map

```
01-architecture-overview.md   ← Mulai dari sini
02-admin-surfaces.md          ← Tiga admin UI + pemiliknya
03-deployment-flow.md         ← Pipeline: formspec apply → deploy → run
04-resource-registration.md   ← Server, DB, Valkey lifecycle
05-failover.md                ← HA, auto-failover, recovery
06-k8s-operator.md            ← FormSpec Operator, CRD, ClusterClass
07-vertical-modules.md        ← ERP module division: verticals/, App/Workspace composition, branch model
08-repo-structure.md          ← Struktur folder repo FormSpec, lensa spec-vs-renderer (untuk kontributor codebase)
09-domain-map.md              ← Peta subdomain formspec.dev, DNS, hosting, email
```

---

## Reading Paths

### 🏗️ App Developer

| Order | Document                                                                                           |
| ----- | -------------------------------------------------------------------------------------------------- |
| 1     | [`01-architecture-overview.md`](./01-architecture-overview.md) — Where does my app run?            |
| 2     | [`03-deployment-flow.md`](./03-deployment-flow.md) — How does `formspec apply` work in production? |

### 🖥️ Platform Operator

| Order | Document                                                                                         |
| ----- | ------------------------------------------------------------------------------------------------ |
| 1     | [`01-architecture-overview.md`](./01-architecture-overview.md) — Full topology                   |
| 2     | [`04-resource-registration.md`](./04-resource-registration.md) — Server, DB, Valkey registration |
| 3     | [`05-failover.md`](./05-failover.md) — HA and auto-failover                                      |
| 4     | [`06-k8s-operator.md`](./06-k8s-operator.md) — FormSpec Operator + ClusterClass                  |

### 👤 Workspace Owner

| Order | Document                                                                                      |
| ----- | --------------------------------------------------------------------------------------------- |
| 1     | [`01-architecture-overview.md`](./01-architecture-overview.md) — Regions, ClusterClass, tiers |
| 2     | [`02-admin-surfaces.md`](./02-admin-surfaces.md) — formspec/console features                  |

### 🏢 ERP Vertical Author

| Order | Document                                                                                                                            |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------- |
| 1     | [`07-vertical-modules.md`](./07-vertical-modules.md) — Module taxonomy, App/Workspace composition, branch model, ERPNext comparison |

### 🧑‍💻 Framework Contributor

| Order | Document                                                                                                                             |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | [`08-repo-structure.md`](./08-repo-structure.md) — Peta folder repo, pemetaan kode ↔ dokumen spec/renderer, kesenjangan implementasi |

---

## Architecture Decisions Index

Semua keputusan desain arsitektur tercatat sebagai D-ARCH-1 sampai D-ARCH-31 di [`01-architecture-overview.md`](./01-architecture-overview.md#12-architecture-decisions). Component inventory ada di [§2](./01-architecture-overview.md#2-component-inventory). Deployment model (satu pipeline, generic image) ada di [§3](./01-architecture-overview.md#3-deployment-model--satu-pipeline-generic-image).

---

## Relationship to Spec Documents

| Architecture Doc              | Related Spec                                                                                                                                                  |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `01-architecture-overview.md` | `01-overview.md` §5–§6, `04-control-plane.md`                                                                                                                 |
| `02-admin-surfaces.md`        | `spec/frontend/01-visual-hierarchy.md` §1                                                                                                                     |
| `03-deployment-flow.md`       | `spec/platform/05-plane-protocol.md` §0                                                                                                                       |
| `04-resource-registration.md` | `spec/platform/06-datastore.md`, `04-control-plane.md`                                                                                                        |
| `05-failover.md`              | `spec/platform/05-plane-protocol.md` §1                                                                                                                       |
| `06-k8s-operator.md`          | `spec/platform/03-kind-system.md`                                                                                                                             |
| `07-vertical-modules.md`      | `spec/platform/02-workspace-app-module.md` §1/§3, `spec/backend/01-core-basic.md` §5, `docs/comparison/formspec-vs-frappe.md`                                 |
| `08-repo-structure.md`        | [`docs/spec/README.md`](../spec/README.md) (prinsip contract-vs-renderer), [`docs/spec/platform/08-project-layout.md`](../spec/platform/08-project-layout.md) |

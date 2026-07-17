# Forma Operator — Binary Reference

**Version:** 1.0
**Status:** Draft (belum diimplementasikan — lihat §7)
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri **Closed Source**
**Governed by:** `docs/architecture/06-k8s-operator.md`, `docs/architecture/01-architecture-overview.md` (D-ARCH-13, D-ARCH-15, D-ARCH-18, D-ARCH-31)

> `forma-operator` adalah **CRD controller** yang berjalan sebagai pod di setiap K8s cluster. Ia menerjemahkan CRD Forma (`Workspace`, `Datastore`, `ResourceClaim`, `ClusterClass`) menjadi resource K8s native (Deployment, Service, Secret, ConfigMap, HPA). **Satu-satunya komponen closed source** di Forma (D-ARCH-15). Dokumen ini fokus pada binary-nya sendiri — desain internal, konfigurasi, dan API/kontrak komunikasinya. Untuk gambaran CRD dan topologi K8s secara umum, lihat `docs/architecture/06-k8s-operator.md`.

---

## 1. Fitur

| Fitur | Deskripsi |
|---|---|
| **Workspace Controller** | Reconcile `Workspace` CRD → Deployment + Service + Secret + ConfigMap (+HPA jika auto-scaling) |
| **Datastore Controller** | Reconcile `Datastore` CRD → validasi endpoint, simpan kredensial sebagai Secret, daftarkan ke registry cluster |
| **ResourceClaim Controller** | Reconcile `ResourceClaim` CRD → verifikasi signature + `allowedTenants`, inject kredensial atau set status `Denied` |
| **Scale-to-zero per ClusterClass** | `minReplicas`/`scaleToZero` dari ClusterClass diterapkan ke Deployment (D-ARCH-31 — lihat `docs/architecture/05-failover.md` §3.2) |
| **Rolling restart saat binary handler berubah** | Baca `deploy_status: restart_required` dari Cluster Control, patch annotation pod template untuk trigger rolling restart |
| **Node labeling awareness** | Placement pod via `nodeSelector`/`nodeAffinity` berdasarkan label `forma.dev/*` |
| **Health & metrics reporting** | Lapor node health (15 detik) dan workspace status (on-change) ke Cluster Control |

---

## 2. Desain Internal

### 2.1 Arsitektur — Controller-Runtime Pattern

Operator dirancang mengikuti pola standar Kubernetes controller (`sigs.k8s.io/controller-runtime`):

```
main()
  → Manager (leader election, jika replicas > 1)
      → WorkspaceReconciler   (watch: Workspace, Deployment, Service, Secret, ConfigMap, HPA)
      → DatastoreReconciler   (watch: Datastore, Secret)
      → ResourceClaimReconciler (watch: ResourceClaim)
      → ClusterClassReconciler  (watch: ClusterClass — cache-only, tidak reconcile resource K8s)
```

Setiap reconciler mengimplementasikan pola standar: `Reconcile(ctx, req) (Result, error)` — idempotent, dipanggil ulang oleh workqueue saat resource watched berubah atau saat requeue terjadwal.

### 2.2 Leader Election

Untuk HA operator sendiri (multi-replica di cluster besar), operator menggunakan leader election berbasis Lease K8s standar — hanya satu replica yang aktif reconcile pada satu waktu; replica lain standby, siap ambil alih kalau leader mati (failover operator sendiri mengikuti pola K8s native yang sama seperti pod lain — lihat `docs/architecture/05-failover.md` §2).

### 2.3 Reconcile Loop — Workspace

```
Workspace Created/Updated:
  1. Ambil ClusterClass workspace (utk scaling policy)
  2. Hitung desired Deployment spec:
     - image: formahub/forma-resource:<version-pinned>
     - replicas: sesuai ClusterClass.scaling (minReplicas/scaleToZero)
     - env: CONTROL_CLUSTER_URL, WORKSPACE_ID
     - affinity: podAntiAffinity jika ClusterClass.scaling.minReplicas >= 2
  3. CreateOrUpdate Deployment, Service (ClusterIP), ConfigMap
  4. Jika ResourceClaim ada & approved → inject Secret (DB credentials)
  5. Jika ClusterClass.features berisi auto-scaling → CreateOrUpdate HPA
  6. Update Workspace.status.conditions (Ready/Progressing/Degraded)

Workspace Deleted:
  1. Delete Deployment, Service, ConfigMap
  2. Secret TIDAK dihapus otomatis (manual cleanup — lihat 06-k8s-operator.md §5)
```

### 2.4 Reconcile Loop — ResourceClaim

```
ResourceClaim Created/Updated:
  1. Verify signature (ed25519, ditandatangani pemilik resource)
  2. Cek workspace ∈ Datastore.allowedTenants
  3. Jika valid → inject credentials via env var (dari Secret Datastore)
     set status.conditions = [{type: Ready, status: True}]
  4. Jika tidak valid → set status.conditions = [{type: Denied, status: True, reason: "..."}]
     TIDAK inject kredensial
```

---

## 3. API & Kontrak Komunikasi

### 3.1 Endpoint yang Di-expose Operator Sendiri

Mengikuti konvensi standar controller-runtime:

| Endpoint | Fungsi |
|---|---|
| `GET /healthz` | Liveness probe operator (K8s) |
| `GET /readyz` | Readiness probe operator |
| `GET /metrics` | Prometheus metrics (reconcile count, error rate, queue depth per controller) |

### 3.2 Kontrak dengan Cluster Control

Operator adalah **klien** dari `forma-ctl --mode=cluster` (lihat `01-forma-ctl.md`). Kontrak pelaporan (desain diusulkan — belum final di plane protocol spec):

| Arah | Endpoint (diusulkan) | Frekuensi | Isi |
|---|---|---|---|
| Operator → Cluster Control | `POST /v1/node-health` | 15 detik | Status per node (healthy/degraded), workspace count, CPU/mem |
| Operator → Cluster Control | `POST /v1/workspace-status` | On-change | Status Deployment per workspace (Ready/Progressing/Degraded/restart_required consumed) |
| Cluster Control → Operator | (pull, bukan push) `GET /v1/snapshot` per workspace, via pod itu sendiri | — | Operator tidak pull snapshot — itu tanggung jawab pod (`forma-resource`/`forma-sidecar`), bukan operator |

> **Catatan desain:** endpoint di atas belum dispesifikasikan secara normatif di `docs/spec/platform/05-plane-protocol.md` (yang baru mendefinisikan kontrak Region↔Cluster↔Resource, bukan Operator↔Cluster). Ini perlu ditambahkan sebagai ekstensi wire protocol saat operator mulai dibangun.

### 3.3 CRD yang Diwatch (Ringkas)

Skema lengkap ada di `docs/architecture/06-k8s-operator.md` §3 — ringkasan field penting untuk konteks reconciler:

| CRD | Field kunci | Status subresource |
|---|---|---|
| `ClusterClass` | `sla`, `nodeType`, `maxWorkspaces`, `scaling.{minReplicas,scaleToZero}`, `pricing` | — (tidak punya status, murni config) |
| `Workspace` | `owner`, `region`, `clusterClass`, `environment`, `resources`, `datastores`, `cache` | `conditions: [Ready, Progressing, Degraded]`, `phase` |
| `Datastore` | `driver`, `endpointSecretRef`, `allowedTenants`, `owner`, `capacity` | `conditions: [Validated, Denied]` |
| `ResourceClaim` | `datastore`, `workspace`, `permission`, `grantedBy`, `signature` | `conditions: [Ready, Denied]` |

### 3.4 RBAC yang Dibutuhkan

Operator butuh `ClusterRole` dengan akses:

```yaml
rules:
  - apiGroups: ["forma.dev"]
    resources: ["workspaces", "datastores", "resourceclaims", "clusterclasses"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["forma.dev"]
    resources: ["workspaces/status", "datastores/status", "resourceclaims/status"]
    verbs: ["update", "patch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["services", "secrets", "configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update"]  # leader election
```

Secret read/write sengaja dibatasi ke namespace workspace masing-masing lewat scoping RBAC per-namespace jika model multi-namespace dipakai (satu namespace per workspace) — keputusan ini perlu difinalkan bersama desain namespace-per-workspace vs shared-namespace.

---

## 4. Konfigurasi & Flags (Diusulkan)

```bash
forma-operator \
  --control-cluster-url https://control-cluster.jkt-premium-01.svc \
  --leader-elect                          # aktifkan leader election (HA operator)
  --metrics-bind-address :8443
  --health-probe-bind-address :8081
  --workspace-concurrency 10              # max reconcile paralel per controller
  --namespace ""                          # kosong = watch semua namespace
```

---

## 5. Deployment Model Operator Sendiri

Operator berjalan sebagai Deployment K8s biasa di dalam cluster yang ia kelola (bukan di region control):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: forma-operator
  namespace: forma-system
spec:
  replicas: 2   # HA — leader election menentukan siapa aktif
  template:
    spec:
      serviceAccountName: forma-operator
      containers:
        - name: operator
          image: registry.forma.dev/forma-operator:1.4.2   # closed source, image terpisah
          args: ["--leader-elect", "--control-cluster-url=$(CONTROL_CLUSTER_URL)"]
```

---

## 6. Lisensi & Distribusi

`forma-operator` **satu-satunya komponen closed source** (D-ARCH-15). Didistribusikan sebagai image container terpisah dari `formahub/forma-resource` (generic image aplikasi) — registry privat, akses berdasarkan lisensi enterprise/paid. Pihak ketiga boleh membangun operator alternatif open source yang kompatibel dengan CRD yang sama (D-ARCH-16) — kompatibilitas dijaga lewat skema CRD sebagai kontrak publik, bukan lewat kode operator itu sendiri.

---

## 7. Status Implementasi Hari Ini

**Implementasi awal sudah ada.** `cmd/forma-operator` + `internal/operator` (controller-runtime, leader election via `--leader-elect`) mengimplementasikan:

- CRD Go types `forma.dev/v1alpha1` (`internal/operator/api/v1alpha1`, deepcopy hand-written — repo tidak memakai controller-gen) + CRD YAML & RBAC manifests di `deploy/operator/`.
- `WorkspaceReconciler` (§2.3): Deployment (generic image via `--resource-image`, env `CONTROL_CLUSTER_URL`/`WORKSPACE_ID`, nodeSelector `forma.dev/*`, anti-affinity saat `minReplicas >= 2`) + Service + ConfigMap + HPA (feature `auto-scaling`), injeksi kredensial dari ResourceClaim ber-status Ready, status conditions Ready/Progressing/Degraded. Delete via owner references; Secret sengaja tidak di-own (retained). Scale-to-zero: annotation `forma.dev/idle: "true"` pada Workspace ber-class `scaleToZero` → 0 replicas.
- `DatastoreReconciler`: validasi keberadaan Secret endpoint → conditions Validated/Denied.
- `ResourceClaimReconciler` (§2.4): verifikasi ed25519 (`spec.ownerPublicKey` hex di Datastore, pesan kanonik `datastore|workspace|permission|grantedBy|grantedAt`) + cek `allowedTenants` → Ready/Denied. `--insecure-skip-signature-verify` untuk dev.
- Reporter §3.2: `POST /v1/node-health` (15s) & `/v1/workspace-status` (on-change) — sisi server di `forma-ctl` **belum ada**, kegagalan di-log rate-limited dan tidak memblokir reconcile.

**Yang masih terbuka:** konsumsi `deploy_status: restart_required` dari Cluster Control (annotation `forma.dev/artifact-binary-hash` sudah disalin ke pod template kalau di-stamp di Workspace, tapi belum ada poller yang men-stamp-nya); endpoint §3.2 belum dinormatifkan di `docs/spec/platform/05-plane-protocol.md`; ClusterClass reconciler terpisah tidak dibuat (cache-only — perubahan class di-fan-out ke Workspace via watch).

### 7.1 Urutan Pembangunan yang Disarankan

1. Definisikan CRD Go types (`+kubebuilder:object` markers) untuk `Workspace`, `Datastore`, `ResourceClaim`, `ClusterClass` — mulai dari skema di `docs/architecture/06-k8s-operator.md` §3.
2. Scaffold operator dengan `kubebuilder`/`operator-sdk` (memberi struktur controller-runtime standar, leader election, metrics gratis).
3. Implementasikan `WorkspaceReconciler` dulu (paling kritikal — tanpa ini tidak ada pod yang pernah dibuat).
4. Selesaikan kontrak komunikasi dengan Cluster Control (§3.2) sebagai bagian dari `docs/spec/platform/05-plane-protocol.md` — jangan biarkan jadi keputusan implisit di kode.
5. `DatastoreReconciler` dan `ResourceClaimReconciler` menyusul, karena bergantung pada `docs/architecture/04-resource-registration.md` yang juga belum ada implementasinya.

---

## 8. References

| Dokumen | Isi |
|---|---|
| `docs/architecture/06-k8s-operator.md` | Skema CRD lengkap, topologi K8s, node labeling |
| `docs/architecture/05-failover.md` | Model workspace↔pod (1 Deployment per workspace), scale-to-zero |
| `docs/architecture/04-resource-registration.md` | Lifecycle registrasi resource (Datastore/node) |
| `docs/runtimes/01-forma-ctl.md` | Sisi Cluster Control yang jadi lawan bicara Operator |

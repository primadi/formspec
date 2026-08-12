# FormSpec Operator & K8s Integration

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** FormSpec Architecture Overview (D-ARCH-13, D-ARCH-15, D-ARCH-18, D-ARCH-19, D-ARCH-28, D-ARCH-29, D-ARCH-31)

> FormSpec Operator adalah **CRD controller** yang berjalan di setiap K8s cluster dalam region. Ia menjembatani FormSpec concepts (Workspace, Datastore, ClusterClass) dengan K8s primitives (Deployment, Service, Secret, ConfigMap). Operator **closed source, enterprise/paid only**.

---

## 1. Why K8s-Native?

### Build vs Leverage

| Kebutuhan | Kalau bangun sendiri | Dengan K8s |
|---|---|---|
| Health check + auto-restart | Harus implementasi heartbeat, health endpoint, restart logic | **Gratis** — liveness/readiness probe |
| Pod placement | Harus implementasi scheduler (bin-packing, affinity, resource accounting) | **Gratis** — K8s scheduler + node affinity |
| Service discovery | Harus implementasi registry + DNS | **Gratis** — K8s Service + CoreDNS |
| Rolling update | Harus implementasi version tracking, drain, rollback | **Gratis** — Deployment strategy |
| Scaling | Harus implementasi metrics collection, scale policy | **Gratis** — HPA |
| Secret management | Harus implementasi encrypted store + injection | **Gratis** — K8s Secrets |
| Workspace → pod | **FormSpec-specific** — tidak ada di K8s | **FormSpec Operator** |
| Datastore → credential injection | **FormSpec-specific** | **FormSpec Operator** |
| Resource permission enforcement | **FormSpec-specific** | **FormSpec Operator** |
| Artifact pipeline | **FormSpec-specific** | **formspec-ctl** |

**Kesimpulan:** K8s handle 80% operational concerns. FormSpec Operator handle 20% FormSpec-specific orchestration. Tidak perlu membangun ulang apa yang sudah K8s sediakan.

---

## 2. Operator Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    K8s Cluster                                │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │                 FormSpec Operator                          │  │
│  │                                                        │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐             │  │
│  │  │Workspace │  │Datastore │  │Resource  │             │  │
│  │  │Controller│  │Controller│  │Claim Ctl │             │  │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘             │  │
│  │       │              │              │                   │  │
│  │       │    ┌─────────┼──────────────┘                   │  │
│  │       │    │         │                                  │  │
│  │       ▼    ▼         ▼                                  │  │
│  │  ┌──────────────────────────────────┐                  │  │
│  │  │        Reconciler                │                  │  │
│  │  │  • Create Deployment             │                  │  │
│  │  │  • Create Service                │                  │  │
│  │  │  • Create Secret (DB creds)      │                  │  │
│  │  │  • Create ConfigMap              │                  │  │
│  │  │  • Apply NodeSelector/Affinity   │                  │  │
│  │  │  • Report health → Cluster Ctl   │                  │  │
│  │  └──────────────────────────────────┘                  │  │
│  └────────────────────────────────────────────────────────┘  │
│       │                                                       │
│       │ Create/Update                                         │
│       ▼                                                       │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Deployment: formspec-resource                            │  │
│  │  Service: formspec-resource (ClusterIP)                   │  │
│  │  Secret: db-credentials                                │  │
│  │  ConfigMap: workspace-config                           │  │
│  │  HPA: formspec-resource (CPU > 70%)                       │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. CRD Definitions

### 3.1 ClusterClass

Didefinisikan oleh Cloud Owner. Menentukan SLA, spesifikasi, dan harga setiap tier.

```yaml
apiVersion: formspec.dev/v1alpha1
kind: ClusterClass
metadata:
  name: premium
  region: jakarta
spec:
  sla: "99.99"
  availability: multi-az
  nodeType: dedicated
  storage: nvme-ssd
  maxWorkspaces: 50
  features:
    - auto-scaling
    - cross-az-failover
    - ddos-protection
  scaling:
    minReplicas: 2        # economy: 0 (scale-to-zero), standard: 1
    scaleToZero: false    # economy: true — idle workspace di-scale ke 0
  pricing:
    currency: IDR
    baseMonthly: 5000000
    perWorkspace: 500000
    perGBStorage: 5000
```

### 3.2 Workspace

Dibuat saat Workspace Owner provision workspace.

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Workspace
metadata:
  name: bank-mandiri-prod
spec:
  owner: ws-owner-key-fingerprint
  region: jakarta
  clusterClass: premium          # ← pilih class
  # cluster: jkt-premium-01     # ← enterprise: pilih cluster langsung
  environment: prod
  resources:
    cpu: "2"
    memory: "4Gi"
  datastores:
    - name: pg-bank-mandiri
      type: postgres
  cache:
    - name: valkey-shared
      type: valkey
```

**Reconciliation:** Operator melihat Workspace CRD baru:
1. Buat Deployment menggunakan **generic image** `formahub/formspec-resource:1.4.2` (version/digest-pinned; 1 image untuk semua app, bukan image custom per app)
2. Set replicas & scaling sesuai ClusterClass: premium `minReplicas: 2` + anti-affinity + HPA; economy `minReplicas: 0` + scale-to-zero (lihat `05-failover.md` §3.2)
3. Inject `CONTROL_CLUSTER_URL` dan `WORKSPACE_ID` sebagai env vars
4. Buat Service (ClusterIP), Secret (DB credentials), ConfigMap (workspace config)
5. Saat pod start → pull artifact dari Cluster Control → load spec + handler → running

### 3.3 Datastore

Dibuat saat DB/Valkey/Redis diregistrasi.

```yaml
apiVersion: formspec.dev/v1alpha1
kind: Datastore
metadata:
  name: pg-bank-mandiri
spec:
  driver: postgres
  endpointSecretRef:
    name: pg-bank-mandiri-creds
    key: connection-string
  allowedTenants:
    - workspace:bank-mandiri-prod
  owner: cloud-owner
  capacity:
    maxConnections: 100
    storageGB: 500
```

**Reconciliation:** Operator validasi endpoint, simpan kredensial sebagai Secret, daftarkan ke registry.

### 3.4 ResourceClaim

Mendefinisikan permission: resource mana bisa diakses workspace mana.

```yaml
apiVersion: formspec.dev/v1alpha1
kind: ResourceClaim
metadata:
  name: bank-pg-claim
spec:
  datastore: pg-bank-mandiri
  workspace: bank-mandiri-prod
  permission: read-write
  grantedBy: cloud-owner
  grantedAt: "2026-07-10T10:00:00Z"
  signature: "hex-encoded-ed25519"   # ditandatangani pemilik resource (lihat 04 §5.2)
```

**Reconciliation:** Operator verifikasi signature claim dan cek apakah workspace diizinkan di `allowedTenants` datastore. Kalau tidak → set status condition `Denied` pada ResourceClaim (dengan reason), kredensial tidak di-inject.

---

## 4. Node Labeling

Node K8s di-label untuk kategorisasi infrastruktur. Label ini dipakai FormSpec Operator untuk placement pod.

| Label | Nilai | Fungsi |
|---|---|---|
| `formspec.dev/environment` | `prod`, `staging`, `dev` | Pisahkan workload prod dari non-prod |
| `formspec.dev/tier` | `enterprise`, `shared`, `dev` | Tentukan isolasi resource |
| `formspec.dev/region` | `jakarta`, `singapore`, `tokyo` | Data residency |
| `formspec.dev/capacity` | `high`, `medium`, `low` | Kapasitas node |

```bash
kubectl label node worker-1 \
  formspec.dev/environment=prod \
  formspec.dev/tier=enterprise \
  formspec.dev/region=jakarta \
  formspec.dev/capacity=high
```

Operator menggunakan `nodeSelector` atau `nodeAffinity` untuk menempatkan pod di node yang sesuai.

---

## 5. Reconciliation Loop

```
┌─────────────────────────────────────────────────────┐
│                 Reconciler Loop                      │
│                                                      │
│  1. Watch CRD changes (Workspace, Datastore, etc.)  │
│  2. Compare desired state (CRD) vs actual (K8s)     │
│  3. If different → reconcile:                        │
│                                                      │
│     Workspace Created:                               │
│     ├── Create Deployment                            │
│     │   image: formahub/formspec-resource:1.4.2         │
│     │   (generic, version-pinned, 1 utk semua app)   │
│     ├── Set replicas sesuai ClusterClass             │
│     │   (premium: 2+ / economy: scale-to-zero)       │
│     ├── Create Service (ClusterIP)                   │
│     ├── Create Secret (DB credentials)               │
│     ├── Create ConfigMap (workspace config)          │
│     └── Create HPA (if auto-scaling enabled)         │
│                                                      │
│     Workspace Updated (spec change):                 │
│     ├── No pod restart needed                        │
│     ├── Pod pull artifact baru saat convergence      │
│     └── Atomic swap — zero downtime                  │
│                                                      │
│     Artifact baru berisi binary handler:             │
│     ├── Pod deteksi hash binary berubah saat pull    │
│     │   → tidak bisa hot-swap → emit deploy_status:  │
│     │     restart_required (via Cluster Control)     │
│     ├── Operator baca status → patch pod template    │
│     │   annotation formspec.dev/artifact-binary-hash    │
│     ├── K8s rolling restart — pod baru extract       │
│     │   binary baru saat start                       │
│     └── Image tetap sama (generic)                   │
│                                                      │
│     Workspace Deleted:                               │
│     ├── Delete Deployment                            │
│     ├── Delete Service                               │
│     ├── Delete ConfigMap                             │
│     └── Secrets retained (manual cleanup)            │
│                                                      │
│     Datastore Created:                               │
│     ├── Validate endpoint                            │
│     ├── Store credentials as Secret                  │
│     └── Register in cluster registry                │
│                                                      │
│     ResourceClaim Created/Updated:                    │
│     ├── Verify signature + workspace ∈ allowedTenants│
│     ├── If yes → inject credentials via env vars     │
│     └── If no → status condition: Denied             │
└─────────────────────────────────────────────────────┘
```

---

## 6. Communication with Cluster Control

Operator melaporkan ke Cluster Control:

| Informasi | Frekuensi |
|---|---|
| **Node health** (per node) | 15 detik |
| **Workspace status** (per workspace) | On-change |
| **Resource usage** (CPU, mem, storage) | 5 menit |

Cluster Control menggunakan informasi ini untuk:
- Merelay kapasitas agregat cluster ke Region Control — dipakai Region Control untuk **workspace→cluster routing** (penempatan pod di dalam cluster tetap urusan K8s scheduler)
- Melaporkan metering ke Region Control
- Mendeteksi node mati (via missed reports)

---

## 7. Standalone Mode (Tanpa Operator)

Untuk dev/small deployment tanpa K8s:

- **Tidak ada Operator** — semua manual via `formspec` CLI
- **Tidak ada CRD** — konfigurasi via file/command
- **Tidak ada auto-reconciliation** — developer manage process manual

```bash
# Standalone: start Control Plane
formspec-ctl serve --mode=standalone --port=8443 --db=sqlite:.formspec/control.db

# Standalone: run Go app (include formspec-resource via import)
./myapp --control-url=http://localhost:8443 --db=sqlite:data.db
```

---

## 8. Lisensi

| Komponen | Lisensi | Keterangan |
|---|---|---|
| **`formspec-operator`** | **Closed source** | **Satu-satunya komponen closed source.** CRD controller untuk K8s orchestration — enterprise/paid only. Repo terpisah dari core |
| `formspec-ctl` | FSL | Open source — semua mode (region, cluster, standalone) |
| `formspec-resource` | FSL | **Go library** (`github.com/primadi/formspec/resource`), di-compile jadi satu dengan app Go |
| `formspec-sidecar` | FSL | Open source — polyglot adapter wrapper |
| `formahub/formspec-resource` (version-pinned) | FSL | **Generic image** — 1 image untuk semua app. Berisi formspec-resource engine + sidecar |
| CLI tools (`formspec`) | FSL | Open source — `apply` implemented, other verbs roadmap |

**Pihak ketiga boleh membuat operator alternatif open source yang kompatibel dengan FormSpec.** FSL melarang menjual managed service kompetitor, tapi tidak melarang membuat tooling alternatif. Ini menjaga ekosistem tetap terbuka sambil memberi FormSpec monetisasi yang adil melalui operator resmi.

---

## 9. References

| Dokumen | Isi |
|---|---|
| `docs/architecture/01-architecture-overview.md` | Control levels, responsibility boundary, ClusterClass |
| `docs/architecture/03-deployment-flow.md` | Deployment pipeline, Stage 1 & 2 |
| `docs/architecture/04-resource-registration.md` | Resource lifecycle, token structure |
| `docs/architecture/05-failover.md` | Pod-level and workspace-level HA |
| `docs/spec/platform/03-kind-system.md` §4 | Authoritative kind → plane mapping |

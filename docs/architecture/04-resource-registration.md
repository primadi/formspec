# Resource Registration

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Architecture Overview (D-ARCH-5, D-ARCH-7, D-ARCH-8)

> Dokumen ini menjelaskan lifecycle registrasi infrastruktur di Forma: K8s node/cluster, datastore (Postgres/SQLite), dan cache (Valkey/Redis). Semua resource menggunakan mekanisme registrasi yang seragam: **token signed + approval**. Tidak ada binary `forma-server` — registrasi dilakukan oleh Cluster Control (`forma-ctl --mode=cluster`) yang berjalan di dalam cluster.

---

## 1. Unified Registration Model

Semua resource — K8s node, database, cache — didaftarkan ke forma-ctl dengan mekanisme yang sama:

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│  Resource    │     │  forma-ctl       │     │  forma/ops   │
│  (server,    │     │  (region)        │     │  (Cloud      │
│   DB, cache) │     │                  │     │   Owner)     │
└──────┬───────┘     └────────┬─────────┘     └──────┬───────┘
       │                      │                      │
       │ 1. POST /v1/register │                      │
       │   {token, identity,  │                      │
       │    capability}       │                      │
       │─────────────────────►│                      │
       │                      │ 2. Verify token sig  │
       │                      │ 3. Verify mTLS       │
       │                      │ 4. Status: pending   │
       │◄─────────────────────│                      │
       │   {id, status:pending}                     │
       │                      │                      │
       │                      │ 5. Cloud Owner       │
       │                      │    approve/reject    │
       │                      │◄─────────────────────│
       │                      │─────────────────────►│
       │                      │                      │
       │ 6. Poll status       │                      │
       │─────────────────────►│                      │
       │◄─────────────────────│                      │
       │   {status: active}   │                      │
       │                      │                      │
       │ 7. Heartbeat (30s)   │                      │
       │─────────────────────►│                      │
```

### 1.1 Three-Factor Verification

| Faktor | K8s Node | Datastore | Cache |
|---|---|---|---|
| **Token (signed)** | ✅ ed25519 | ✅ ed25519 | ✅ ed25519 |
| **mTLS** | ✅ | ✅ (opsional) | ✅ (opsional) |
| **Approval** | ✅ via forma/ops | ✅ via forma/ops | ✅ via forma/ops |

### 1.2 Token Structure

```json
{
  "resource_type": "node|datastore|cache",
  "resource_id": "jkt-premium-01",
  "owner": "cloud-owner-key-fingerprint",
  "capabilities": {
    "maxWorkspaces": 50,
    "environment": ["prod", "staging"],
    "region": "jakarta"
  },
  "allowedTenants": ["*"],  // atau list spesifik
  "expires_at": "2027-07-10T00:00:00Z",
  "signature": "hex-encoded-ed25519-signature"
}
```

---

## 2. Resource Types

### 2.1 K8s Node/Cluster

Setiap K8s worker node yang menjalankan forma-resource pods harus diregistrasi ke forma-ctl melalui Cluster Control. Tidak ada binary `forma-server` — node K8s dengan label `forma.dev/*` yang sudah disetujui oleh Cloud Owner sudah cukup.

| Attribute | Description |
|---|---|
| `resource_type` | `node` |
| `resource_id` | Nama cluster + node (e.g., `jkt-premium-01:worker-3`) |
| `capabilities.maxWorkspaces` | Kapasitas maksimum workspace di node ini |
| `capabilities.clusterClass` | premium / standard / economy |
| `capabilities.region` | Jakarta / Singapore / Tokyo |
| `capabilities.features` | auto-scaling, multi-az, ddos-protection |
| `allowedTenants` | `["*"]` (shared) atau list tenant spesifik (dedicated) |

**Heartbeat:** satu jalur pelaporan — **Forma Operator** (yang watch K8s API) melaporkan node health ke Cluster Control setiap 15 detik; **Cluster Control** merelay agregat per node ke Region Control setiap 30 detik. Isi heartbeat:
- Status (healthy / degraded)
- Current workspace count
- CPU/memory usage
- Node count (active / total)

**Missed heartbeat:** 3x missed di level Region Control (90 detik) → status: `dead`. Workspace di node ini tidak available.

### 2.2 Datastore (Postgres / SQLite)

| Attribute | Description |
|---|---|
| `resource_type` | `datastore` |
| `resource_id` | Nama datastore (e.g., `pg-bengkel-prod`) |
| `driver` | `postgres` / `sqlite` |
| `endpoint` | Connection string (tidak disimpan di token — di-inject via K8s Secret) |
| `capabilities.maxConnections` | Pool size |
| `capabilities.storageGB` | Kapasitas |
| `allowedTenants` | List workspace ID yang boleh pakai |
| `owner` | `cloud-owner` (shared) atau `workspace-owner:{id}` (dedicated) |

**Credentials:** Disimpan sebagai K8s Secret, tidak pernah dikirim dalam token. Hanya Forma Operator yang membaca Secret dan meng-inject sebagai environment variable ke pod forma-resource.

### 2.3 Cache (Valkey / Redis)

| Attribute | Description |
|---|---|
| `resource_type` | `cache` |
| `resource_id` | Nama cache instance (e.g., `valkey-shared-jkt`) |
| `driver` | `valkey` / `redis` |
| `endpoint` | Connection string (via K8s Secret) |
| `capabilities.maxMemoryMB` | Kapasitas memori |
| `capabilities.evictionPolicy` | allkeys-lru / volatile-lru / noeviction |
| `allowedTenants` | List workspace ID |

---

## 3. Resource Ownership

| Pemilik | Skenario | Contoh |
|---|---|---|
| **Cloud Owner** | Infrastruktur shared yang disediakan platform | DB cluster untuk semua tenant economy/standard |
| **Workspace Owner** | Infrastruktur dedicated yang dibawa sendiri | Postgres dedicated untuk enterprise |

**Tidak ada persona baru.** Cloud Owner dan Workspace Owner sudah cukup untuk mencakup semua skenario kepemilikan resource.

### 3.1 Ownership Transfer

Resource bisa dipindahkan kepemilikannya:
- Cloud Owner → Workspace Owner: enterprise upgrade (shared → dedicated)
- Workspace Owner → Cloud Owner: downgrade atau terminasi

Transfer memerlukan approval dari kedua belah pihak.

---

## 4. Resource Lifecycle

```
register ──► pending ──► active ──► degraded ──► dead ──► deregistered
                │                       │
                ▼                       ▼
             rejected              (auto-recover
                                   jika heartbeat
                                   kembali normal)
```

| Status | Arti | Tindakan |
|---|---|---|
| `register` | Resource baru mendaftar | Menunggu verifikasi token + mTLS |
| `pending` | Token verified, menunggu approval | Cloud Owner review via forma/ops |
| `active` | Disetujui, beroperasi normal | Menerima workspace assignment |
| `degraded` | Sebagian kapasitas berkurang | Alert Cloud Owner, hentikan assignment baru |
| `dead` | Heartbeat missed, tidak merespons | Workspace di resource ini tidak available |
| `deregistered` | Dihapus secara permanen | Tidak bisa direaktivasi |

### 4.1 Drain Before Deregister

Sebelum resource di-deregister:
1. Cloud Owner set status → `draining`
2. Tidak ada workspace baru yang di-assign ke resource ini
3. Workspace existing dimigrasikan ke resource lain (jika memungkinkan — shared DB bisa, dedicated DB tidak)
4. Setelah semua workspace pindah → status → `deregistered`

---

## 5. Permission Scoping

### 5.1 Tenant Affinity

Datastore dan cache bisa di-scope ke tenant tertentu:

```yaml
# Datastore hanya untuk tenant bengkel-xyz
allowedTenants:
  - workspace:bengkel-xyz-prod
  - workspace:bengkel-xyz-staging
```

```yaml
# Datastore shared — semua tenant bisa
allowedTenants:
  - "*"
```

### 5.2 ResourceClaim CRD Enforcement

Forma Operator membaca `ResourceClaim` CRD dan meng-enforce permission:

```yaml
apiVersion: forma.dev/v1alpha1
kind: ResourceClaim
metadata:
  name: bengkel-pg-claim
spec:
  datastore: pg-bengkel-prod
  workspace: bengkel-xyz
  permission: read-write
```

Operator menolak klaim yang:
- Datastore tidak mengizinkan workspace tersebut di `allowedTenants`
- Workspace tidak ada
- Claim tidak ditandatangani oleh pemilik resource

---

## 6. Metering Model

| Metrik | Sumber | Agregasi |
|---|---|---|
| **CPU usage** | K8s metrics API | Per pod → per workspace |
| **Memory usage** | K8s metrics API | Per pod → per workspace |
| **DB storage** | Datastore metrics | Per datastore → per workspace |
| **DB connections** | Datastore metrics | Per datastore → per workspace |
| **Cache memory** | Cache metrics | Per cache → per workspace |
| **Network egress** | K8s metrics API | Per pod → per workspace |
| **API calls** | Resource plane counters | Per workspace |

Semua metrik diagregasi per workspace untuk billing. Workspace owner bisa lihat usage mereka di forma/console.

---

## 7. Standalone Mode

Dalam standalone mode (non-K8s), registrasi resource dilakukan manual via CLI:

```bash
# Register datastore
forma datastore register pg-myapp \
  --driver postgres \
  --endpoint "postgres://localhost:5432/myapp"

# Register cache
forma cache register valkey-myapp \
  --driver valkey \
  --endpoint "localhost:6379"
```

Tanpa approval workflow — semua auto-approved di standalone mode.

---

## 8. References

| Dokumen | Isi |
|---|---|
| `docs/spec/12-datastore.md` | Datastore kind spec |
| `docs/spec/04-control-plane.md` | Control Plane API, Environment, Policy |
| `docs/architecture/01-architecture-overview.md` | Multi-region topology, ClusterClass |
| `docs/architecture/06-k8s-operator.md` | CRD definitions, Operator reconciliation |

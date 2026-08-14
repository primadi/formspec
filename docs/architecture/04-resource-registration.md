# Resource Registration

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** FormSpec Architecture Overview (D-ARCH-5, D-ARCH-7, D-ARCH-8)

> Dokumen ini menjelaskan lifecycle registrasi infrastruktur di FormSpec: K8s node/cluster, datastore (Postgres/SQLite), dan cache (Valkey/Redis). Semua resource menggunakan mekanisme registrasi yang seragam: **token signed + approval**. Tidak ada binary `formspec-server` — registrasi dilakukan oleh Cluster Control (`formspec-ctl --mode=cluster`) yang berjalan di dalam cluster.

---

## 1. Unified Registration Model

Semua resource — K8s node, database, cache — didaftarkan ke formspec-ctl dengan mekanisme yang sama:

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│  Resource    │     │  formspec-ctl       │     │  formspec/ops   │
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
| **Approval** | ✅ via formspec/ops | ✅ via formspec/ops | ✅ via formspec/ops |

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

Setiap K8s worker node yang menjalankan formspec-resource pods harus diregistrasi ke formspec-ctl melalui Cluster Control. Tidak ada binary `formspec-server` — node K8s dengan label `formspec.dev/*` yang sudah disetujui oleh Cloud Owner sudah cukup.

| Attribute | Description |
|---|---|
| `resource_type` | `node` |
| `resource_id` | Nama cluster + node (e.g., `jkt-premium-01:worker-3`) |
| `capabilities.maxWorkspaces` | Kapasitas maksimum workspace di node ini |
| `capabilities.clusterClass` | premium / standard / economy |
| `capabilities.region` | Jakarta / Singapore / Tokyo |
| `capabilities.features` | auto-scaling, multi-az, ddos-protection |
| `allowedTenants` | `["*"]` (shared) atau list tenant spesifik (dedicated) |

**Heartbeat:** satu jalur pelaporan — **FormSpec Operator** (yang watch K8s API) melaporkan node health ke Cluster Control setiap 15 detik; **Cluster Control** merelay agregat per node ke Region Control setiap 30 detik. Isi heartbeat:
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

**Credentials:** Disimpan sebagai K8s Secret, tidak pernah dikirim dalam token. Hanya FormSpec Operator yang membaca Secret dan meng-inject sebagai environment variable ke pod formspec-resource.

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
| `pending` | Token verified, menunggu approval | Cloud Owner review via formspec/ops |
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

FormSpec Operator membaca `ResourceClaim` CRD dan meng-enforce permission:

```yaml
apiVersion: formspec.dev/v1
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

## 6. Registry Resolution & Routing

Ketika sebuah caller me-resolve target (resource/service lain) lewat registry,
registry — bukan caller — yang memilih rute ke instance sehat terdekat.
**Location transparency dipertahankan sepenuhnya:** caller tidak pernah perlu
tahu atau menyebut di mana target sebenarnya berjalan; ia cukup menyebut
identitas target, dan registry menentukan rutenya.

### 6.1 Locality-Aware Routing Order

Resolusi mengikuti urutan preferensi **wajib** dari yang termurah ke yang
termahal secara latency, jatuh ke tingkat berikutnya hanya bila tidak ada
instance sehat di tingkat saat ini:

| Urutan | Tingkat | Mekanisme | Biaya |
|---|---|---|---|
| 1 | **Same process** | Direct function dispatch | Zero network hop |
| 2 | **Same host** | Loopback | Tanpa keluar host |
| 3 | **Same zone/region** | Jaringan intra-zone/region | Latency rendah |
| 4 | **Any healthy instance** | Jaringan lintas-zone/region | Fallback terakhir |

Urutan ini adalah preferensi, bukan pembatasan: bila tingkat terdekat tidak
punya instance sehat, registry otomatis turun ke tingkat berikutnya tanpa
melibatkan caller.

### 6.2 Load-Balancing Strategies & Circuit Breaker

Ketika lebih dari satu instance target tersedia di tingkat resolusi yang
dipilih, registry mendistribusikan panggilan memakai strategi load-balancing
yang dapat dipilih:

| Strategi | Perilaku |
|---|---|
| `round_robin` | Rotasi merata antar instance |
| `least_connections` | Ke instance dengan koneksi aktif paling sedikit |
| `latency_aware` | Ke instance dengan latency teramati terendah |
| `tenant_affinity` | Panggilan satu tenant konsisten diarahkan ke instance yang sama |

**`tenant_affinity` di sini adalah *request-routing affinity*, bukan
otorisasi.** Ia menstabilkan instance mana yang melayani tenant tertentu (demi
cache locality/konsistensi), dan **berbeda tegas** dari filter `allowedTenants`
pada `kind: Datastore` ([`../spec/platform/06-datastore.md`](../spec/platform/06-datastore.md)
§4) yang merupakan *authorization filter* — menentukan tenant mana yang boleh
sama sekali memakai resource. Perhatikan juga §5.1 "Tenant Affinity" di dokumen
ini yang membahas scoping `allowedTenants` untuk permission, bukan routing.

**Per-instance circuit breaker.** Tiap instance target dilindungi circuit
breaker independen:

| Transisi | Pemicu |
|---|---|
| `closed` → `open` | 5 kegagalan berturut-turut **atau** failure rate > 50% dalam jendela 60 detik |
| `open` (fail-fast) | Selama terbuka, request ke instance itu gagal cepat tanpa menyentuh instance |
| `open` → `half-open` | Setelah 30 detik, sebagian request diuji untuk mendeteksi pemulihan |
| `half-open` → `closed` | Uji berhasil → instance kembali menerima traffic normal |
| `half-open` → `open` | Uji gagal → kembali fail-fast, ulangi timer 30 detik |

Instance dengan breaker `open` dikeluarkan sementara dari pool load-balancing;
resolusi (§6.1) memperlakukannya seperti instance tidak sehat dan jatuh ke
kandidat berikutnya.

---

## 7. Metering Model

| Metrik | Sumber | Agregasi |
|---|---|---|
| **CPU usage** | K8s metrics API | Per pod → per workspace |
| **Memory usage** | K8s metrics API | Per pod → per workspace |
| **DB storage** | Datastore metrics | Per datastore → per workspace |
| **DB connections** | Datastore metrics | Per datastore → per workspace |
| **Cache memory** | Cache metrics | Per cache → per workspace |
| **Network egress** | K8s metrics API | Per pod → per workspace |
| **API calls** | Resource plane counters | Per workspace |

Semua metrik diagregasi per workspace untuk billing. Workspace owner bisa lihat usage mereka di formspec/console.

---

## 8. Standalone Mode

Dalam standalone mode (non-K8s), registrasi resource dilakukan manual via CLI:

```bash
# Register datastore
formspec datastore register pg-myapp \
  --driver postgres \
  --endpoint "postgres://localhost:5432/myapp"

# Register cache
formspec cache register valkey-myapp \
  --driver valkey \
  --endpoint "localhost:6379"
```

Tanpa approval workflow — semua auto-approved di standalone mode.

---

## 9. References

| Dokumen | Isi |
|---|---|
| `docs/spec/platform/06-datastore.md` | Datastore kind spec |
| `docs/spec/platform/04-control-plane.md` | Control Plane API, Environment, Policy |
| `docs/architecture/01-architecture-overview.md` | Multi-region topology, ClusterClass |
| `docs/architecture/06-k8s-operator.md` | CRD definitions, Operator reconciliation |

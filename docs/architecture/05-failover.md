# Failover & High Availability

**Version:** 1.1
**Status:** Draft
**License:** Creative Commons CC0
**Governed by:** Forma Architecture Overview (D-ARCH-10, D-ARCH-11, D-ARCH-12, D-ARCH-27, D-ARCH-28, D-ARCH-31)

> Forma tidak membangun mekanisme HA sendiri. **K8s menangani availability di level pod dan node.** Karena **1 workspace = 1 Deployment** (D-ARCH-31), identity workspace melekat pada Deployment — failover workspace *adalah* failover pod, dan itu urusan K8s. Forma menambahkan yang K8s tidak punya: scale-to-zero per tier, rollback artifact, dan eligibility rules untuk dedicated resources.

---

## 1. Two Layers of Availability

| Layer | Ditangani oleh | Mekanisme |
|---|---|---|
| **Pod/Node** | K8s native | Liveness probe, restart, reschedule ke node lain |
| **Workspace** | K8s (Deployment identity) + Forma | 1 workspace = 1 Deployment; scale-to-zero per ClusterClass; rollback artifact via `forma apply` |

```
┌─────────────────────────────────────────────────────────────┐
│                    K8s Cluster                               │
│                                                              │
│  K8s handles:                                                │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                │
│  │ Pod mati │──►│ Restart  │──►│ Running  │                │
│  └──────────┘   └──────────┘   └──────────┘                │
│                                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                │
│  │Node mati │──►│Reschedule│──►│Node lain │                │
│  └──────────┘   └──────────┘   └──────────┘                │
│                                                              │
│  Forma handles:                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                │
│  │Artifact  │──►│ Rollback │──►│ Versi    │                │
│  │rusak     │   │ (apply)  │   │ stabil   │                │
│  └──────────┘   └──────────┘   └──────────┘                │
│                                                              │
│  ┌──────────┐                                               │
│  │ Cluster  │──► Insiden infra — TIDAK auto-recover         │
│  │ down     │    Cloud Owner handle manual                  │
│  └──────────┘                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Pod-Level HA (K8s Native)

Ini **tidak memerlukan kode Forma**. K8s menyediakan out-of-the-box:

### 2.1 Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  failureThreshold: 3
```

Pod yang gagal liveness probe 3x berturut-turut → K8s restart pod. Ini juga menangkap workspace yang *hang* (proses hidup tapi tidak merespons) — `/health` tidak menjawab → restart.

### 2.2 Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

Pod yang belum ready → tidak menerima traffic (dikeluarkan dari Service).

### 2.3 Node Failover

Node mati → K8s reschedule pod ke node lain yang sehat. Pod dengan `PodAntiAffinity` dijamin tidak dijadwalkan di node yang sama (untuk true HA).

### 2.4 Multi-Replica Deployment

```yaml
spec:
  replicas: 3
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: forma-resource
              topologyKey: kubernetes.io/hostname
```

Dengan 3 replica + anti-affinity → pod tersebar di 3 node berbeda. 1 node mati → 2 pod tetap berjalan. Semua replica melayani workspace yang sama (stateless — state ada di DB/Valkey), jadi tidak perlu koordinasi "pod mana yang memegang workspace".

---

## 3. Workspace-Level Availability

**1 workspace = 1 Deployment (D-ARCH-31).** Identity workspace melekat pada Deployment, bukan pada pod. Pod mati → K8s recreate pod di bawah Deployment yang sama → pod baru pull artifact dari Cluster Control saat start → lanjut melayani workspace yang sama. **Tidak ada mekanisme "reassign workspace antar pod"** — konsep itu tidak diperlukan dalam model dedicated Deployment.

### 3.1 Failure Handling per Skenario

| Skenario | Ditangani oleh | Mekanisme |
|---|---|---|
| Proses crash | K8s | Liveness probe → restart container; pod baru re-pull artifact |
| Proses hang / unresponsive | K8s | Liveness probe gagal 3x → restart |
| Node mati | K8s | Reschedule pod ke node sehat (multi-replica: anti-affinity) |
| Beban naik | K8s | HPA menambah replica (tier dengan auto-scaling) |
| Artifact rusak (crash loop setelah deploy) | Forma | Pod emit `deploy_status: failed` → alert via forma/ops; developer rollback dengan `forma apply` versi sebelumnya |

### 3.2 Scale-to-Zero — Menjawab Biaya Tenant Murah/Gratis

Dedicated Deployment per workspace menimbulkan biaya untuk tenant murah/gratis: ribuan workspace idle = ribuan pod idle. Jawabannya bukan pool multi-workspace (yang menuntut distributed lock + reassignment + isolasi dalam-proses), melainkan **scale-to-zero per ClusterClass**:

| | Premium | Standard | Economy/Free |
|---|---|---|---|
| `minReplicas` | 2+ (anti-affinity) | 1 | 0 — scale-to-zero |
| HPA | ✅ | Opsional | ❌ |
| Resource request | Sesuai spec workspace | Medium | Kecil (mis. `64Mi` / `50m`) |
| Cold start | — | — | ~1–3 detik |

Workspace economy yang idle (tanpa traffic beberapa menit) di-scale ke 0 replica (KEDA/activator-style: request pertama menahan koneksi, membangunkan pod, lalu meneruskan). Cold start murah karena **generic image sudah ter-cache di node** dan **artifact sudah di Cluster Control** — pod hanya perlu start proses + pull artifact lokal. Density node economy dinaikkan dengan resource request kecil dan `maxPods` yang lebih tinggi.

### 3.3 Idempotency Key Guarantee

Semua aksi di Forma menggunakan **idempotency key** (Overview §2). Ini membuat retry aman lintas restart maupun lintas replica:

- Client mengirim request dengan key `idem-abc-123`
- Replica A memproses, menyimpan key → hasil (di DB, bukan di memori pod)
- Replica A mati sebelum mengembalikan response
- Client retry dengan key yang sama → dilayani replica B (atau pod hasil restart)
- Replica B lihat key `idem-abc-123` → "already processed" → return cached result

**Tidak ada duplicate processing** meskipun restart/failover terjadi di tengah transaksi.

### 3.4 ctx.lock — Primitif Aplikasi, Bukan Mekanisme Failover

`ctx.lock` (Valkey) tetap tersedia sebagai **mutual exclusion level aplikasi**: job terjadwal, proses batch, atau critical section yang hanya boleh dijalankan satu replica dalam satu waktu (mis. cron harian pada workspace multi-replica). Lock **bukan** penentu "pod mana melayani workspace" — routing traffic adalah urusan K8s Service.

### 3.5 Failover Eligibility

Tidak semua workspace bisa pindah node secara bebas. Tergantung tipe resource:

| Resource type | Bisa auto-failover? | Syarat |
|---|---|---|
| **DB shared** (Postgres cluster) | ✅ Ya | Pod pengganti cukup connect ke DB yang sama |
| **Valkey shared** | ✅ Ya | Lock, cache, pubsub tetap bisa diakses |
| **DB dedicated** (Postgres khusus, node-local) | ❌ Tidak | DB tidak bisa diakses dari node lain |
| **SQLite lokal** | ❌ Tidak | File db hanya ada di node yang mati |

Untuk dedicated resource yang terikat node, failover memerlukan intervensi Cloud Owner — memindahkan DB, update kredensial, restart workspace. Inilah kualifikasi pada D-ARCH-10: failover otomatis berlaku untuk shared resources; dedicated resources adalah pengecualian yang disadari.

---

## 4. Cluster-Level Failure

**Seluruh K8s cluster down** adalah insiden infrastruktur — **bukan** sesuatu yang Forma auto-recover.

| Yang terjadi | Tindakan |
|---|---|
| Cluster down | Semua workspace di cluster itu tidak available |
| Region Control | Mendeteksi cluster down (missed heartbeat dari Cluster Control) |
| Alert | Cloud Owner di-notifikasi via forma/ops |
| Recovery | Cloud Owner restore cluster (K8s admin task) ATAU migrasikan workspace ke cluster lain secara manual |
| Workspace owner | Diberitahu via forma/console: "Workspace Anda mengalami gangguan. Tim kami sedang menangani." |

**Kenapa tidak auto-recover:** Memindahkan workspace antar cluster memerlukan:
1. Memastikan data konsisten (DB dedicated tidak bisa dipindahkan otomatis)
2. Memastikan tidak ada dua cluster menjalankan workspace yang sama (global lock via region)
3. Koordinasi DNS/routing
4. Keputusan bisnis (apakah worth it? cluster mungkin recover 2 menit lagi)

Terlalu kompleks untuk otomatisasi. Human judgment (Cloud Owner) lebih aman.

---

## 5. Control Plane & Dependency Availability

Bagian ini menjawab pertanyaan yang sering muncul: **apa yang terjadi kalau control plane-nya sendiri yang mati?** Prinsip dasarnya: karena distribusi artifact **pull-based dengan cache di setiap lapis**, matinya control plane menurunkan *kemampuan mengubah sistem*, bukan *kemampuan sistem melayani traffic*.

### 5.1 Region Control Down

| Tetap berjalan | Berhenti |
|---|---|
| Semua workspace melayani traffic (artifact sudah ter-load di pod) | Deploy baru (`forma apply` gagal) |
| Cluster Control melayani snapshot/artifact dari cache | Perubahan policy, approval, signing |
| Pod restart & failover K8s (image + artifact di cache cluster) | Registrasi resource baru |
| Evidence dikumpulkan Cluster Control (buffered) | Transparency log append & checkpoint |

**HA Region Control:** `forma-ctl --mode=region` adalah proses stateless di depan Postgres — dijalankan multi-instance di belakang load balancer, dengan Postgres HA (streaming replication / managed HA). Recovery target ditentukan SLA internal Cloud Owner; evidence yang ter-buffer di Cluster Control di-relay ulang setelah region control kembali (at-least-once, idempotent by design).

### 5.2 Cluster Control Down

Cluster Control **stateless** (D-ARCH-26: tidak punya DB — cache bisa dibangun ulang dari Region Control), jadi dijalankan sebagai Deployment multi-replica di belakang Service. Kalau seluruh replica down:

- Pod yang sudah running tetap melayani traffic dengan artifact ter-load
- Pod **baru** (restart/scale-up) tidak bisa pull artifact → tertahan sampai Cluster Control kembali; K8s terus mencoba
- Evidence dari pod tertahan di buffer lokal pod, dikirim ulang saat Cluster Control kembali

### 5.3 Valkey (ctx.lock / cache / pubsub)

`ctx.lock` dipakai aplikasi untuk mutual exclusion (§3.4). Valkey yang menopangnya adalah **Valkey shared milik platform** (diregistrasi Cloud Owner, direplikasi + failover via Sentinel/cluster mode) — bukan Valkey milik workspace. Kalau Valkey down: operasi yang butuh lock/cache gagal dengan error eksplisit (bukan silent), CRUD biasa yang tidak menyentuh Valkey tetap jalan. Aplikasi disarankan memperlakukan `ctx.lock` failure sebagai retryable error.

---

## 6. Recovery: Pod Kembali Hidup

Dalam model 1 workspace = 1 Deployment, recovery pod sederhana dan sepenuhnya K8s-native:

1. K8s recreate pod di bawah Deployment workspace yang sama (identity dari env `WORKSPACE_ID`, bukan negosiasi)
2. Pod start → verifikasi identity ke Cluster Control (token + mTLS) → pull artifact
3. Pod ready → masuk kembali ke Service endpoints → melayani traffic

Tidak ada konsep "pod kosong menunggu assignment" dan tidak ada negosiasi reclaim — pod selalu lahir sudah tahu workspace-nya. Request yang terputus saat pod mati aman di-retry oleh client berkat idempotency key (§3.3).

---

## 7. Health Monitoring

| Komponen | Cek | Frekuensi |
|---|---|---|
| **Resource pod** | Liveness `/health` | 10 detik (K8s) |
| **Resource pod** | Readiness `/health` | 5 detik (K8s) |
| **Workspace health** (observability) | Evidence `health` via Cluster Control | 30 detik |
| **Node health** | Operator → Cluster Control | 15 detik |
| **Cluster Control** | Heartbeat ke Region Control | 30 detik |
| **Region Control** | Self health check + LB probe | — |

### 7.1 Health Status

| Status | Arti |
|---|---|
| `healthy` | Semua normal |
| `degraded` | Masih berfungsi, tapi performa menurun (high latency, errors) |
| `unhealthy` | Tidak merespons health check |
| `dead` | 3x missed heartbeat (~90 detik) |

Catatan: status di tabel ini adalah **observability di control plane** (apa yang dilihat Cloud Owner di forma/ops). Aksi restart/reschedule tetap dipicu oleh probe K8s (§2), bukan oleh heartbeat control plane.

---

## 8. References

| Dokumen | Isi |
|---|---|
| `docs/spec/01-overview.md` §2 | Idempotency key convention |
| `docs/spec/06-plane-protocol.md` §1 | Direction of authority, outage semantics |
| `docs/architecture/01-architecture-overview.md` | Control levels, responsibility boundary, D-ARCH-31 |
| `docs/architecture/04-resource-registration.md` | Resource lifecycle, heartbeat |

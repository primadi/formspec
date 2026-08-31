# Plane Protocol

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Tujuan Protokol

Kontrak yang membuat implementasi Control Plane dan Resource Plane yang
independen bisa saling beroperasi: bagaimana Resource Plane belajar apa yang
boleh ia jalankan, membuktikan apa yang sudah ia lakukan, dan tetap melayani
saat Control Plane tidak terjangkau.

**Dua channel, asimetris ketat:**

1. **Desired-state (Control → Resource, pull-only).** Resource Plane
   menarik snapshot bertanda tangan dari segala sesuatu yang governance
   putuskan: policy, deployment yang diinginkan, trust anchor, revocation.
   Resource Plane **tidak pernah** bisa mengubahnya.
2. **Evidence (Resource → Control, append-only).** Resource Plane mengirim
   evidence bertanda tangan, write-once: hasil deploy, rekaman metering,
   audit anchor, insiden pelanggaran. Evidence hanya bisa ditambah, tidak
   pernah diedit — dan evidence tidak pernah mengubah governance state
   dengan sendirinya.

Ini menegaskan aturan "tidak ada write-back":
[`04-control-plane.md`](04-control-plane.md) §1 — Resource Plane tidak bisa
memutasi governance state, hanya bisa menambah evidence.

**Kenapa channel evidence tak tergantikan.** Tanpa channel evidence
(Resource→Control, append-only), tiga hasil governance tidak punya jalur untuk
sampai ke Control Plane agar bisa direview: hasil canary rollout
([`10-deployment-operations.md`](10-deployment-operations.md) §4), metering yang
verifiable ([`07-marketplace.md`](07-marketplace.md) §5), dan insiden
pelanggaran permission
([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5
`USES_VIOLATION`). Justru channel evidence inilah yang membuat ketiganya
_governable_ — bukan sekadar telemetry.

## 2. Transport & Identitas

**Transport.** gRPC adalah transport normatif untuk kedua channel — tapi
implementasi **wajib** juga menawarkan binding HTTPS/JSON yang setara untuk
lingkungan terbatas yang tidak bisa menjalankan gRPC (proxy korporat, edge
runtime, environment berkebijakan ketat). Kedua binding membawa payload dan
jaminan yang sama; pilihan transport tidak pernah mengubah kontrak.

**Identitas instance plane.** Setiap instance plane memperoleh identitasnya
lewat lifecycle berikut, sekali per instance:

1. **Bootstrap token sekali pakai** — dikeluarkan saat provisioning, hanya
   valid untuk satu registrasi.
2. **Keypair digenerate lokal di instance** — kunci privat lahir di instance
   dan **tidak pernah** meninggalkannya (tidak pernah ditransmisikan, bahkan ke
   Control Plane).
3. **Instance certificate berumur pendek** (default 24 jam) — mengikat
   `{instance ID, environment, workspace scope}`, ditandatangani Control Plane
   setelah bootstrap token divalidasi.
4. **Renewal di atas sesi yang sudah ada** — instance memperbarui sertifikatnya
   sebelum kedaluwarsa lewat sesi yang sudah terautentikasi, tanpa bootstrap
   token baru.

Kunci privat instance **tidak pernah** ditransmisikan; Control Plane hanya
pernah melihat public key dan certificate signing request — pola custody yang
sama dengan kunci owner ([`04-control-plane.md`](04-control-plane.md) §4).

## 3. Pipeline Spec YAML

Setiap manifest YAML wajib melewati **pipeline dua-tahap** di semua
environment (termasuk dev) — loading langsung dari filesystem oleh Resource
Plane **tidak konform**.

```
Developer          Control Plane              Resource Plane
    │ formspec apply       │                           │
    │──────────────────►│ validasi, sha256, sign,   │
    │◄──────────────────│ simpan artifact           │
    │                    │        GET /v1/snapshot   │
    │                    │◄──────────────────────────│
    │                    │──────────────────────────►│ 304 / snapshot baru
    │                    │        GET /v1/artifacts   │
    │                    │◄──────────────────────────│ (kalau sha256 beda)
    │                    │──────────────────────────►│ verify → load
    │                    │◄──────────────────────────│ POST /v1/evidence
```

**Tahap 1 — Registrasi.** `formspec apply -f <path>` (atau `--watch` untuk
hot-reload) adalah **satu-satunya** cara mendaftarkan manifest. Control
Plane memvalidasi, menghitung sha256 per file + envelope agregat,
menandatangani artifact (kunci platform, atau self-signed di dev), dan
menyimpannya.

**Tahap 2 — Deployment.** Resource Plane **menarik** desired-state dari
Control Plane; Control Plane tidak pernah menginisiasi deployment lewat
jaringan.

**Mode dev:** kedua plane di mesin yang sama, pipeline sama secara
struktural, disederhanakan: signing self-signed ed25519, tanpa approval,
interval pull 10 detik (bukan 5 menit), plus endpoint dev-only `POST
/v1/poll` — trigger lokal setelah `formspec apply --watch` mendaftarkan
artifact baru, memangkas latensi ke ~100ms. Ini **bukan** push Control→
Resource, murni orkestrasi lokal di mesin developer.

## 4. Pesan dan Status

### 4.1 Snapshot

`GET /v1/snapshot` pakai **conditional pull berbasis ETag** — Resource
Plane kirim `If-None-Match` versi terakhir yang diketahui. Control Plane
balas `304 Not Modified` (versi sama) atau `200` + bundle bertanda tangan
(versi berubah), berisi: `meta` (versi monotonik, waktu terbit, environment,
signature), `policy` (bundle terkompilasi), `deployments` (artifact ID +
sha256 + versi per workspace), `datastores` (Workspace Binding — service
infrastruktur yang diotorisasi untuk workspace ini, lihat di bawah),
`trust` (public key owner/vendor/platform), `grants`, `licenses`,
`revocations`, `memberships`.

**`datastores` — Workspace Binding.** Setiap entri adalah service
infrastruktur (registrasi `kind: Datastore`, [`06-datastore.md`](06-datastore.md))
yang **access.filter**-nya cocok dengan workspace ini — service yang tidak
cocok tidak muncul sama sekali (workspace tidak bisa melihatnya). Entri
membawa: `name` (logical service name), `spec` (DatastoreSpec penuh —
driver, connection, serves; kredensial sudah di-resolve Control Plane dari
`credential_ref`), dan `permission` (ceiling operasi workspace). Resource
Plane mengisi Infra Registry-nya dari daftar ini — jalur manifest lokal
hanya untuk dev mode.

Kenapa conditional pull, bukan stream persisten: Control Plane stateless
(stream persisten butuh state in-memory yang bertentangan dengan horizontal
scaling); perbandingan ETag O(1) per request; response `304` ~250 byte;
tidak butuh sticky session untuk Control yang load-balanced.

**Aturan:** Resource Plane **wajib** memverifikasi signature dan **wajib**
menolak snapshot yang versinya tidak strictly-greater dari yang terakhir
di-apply (proteksi serangan rollback/downgrade). Kadensi pull: saat boot,
lalu tiap 5 menit (prod) atau 10 detik (dev).

**Snapshot lengkap vs delta.** Control Plane **boleh** mengirim snapshot delta
yang menyebut **base version**-nya secara eksplisit. Plane instance yang tidak
punya base version itu ter-cache **wajib** jatuh ke pengambilan snapshot lengkap
alih-alih mencoba menerapkan delta secara buta — delta hanya boleh diterapkan di
atas base yang persis cocok.

### 4.2 Artifacts

`GET /v1/artifacts/{id}` mengembalikan **envelope artifact bertanda
tangan**: bundle manifest (yaml + script + asset + binary), content manifest
per-file sha256, dan **signature chain**: tanda tangan author → tanda
tangan approval (sesuai policy) → otorisasi deploy (kunci platform).

Sebelum memuat apa pun, Resource Plane **wajib** memverifikasi urut:
integritas envelope (hash) → identitas author terhadap `trust` → chain
approval terhadap policy yang berlaku → otorisasi deploy → gerbang
trust-tier tipe impl. Gagal di mana pun = artifact ditolak + evidence
pelanggaran diterbitkan (§4.4).

**Optimisasi berbasis hash:** Resource Plane menyimpan
`deployment_manifest.json` lokal (artifact_id, versi, sha256, waktu load,
status per artifact). Re-deploy YAML yang sama (umum di dev dengan
`--watch`) jadi no-op murni lewat perbandingan sha256 — tanpa transfer
jaringan untuk artifact yang tidak berubah.

### 4.3 Konvergensi

Resource Plane konvergen ke `deployments` gaya GitOps: hitung diff →
bandingkan sha256 → ambil artifact yang hilang/berubah → verify → load/
unload → terbitkan evidence deploy. Canary plan datang sebagai bagian entry
deployment yang diinginkan; eksekusi & rollback lokal, hasilnya jadi
evidence.

### 4.4 Evidence

`POST /v1/evidence` menerima batch bertanda tangan. Tiap record: `{type,
instance_id, sequence, payload, signature}` — `sequence` monotonik
per-instance (deteksi gap). Payload **tidak pernah** berisi data bisnis:

| Type            | Isi                                                                                                   |
| --------------- | ----------------------------------------------------------------------------------------------------- |
| `deploy_status` | Artifact ID, phase (`up_to_date`/`fetched`/`verified`/`loaded`/`failed`/`rolled_back`), sha256, versi |
| `metering`      | Grant/license ID, counter per periode — **hitungan saja**                                             |
| `audit_anchor`  | Merkle root segmen audit lokal plane                                                                  |
| `violation`     | Insiden `USES_VIOLATION`: module, action, declared-vs-attempted, status auto-suspend                  |
| `health`        | Ringkasan heartbeat instance                                                                          |

Control Plane **wajib** memperlakukan evidence sebagai append-only dan
menjangkarkan tiap batch di transparency log
([`04-control-plane.md`](04-control-plane.md) §7). Evidence **wajib**
di-buffer lokal saat Control tidak terjangkau, di-flush berurutan saat
reconnect — buffer dibatasi disk, bukan waktu.

## 5. Jaminan

**Semantik outage.** Resource Plane **wajib** tetap melayani dengan
snapshot terakhir yang diketahui — ketersediaan beban kerja tenant tidak
pernah bergantung Control Plane. Degradasi bertahap:

| Umur snapshot   | Efek                                                                                                                                                                  |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| < 15 menit      | Normal                                                                                                                                                                |
| ≥ 15 menit      | Peringatan di kanal ops; evidence buffering aktif                                                                                                                     |
| ≥ ambang policy | **Operasi high-governance ditolak lokal:** deployment baru, perubahan yang memperluas permission, persetujuan REPL write produksi. Traffic runtime tidak terpengaruh. |

Revocation adalah trade-off yang disadari dari distribusi berbasis pull:
grant/sesi yang dicabut bisa hidup sampai pull berikutnya berhasil.

**Fast-path revocation.** Plane instance **wajib** menghormati push-hint untuk
snapshot yang membawa revocation — menariknya segera alih-alih menunggu siklus
pull berikutnya — dan **sebaiknya** memperpendek interval pull selama masih ada
revocation yang belum ter-acknowledge. Ini mengubah trade-off di atas dari
"sampai pull terjadwal berikutnya" menjadi "secepat push-hint tiba", tanpa
mengganggu asimetri pull-only: push-hint hanya memberi tahu _ada snapshot baru_,
Resource Plane tetap yang menarik dan memverifikasinya.

**Versi & kompatibilitas.** Versi protokol dinegosiasikan saat sesi mulai
(`formspec-plane/1`); plane wajib menolak beroperasi terhadap versi mayor yang
tidak dikenal. Skema snapshot berevolusi aditif dalam satu versi mayor —
section tak dikenal diabaikan (forward-compatible), tapi konstruk
signature/trust yang tak dikenal **tidak** — fail-closed. Toleransi
clock-skew ±5 menit; `version` monotonik yang mengurutkan snapshot, bukan
wall-clock.

## 6. Operator Wire Protocol

§1–5 mendefinisikan kontrak Region ↔ Cluster ↔ Resource (bagaimana beban
kerja tenant belajar & membuktikan). Section ini menambahkan **channel
keempat**: **Operator ↔ Cluster Control** — bagaimana K8s Operator (atau
reimplementasi kompatibel apapun) mendaftarkan node, melapor health,
menyetor metering, dan melaporkan konvergensi Deployment ke Cluster Control.
Menormatifkan sketsa "diusulkan" di
[`../../runtimes/03-formspec-operator.md`](../../runtimes/03-formspec-operator.md)
§3.2 inilah yang menepati janji operator pihak-ketiga (D-ARCH-16,
[`../../architecture/01-architecture-overview.md`](../../architecture/01-architecture-overview.md)
§11): kompatibilitas dijaga lewat kontrak wire publik ini + skema CRD, bukan
lewat kode `formspec-operator` yang closed source.

**Asimetri yang sama berlaku.** Operator **tidak pernah** memutasi governance
state; ia (a) **mengajukan** registrasi node yang menunggu approval, dan (b)
**menambah** evidence append-only (health, metering, status Deployment). Ia
**tidak** menarik snapshot desired-state — itu tetap tanggung jawab pod
(`formspec serve`), bukan operator ([`../../runtimes/03-formspec-operator.md`](../../runtimes/03-formspec-operator.md)
§3.2).

**Versi & tanda tangan.** Subprotokol dinegosiasikan sebagai
`formspec-operator/1`; semua request Operator → Cluster Control lewat **mTLS**
dan **ditandatangani** identitas operator, dengan `instance_id` + `sequence`
monotonik per-operator (deteksi gap) — pola yang sama dengan evidence §4.4.
Payload **tidak pernah** berisi data bisnis tenant.

### 6.1 Registrasi Node/Cluster

Node join mengikuti D-ARCH-7 (token signed Cloud Owner → register → pending →
approval → active):

| Arah                       | Endpoint                     | Isi                                                                                                |
| -------------------------- | ---------------------------- | -------------------------------------------------------------------------------------------------- |
| Operator → Cluster Control | `POST /v1/operator/register` | Join token bertanda tangan Cloud Owner, identitas node/operator, label `formspec.dev/*`, kapasitas |

Cluster Control merelay pendaftaran ke Region Control (satu-satunya pemegang
state governance); node masuk status `pending` sampai di-approve lewat
formspec/ops. **Approval adalah keputusan governance** ([`04-control-plane.md`](04-control-plane.md)
§9) — bukan efek samping registrasi. Node non-approved **tidak** dapat
kredensial dan **tidak** menjalankan beban kerja.

### 6.2 Heartbeat & Health

| Arah                       | Endpoint                        | Frekuensi | Isi                                                                                                                                                              |
| -------------------------- | ------------------------------- | --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Operator → Cluster Control | `POST /v1/operator/node-health` | 15 detik  | Status per node memakai kosakata health [`09-observability.md`](09-observability.md) §5 (`healthy`/`degraded`/`unhealthy`), jumlah workspace, CPU/mem beragregat |

Heartbeat hilang menurunkan status node yang diamati Cluster Control; **3×
missed (~90 detik) → `dead`** ([`../../architecture/05-failover.md`](../../architecture/05-failover.md)
§7). Health ini **observability** — aksi restart/reschedule tetap dipicu probe
K8s, bukan channel ini ([`../../architecture/05-failover.md`](../../architecture/05-failover.md)
§2, §7).

### 6.3 Metering

| Arah                       | Endpoint                     | Frekuensi | Isi                                                                          |
| -------------------------- | ---------------------------- | --------- | ---------------------------------------------------------------------------- |
| Operator → Cluster Control | `POST /v1/operator/metering` | 5 menit   | Konsumsi resource per node/workspace — **hitungan/agregat saja** (D-ARCH-30) |

Cluster Control mem-**batch relay** metering ke Region Control
([`../../architecture/03-deployment-flow.md`](../../architecture/03-deployment-flow.md)
§6), yang menjangkarkannya sebagai evidence `metering`
([`04-control-plane.md`](04-control-plane.md) §7). Sama seperti evidence
§4.4: **tidak pernah data bisnis**, hanya counter per periode.

### 6.4 Pelaporan Konvergensi Deployment

| Arah                       | Endpoint                             | Frekuensi | Isi                                                                                                          |
| -------------------------- | ------------------------------------ | --------- | ------------------------------------------------------------------------------------------------------------ |
| Operator → Cluster Control | `POST /v1/operator/workspace-status` | On-change | Status reconcile Deployment K8s per workspace: `Ready`/`Progressing`/`Degraded`, `restart_required` consumed |

Ini **melengkapi**, bukan menggantikan, evidence `deploy_status` dari pod
(§4.4): pod melaporkan konvergensi **artifact** (`loaded`/`failed`/…),
Operator melaporkan konvergensi **Deployment K8s** (pod ada, ready, replica
sesuai ClusterClass). Keduanya bersama membentuk status konvergensi yang
dilihat `formspec get deployment` ([`10-deployment-operations.md`](10-deployment-operations.md)
§2). Buffering saat Cluster Control tak terjangkau mengikuti aturan evidence
§4.4 (buffer lokal, flush berurutan, dibatasi disk).

### 6.5 Kegagalan Graceful

Sebelum sisi server endpoint ini ada, Operator **wajib** memperlakukan
kegagalannya sebagai non-fatal — di-log rate-limited, **tidak** memblokir
reconcile ([`../../runtimes/03-formspec-operator.md`](../../runtimes/03-formspec-operator.md)
§7). Reconcile K8s tetap sumber kebenaran lokal cluster; channel ini
observability + metering, bukan jalur kontrol beban kerja.

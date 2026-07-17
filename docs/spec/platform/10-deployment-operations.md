# Deployment & Operations

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Cakupan
Kontrak operasional untuk siklus hidup deployment: bagaimana artifact
berpindah dari `forma apply` menjadi beban kerja yang konvergen, bagaimana
rollback, canary, dan promotion antar environment bekerja, dan syarat DR/HA
minimal yang membedakan deployment produksi konform dari dev. Dokumen ini
**menaikkan** intent yang selama ini hanya di dokumen arsitektur
([`../../architecture/03-deployment-flow.md`](../../architecture/03-deployment-flow.md),
[`../../architecture/05-failover.md`](../../architecture/05-failover.md))
menjadi kontrak normatif.

Ia berdiri di atas dua kontrak yang sudah ada dan **tidak** menggandakannya:
mekanik wire register→deploy ada di [`05-plane-protocol.md`](05-plane-protocol.md);
governance keputusan (policy, approval, signing, transparency log) ada di
[`04-control-plane.md`](04-control-plane.md). Yang ditambahkan di sini adalah
**jaminan siklus operasional** yang mengikat keduanya.

## 2. Kontrak Pipeline Deployment
`forma apply` adalah satu-satunya jalan masuk
([`05-plane-protocol.md`](05-plane-protocol.md) §3). Kontrak durabilitasnya:

1. **Artifact.** `forma apply` menghasilkan artifact ber-sha256 per file +
   envelope agregat, ditandatangani (kunci platform di prod, self-signed
   ed25519 di dev) — [`05-plane-protocol.md`](05-plane-protocol.md) §4.2.
2. **Registrasi wajib membuat Deployment record.** Registrasi yang berhasil
   **wajib** menciptakan/memperbarui satu **Deployment record** durable di
   storage Control Plane (`forma_control`) sebelum `forma apply` melaporkan
   sukses. Deployment record memuat: `artifact_id`, `version` (monotonik),
   `sha256`, environment, workspace target, waktu registrasi, dan status
   konvergensi awal `registered`.
3. **Propagasi snapshot.** Deployment yang diinginkan masuk ke section
   `deployments` snapshot dan didistribusikan pull-only
   ([`05-plane-protocol.md`](05-plane-protocol.md) §4.1).
4. **Konvergensi.** Resource Plane konvergen gaya GitOps dan menerbitkan
   evidence `deploy_status` ([`05-plane-protocol.md`](05-plane-protocol.md)
   §3.3–3.4).

**Registered ≠ converged (normatif).** `forma apply` melapor sukses saat
registrasi **durable** — **bukan** saat beban kerja konvergen. Konvergensi
bersifat eventual dan pull-based; statusnya diamati lewat evidence, bukan
dijamin oleh return `apply`. `forma get deployment <name>` **wajib**
menampilkan status konvergensi turunan dari evidence terakhir, minimal:
`registered` → `converging` → `converged` → `failed` / `rolled_back`.
Melaporkan sukses sebelum registrasi durable **tidak konform** (kegagalan
Control Plane di tengah registrasi tidak boleh menghasilkan "sukses" yang
tidak tercermin di Deployment record).

## 3. Rollback
Rollback adalah **redeploy artifact versi sebelumnya**, bukan mekanisme
terpisah: Control Plane menetapkan Deployment record menunjuk `version`
sebelumnya (artifact-nya masih tersimpan dan tertanda tangan), snapshot
membawa versi itu, Resource Plane konvergen mundur seperti deploy biasa.
Karena hanya menunjuk artifact yang sudah pernah lolos verifikasi, rollback
**tidak** melewati approval untuk versi yang sama — chain tanda tangannya
sudah ada.

`forma rollback` adalah verb CLI untuk memicunya:

```bash
forma rollback deployment myapp --to-version 41     # ke versi eksplisit
forma rollback deployment myapp                      # ke versi konvergen sebelumnya
```

**Yang di-rollback vs tidak (normatif):**

| Objek | Rollback? |
|---|---|
| Spec, script, asset, binary handler (isi artifact) | **Ya** — kembali ke versi sebelumnya utuh |
| Data bisnis (record entity, storage) | **Tidak** — data tidak pernah di-rewind oleh rollback deployment |
| Perubahan structural (skema) | **Mengikuti aturan dua-fase** [`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §2 — field removal butuh dua versi (deprecate → remove), jadi rollback spec **tidak** otomatis mengembalikan kolom yang sudah di-drop di versi sebelumnya |

Rollback yang menyentuh perubahan structural non-reversible (kolom sudah
di-drop) **wajib** ditolak dengan pesan yang menunjuk aturan dua-fase —
memaksa developer maju dengan versi perbaikan, bukan mundur ke skema yang
tidak lagi cocok. Pemulihan **data** adalah jalur restore-from-backup
([`04-control-plane.md`](04-control-plane.md) §6.1), bukan rollback.

## 4. Canary
Deployment entry **boleh** membawa `canary` plan opsional; tanpa itu,
deployment langsung (semua target sekaligus). Ini memperluas one-liner
[`05-plane-protocol.md`](05-plane-protocol.md) §4.3 menjadi kontrak.

```yaml
# bagian dari Deployment yang diinginkan (desired-state, snapshot)
canary:
  percentage: 10                 # atau: target_workspaces: [ws-a, ws-b]
  health_gate:
    metric: http_request_errors_total
    max_error_rate: 0.02         # ambang lolos
    observe: 10m                 # jendela observasi sebelum promote penuh
  on_gate_failure: auto_rollback # auto_rollback | hold
```

Kontrak normatif:
- **Bentuk canary datang sebagai desired-state**, dievaluasi Control Plane
  policy sebelum masuk snapshot; **eksekusinya lokal di cluster** (Resource
  Plane/Operator) — Control Plane tidak mengorkestrasi rollout lewat jaringan.
- **Health gate** dievaluasi terhadap metric yang didefinisikan
  [`09-observability.md`](09-observability.md) §3. Selama jendela `observe`,
  hanya subset target (percentage/list) yang menjalankan versi baru.
- **Gate lolos** → deployment melebar ke seluruh target; **gate gagal** →
  `on_gate_failure`: `auto_rollback` (default) memicu rollback (§3) otomatis,
  `hold` membekukan di persentase saat ini menunggu keputusan manusia.
- **Hasil canary (lolos/gagal + metric ringkas) adalah evidence**
  ([`05-plane-protocol.md`](05-plane-protocol.md) §4.4), tercatat, bukan
  keputusan yang menghilang. Metric ringkas — **tidak pernah data bisnis**.

## 5. Environment Promotion
Promotion mengikuti siklus **Sign → Apply → Approve → Promote**
([`04-control-plane.md`](04-control-plane.md) §2–3): artifact yang identik
(checksum sama diverifikasi) dipindahkan dari satu environment ke environment
lain tanpa build ulang. "staging" dan "production" adalah *nama* environment
`mode: prod` yang dibedakan hanya oleh Policy-nya
([`04-control-plane.md`](04-control-plane.md) §1) — promotion adalah
transisi artifact yang sama melewati Policy yang makin ketat.

```bash
forma promote myapp --from staging --to production   # checksum diverifikasi identik
```

Kontrak normatif:
- Promotion **wajib** memverifikasi sha256 artifact di environment sumber
  identik dengan yang dipromosikan — promotion **tidak pernah** membangun
  ulang atau menandatangani ulang isi; ia menambah otorisasi deploy untuk
  environment tujuan pada artifact yang sudah ada.
- Approval untuk promotion mengikuti Policy environment **tujuan**
  ([`04-control-plane.md`](04-control-plane.md) §5), termasuk gate re-consent
  Data Owner kalau footprint melebar ([`04-control-plane.md`](04-control-plane.md)
  §3).
- Seluruh rangkaian (apply, approve, promote) adalah satu chain di
  transparency log ([`04-control-plane.md`](04-control-plane.md) §7).

`forma promote` **wajib** ada sebagai verb CLI dan ditambahkan ke tabel verb
[`../../cli-tools/01-forma-cli.md`](../../cli-tools/01-forma-cli.md) §1.

## 6. DR & HA — Requirement Minimal
Forma tidak membangun mekanisme HA sendiri di level pod/node — itu K8s
([`../../architecture/05-failover.md`](../../architecture/05-failover.md) §1).
Yang **wajib** dijamin di level kontrak:

### 6.1 Durabilitas Control Plane
- Storage Control Plane (`forma_control`) **wajib** durable di produksi.
  Store in-memory adalah **dev-only, non-konform untuk produksi** — kehilangan
  Deployment record, transparency log, atau kontrak tidak boleh mungkin
  akibat restart proses.
- Kunci signing platform **wajib** persisten dan KMS/HSM/Vault-backed lewat
  `key_ref` ([`04-control-plane.md`](04-control-plane.md) §1, §4) — kunci
  signing yang hidup hanya di memori proses non-konform di produksi.
- Region Control dijalankan stateless multi-instance di depan storage durable
  ber-HA (mis. Postgres streaming replication) —
  [`../../architecture/05-failover.md`](../../architecture/05-failover.md) §5.1.

### 6.2 RTO/RPO sebagai Parameter Environment
RTO dan RPO **bukan** angka hardcoded — mereka **parameter profil deployment
yang dideklarasikan per Environment** ([`04-control-plane.md`](04-control-plane.md)
§2), diturunkan dari ClusterClass SLA
([`../../architecture/06-k8s-operator.md`](../../architecture/06-k8s-operator.md)).
Jadwal backup ([`04-control-plane.md`](04-control-plane.md) §6.1) **wajib**
konsisten dengan RPO yang dideklarasikan (interval backup ≤ RPO). Deklarasi
yang tidak konsisten (RPO 1 jam tapi backup harian) **wajib** ditolak
`forma apply`.

```yaml
# Environment spec — lihat 04-control-plane.md §2
spec:
  mode: prod
  dr:
    rpo: 1h          # data loss maksimum yang ditoleransi
    rto: 4h          # waktu pemulihan target
```

### 6.3 Cross-Region DR — Batas Jujur
- DR lintas region = **jalur restore-from-backup** ([`04-control-plane.md`](04-control-plane.md)
  §6.1, [`../backend/04-persist-backend.md`](../backend/04-persist-backend.md)
  §3): backup dari region primer di-restore ke region pemulihan. Ini
  memanfaatkan garansi credible-exit yang sudah ada, bukan mekanisme baru.
- **Failover cross-cluster/cross-region otomatis: eksplisit di luar cakupan
  v1.** Memindahkan workspace antar cluster/region butuh konsistensi data,
  jaminan tidak ada dua cluster menjalankan workspace yang sama, dan
  koordinasi routing — kompleksitas yang sengaja tidak diotomasi
  ([`../../architecture/05-failover.md`](../../architecture/05-failover.md) §4).
  Pemulihan region adalah **runbook manual Cloud Owner**: restore backup ke
  region pemulihan → repoint routing → `forma apply` desired-state di region
  baru. Kontrak ini mendokumentasikan batas itu dengan jujur alih-alih
  menjanjikan otomasi yang tidak ada.

### 6.4 Cutover Near-Zero-Downtime
Memigrasikan data sebuah workspace ke infrastruktur baru — mis. migrasi
datastore atau cutover major version — **tidak** boleh menuntut outage panjang.
Prosedur normatif berikut memakai ulang kontrak backup/restore
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §3),
bukan mekanisme baru, dan berbeda dari restore-from-backup DR (§6.3) karena
sistem lama **tetap melayani** selama sebagian besar proses:

1. **Full backup.** Ambil backup penuh dari source sesuai kontrak
   [`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §3.
2. **Restore ke target di background.** Restore backup itu ke target baru
   sementara sistem lama **terus melayani traffic** — tanpa gangguan bagi
   pengguna.
3. **Incremental catch-up.** Terapkan backup/replay inkremental berulang ke
   target untuk mempersempit selisih (gap) terhadap source yang masih menulis.
4. **Write-freeze singkat di source.** Bekukan tulisan di source untuk jendela
   sesingkat mungkin — hanya read yang tersisa; inilah satu-satunya window
   dengan dampak terlihat, dan sengaja dijaga minimal.
5. **Final incremental catch-up.** Terapkan satu catch-up inkremental terakhir
   sehingga target identik dengan source per titik freeze.
6. **Switch traffic.** Alihkan routing ke target baru, lepaskan freeze.

Kontrak normatif:
- Langkah 1–3 **wajib** berjalan tanpa memutus layanan source; downtime yang
  terlihat pengguna terbatas pada window write-freeze langkah 4–6.
- Cutover **wajib** memverifikasi target konvergen dengan source (langkah 5)
  **sebelum** switch traffic (langkah 6); switch tanpa final catch-up yang
  sukses **tidak konform** (risiko kehilangan tulisan terakhir).
- Bila langkah mana pun gagal sebelum switch, source tetap otoritatif dan
  cutover dibatalkan tanpa dampak data — target dibuang, source tak pernah
  berhenti melayani.

## 7. Kode Error
| Kode | Kondisi |
|---|---|
| `DEPLOYMENT_REGISTRATION_NOT_DURABLE` | Registrasi gagal dipersist sebelum `apply` melapor sukses |
| `ROLLBACK_VERSION_NOT_FOUND` | Versi rollback target tidak ada di artifact store |
| `ROLLBACK_STRUCTURAL_IRREVERSIBLE` | Rollback menyentuh perubahan structural non-reversible (aturan dua-fase §3) |
| `PROMOTE_CHECKSUM_MISMATCH` | sha256 artifact sumber ≠ yang dipromosikan |
| `CANARY_GATE_FAILED` | Health gate canary gagal dalam jendela observasi |
| `DR_PROFILE_INCONSISTENT` | Jadwal backup tidak konsisten dengan RPO Environment |
| `CONTROL_STORE_NOT_DURABLE` | Store in-memory dipakai di environment `mode: prod` |
| `CUTOVER_SWITCH_BEFORE_CATCHUP` | Traffic dialihkan ke target sebelum final incremental catch-up sukses (§6.4) |

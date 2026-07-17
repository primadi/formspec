# forma-ctl — Binary Reference

**Version:** 1.0
**Status:** Draft
**License:** Creative Commons CC0 (dokumen) — binary-nya sendiri FSL (open source)
**Governed by:** `docs/architecture/01-architecture-overview.md` §4–5, `docs/spec/platform/04-control-plane.md`, `docs/spec/platform/05-plane-protocol.md`

> `forma-ctl` adalah **satu-satunya binary Control Plane** di Forma. Subcommand `serve` menjalankannya dalam tiga mode (`region`, `cluster`, `standalone`) dari **satu codebase yang sama** — bukan tiga binary berbeda; subcommand lain (`freeze`, `revoke`, `key`, `policy`, `log`) adalah emergency CLI Cloud Owner (D43 "bedrock exception" — lihat `docs/cli-tools/04-forma-ctl.md`), dalam binary yang sama. Dokumen ini menjelaskan fitur, desain internal, dan API HTTP-nya secara rinci, sebagai pelengkap gambaran topologi besar di `docs/architecture/`.

---

## 1. Peran & Mode

Flag `--mode` ada di bawah subcommand `serve` (`forma-ctl serve --mode=<mode>`):

| Mode | Dijalankan di | Peran |
|---|---|---|
| `--mode=region` | 1 proses per region (Jakarta/Singapore/dll) | **Source of truth**: artifact store, signing, policy, deployment routing, transparency log |
| `--mode=cluster` | 1+ pod per K8s cluster | **Cache proxy**: cache artifact lokal, proxy snapshot ber-ETag, batch relay evidence ke region |
| `--mode=standalone` | 1 proses, single machine | **All-in-one**: gabungan region+cluster untuk dev/small deployment, tanpa HA |

Ketiganya berbagi kode yang sama (`internal/control`) — perbedaan mode menentukan *storage backend* (in-memory/SQLite/Postgres), *apakah endpoint tertentu aktif* (mis. `/v1/poll` hanya di dev), dan *ke mana ia sync* (cluster mode sync dari URL region; region mode adalah puncak hierarki).

Lihat `docs/architecture/01-architecture-overview.md` §1 dan §4 untuk gambaran topologi multi-region lengkap.

---

## 2. Fitur per Mode

### 2.1 Region Mode — Fitur

| Fitur | Deskripsi |
|---|---|
| **Artifact registration** | Terima artifact (YAML+script+binary+asset) dari `forma apply`, validasi, hash, sign, simpan |
| **Artifact store** | Database authoritatif semua versi artifact per app/workspace |
| **Signing & keys** | Tanda tangan artifact envelope (ed25519); target produksi: key di HSM/KMS |
| **Deployment routing** | Tentukan workspace → cluster berdasarkan ClusterClass + kapasitas (lihat §5) |
| **Policy engine** | Evaluasi deployment policy, approval chain, trust tier (target: OPA/Rego — lihat §7) |
| **Transparency log** | Merkle append-only audit atas semua artifact & approval (target — lihat §7) |
| **Evidence collection** | Terima `deploy_status`/`health`/`metering`/`violation` dari cluster control (batched) |
| **Resource registration** | Terima registrasi node/datastore/cache (token signed + approval) — lihat `docs/architecture/04-resource-registration.md` |

### 2.2 Cluster Mode — Fitur

| Fitur | Deskripsi |
|---|---|
| **Artifact cache** | Cache lokal artifact yang sudah di-fetch dari region, keyed by `artifact_id` |
| **Snapshot proxy** | Serve `GET /v1/snapshot` ke resource pods dengan ETag, dibangun dari sync terakhir ke region |
| **Evidence batching** | Kumpulkan evidence dari semua pod dalam cluster, relay ke region per 60 detik |

Cluster mode **tidak punya** policy engine, signing capability, atau transparency log sendiri — murni cache+proxy (D-ARCH-26).

### 2.3 Standalone Mode — Fitur

Gabungan region+cluster dalam satu proses, tanpa split, tanpa approval workflow (auto-approved), storage SQLite. Untuk dev dan small deployment (lihat `docs/architecture/01-architecture-overview.md` §10).

---

## 3. Desain Internal

### 3.1 Package Map

| Package | Tanggung jawab |
|---|---|
| `internal/control` | HTTP handler untuk semua endpoint Control Plane (`server.go`, `register.go`, `snapshot.go`, `evidence.go`, `poll.go`) |
| `internal/artifact` | Model data (`ArtifactEnvelope`, `Artifact`, `Deployment`, `Snapshot`, `EvidenceRecord`), signing (ed25519), dan store |
| `internal/manifest` | Parsing & validasi YAML manifest (dipakai `RegisterHandler` untuk validasi sebelum simpan) |

### 3.2 Alur Registrasi (Register → Store → Sign)

```
POST /v1/artifacts (multipart atau JSON)
  → manifest.Loader.Validate(raw)       — schema, kind, cross-reference
  → artifact.ComputeSHA256 per file     — dan aggregate envelope hash
  → signer.Sign(envelope)               — ed25519, zero-out Signature dulu sebelum sign
  → store.SaveArtifact(artifact)        — status: active
  → store.IncrementSnapshotVersion()
  → response: { artifact_id, version, sha256 }
```

### 3.3 Alur Snapshot (Serve ke Cluster/Resource)

```
GET /v1/snapshot?workspace={id}  (header If-None-Match: {etag})
  → store.ListDeployments(ctx, workspaceID)
  → bandingkan versi dengan ETag request
  → 304 Not Modified  (kalau sama)
  → 200 + Snapshot{Version, IssuedAt, Deployments[]}  (kalau beda)
```

### 3.4 Storage Backend per Mode (Target)

| Mode | Storage target |
|---|---|
| `region` (prod) | Postgres |
| `region` (dev) | SQLite |
| `cluster` | In-memory/on-disk cache (bukan source of truth) |
| `standalone` | SQLite |

---

## 4. Model Data

```go
type ArtifactEnvelope struct {
    ArtifactID   ArtifactID
    App          string
    Version      int
    SHA256       string          // aggregate hash semua Files
    Files        []FileManifest  // {Path, SHA256, Content []byte}
    Signature    string          // hex ed25519 signature
    SigningKeyID string
    PrevVersion  int
    PrevSHA256   string
    CreatedAt    time.Time
}

type Deployment struct {
    WorkspaceID string
    ArtifactID  ArtifactID
    // ... desired-state binding workspace -> artifact version
}

type Snapshot struct {
    Version      int
    IssuedAt     time.Time
    Environment  string
    Signature    string
    Deployments  []Deployment
    // Target (belum dipopulasikan hari ini): Policy, Trust, Grants, Licenses, Revocations, Memberships
}

type EvidenceRecord struct {
    Type EvidenceType // deploy_status | metering | audit_anchor | violation | health
    // ...
}
```

`DeployPhase` enum: `up_to_date`, `fetched`, `verified`, `loaded`, `failed`, `rolled_back` — dilaporkan resource pod di setiap tahap konvergensi (lihat `02-forma-resource.md` §5).

---

## 5. API

### 5.1 Endpoint Reference

| Method | Path | Fungsi | Mode |
|---|---|---|---|
| `POST` | `/v1/artifacts` | Registrasi artifact baru (JSON atau multipart) | region, standalone |
| `GET` | `/v1/artifacts/{id}` | Ambil artifact envelope by ID (dipanggil cluster/resource untuk fetch yang berubah) | region, cluster, standalone |
| `GET` | `/v1/snapshot?workspace={id}` | Snapshot deployments untuk workspace, dengan ETag caching | region, cluster, standalone |
| `POST` | `/v1/evidence` | Terima batch evidence (`deploy_status`, `health`, `metering`, `violation`) | region, cluster, standalone |
| `POST` | `/v1/poll` | **Dev-only** — trigger pull segera setelah `forma apply --watch` | standalone (dev) |
| `GET` | `/health` | Health check proses | semua mode |

### 5.2 Request/Response Sketches

```http
POST /v1/artifacts
Content-Type: multipart/form-data | application/json

Response 200:
{ "artifact_id": "art_abc123", "version": 4, "sha256": "e3b0c4..." }

Response 422: validasi gagal (schema/kind/cross-reference tidak valid)
```

```http
GET /v1/snapshot?workspace=bank-mandiri-prod
If-None-Match: "v12"

Response 304 Not Modified   (tidak ada perubahan)
Response 200:
{ "version": 13, "issued_at": "...", "deployments": [ {...} ] }
```

```http
POST /v1/evidence
{ "records": [ {"type": "deploy_status", "workspace": "...", "phase": "loaded", ...} ] }

Response 202 Accepted
```

### 5.3 Endpoint yang Direncanakan Tapi Belum Ada

`docs/spec/platform/05-plane-protocol.md` menyebut `POST /v1/register` sebagai endpoint registrasi resource (node/datastore/cache — lihat `docs/architecture/04-resource-registration.md`). Endpoint ini **belum ada** di kode; hanya `POST /v1/artifacts` (registrasi *artifact* aplikasi) yang terimplementasi. Perbedaan ini perlu direkonsiliasi: apakah `/v1/register` akan jadi endpoint terpisah, atau digabung ke bawah `/v1/artifacts` dengan `resource_type` field.

---

## 6. Konfigurasi & Flags

`forma-ctl` adalah binary bersubcommand: `serve` menjalankan server (region/cluster/standalone); verb emergency (`freeze`, `revoke`, `key`, `policy`, `log`) adalah subcommand terpisah di binary yang sama (lihat `docs/cli-tools/04-forma-ctl.md`).

Flag CLI aktual hari ini (`cmd/forma-ctl serve`):

```bash
forma-ctl serve \
  --dev              # dev mode: signing key ephemeral, /v1/poll aktif, no mTLS
  --port 8443        # port HTTP
  --control-db PATH  # path DB (belum di-wire — lihat §7)
```

Subcommand emergency hari ini hanya stub — memanggilnya mencetak pesan "not implemented" dan exit 1 (lihat `docs/cli-tools/04-forma-ctl.md` §5):

```bash
forma-ctl freeze --reason "..."   # → "forma-ctl freeze: not implemented yet"
```

Flag target produksi untuk `serve` (mode-split belum ada di kode — lihat §7):

```bash
forma-ctl serve --mode=region --port=8443 --db=postgres://...
forma-ctl serve --mode=cluster --region-url=https://control.jakarta.forma.dev
forma-ctl serve --mode=standalone --port=8443 --db=sqlite:.forma/control.db
```

---

## 7. Status Implementasi Hari Ini

Bagian ini secara sengaja jujur tentang jarak antara desain di atas dan kode saat ini (`cmd/forma-ctl`, `internal/control`, `internal/artifact`), supaya dokumen ini berguna sebagai peta kerja, bukan cuma aspirasi:

1. **Subcommand dispatcher ada, tapi cuma `serve` yang nyata.** `cmd/forma-ctl/main.go` mengenali `serve` (fungsional, lihat di bawah) dan `freeze`/`revoke`/`key`/`policy`/`log` (langsung print "not implemented yet" dan exit 1 — bukan silent-fail, dan bukan pura-pura jalan). Ini sengaja: dispatcher sudah dibentuk sesuai desain akhir supaya penambahan verb emergency nanti tidak perlu merombak struktur CLI, tapi tidak ada logic emergency yang di-fake.
2. **Tidak ada split mode di `serve`.** `serve` hari ini adalah satu proses dengan flag `--dev` (bool) saja — tidak ada `--mode=region|cluster|standalone`. Semua fungsi region+cluster ada dalam satu `Server` struct.
3. **Storage in-memory saja.** `artifact.MemStore` adalah satu-satunya implementasi store. Flag `--control-db` diparse tapi **dibuang** (`_ = controlDB // TODO: wire SQLite/Postgres store`) — restart proses = kehilangan semua artifact terdaftar.
4. **Signing key ephemeral.** `NewDevSigner()` generate keypair ed25519 baru setiap kali proses start — tidak ada persistent key, apalagi HSM/KMS.
5. **`POST /v1/register` tidak ada** — lihat §5.3.
6. **Celah kritis: register tidak membuat Deployment.** `RegisterHandler.HandleRegister` menyimpan artifact dan menaikkan snapshot version, **tapi tidak pernah memanggil `Store.UpsertDeployment`**. Karena `SnapshotHandler` membangun snapshot murni dari `ListDeployments`, **`Deployments` selalu kosong** — pipeline `forma apply → forma-ctl → forma-resource` secara end-to-end **tidak berfungsi** hari ini, walau tiap leg (register, fetch, verify, evidence) berfungsi sendiri-sendiri secara terisolasi.
7. **Tidak ada OPA/policy engine, tidak ada transparency log, tidak ada mTLS** — semuanya target desain, belum ada baris kode. Ini juga alasan verb emergency (§6) belum diimplementasikan sungguhan — `policy test` dan `log verify` butuh keduanya lebih dulu ada.
8. **HTTP server pakai `net/http.ServeMux` polos**, bukan chi (berbeda dengan generator route di `resource/forma.go`/`internal/api` yang pakai chi) — routing manual prefix-strip untuk path param seperti `/v1/artifacts/{id}`.

### 7.1 Prioritas Perbaikan (untuk membuat pipeline berfungsi)

1. Tambahkan `store.UpsertDeployment` call di `RegisterHandler.HandleRegister` — ini yang paling mendesak, tanpa ini seluruh deployment flow adalah dead code.
2. Wire `--control-db` ke SQLite (dev) — minimal persistence sebelum multi-instance/HA jadi relevan.
3. Tambahkan `POST /v1/register` untuk resource registration (terpisah dari artifact registration), atau dokumentasikan keputusan untuk menggabungkannya.
4. Baru setelah dua di atas solid, pertimbangkan mode-split (`--mode=region/cluster/standalone`) sebagai flag eksplisit.

---

## 8. References

| Dokumen | Isi |
|---|---|
| `docs/architecture/01-architecture-overview.md` §1, §4 | Topologi multi-region, tiga level kontrol |
| `docs/architecture/03-deployment-flow.md` | Pipeline dua-stage lengkap (register → deploy) |
| `docs/architecture/04-resource-registration.md` | Registrasi node/datastore/cache |
| `docs/spec/platform/05-plane-protocol.md` | Wire protocol normatif |
| `docs/runtimes/02-forma-resource.md` | Sisi klien dari plane protocol (resource pod) |

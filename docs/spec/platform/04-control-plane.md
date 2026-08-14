# Control Plane

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Peran Control Plane
`formspec-control` adalah salah satu dari dua binary FormSpec (selalu proses
terpisah, termasuk saat development —
[`01-overview.md`](01-overview.md) §3). Ia menguasai: **Environment**
(identitas ditetapkan infrastruktur, immutable dari Resource Plane),
**Policy** (aturan deploy/approval/promotion/emergency, dievaluasi OPA
embedded), **kunci & signing** (kunci platform di HSM/Vault/KMS; kunci
owner didaftarkan, tidak pernah dipegang Control), **kontrak** (grant,
consent, license — ditandatangani dua pihak, portable, tercatat log),
**transparency log** (Merkle append-only dengan checkpoint dipublikasikan),
**lifecycle workspace & membership** (provisioning, suspensi, grant
impersonation — *lifecycle saja, tidak pernah isinya*).

**Larangan keras (normatif, tidak bisa dikonfigurasi lepas):** Control
Plane **tidak boleh** membaca data bisnis tenant atau mengeksekusi handler
bisnis. Resource Plane **tidak boleh** menulis balik ke Control Plane — ia
menarik policy saat boot dan tiap 5 menit lewat mTLS, tetap melayani
dengan policy terakhir yang diketahui kalau Control tidak terjangkau.
Compute Control **stateless**; seluruh state hidup di storage sendiri
(schema `formspec_control`), tidak pernah dibagi dengan schema aplikasi.

## 2. Kind Control Plane
| Kind | Concern |
|---|---|
| `Environment` | Satu target deployment: identitas, endpoint, pemetaan resource pool, tier, mode (dev/prod) |
| `Policy` | Aturan yang dievaluasi pada keputusan control-plane — lihat §5 |
| `Datastore` | Backend infrastruktur bernama — [`06-datastore.md`](06-datastore.md) |

```yaml
apiVersion: formspec.dev/v1
kind: Environment
metadata: { name: production }
spec:
  mode: prod                        # dev | prod
  tier: enterprise                  # standalone | cloud | enterprise
  resource_pool: shared             # dev: selalu shared. prod: shared (prod-shared) | exclusive
  resource_planes:
    - url: https://api.myapp.com
  key_ref: kms://prod-signing        # lokasi kunci signing platform
  policy: prod-policy                # kind: Policy yang berlaku untuk environment ini
```

Identitas Environment sampai ke Resource Plane hanya lewat infrastruktur
(`FORMA_ENVIRONMENT`, atau label namespace K8s) — **tidak pernah** lewat kode
aplikasi. Hanya ada dua mode — `dev` dan `prod`; "staging" bukan mode
ketiga, cuma nama konvensional untuk environment `mode: prod` dengan profil
Policy yang lebih longgar-tapi-tercatat.

## 3. Operasi
Siklus artifact: **Sign → Apply → Approve → Promote**.

```bash
formspec sign -f order.yaml --key ~/.formspec/keys/billing-team.key --environment staging
formspec apply -f order.yaml            # → approval request sesuai policy
formspec promote order --from staging --to production   # checksum sama diverifikasi
```

`staging` dan `production` di sini adalah *nama* environment — keduanya
`mode: prod`; yang membedakan cuma Policy yang berlaku pada masing-masing.
Approval adalah pernyataan bertanda tangan approver; seluruh rangkaiannya
satu chain di transparency log (§7).

**Consent gate** (deployment yang memperluas footprint): kalau versi baru
memperluas agregat `uses`/`required_permission`, menambah Subscription, atau
mengubah capability footprint Page, **re-consent Data Owner terdampak adalah
approval wajib** — mekanisme yang sama dengan approver manusia. Presentasi
consent normatif: layar **wajib** merender footprint dalam bahasa manusia
biasa ("versi app ini akan bisa membaca data billing Anda"), dengan detail
teknis yang bisa di-expand. Dalam budget cap yang dikonfigurasi, charge
berulang tidak re-prompt (top-up prepaid = persetujuan billing itu sendiri).

**Klasifikasi data.** Resource boleh mendeklarasikan `classification: public
| internal | confidential | restricted` — metadata governance; policy bisa
mengunci berdasarkan ini, implementasi wajib memblokir artifact membaca
resource di atas klasifikasi yang ia deklarasikan sendiri.

**Provisioning workspace:** `create → provisioning → seed default role +
reference seed → active` (emit `workspace.activated`); `suspend ⇄
reactivate`; `terminate`. Config per-workspace lewat
`ctx.tenant.config("key", default)`.

## 4. Model Keamanan
Dua kelas kunci, sengaja beda custody: **kunci owner** (workspace, app
vendor, module vendor) — dipegang owner sendiri (ed25519 self-custodied),
Control cuma menyimpan **public key**; **kunci platform** (per environment)
— di HSM/Vault/Cloud KMS lewat `key_ref`.

**Empat peran owner simetris** (satu per objek yang dimiliki; satu identitas
boleh pegang beberapa): Workspace Owner (data, user, billing), App Owner
(artifact app), Module Owner (package module), Cloud Owner (instance FormSpec
Cloud). Admin bertindak lewat **delegation certificate**: admin punya kunci
sendiri, owner menandatangani sertifikat (scope, masa berlaku, revocable) —
setiap tanda tangan admin wajib membawa sertifikatnya. **Non-delegable
(owner-only):** menerima/melepas ownership, menerbitkan/mencabut delegasi,
rotasi kunci owner.

**Transfer ownership:** *konsensual* (kontrak ditandatangani owner lama DAN
baru, Cloud Owner memfasilitasi & mencatat — bukan pemberi wewenang) atau
*recovery* (kematian/kunci hilang — dimediasi operator dengan proses baku:
re-atestasi identitas, masa tunggu wajib, notifikasi ke semua admin
terdaftar, entri transparency log publik). Reassignment sepihak oleh
operator **dilarang** (operator backdoor).

**Rotasi kunci owner adalah signed chain.** Registrasi kunci baru
ditandatangani oleh kunci lama, sehingga kelangsungan identitas bisa
diverifikasi tanpa mempercayai Control Plane — seluruh rantai rotasi tercatat di
transparency log (§7). Jalur *lost-key recovery* (saat rantai putus karena kunci
hilang) adalah re-atestasi identitas owner yang **dimediasi Platform Operator**
lewat proses baku di atas (masa tunggu wajib, notifikasi semua admin terdaftar,
entri log publik). Dalam kedua jalur, platform **tidak pernah** memegang salinan
cadangan kunci privat owner — tidak ada spare key untuk dipulihkan, yang
dipulihkan adalah identitas yang di-atestasi ulang, bukan kunci lama.

**Bentuk kunci owner default = passkey (WebAuthn).** Kunci privat owner
hidup di secure enclave perangkat (sinkronisasi iCloud/Google Keychain
menjawab kasus perangkat hilang); platform tetap **hanya** menyimpan public
key — self-custody utuh, UX-nya "login dengan sidik jari". Hash kontrak yang
ditandatangani di-*bind* ke challenge assertion, sehingga menandatangani =
notifikasi + tap + biometrik. ed25519 mentah lewat CLI tetap tersedia untuk
power user — **dua amplop tanda tangan, satu model kontrak**. Ini menegakkan
prinsip *non-technical owner*: kewajiban owner dikunci ke minimum (approve
consent, grant, billing di atas ambang, restore, penunjukan admin,
transfer), sisanya delegasi; layar consent **wajib** berbahasa manusia biasa
dengan detail teknis yang bisa di-expand (§3).

Batas Control ↔ Resource Plane: lihat §1 (larangan keras) dan
[`05-plane-protocol.md`](05-plane-protocol.md) §1 (dua channel asimetris).

## 5. `kind: Policy` — Structured YAML + Rego Escape Hatch

Vocabulary terstruktur untuk kasus governance umum, escape hatch Rego penuh
untuk kasus khusus — keduanya dikompilasi ke **satu engine evaluasi** (OPA
embedded sebagai Go library di dalam binary Control Plane, tanpa proses
tambahan), sehingga hanya ada satu decision log, bukan dua sistem paralel.

```yaml
apiVersion: formspec.dev/v1
kind: Policy
metadata: { name: prod-policy }
spec:
  require_signing: true
  require_staging_first: true
  require_approval:
    - { impl: [script_ref, script], approvers: 2, approver_roles: [tech-lead, module-owner] }
    - { impl: [native, compiled, sidecar], approvers: 3, approver_roles: [cto, tech-lead, security] }
  blocked:                    # policy floor — tidak bisa dikonfigurasi lepas
    - no_signature
    - environment_override_attempt
  rego: |                     # escape hatch — full OPA
    package formspec.deploy
    deny[msg] { input.time.weekday == "Friday"; input.time.hour >= 17; msg := "No production deploys on Friday evening" }
```

**Policy floor** (tidak bisa dikonfigurasi lepas, di tier manapun): tidak ada
self-approval (submitter ≠ approver); tidak ada artifact tanpa signature di
environment non-dev; tidak ada override identitas environment dari Resource
Plane. OPA hanya mengevaluasi keputusan governance — **tidak pernah** otorisasi
data bisnis, yang tetap jadi urusan `required_permission` di Resource Plane
(lihat `spec/backend/01-core-basic.md` §5).

**Input evaluasi policy.** Setiap keputusan policy dievaluasi terhadap dokumen
input yang tetap dan terenumerasi — sebuah policy (baik struktur maupun escape
hatch Rego) hanya boleh membaca dari sini, **tidak pernah** dari data bisnis
tenant:

- **Metadata artifact + checksum** — ID, versi, tipe impl, sha256 per file +
  envelope agregat.
- **Footprint terdeklarasi** — agregat `required_permission` + `uses` dari
  manifest ([`../backend/01-core-basic.md`](../backend/01-core-basic.md) §5),
  plus Subscription dan capability footprint Page yang ditambah/diperluas.
- **Identitas submitter** — siapa yang menjalankan `apply`/`promote`, beserta
  delegation certificate yang dibawanya (§4).
- **Approval sejauh ini** — approver yang sudah menandatangani (untuk policy
  multi-approver: mendeteksi self-approval dan menghitung kuorum terhadap
  `approver_roles`).
- **Target environment** — nama, mode (`dev`/`prod`), tier, profil policy.
- **Waktu sekarang** — untuk aturan berbasis jadwal (mis. no-deploy Jumat sore).
- **Riwayat staging/promotion** — apakah artifact ini sudah lewat staging, dan
  dari environment mana ia dipromosikan (checksum yang sama diverifikasi, §3).
- **Tag klasifikasi artifact** — `public`/`internal`/`confidential`/`restricted`
  (§3).

Verifikasi: `formspec-ctl policy test` menjalankan table-driven test terhadap
policy yang sudah dikompilasi ke Rego, dipakai sebelum perubahan `kind: Policy`
diterapkan.

## 6. Contracts — Grants, Consents, Licenses
Satu model dokumen untuk ketiganya: **kontrak adalah dokumen portable
bertanda tangan** — isi + tanda tangan kedua pihak (kunci owner) + timestamp.
Kedua pihak menyimpan salinan; Control Plane juga menyimpan satu dan
menjangkarkannya di transparency log (§7) dengan inclusion proof yang bisa
diambil pihak manapun.

- **Grant** (lintas-app): diminta konsumen, ditandatangani Data Owner
  provider; pencabutan adalah kontrak bertanda tangan & tercatat juga.
  Rekaman metering merujuk grant ID.
- **Consent**: footprint saat instalasi, dan re-consent saat footprint
  berubah (§3).
- **License token**: divalidasi lokal oleh Resource Plane, tanpa call-home,
  air-gap safe. **Wajib tidak boleh** menggerbang `list/find/export/backup`
  — implementasi wajib menolak token yang mencoba itu (lihat
  [`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §3).
- Portabilitas adalah tulang punggung Credible Exit: kontrak membuktikan
  dirinya ke operator lain yang konform tanpa butuh kerja sama operator lama.

### 6.1 Backup & Restore Governance
| Objek | Diatur oleh |
|---|---|
| Data workspace (entity, storage, preference, config) | Workspace Owner — jadwal, retention, scope, **target eksternal** |
| Artifact app & module | Reproducible dari git + registry; retention registry = Cloud Owner |
| Kontrak, transparency log, membership | Cloud Owner + tiap pihak menyimpan salinan sendiri |

**Infra durability ≠ logical backup.** Replikasi tingkat infrastruktur dan
snapshot storage (disk snapshot cloud, replikasi DB) **bukan** pengganti kontrak
*logical backup*
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §3) —
keduanya tanggung jawab infra Cloud Owner yang digerakkan SLA, **ortogonal**
terhadap kontrak backup/restore tingkat app di tabel ini. Durabilitas infra
melindungi dari kegagalan perangkat; logical backup memberi Credible Exit dan
restore selektif — satu tidak menggantikan yang lain.

Aturan normatif: `backup create` boleh didelegasikan; **`restore` yang
menimpa data wajib tanda tangan owner atau delegasi ber-scope
`backup.restore` eksplisit**, selalu tercatat di transparency log. Workspace
Owner **boleh** menetapkan target backup eksternal yang ia kuasai sendiri —
wujud hidup dari Credible Exit, **tidak boleh** license-gated. Backup
terenkripsi: default kunci platform per-tenant, opsi **kunci yang disediakan
owner sendiri**.

Ini melengkapi tangga eskalasi `ctx.db` di
[`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §5 —
bagian *governance* siapa-boleh-apa atas backup/restore ada di sini, bagian
*kemampuan teknis* PersistBackend (format backup, mode restore) ada di sana.

**Konsistensi backup & rekonsiliasi outbox.** Konsistensi backup dijamin **per
batas transaksi App** — setiap App di-backup pada boundary transaksinya sendiri.
Backup lintas-App karena itu hanya *near-point-in-time*, **bukan** snapshot
atomik tunggal melintasi semua App. Konsekuensinya normatif: **restore dari
backup WAJIB diikuti pass rekonsiliasi outbox** — entri outbox yang masih
pending di-replay/diverifikasi terhadap state yang direstore sebelum workspace
kembali melayani. Ini MUST, bukan pembersihan opsional.

**Vendor provider-app** — vendor yang menjalankan provider app
([`../backend/04-persist-backend.md`](../backend/04-persist-backend.md) §5) —
tunduk pada governance dan tanggung jawab backup yang **sama** dengan Workspace
Owner manapun atas provider workspace mereka sendiri. Tidak ada pengecualian
khusus karena statusnya sebagai vendor.

## 7. Transparency Log

Merkle tree **append-only** atas: apply, approval, rejection, promotion,
rotasi key, kontrak, aksi emergency, sesi impersonation, dan sesi REPL
production. Setiap entri menghasilkan inclusion proof; tree head (checkpoint)
ditandatangani platform key dan **dipublikasikan di luar kendali operator**
(mirror pihak ketiga atau endpoint publik) pada kadensi tetap.

**Opsi timestamp RFC 3161.** Checkpoint **boleh** di-externalisasi tambahan
lewat RFC 3161 timestamp authority sebagai opsi verifikasi-independen ekstra — di
samping publikasi mirror/endpoint publik di atas. Ini memberi bukti waktu yang
bisa diverifikasi pihak ketiga tanpa mempercayai jam platform, dan bersifat
melengkapi (bukan menggantikan) mekanisme externalisasi yang sudah ada.

Verifikasi independen: `formspec-ctl log verify --checkpoint <file>` membuktikan
tidak ada perubahan sejarah (history rewrite) sejak checkpoint tertentu.

## 8. REPL Governance

Scope `formspec repl` mengikuti profil kebijakan environment tempat ia dijalankan:

| Environment | Akses REPL |
|---|---|
| dev-mode | Read-write penuh |
| prod-mode, profil staging | Read-write, sesi direkam di audit log |
| prod-mode, profil production | **Read-only** — write butuh persetujuan policy eksplisit yang time-boxed; setiap sesi tercatat di transparency log |

REPL selalu berjalan di bawah identitas user nyata dengan permission user
tersebut — **tidak pernah** superuser shell.

**Semantik "production read-only" secara presisi.** REPL production adalah
*inspect-only*: hanya akses `ctx.db` tier-baca, **tanpa** method mutasi apa pun,
**tanpa** kapabilitas emit `ctx.queue`/`ctx.pubsub`. Artinya REPL production
tidak pernah bisa memicu efek samping — hanya membaca dan memeriksa. Write
(termasuk emit queue/pubsub) baru terbuka lewat persetujuan policy eksplisit yang
time-boxed dari tabel di atas, yang mengangkat pembatasan ini hanya untuk sesi
bersangkutan dan tetap tercatat di transparency log.

## 9. Emergency Controls

Dua permukaan darurat untuk dua sisi yang berbeda, sengaja tidak disatukan:

| | Resource Plane (`formspec freeze`/`rollback`/`lock workspace`) | Control Plane (`formspec-ctl freeze`/`revoke sessions`/`key rotate`) |
|---|---|---|
| Dijalankan oleh | App Admin yang diotorisasi | Cloud Owner / Platform Operator |
| Scope | Satu workspace/app | Seluruh environment |

Setiap aksi darurat **wajib** menyertakan alasan, ditandatangani aktor, dan
tercatat di transparency log — darurat bukan alasan melewati audit.

**Lever bedah — `formspec suspend scripts --all`.** Di antara freeze penuh dan
operasi normal ada tuas darurat yang lebih sempit: `formspec suspend scripts --all`
menghentikan **seluruh eksekusi handler berskrip (Starlark)** se-workspace
sambil engine **tetap** melayani traffic read/CRUD secara normal. Dipakai saat
sumber masalah adalah logika scripted (hook, action) tapi data path inti sehat —
mematikan handler tanpa membekukan workspace seluruhnya. Sama seperti aksi
darurat lain: **wajib** `--reason`, ditandatangani aktor, tercatat di
transparency log.

**Prinsip desain "bedrock exception":** entrypoint darurat sisi Control Plane
(`formspec-ctl`) wajib berupa kode konvensional yang di-compile langsung ke dalam
binary yang sama dengan proses `serve` — bukan proses/binary terpisah yang
bergantung pada komponen yang sedang ia perbaiki (database artifact, policy
engine, dsb). Kalau alat darurat butuh sistem yang sedang rusak untuk bisa
dipakai, ia gagal tepat saat paling dibutuhkan. Jalur eksekusinya konvensional/
imperatif, bukan lewat evaluasi OPA/Rego yang mungkin justru jadi sumber
masalah. Konsol resmi (`formspec/console`, `formspec/studio`, `formspec/ops`) tidak
tunduk pada aturan ini — mereka aplikasi FormSpec biasa di atas Control Plane;
pengecualian ini hanya berlaku untuk `formspec-ctl`.

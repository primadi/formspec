# Core Extended

**Version:** 0.1.0 · **Status:** Draft

> Draft: isi di bawah kontrak yang berlaku.

## 1. Lifecycle & State Machine
FormSpec punya model status **dua lapis**, independen satu sama lain:

1. **`doc_status`** — lifecycle bawaan, framework-enforced
   ([`01-core-basic.md`](01-core-basic.md) §1.2). Closed set: `draft |
   submitted | cancelled`. Tidak bisa ditambah nilai baru — kebutuhan proses
   granular pakai layer kedua.
2. **State machine bisnis** — didefinisikan developer di field terpisah:

```yaml
state_machine:
  field: fulfillment_stage        # field bisnis, independen dari doc_status
  initial: awaiting_payment
  transitions:
    - from: awaiting_payment
      to: paid
      via: mark-paid              # nama action
      guard: "doc_status == 'submitted'"
    - from: paid
      to: fulfilled
      via: fulfill
```

Tidak ada `maps_to` antara field bisnis dan `doc_status` — relasi antar
keduanya (kalau dibutuhkan) diekspresikan lewat `conditions` biasa di action,
bukan mekanisme baru. Transisi yang tidak dideklarasikan → ditolak dengan
`STATE_TRANSITION_ERROR`. Guard adalah Starlark inline. Selain perbandingan
field biasa, guard boleh memanggil sekumpulan **builtin agregat-atas-child**
untuk pola umum atas koleksi child: `sum_line(field)` (menjumlahkan field
numerik pada seluruh child record), `len(resource.items)` (jumlah record pada
koleksi child), dan builtin agregat sejenis — sehingga guard seperti "total
baris harus > 0 sebelum boleh submit" bisa ditulis tanpa handler terpisah.
Approval berbasis role atas transisi adalah `kind: Workflow` (§2) — bukan
bagian state machine dasar.

### 1.1 Denormalisasi Field Finansial (Normatif)

Field `characteristic: master` yang nilainya memengaruhi perhitungan
finansial pada Entity transaksi (harga, diskon, tier, tarif pajak)
**wajib** disalin (snapshot) ke Entity transaksi saat `create`/`submit` —
**tidak boleh** dibaca ulang lewat live-join ke Master saat transaksi
ditampilkan atau dihitung ulang. Kalau nilai Master berubah kemudian (mis.
customer naik tier bulan depan), transaksi lama yang sudah dibuat **tidak
boleh** ikut berubah retroaktif. Field non-finansial pada relasi yang sama
(nama, alamat) boleh tetap live-join. Aturan praktis: setiap field Master
yang dipakai dalam `conditions`/kalkulasi total pada action transaksi
disalin sebagai field milik Entity transaksi sendiri (konvensi penamaan
bebas, mis. suffix `_at_transaction`). Ini berbeda dari master snapshot
saat archiving (§10) — snapshot finansial ini terjadi *setiap transaksi*,
bukan cuma saat archive run.

## 2. Workflow
Lifecycle sederhana cukup inline di Entity (§1). Approval berbasis role
hidup di `kind: Workflow` dan **menempel tanpa mengubah Entity** — pola yang
sama dengan Subscription (§3), diterapkan ke transisi state machine:

```yaml
apiVersion: formspec.dev/v1
kind: Workflow
metadata: { name: journal-posting-approval, module: gl }
spec:
  entity: gl.journal-entry
  on: { transition: { from: draft, to: posted } }
  steps:
    - { roles: [gl.supervisor], approvers: 1 }
    - { roles: [gl.controller], approvers: 1,
        when: "resource.amount > 100000000" }
  on_reject: { to: rejected }
  escalation: { after: 48h, notify_roles: [gl.manager] }
```

Transisi yang di-intercept baru eksekusi setelah seluruh step yang berlaku
mencapai quorum-nya; approval adalah pernyataan bertanda tangan yang tercatat
di audit. Eligibilitas approver = keanggotaan role per-App. **Pemohon tidak
pernah bisa menyetujui permintaannya sendiri.** Workflow selalu tampil di
output gabungan `formspec describe document` — perilaku yang menempel selalu
ter-compile, tidak pernah tersembunyi.

### 2.1 Multi-Approver & Percabangan per Step

Satu step mendeklarasikan **berapa banyak** persetujuan yang dibutuhkan dan
**bagaimana** persetujuan itu dikumpulkan:

- `approvers: N` — kuorum: jumlah persetujuan berbeda yang harus terkumpul agar
  step lolos (default `1`). Approver yang menghitung wajib memenuhi
  `roles`-nya. `quorum` diterima sebagai alias `approvers`.
- `mode` — cara kuorum dikumpulkan di dalam satu step:

  | `mode` | Arti | Contoh |
  |---|---|---|
  | `all` (default) | Semua approver yang berhak wajib menyetujui — kuorum = jumlah yang berhak | Dua direktur wajib tanda tangan bersama |
  | `any` | Cukup `approvers: N` dari kumpulan yang berhak (mana pun) | Salah satu dari tiga manajer cukup |
  | `sequential` | Approver menyetujui **berurutan** sesuai urutan `roles`; approver berikutnya baru bisa bertindak setelah yang sebelumnya | Rantai atasan berjenjang |

  `mode` mengatur pengumpulan **di dalam** satu step; urutan **antar** step selalu
  berurutan (step 2 tidak mulai sebelum step 1 lolos).

- `when` — kondisi FormSpecExpr atas `resource`: step hanya berlaku bila `when`
  bernilai true (mis. `resource.amount > 100000000`). Step yang tidak berlaku
  di-skip tanpa menahan transisi.

**Timeout & eskalasi.** `escalation.after` menandai durasi diam sebelum step
dieskalasi; `notify_roles` diberi tahu, dan `reassign_roles` (opsional)
memindahkan hak persetujuan ke role lain setelah durasi itu lewat — sehingga
approval tidak menggantung selamanya karena satu orang cuti. Eskalasi,
reassignment, setiap persetujuan, dan setiap penolakan **wajib** tercatat di
audit trail bisnis (§11) — siapa, kapan, keputusan apa.

```yaml
# Purchase order — dua approver paralel, salah satu jalur eskalasi
kind: Workflow
metadata: { name: po-approval, module: procurement }
spec:
  entity: procurement.purchase-order
  on: { transition: { from: draft, to: approved } }
  steps:
    - roles: [procurement.manager]
      approvers: 2                 # kuorum dua manajer
      mode: any                    # dua mana pun dari kumpulan yang berhak
      escalation: { after: 24h, reassign_roles: [procurement.head] }
  on_reject: { to: rejected }

# Leave request — rantai berjenjang (atasan → HR)
kind: Workflow
metadata: { name: leave-approval, module: hr }
spec:
  entity: hr.leave-request
  on: { transition: { from: submitted, to: approved } }
  steps:
    - { roles: [hr.line-manager], approvers: 1 }
    - { roles: [hr.hr-officer],   approvers: 1,
        when: "resource.days > 5" }         # cuti panjang butuh HR
  on_reject: { to: rejected }
```

## 3. Subscription & Event Delivery
`kind: Subscription` ([`01-core-basic.md`](01-core-basic.md) §7) punya dua
tier:

| | Tier 1 — Core (outbox) | Tier 2 — Streaming |
|---|---|---|
| Storage | Outbox PersistBackend | Redis Stream / Kafka |
| Konsistensi | Transaksional | At-least-once, positioned replay |
| Fan-out | Satu target per entry | Banyak subscriber |
| Pemakaian | GL, billing, inventory | Analytics, audit, monitoring |

Tier 2 menambah `durability: durable` dengan `store`, `retention`, `position`,
`max_retry`, `dead_letter`, plus `filter`/`transform` Starlark atas payload
event (`event.name`, `event.resource_id`, `event.occurred_at`, field payload
event itu sendiri). **Subscription dinamis** (dibuat runtime lewat API/admin
panel) adalah *data, bukan manifest* — manifest Subscription mendefinisikan
apa yang ikut ter-ship bersama module, subscription dinamis mencatat pilihan
operator, hidup di `formspec.core`.

**Delivery channel** yang tersedia (di luar `queue`/`websocket`/`audit_log`
Core Basic): `webhook` (keluar ke subscriber terdaftar, HMAC signed, retry),
`notification` (bridge tipis ke module `formspec/notify` — template & channel
provider live di module resmi, bukan di kontrak ini), `pubsub` (non-durable,
at-most-once eksplisit).

**`emits:` — event kustom sebagai event source.** Selain event lifecycle
reserved (`before_*`/`on_*`, [`01-core-basic.md`](01-core-basic.md) §7), sebuah
action kustom boleh menautkan dirinya ke satu event bernama lewat keyword
`emits: <event-name>` — event itu dipancarkan **saat action sukses** (dengan
semantik durabilitas yang sama seperti event lain, [`01-core-basic.md`](01-core-basic.md)
§7). Nama event yang di-`emits` menjadi event source yang bisa dilanggan
`kind: Subscription` persis seperti event lifecycle, sehingga module lain
bereaksi terhadap peristiwa bisnis bernama tanpa perlu menebak action mana yang
memicunya.

## 4. Webhook
`kind: Webhook` — endpoint masuk yang **diverifikasi sebelum handler
berjalan**; handler cuma pernah melihat payload yang sudah terverifikasi:

```yaml
apiVersion: formspec.dev/v1
kind: Webhook
metadata: { name: midtrans-webhook, module: billing }
spec:
  for: payment-gateway.webhook       # Service action yang menangani
  method: POST
  path: /webhooks/midtrans           # auto-derive kalau tidak diisi
  auth:
    strategy: signature              # signature | token
    signature:
      algorithm: hmac-sha512
      header: X-Midtrans-Signature
      key: { config: midtrans.server_key, secret: true }
      payload: raw_body
  idempotent: true
  idempotency_key: { from: payload, field: transaction_id }
```

`spec.for` wajib merujuk satu Service action. Verifikasi gagal → ditolak
sebelum handler manapun berjalan, terhitung, bisa dialert. Strategi `token`
untuk webhook internal sederhana; `signature` untuk provider kriptografi.

## 5. Integrator
`kind: Integrator` menjembatani dua Entity/Module yang **tidak saling kenal
langsung** — konsisten dengan prinsip "module tidak saling `import` definisi
satu sama lain":

```yaml
kind: Integrator
name: invoice-to-gl
listen:
  resource: billing.invoice
  event: before_cancel
call:
  resource: gl.journal-entry
  action: cancel
compensate: recreate_gl_journal   # opsional; framework yang memutuskan kapan dipanggil
```

`listen.resource`/`call.resource` di-resolve lewat registry — Integrator tidak
pernah `import` definisi Invoice/JournalEntry secara langsung.

**Aturan wajib:** setiap Integrator yang membuat efek samping dari satu event
**wajib** juga menyediakan handler simetris untuk event pembatalannya —
tanpa itu, cancel di sisi source akan terblokir permanen karena reference
guard generik selalu memblokir tanpa ada yang tahu cara membuka jalannya.

Target action **wajib** `idempotent: true` untuk pemanggilan cross-boundary
(dataspace/proses berbeda) — `formspec apply` menolak Integrator yang menyasar
action non-idempotent. Same-transaction call tidak butuh `compensate` (ACID
rollback sudah cukup); cross-boundary call mendaftarkan `compensate` ke Saga
log.

**Service action `call: async` (fire-and-forget).** Action `type: service`
([`01-core-basic.md`](01-core-basic.md) §1.1) boleh dideklarasikan
`call: async` untuk semantik **satu arah, fire-and-forget**: pemanggil tidak
menunggu dan tidak menerima hasil apa pun, cocok untuk efek samping bergaya
notifikasi (mis. mengirim pesan WhatsApp). Ini **berbeda tegas** dari async job
yang di-track di §13 — yang mengembalikan `job_id` dan melaporkan progres;
fire-and-forget tidak punya job_id, tidak punya kanal progres, dan tidak punya
kontrak hasil. Karena tidak ada hasil yang dinanti, keandalan pengirimannya
bergantung pada channel delivery yang dipilih (durable vs non-durable, §3),
bukan pada return value.

## 6. Summary & Multi-Source
Kontrak "gabungkan sources by join_key" — cara memenuhi kontrak ini adalah
urusan masing-masing PersistBackend (lihat
[`../../renderers/jsonb-persist/04-query-and-keys.md`](../../renderers/jsonb-persist/04-query-and-keys.md)
untuk jawaban konkret jsonb-persist).

`characteristic: summary` diisi **eksklusif** lewat event durable — bukan
lewat action call biasa dari luar (`create`/`update`/`delete` permanen
nonaktif via API, §1 Core Basic). Rebuild: `formspec summary rebuild <document>`
me-replay event stream sumbernya ke projeksi baru — inilah alasan backup
mengecualikan Summary (selalu bisa dihitung ulang selama transaksi sumbernya
masih queryable, live maupun via archive).

## 7. Named Scripts & Cross-Module Starlark
Script Starlark yang dirujuk lewat `impl.script_ref`/`ref` (mis.
`ref: billing/invoice_send`) memakai notasi qualifier yang sama dengan
referensi lintas-module lainnya di spec ini (`module/resource`, konsisten
dengan `sources.resource` dan qualifier entity App — lihat
[`../frontend/01-visual-hierarchy.md`](../frontend/01-visual-hierarchy.md)
untuk latar belakang notasinya). Referensi di dalam module sendiri tetap
tanpa qualifier (`ref: invoice_send`, konteksnya sudah jelas satu module);
qualifier `{module}/{script-name}` dibutuhkan saat script dirujuk dari module
lain — visibilitasnya tunduk aturan `uses` yang sama seperti pemanggilan
resource lintas-module (`01-core-basic.md` §5): module pemanggil wajib
mendeklarasikan akses itu, muncul di consent footprint-nya.

Permukaan API yang tersedia di dalam script (entrypoint, objek `resource`,
`ctx.*`, kontrak return) dikatalogkan normatif di
[`06-script-runtime.md`](06-script-runtime.md).

### 7.1 Batas Sandbox Starlark (Normatif)

Setiap eksekusi Starlark berjalan di dalam sandbox dengan **batas keras** yang
ditegakkan engine — bukan sekadar rekomendasi:

| Batas | Nilai |
|---|---|
| Wall-clock | 5000 ms |
| Memori | 64 MB |
| Iterasi | 100.000 |
| Query DB per eksekusi | maks. 50 |
| Record dibaca per eksekusi | maks. 1.000 |
| Jaringan / filesystem / subprocess | **tidak ada akses** |

Melewati **salah satu** batas **membatalkan** eksekusi dengan error — **tidak
pernah** mengembalikan hasil parsial. Batas ini menjaga satu script yang
melar tidak menyandera worker atau menguras datastore; kebutuhan volume di atas
batas ini adalah pekerjaan `type: service`/native (`impl.type: native`,
[`06-script-runtime.md`](06-script-runtime.md)) atau async job yang di-track
(§13), bukan pelonggaran sandbox.

## 8. Mockup & Environment Binding

`kind: Mockup` mengimplementasikan kontrak yang sama dengan konektor asli
(mis. payment gateway) — pemanggil tidak pernah tahu mana yang menjawab.

**Environment binding bersifat normatif:** business handler **tidak pernah**
bercabang berdasarkan environment. Routing ke Mockup vs koneksi asli murni
config-driven lewat `kind: Config` (`mock_enabled: true`, default `true` di
dev/CI; `false` → konektor asli). `ctx.environment` hanya boleh dipakai untuk
logging — bukan untuk keputusan bisnis. `formspec validate` SHOULD memperingatkan
bila ditemukan percabangan bisnis atas `ctx.environment` di script.

## 9. Period Closing & Backdating

Governing prose untuk kode `FORMSPEC.PERIOD.*` dan `FORMSPEC.TXN.*`
([`error-glossary.yaml`](error-glossary.yaml)). Semua guard di bawah ditegakkan
**server-side, selalu** ([`01-core-basic.md`](01-core-basic.md) §3) — tidak
peduli klien mengirim lewat HTTP, script, atau event.

### 9.1 `transaction_date` vs `created_at`

Untuk `characteristic: transaction`, field `transaction_date` **wajib**
dideklarasikan eksplisit ([`01-core-basic.md`](01-core-basic.md) §1.2). Keduanya
tanggal, perannya berbeda dan tidak boleh tertukar:

| | `created_at` (tanggal sistem) | `transaction_date` (tanggal bisnis) |
|---|---|---|
| Fungsi | Urutan kejadian nyata, audit | Periode akuntansi/pelaporan mana yang mengakuinya |
| Bisa dimanipulasi? | Tidak | Ya, tunduk `backdate_policy`/`forward_date_policy` |
| Dipakai untuk sequencing/audit? | **Selalu** | **Tidak pernah** |

Sequencing dan audit **selalu** memakai `created_at`. Memakai
`transaction_date` untuk sequencing memicu recompute berantai saat backdate —
masalah mahal di dunia nyata.

### 9.2 Backdate & Forward-date

Seberapa jauh `transaction_date` boleh mundur/maju dari hari ini diatur global
dengan default konservatif, dapat di-override per-resource:

```yaml
# Default global (namespace settings.*, 01 §10)
settings:
  transaction_defaults:
    backdate_policy:
      max_days_back: 3
      override_permission: null                 # null = tidak ada yang boleh override
    forward_date_policy:
      max_days_forward: 0                        # default paling konservatif
      override_permission: accounting.post_forward_dated
    period_guard: { enabled: true }

# Override per-resource
spec:
  backdate_policy: { max_days_back: 7, override_permission: accounting.post_backdated }
```

`transaction_date` yang mundur melebihi `max_days_back` → `FORMSPEC.TXN.BACKDATE_EXCEEDED`;
yang maju melebihi `max_days_forward` → `FORMSPEC.TXN.FORWARD_DATE_EXCEEDED`.
Override hanya mungkin bila pemanggil memegang `override_permission` yang
dideklarasikan; `null` berarti mutlak tidak bisa di-override.

### 9.3 Period Closing

Period boleh ditutup **per module maupun global** — konfigurasinya hidup di
`formspec.core` (`kind: Config`), sejalan dengan awal tahun fiskal
(`settings.fiscal_year_start`, [`01-core-basic.md`](01-core-basic.md) §10) yang
menentukan batas periode. Transaksi dengan `transaction_date` yang jatuh di
periode tertutup **ditolak** dengan `FORMSPEC.PERIOD.CLOSED` — berlaku untuk
`create`, `update`, `submit`, maupun `amend` yang menyentuh periode itu.

Period closing itu sendiri dimodelkan sebagai **Entity** (`period-closing`),
bukan sekadar perintah CLI — sehingga otomatis mendapat `doc_status`, reference
guard, audit trail, dan model permission gratis. `submit` di dokumen ini
memicu finalisasi summary periode; `cancel` (reopen) memicu unfinalize.

**Reopen butuh permission elevated + audit.** Membuka kembali periode tertutup
mensyaratkan permission khusus (mis. `accounting.reopen_period`) dan alasan
tercatat; tanpa keduanya → `FORMSPEC.PERIOD.REOPEN_DENIED`. Setiap penutupan dan
pembukaan periode masuk audit trail bisnis (§11).

### 9.4 Resolusi `today`/`current` dari Kalender Bisnis

Shortcut periode pada Summary (§6, mis. "bulan berjalan") **resolve dari
kalender bisnis** — tanggal EOD (end-of-day) terakhir yang sudah closed,
+1 — **bukan** dari jam sistem operasi. Ini menjaga perhitungan periode
tetap benar saat proses EOD tertunda (sistem sudah menunjukkan tanggal 5,
tapi bisnis belum menutup tanggal 4 — maka "hari ini" bisnis tetaplah
tanggal 4 sampai EOD-nya selesai).

## 10. Data Archiving & Retention

Governing prose untuk kode `FORMSPEC.ARCHIVE.*`
([`error-glossary.yaml`](error-glossary.yaml)). Verb CLI-nya di
[`../../cli-tools/02-formspec-cli.md`](../../cli-tools/02-formspec-cli.md)
(`archive run|view|restore-batch`).

**Hanya transaksi yang diarsipkan; master di-snapshot** demi konsistensi
temporal. Prinsipnya: arsip view-only harus swasembada dan konsisten — tidak
boleh query DB live. Saat transaksi lama diarsipkan, master yang direferensikan
di-snapshot "as-of" tanggal arsip dan disimpan bersama transaksinya.

**Apa yang diarsipkan:**
- **Transaksi** (`characteristic: transaction`) — selalu diarsipkan saat umur ≥
  cutoff `retention.archive_after` (dihitung dari `transaction_date`): Invoice,
  Payment, Journal Entry, Purchase Order, Stock Movement, dst.
- **Master** (`characteristic: master`) — **hanya di-snapshot** bila
  direferensikan transaksi yang diarsipkan. Baris master di produksi tetap utuh
  (tidak dihapus) dan ditandai `locked_for_deletion = true` selama masih ada
  transaksi terarsip yang menunjuknya.
- **Summary** (`characteristic: summary`) — **tidak pernah** diarsipkan; selama
  transaksi sumbernya masih queryable (live maupun via archive) ia bisa dihitung
  ulang (§6).

**Rencana arsip (`formspec archive run`).** `--dry-run` memindai transaksi di atas
cutoff, mengidentifikasi master yang direferensikan, dan menampilkan rencana
(apa yang diarsipkan, master apa yang di-snapshot) untuk konfirmasi operator.
Eksekusi menulis transaksi + snapshot master ke Parquet, menyetel flag
`locked_for_deletion` pada master yang direferensikan, lalu menghapus baris
transaksi terarsip dari produksi.

**Data terarsip terkunci dari penghapusan.** Selama sebuah master masih
direferensikan transaksi terarsip, upaya menghapusnya → `FORMSPEC.ARCHIVE.LOCKED_FOR_DELETION`
(dengan `archived_reference_count`). Ini memperluas reference guard `delete`
([`01-core-basic.md`](01-core-basic.md) §1.2) ke referensi yang sudah pindah ke
arsip — dangling reference tidak boleh terbentuk hanya karena transaksi
penunjuknya diarsipkan.

**Restore.** `formspec archive view --batch-id <id>` mengueri Parquet langsung
tanpa DB live. `formspec archive restore-batch` hanya me-restore **ke staging**,
dengan urutan restore mengikuti dependency; restore selektif per-dokumen tidak
didukung (risiko state korup).

**Retention config (global, `formspec.yaml`):**

```yaml
retention:
  archive_after: "3y"           # dihitung dari transaction_date
  strategy: cold_storage        # cold_storage | delete
  destination: s3://archive-bucket
```

Dokumen boleh opt-out lewat `retention: { disabled: true }`.

## 11. Business Audit Trail

`audit: true` pada sebuah action mengaktifkan pencatatan **audit bisnis**.
Kontraknya normatif; ini sumber "timeline" per-record yang dilihat pengguna.

**Yang direkam per entri:**
- **Actor** — identitas pemanggil (`ctx.user.id`), tenant, request ID.
- **Action** — nama action yang dijalankan (bukan "document updated" generik;
  action bernama tercatat *dengan namanya*, [`01-core-basic.md`](01-core-basic.md)
  §1.2).
- **Timestamp** — waktu kejadian (`created_at`, bukan `transaction_date`, §9.1).
- **Before/after diff** — untuk action kelas-update, snapshot nilai field
  sebelum dan sesudah; untuk create, hanya after; untuk delete/cancel, before +
  transisi status.

**Immutability.** Audit trail bersifat **append-only** — tidak ada API
update/delete atasnya; framework yang menulis, kode developer tidak pernah
memutasi entri yang sudah ada. Ini konsisten dengan `audit-log` di `formspec.core`
([`01-core-basic.md`](01-core-basic.md) §7 channel `audit_log`).

**Queryable per record.** Entri bisa ditarik per record (menjadi sumber blok
timeline di UI) maupun difilter lintas record dengan operator query standar
([`01-core-basic.md`](01-core-basic.md) §6). Retensi audit dapat dikonfigurasi
global; menghapus entri lama tunduk retention, bukan aksi manual sembarang.

**Berbeda tegas dari transparency log governance.** Audit trail bisnis mencatat
**data bisnis** (siapa mengubah invoice apa) dan dimiliki Workspace Owner.
**Transparency log** platform ([`../platform/04-control-plane.md`](../platform/04-control-plane.md)
§7) adalah Merkle append-only atas peristiwa *governance* (apply, approval,
rotasi key, emergency, sesi REPL production) — ia **tidak pernah** memuat data
bisnis. Keduanya append-only tapi domainnya terpisah dan tidak saling
menggantikan.

## 12. kind: Api — Override Permukaan External

`kind: Api` meng-override **permukaan external** (`/api/v1/`) sebuah entity
([`../platform/03-kind-system.md`](../platform/03-kind-system.md)) — ia tidak
membuat exposure baru (itu `spec.expose`, [`01-core-basic.md`](01-core-basic.md)
§8.4), melainkan menyetel bagaimana permukaan external yang sudah opt-in itu
dipublikasikan. Permukaan UI (`/_ui/entity/`) **tidak** terpengaruh oleh
`kind: Api` — path UI mengikuti konvensi `{module}/{entity}` tetap, tidak bisa
di-override.

```yaml
apiVersion: formspec.dev/v1
kind: Api
metadata: { name: public, module: billing }
spec:
  rest:
    base_path: /public              # override prefix di bawah workspace prefix
    version: v2                     # override {version} route (01 §8)
    disable: [invoice]              # opt-out per-entity dari permukaan REST ini
  grpc:
    enabled: true
    package: billing.public.v2
```

Body override (hanya berlaku untuk permukaan external `/api/v1/`):

| Field | Arti |
|---|---|
| `rest.base_path` | Segmen path yang menggantikan `{module}` di route external ([`01-core-basic.md`](01-core-basic.md) §8.2). Tidak memengaruhi `/_ui/entity/`. |
| `rest.version` | Menyetel `{version}` route untuk permukaan external ini |
| `rest.disable` | Daftar entity yang **opt-out** dari permukaan external REST ini. Entity tetap bisa diakses via UI (`/_ui/entity/`) selama permission terpenuhi. |
| `grpc.enabled` | Mengaktifkan permukaan gRPC (external) |
| `grpc.package` | Nama package proto untuk permukaan gRPC |

**Beberapa permukaan Api bernama per module.** Satu module boleh punya lebih dari
satu `kind: Api` (dibedakan `metadata.name`, mis. `public` vs `partner`), masing
dengan `base_path`/`version`/exposure sendiri — sehingga satu entity yang sama
bisa tampil beda di permukaan `partner` (path/version berbeda, sebagian entity
di-`disable`) dibanding permukaan `public`. Deskriptor route tetap
protocol-agnostic ([`01-core-basic.md`](01-core-basic.md) §8.3); `kind: Api` hanya
mengatur bagaimana deskriptor itu dipublikasikan per permukaan external.

## 13. Async Action & Job Tracking

Action `call: async` (untuk kerja yang di-track dengan hasil, berbeda dari
fire-and-forget §5) **langsung mengembalikan `202`** tanpa menunggu handler
selesai:

```jsonc
// Respons 202 seketika (envelope 01 §8)
{
  "data": { "job_id": "job_01H...", "status": "pending" },
  "meta": {
    "track": {
      "websocket_event": "jobs",             // kanal untuk progres/hasil
      "poll_url": "/.../jobs/job_01H..."     // alternatif polling
    }
  }
}
```

Progres dan penyelesaian didorong di kanal `jobs` sebagai event bernama
`progress` | `completed` | `failed`:

```jsonc
{ "event": "progress",  "job_id": "job_01H...", "progress": 40, "message": "processing batch 2/5" }
{ "event": "completed", "job_id": "job_01H...", "status": "completed", "result": { /* ... */ } }
{ "event": "failed",    "job_id": "job_01H...", "status": "failed",    "message": "..." }
```

Handler melaporkan progres lewat `ctx.job.progress(pct, message)` dan
**mengembalikan objek hasil** — objek itulah yang muncul di payload `completed`.
Payload event: `{job_id, progress?, message?, status?, result?}` (field opsional
hanya hadir sesuai `event`-nya). Pola ini berbeda tegas dari service action
fire-and-forget (§5), yang tidak punya `job_id`, progres, maupun hasil.

### 13.1 Hasil Async via Callback Webhook

Sebagai alternatif kanal websocket `jobs` untuk penyampaian hasil, sebuah async
job boleh mengirim hasilnya lewat **callback webhook** — pemanggil menyuplai URL
callback lewat request header, delivery HMAC-signed persis seperti panggilan
keluar `kind: Webhook` (§4):

```yaml
deliver:
  channel: webhook
  url_from: header            # URL diambil dari header request
  header: X-Callback-URL      # header yang membawa URL callback
  sign: true                  # HMAC-signed, sama seperti webhook keluar (§4)
  retry: { max: 5, backoff: exponential, initial_delay_ms: 1000 }
```

Delivery memakai **semantik retry durable yang sama** dengan jalur delivery
durable lain (§3, §4) — termasuk `initial_delay_ms` sebagai jeda sebelum retry
pertama ([`01-core-basic.md`](01-core-basic.md) §7). Callback webhook dan kanal
`jobs` sama-sama menyampaikan hasil job; keduanya bisa dipakai bersamaan atau
salah satu, tergantung apakah pemanggil punya endpoint callback yang bisa
menerima.

## 14. Validation Levels 4-6

Di atas rule field-level (level 1–3, dikatalogkan di
[`05-field-types.md`](05-field-types.md) §3), ada tiga level validasi yang lebih
tinggi. Level dievaluasi **berurutan** (level lebih rendah lolos dulu) dan
di-gate oleh `uses` ([`01-core-basic.md`](01-core-basic.md) §5) — akses yang
dibutuhkan tiap level wajib dideklarasikan:

| Level | Nama | Cakupan | Butuh |
|---|---|---|---|
| L4 | `business_rules` | Batasan bisnis yang dievaluasi script atas satu record (mis. "diskon ≤ plafon peran") | — (kalau murni atas record & config) |
| L5 | `cross_validate` | Validasi yang menjangkau **beberapa field / child record** dalam record yang sama | — |
| L6 | `consistency` | Konsistensi **lintas-entity** (mis. saldo agregat harus cocok dengan buku besar) | `uses: db` untuk membaca entity terkait |

L4–L6 dievaluasi **server-side, selalu** ([`01-core-basic.md`](01-core-basic.md)
§3), setelah L1–L3 lolos dan sebelum handler action berjalan. L6 boleh membaca
entity lain, karena itu **wajib** mendeklarasikan akses baca yang dipakainya di
`uses` — sama seperti akses lintas-resource lain; akses yang tidak dideklarasikan
diblokir saat runtime ([`01-core-basic.md`](01-core-basic.md) §5). Kegagalan di
level mana pun mengembalikan envelope error normatif dengan `details: [{level,
field?, message}]` ([`01-core-basic.md`](01-core-basic.md) §8.5), `level` menandai
di level berapa validasi gagal.

## 15. Hook Spec

Blok `hooks:` di top-level `spec` sebuah resource menautkan handler ke titik-titik
dalam siklus action — memperluas `before`/`after`/`on_error`
([`01-core-basic.md`](01-core-basic.md) §5) plus titik pipeline delivery:

```yaml
spec:
  hooks:
    - point: before
      action: submit
      run: check_credit_limit       # ref script (§7)
      priority: 10
    - point: before_deliver
      channel: webhook
      run: enrich_payload
      priority: 20
```

**Titik hook:**

| Titik | Kapan | Kemampuan |
|---|---|---|
| `before` | Sebelum handler action | Boleh **mengubah params** action atau memanggil `fail()` untuk membatalkan |
| `after` | Setelah handler sukses | Efek samping pasca-aksi |
| `on_error` | Saat handler gagal | Kompensasi/pembersihan |
| `before_deliver` | Sebelum sebuah delivery dikirim (§3, §4) | Boleh **menekan** delivery (suppress) atau memperkaya payload |
| `after_deliver` | Setelah delivery terkirim | Efek samping pasca-kirim |

**Priority ordering.** Bila beberapa hook menempel di titik yang sama, urutan
eksekusi mengikuti `priority` (kecil dijalankan lebih dulu) — konsisten dengan
prioritas handler event ([`01-core-basic.md`](01-core-basic.md) §7).

**Cross-module hook.** Sebuah module boleh meng-hook action milik module lain;
deklarasinya hidup di **module yang meng-hook** (bukan di module yang di-hook),
memakai notasi qualifier `{module}/...` (§7) dan **tunduk aturan `uses`/consent
footprint yang sama** seperti akses lintas-module lain
([`01-core-basic.md`](01-core-basic.md) §5). Rantai hook terlihat di output
gabungan `formspec describe` (combined view) dan **muncul di consent footprint** —
perilaku yang menempel selalu ter-compile, tidak pernah tersembunyi (pola yang
sama dengan Workflow §2 dan Subscription §3).

## 16. Query Builder

Di atas filter/sort dasar ([`01-core-basic.md`](01-core-basic.md) §6), Query
Builder adalah kemampuan agregasi normatif yang wajib disediakan setiap
PersistBackend dengan semantik identik:

| Kemampuan | Cakupan |
|---|---|
| Fungsi agregat | `sum`, `count`, `avg`, `min`, `max` |
| `group_by` | Satu atau **beberapa** field pengelompokan |
| `having` | Filter atas hasil agregat (post-aggregation) |
| `date_trunc` | Pembucketan waktu (hari/minggu/bulan/kuartal/tahun) |
| Window function | Running total, ranking, dan sejenisnya |
| `include()` batched | Eager-load relasi ter-batch untuk menghindari N+1 |

**Larangan lintas-schema/lintas-kategori mutlak.** Query Builder **tidak pernah**
boleh menjangkau lintas `category` ([`01-core-basic.md`](01-core-basic.md) §3 —
"tidak boleh di-join lintas kategori"); tidak ada raw SQL lintas-schema/
lintas-kategori dalam bentuk apa pun. Upaya join lintas kategori adalah
`FORMSPEC.PERSIST.CROSS_CATEGORY` ([`error-glossary.yaml`](error-glossary.yaml)) —
isolasi ini ditegakkan framework, bukan sekadar konvensi. Bagaimana backend
mewujudkan agregasi/window/hierarki adalah detail implementasi
([`04-persist-backend.md`](04-persist-backend.md) §2, "Query resolution");
kontraknya hanya: hasilnya benar dan identik antar backend.

**N+1.** Pemuatan relasi memakai `include()` yang **di-batch** (satu query per
level relasi, bukan satu query per parent record) — kontrak ini mencegah pola
N+1 yang jadi jebakan diam-diam pada eager-loading naif.

## 17. Rate Limiting

`rate_limit` membatasi laju pemanggilan di **level resource**, dapat di-override
**per-action**:

```yaml
spec:
  rate_limit: { max: 1000, per: 1m, scope: tenant, strategy: sliding_window }
  actions:
    export-report:
      rate_limit: { max: 5, per: 1m, scope: user, strategy: token_bucket }
```

| Field | Arti |
|---|---|
| `max` | Jumlah pemanggilan maksimum dalam jendela `per` |
| `per` | Panjang jendela (mis. `1s`, `1m`, `1h`) |
| `scope` | Apa yang dibatasi bersama: `tenant` \| `user` \| `ip` \| `global` |
| `strategy` | Algoritma: `sliding_window` \| `token_bucket` |

`scope` menentukan **kunci** penghitungan (mis. `tenant` = kuota per tenant,
`ip` = per alamat IP, `global` = satu kuota lintas semua pemanggil). Rate limit
di-override per-action **menimpa** default resource untuk action itu saja.
Pemanggil yang melampaui kuota ditolak `429` sebelum handler berjalan.

## 18. `ctx.secrets`

`ctx.secrets` adalah **satu-satunya** jalur baca untuk key `kind: Config` yang
`secret: true` ([`01-core-basic.md`](01-core-basic.md) §10) — key non-secret
tetap dibaca lewat `ctx.config.get(...)`. Aksesnya dideklarasikan eksplisit di
consent footprint action lewat `uses`:

```yaml
uses:
  secrets: [midtrans.server_key]     # muncul di consent footprint (01 §5)
```

Kontrak normatif:

- Akses `ctx.secrets` yang **tidak** dideklarasikan di `uses` diblokir saat
  resolusi ([`01-core-basic.md`](01-core-basic.md) §5) — konsisten dengan model
  `uses` untuk primitive lain.
- Nilai secret **tidak pernah** muncul di log pada level mana pun — sejalan
  dengan disiplin PII/redaction logging
  ([`../platform/09-observability.md`](../platform/09-observability.md) §2.2).
- **Setiap pembacaan secret di-audit** ([§11](#11-business-audit-trail)) — siapa
  membaca secret apa, kapan.
- **Penyimpanan** secret itu sendiri (env var, file, Vault, KMS) adalah
  **konfigurasi deployment** yang digovern Control Plane
  ([`../platform/04-control-plane.md`](../platform/04-control-plane.md) §2),
  bukan bagian kontrak ini — kontrak ini hanya mengatur *jalur baca* dari dalam
  action.

## 19. Soft-Deactivation (`is_active`)

Pola yang direkomendasikan untuk entity `characteristic: master`
([`01-core-basic.md`](01-core-basic.md) §1.1) yang butuh **kontrol visibilitas
tanpa penghapusan**: field boolean `is_active` plus action `deactivate` /
`reactivate`. Ini **berbeda** dari guard `delete` yang absolut
([`01-core-basic.md`](01-core-basic.md) §1.2) — `delete` soal integritas
referensial (baris benar-benar hilang, ditolak bila masih direferensikan),
sedangkan `is_active` soal **visibilitas** (baris tetap ada, tetap
direferensikan transaksi lama, cuma disembunyikan dari pilihan baru).

**Perilaku renderer normatif:**

- Widget pemilih pada **transaksi baru** (dropdown/relation-picker saat
  `create`) **wajib** memfilter ke `is_active: true` secara default — master
  yang sudah di-deactivate tidak muncul sebagai pilihan baru.
- **List/table view penuh** menampilkan **semua** record apa adanya, terlepas
  dari status aktif — sehingga operator tetap bisa melihat, mengedit, dan
  me-`reactivate` master yang nonaktif.

Karena baris tidak dihapus, transaksi lama yang menunjuk master nonaktif tetap
utuh dan konsisten — pola ini melengkapi denormalisasi finansial (§1.1), bukan
menggantikannya.

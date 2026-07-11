# Forma Technical Note: Document Model — Reserved Words, Lifecycle, dan Tanggal Bisnis

**Catatan internal — hasil diskusi tim, bukan bagian resmi Forma Core Spec.**
**Status: bahan eksplorasi desain, belum committed ke Forma Core Basic Spec v0.1.9. Beberapa poin di sini merevisi/mengganti mekanisme yang sudah ada di v0.1.9 Section 12 (State Machine Spec).**

---

## 0. Latar Belakang

Diskusi ini bermula dari pertanyaan tentang bagaimana Forma menjawab tantangan "sistem baku vs kebutuhan unik pelanggan" di era AI, lalu berkembang lewat studi banding dengan Frappe/ERPNext (framework battle-tested puluhan tahun untuk kasus serupa) menjadi serangkaian keputusan konkret tentang bagaimana Resource bertipe `document` seharusnya berperilaku. Prinsip dasar yang dipegang sepanjang diskusi: **kalau butuh rombak ulang di tahap desain, itu jauh lebih murah daripada menyesal setelah launching.**

---

## 1. Taksonomi Resource: Rename dari "Entity" ke "Document"

```
Resource (istilah payung, TIDAK berubah — tetap "Resource Plane", resource.yaml, resource.call())
├── type: document   (rename dari "entity")  → istilah bisnis: "Dokumen"
│     characteristics: []   (opsional, TIDAK wajib) — combinable hint:
│       - master        → data referensi stabil (Customer, Product) → istilah bisnis: "Dokumen" biasa
│       - transaction    → append-heavy, butuh transaction_date (§10) → istilah bisnis: "Dokumen" transaksi
│       - reference      → seed data read-only, diisi sistem
│       - summary        → proyeksi terkelola sistem, create/update/delete via API otomatis disabled
│                           → istilah bisnis: "Laporan"
└── type: service    (tidak berubah)          → istilah bisnis: "Layanan"
```

**Koreksi penting:** `summary` **bukan** `type` resource terpisah — dia salah satu nilai `characteristics` pada `type: document`, persis seperti `master`/`transaction`/`reference`. "Laporan" bukan jenis resource ketiga sejajar Document/Service; dia Document dengan `characteristics: [summary]`.

**Alasan rename `entity` → `document`:** menghindari dua-lapis istilah (developer bilang "entity", bisnis bilang "dokumen"). Dengan `document` di level YAML, developer, dokumentasi, dan orang bisnis pakai kata yang sama — tidak ada tabel terjemahan bolak-balik.

**"Resource" tetap dipertahankan sebagai istilah payung** karena dia mencakup dua hal berbeda (Document + Service + Summary) — mengganti seluruhnya ke "Document" akan menghilangkan kata yang menaungi ketiganya. "Entity" sebaiknya tidak dipakai sebagai istilah user-facing sama sekali (lebih teknis dari "Resource" sendiri), boleh tetap dipakai sebagai istilah klasifikasi internal kalau diperlukan.

**Perbedaan mendasar:**

| | Document (biasa/master/transaction) | Service | Document dengan `characteristics: [summary]` (Laporan) |
|---|---|---|---|
| Persistensi | Ya, punya identitas & riwayat | Tidak, stateless | Ya, tapi computed/derived |
| Punya `doc_status`? | Ya | Tidak relevan | Tidak relevan — create/update/delete API disabled |
| Bagaimana berubah | Action bernama, oleh manusia/sistem | N/A — panggilan sinkron | Otomatis dihitung ulang dari Document sumber via `reliable_event` |
| Immutable? | Ya, setelah `submit` (lihat §3) | N/A | Tidak — live selama periode belum `finalize` |

---

## 2. Reserved Fields

Field-field berikut **tidak boleh** dipakai ulang sebagai nama field custom. Berlaku otomatis untuk `type: document`, di-set framework, read-only dari sisi kode developer.

| Field | Fungsi | Ditulis manual oleh developer? |
|---|---|---|
| `name` | Identitas unik | Tidak |
| `owner` | Siapa yang membuat | Tidak |
| `created_at` | Tanggal sistem — kapan record dibuat, urutan kejadian sebenarnya | Tidak |
| `modified` | Tanggal sistem — kapan terakhir diubah | Tidak |
| `doc_status` | Status lifecycle baku (lihat §3) | Tidak — hanya berubah lewat action baku |
| `transaction_date` | Tanggal bisnis/periode akuntansi (lihat §7), **wajib dideklarasikan eksplisit** kalau `characteristics: [transaction]` — `forma apply` REJECT kalau tidak ada field ini, TIDAK di-auto-inject seperti `doc_status` | Ya, tapi tunduk pada `backdate_policy`/`forward_date_policy` |

**Prinsip:** field bisnis biasa bebas dinamai apa pun oleh developer (termasuk nama seperti `status`, `fulfillment_stage`, dll — ini valid dan justru dianjurkan supaya tidak bentrok makna dengan `doc_status` yang sudah reserved).

---

## 3. Reserved Actions (Lifecycle Baku)

Enam action ini adalah **reserved word** — kalau nama action persis cocok, guard baku otomatis aktif. Developer boleh menambah `conditions` di atasnya, tapi tidak bisa menghapus guard dasarnya. Nama lain = murni custom, tidak ada guard tersembunyi.

```yaml
doc_status:
  values: [draft, submitted, cancelled]   # TERTUTUP, tidak bisa ditambah state baru
                                            # kebutuhan granularitas bisnis → field terpisah
                                            # (lihat "Order" contoh di §3.1)

actions:
  guards:
    create: "doc_status diset otomatis = draft"
    update: "doc_status == draft"
    submit: "doc_status == draft  →  doc_status = submitted"
    cancel: "doc_status == submitted  AND  no_pending_references  →  doc_status = cancelled"
    delete: "doc_status == draft  AND  no_referencing_documents"   # TIDAK bisa di-override
    amend:  "doc_status == cancelled  →  buat Document baru terhubung, mulai lagi dari draft"
```

**`delete` vs `cancel` — dua kelas guard referensi yang berbeda ketatnya:**

- `delete` **menghapus row sepenuhnya**. Kalau ada Document lain dengan field `relation` menunjuk ke sini, guard-nya **mutlak, tidak ada `override_permission`** — persis `ON DELETE RESTRICT` di database relasional. Ini pelanggaran integritas data (dangling reference), bukan sekadar masalah proses bisnis.
- `cancel` **tidak menghapus row**, cuma ubah `doc_status`. Guard-nya masih bisa dibuka lewat `before_cancel` handler yang unwind dependency-nya lebih dulu (§4-5), atau lewat `override_permission` di kasus tertentu.

Guard `delete` murni berdasarkan tipe field (`relation`) di resource lain yang menunjuk ke sini — berlaku otomatis berapa pun `doc_status` resource ini, tidak bergantung apakah dia pernah di-`submit` atau tidak. Master data yang tidak pernah di-`submit` sekalipun (selalu `draft`, lihat §3.4) tetap terlindungi dari `delete` kalau sudah direferensi transaksi lain.

**`update` setelah `submit` selalu ditolak — tidak ada pengecualian.** Ini yang membuat Document "immutable" setelah submit. Tapi custom action tetap boleh mengubah field tertentu setelah submit (mis. `mark-paid` mengisi `paid_at`), asal lewat jalur bernama dengan guard eksplisit dan tercatat di audit *by-name* (bukan "document diupdate", tapi "action mark-paid dijalankan").

**Guard referensi (`cancel`) generik berbasis tipe field, bukan self-awareness manual:** field bertipe `relation`/reference — baik field standar maupun field ekstensi (`ext_*`) — otomatis diikutsertakan pengecekan "apakah dokumen ini masih direferensi oleh Document submitted lain?" Document tidak perlu menulis kode custom untuk ini.

**Tidak boleh menulis field terkontrol langsung (`resource.doc_status = "x"`).** Kalau custom action perlu memicu transisi, **wajib** panggil action baku sebagai method (`ctx.call_action(resource, "submit")`), bukan menulis field mentah — supaya guard, hook, dan audit tetap tereksekusi penuh.

### 3.1 Contoh: Base `doc_status` vs Field Bisnis Granular

```yaml
resource:
  name: order
  type: document

fields:
  - name: fulfillment_stage        # nama bebas, TIDAK bentrok dengan doc_status
    type: enum
    enum_values: [awaiting_payment, paid, fulfilled]
  # doc_status TIDAK ditulis di sini — reserved, otomatis ada

state_machine:
  field: fulfillment_stage         # murni bisnis, independen dari doc_status
  initial: awaiting_payment
  transitions:
    - from: awaiting_payment
      to: paid
      via: mark-paid
    - from: paid
      to: fulfilled
      via: fulfill

actions:
  - name: mark-paid                 # nama bebas -> custom, guard ditulis manual
    conditions:
      - script: "doc_status == 'submitted'"
      - script: "fulfillment_stage == 'awaiting_payment'"
```

Tidak ada `maps_to` antara `fulfillment_stage` dan `doc_status` — keduanya independen. Hubungan antar keduanya (kalau perlu) cukup lewat `conditions` biasa di action, bukan mekanisme baru.

### 3.2 Composite Action

Action custom yang isi handler-nya semata-mata memanggil ≥2 action lain lewat `ctx.call_action` secara berurutan pada **resource yang sama**.

```python
def handle(params, ctx):
    order = ctx.call_action("order", "create", params)
    ctx.call_action("order", "submit", {"id": order.id})
    return order
```

**Prinsip: tidak ada partial success.** Composite Action diperlakukan sebagai satu unit atomik — hasilnya cuma tiga kemungkinan: sukses penuh, gagal bersih (tidak ada apa pun tertinggal), atau `outcome_unknown` (§7-8, hanya untuk kasus cross-boundary yang benar-benar ambigu).

**Same-dataspace — dibungkus SATU transaksi database, bukan orkestrasi step-by-step yang masing-masing commit sendiri:**

```
BEGIN TRANSACTION
  → jalankan create: insert row, doc_status=draft, tulis audit
  → jalankan submit: guard cek, update doc_status=submitted, tulis audit
  → kalau guard submit GAGAL → ROLLBACK seluruhnya
COMMIT (hanya kalau semua langkah sukses)
```

Kalau rollback terjadi, row yang dibuat `create` **tidak pernah benar-benar ada** secara persisten — bukan tersisa sebagai draft. Ini didapat gratis dari properti ACID transaksi standar, tidak perlu mekanisme baru. Konsekuensi penting: event `on_create` (async, post-commit) baru terkirim setelah `COMMIT` di akhir seluruh Composite Action — bukan segera setelah `create` "selesai" secara internal — supaya tidak ada notifikasi keluar untuk sesuatu yang ternyata batal.

**Cross-boundary — Saga, dengan tujuan yang sama: hasil akhir setara "tidak pernah terjadi", bukan sekadar "sudah dicoba".** Compensate (§5-6) bukan pembersihan setelah gagal, tapi bagian wajib dari definisi sukses/gagal Composite Action itu sendiri. Satu-satunya kasus yang jujur tidak bisa dijanjikan bersih 100%: compensate sendiri gagal, atau hasil network ambigu setelah retry habis — di situ respons ke caller memakai `FORMA.SAGA.OUTCOME_UNKNOWN` (§14), bukan error biasa, supaya caller tahu ini bukan rollback bersih dan tidak boleh asal retry.

### 3.4 Master Data: Draft-selamanya, Tanpa Kategori Resource Baru

Resource `type: document` yang tidak pernah memanggil `submit` (mis. Customer, Product) otomatis berperilaku seperti CRUD biasa tanpa lifecycle — `doc_status` selalu `draft`, `update`/`delete` selalu diizinkan sepanjang guard lain terpenuhi. Ini **zero-cost**, bukan celah desain: tidak perlu kategori resource keempat untuk "master data", developer cukup tidak pernah memanggil `submit`.

Perlindungan dari `delete` yang berbahaya (§3, revisi guard) tetap berlaku otomatis berdasarkan tipe field `relation` di resource lain — tidak bergantung apakah Master ini pernah di-`submit`. Jadi tidak perlu guidance "submit dulu supaya aman" — guard referensi pada `delete` sudah cukup ketat dengan sendirinya, terlepas dari status lifecycle Master itu sendiri.

**Sinyal eksplisit untuk UI generator (lihat §15):** resource yang memang tidak pernah bermaksud punya lifecycle submit sebaiknya menonaktifkan action tersebut secara eksplisit, reuse pola `disabled: true` yang sudah ada untuk standard action:

```yaml
resource:
  name: customer
  type: document

actions:
  - name: submit
    disabled: true
  - name: cancel
    disabled: true
  - name: amend
    disabled: true
```

Ini bukan cuma dokumentasi niat — ini sinyal yang dipakai UI generator untuk memutuskan menampilkan CRUD polos (§15) alih-alih pola lifecycle draft→submit.

---

## 4. Reserved Events: Sync vs Async

**Untuk event reserved (mengikuti prefix `before_*`/`on_*`), `type` TIDAK ditulis — terkunci oleh nama, bukan default yang bisa diubah.**

- `before_*` (before_cancel, before_submit, before_delete, dst.) → **selalu sync**. Dia gate yang harus tuntas sebelum state berubah — tidak masuk akal secara logis kalau async.
- `on_*` (on_cancel, on_submit, on_delete, dst.) → **selalu async**. Terjadi setelah commit, notifikasi murni — tidak masuk akal kalau sync dan bisa membatalkan sesuatu yang sudah committed.

`forma apply` **reject** kalau ada yang menulis `type` bertentangan dengan prefix ini (mis. `type: async` di bawah `before_cancel`) — bukan override yang diterima.

Aturan ini otomatis berlaku untuk semua action baku (§3) — setiap action punya pasangan `before_{action}` (sync) dan `on_{action}` (async) tanpa perlu dispesifikasikan manual satu-satu per action.

**Untuk custom event yang TIDAK mengikuti pola prefix `before_*`/`on_*`** (mis. `reconcile_needed`, `stock_low_alert`) — `type` **wajib** ditulis eksplisit, karena tidak ada sinyal dari nama sama sekali.

```yaml
events:
  before_cancel:              # type TIDAK ditulis — terkunci sync oleh prefix
    handlers:
      - name: unwind_gl_journal
        priority: 10           # lihat §4.1 untuk konvensi angka
        on_error: abort         # abort | continue
        compensate: recreate_gl_journal   # wajib kalau lintas-boundary DAN on_error: abort

  on_cancel:                  # type TIDAK ditulis — terkunci async oleh prefix
    handlers: [...]
    # tidak ada priority wajib-ordered, tidak ada compensate — post-commit, retry only

  reconcile_needed:            # custom, TIDAK ikut pola before_*/on_* → type WAJIB
    type: sync
    handlers: [...]
```

### 4.1 Konvensi Priority untuk Sync Event Handler

Angka kecil jalan lebih dulu. Dipakai kelipatan 10 (bukan 1-2-3 berurutan) supaya mudah menyisipkan handler baru di antara dua yang sudah ada tanpa menomori ulang handler lain (termasuk milik Module/Integrator pihak ketiga yang mungkin tidak dikontrol source-nya).

| Tier | Range | Kapan dipakai | Default kalau tidak ditulis |
|---|---|---|---|
| **Critical** | 1–9 | Gate yang harus dicek paling awal, sebelum business logic apa pun — mis. fraud check, compliance block. Jarang dipakai, hindari kecuali benar-benar wajib paling depan. | — |
| **Normal** | 10–89 | Mayoritas handler business logic — Integrator, custom hook. | **10** |
| **Low** | 90–99 | Efek samping non-kritis yang tetap harus sync tapi urutan tidak penting terhadap handler lain — mis. update cache lokal. | — |

```yaml
before_cancel:
  handlers:
    - name: unwind_gl_journal
      priority: 10
    - name: unwind_stock_reservation
      priority: 20
    - name: notify_finance_review
      priority: 90        # low — urutan tidak penting
```

Handler tanpa `priority` → otomatis `10` (Normal). Tie di priority sama → undefined, tergantung sort algoritma (§4, sudah disepakati sebelumnya).

**Guard referensi generik (§3) bukan salah satu handler ber-priority ini** — dia step terpisah yang selalu jalan *setelah* semua handler `before_cancel` selesai (lihat urutan eksekusi di bawah). Priority hanya mengatur urutan sesama handler custom pada satu event, tidak bersaing posisi dengan guard bawaan framework.

**Sync event** — boleh error/gagal, itu memang fungsinya (gate). Dijalankan berurutan sesuai `priority` (tie-break: undefined). Dua kelas penanganan kegagalan:
- **Efek dalam transaksi DB yang sama** → tidak butuh `compensate` — `ROLLBACK` SQL native sudah menangani atomicity, gratis.
- **Efek lintas-boundary** (lihat §6) → **wajib** `compensate` kalau `on_error: abort`.

**Async event** — **tidak boleh** membatalkan apa pun, karena selalu terjadi setelah commit — secara logis mustahil "membatalkan" sesuatu yang sudah committed. Kegagalan handler async → retry/dead-letter (reliable event delivery yang sudah ada), bukan rollback.

**Urutan eksekusi `Invoice.cancel()`:**
```
1. Jalankan semua before_cancel handler terdaftar (sync, berurutan by priority)
   → kalau ada Integrator, dia wajib daftar DI SINI untuk auto-unwind
2. Jalankan guard referensi generik (§3)
   → gagal di sini kalau tidak ada yang unwind dependency di step 1
3. Guard lolos → set doc_status = cancelled, commit
4. Emit on_cancel (async) → siapa pun boleh dengar setelahnya, tidak bisa membatalkan
```

---

## 5. Spec Type: Integrator

Menjembatani dua Document/Module yang **tidak saling kenal** — konsisten dengan prinsip "Module tidak saling kenal secara langsung".

```yaml
kind: Integrator
name: invoice-to-gl
listen:
  entity: Invoice
  event: before_cancel        # atau on_paid, dst — nama event dari Contract
call:
  entity: GLJournal
  action: cancel
compensate: recreate_gl_journal   # opsional ditulis; framework putuskan perlu dipanggil atau tidak
```

`listen.entity` dan `call.entity` di-resolve lewat registry (`forma.resource:{name}:{version}`) — Integrator tidak pernah `import` definisi Invoice atau GLJournal secara langsung.

**Aturan wajib:** setiap Integrator yang membuat side-effect dari satu event, wajib juga menyediakan handler simetris untuk event pembatalannya — kalau tidak, cancel di sisi sumber akan macet permanen karena guard referensi generik selalu blokir tanpa ada yang tahu cara membuka jalannya.

---

## 6. Boundary Detection: Runtime, Bukan Design-Time

**Keputusan penting:** boundary "sama dataspace atau tidak" TIDAK bisa diketahui di design-time — itu fakta topologi deployment (tier shared-schema vs sidecar polyglot vs lintas Resource Plane), bukan fakta resource. Developer tidak boleh dipaksa mendeklarasikan `boundary: local/cross` di YAML.

**Mekanisme:** karena `resource.call()` sudah dirancang location-transparent, framework di titik itu sudah tahu secara internal apakah panggilan resolve dalam transaksi DB yang sama atau tidak.

```
Saat sync handler jalankan resource.call(Y, "aksi"):
  → framework cek: apakah panggilan ini resolve dalam transaksi DB yang sama?

  SAMA transaksi:
    → tidak perlu compensate sama sekali, kalaupun ditulis, tidak pernah dipanggil

  BEDA transaksi/proses:
    → ADA compensate terdaftar → daftarkan di saga log, siap dipanggil kalau step
      berikutnya gagal
    → TIDAK ADA compensate → JANGAN lanjut diam-diam. Kalau step berikutnya
      gagal dan butuh unwind handler ini → langsung escalate ke manual
      intervention (§8), sama seperti "compensate gagal dieksekusi"
```

Ini menghasilkan **error di runtime saat memang dibutuhkan** — bukan reject di `forma apply` karena boundary belum diketahui, dan bukan silent-success yang meninggalkan data yatim.

---

## 7. Error Classification: Bisnis / Server vs Network

| Kelas | Sifat | Penanganan |
|---|---|---|
| Error bisnis | Respons definitif (ditolak validasi) | Compensate langsung (§6) |
| Error server | Respons definitif (500, dst — tapi tetap terkirim balik) | Compensate langsung (§6) |
| Error network | **Tidak diketahui** apakah sudah eksekusi atau belum | Idempotent retry (lihat di bawah), BUKAN langsung compensate |

**Kenapa network error tidak boleh langsung compensate:** kalau ternyata sisi remote sudah sukses duluan (cuma respons yang hilang), compensate akan menciptakan inkonsistensi baru, bukan memperbaikinya.

**Wajib untuk pemanggilan lintas-boundary:**
- Action target **wajib** `idempotent: true` — `forma apply` reject Integrator yang target-nya tidak idempotent.
- Idempotency key dibuat **sekali** di awal (disimpan `ctx.kvstore`), dipakai ulang di semua retry — tidak digenerate ulang per attempt.
- Setelah retry habis tanpa respons definitif → masuk `outcome_unknown` (§8), **bukan** diasumsikan sukses atau gagal.

**Config default + override**, konsisten pola `ctx.config`:
```yaml
# Global
integrator_defaults:
  retry:
    max_attempts: 5
    backoff: exponential
    base_delay_ms: 500
    max_delay_ms: 30000
  outcome_unknown_after: 5

# Per-Integrator, override eksplisit
kind: Integrator
name: invoice-to-payment-gateway
retry:
  max_attempts: 10      # gateway pihak ketiga: lebih longgar
```

---

## 8. Manual Intervention Queue

Resource baru: `compensation-failure-log` (`forma.core`/`forma.observe`, `persist.category: compliance`, pola sama seperti `app-log`).

| Sub-status | Arti | Aksi yang benar |
|---|---|---|
| `compensation_failed` | Tahu step gagal, coba undo, undo-nya juga gagal | Manusia perbaiki manual, state diketahui |
| `outcome_unknown` | Tidak tahu apakah step berhasil, retry habis | Manusia **verifikasi dulu** state sebenarnya sebelum tindakan apa pun — TIDAK boleh langsung dikasih tombol retry/compensate otomatis |

CLI: `forma saga list` / `forma saga resolve <id>`. Tidak ada retry otomatis tanpa batas — kalau butuh manusia, sistem tidak berpura-pura bisa selesaikan sendiri.

---

## 9. Characteristics: Master — Spec Lengkap

`characteristics: [master]` menandai Document sebagai data referensi stabil — Customer, Product, Vendor, GL Account. Berbeda dari `transaction`, **tidak ada field wajib tambahan dan tidak ada validasi apply-time khusus** untuk `master` — nilai ini murni hint yang mempengaruhi archiving (§11) dan backup, plus beberapa pola desain yang direkomendasikan di bawah.

### 9.1 Lifecycle: Draft-selamanya (ringkasan dari §3.4)

Master data biasanya tidak pernah memanggil `submit` — `doc_status` tetap `draft` selamanya, `update`/`delete` selalu diizinkan sepanjang guard referensi (§3) terpenuhi. Sinyal eksplisit untuk UI generator: nonaktifkan `submit`/`cancel`/`amend` lewat `disabled: true` (detail lengkap di §3.4).

### 9.2 Natural Key — Lookup by Business Identifier

Master data jarang dicari lewat UUID — user mencari "Customer ABC" atau kode SKU, bukan `a1b2c3d4-...`. Field `natural_key: true` (sudah ada di Core Basic Spec, Field Spec) layak dipertimbangkan untuk setiap `characteristics: [master]`:

```yaml
resource:
  name: customer
  type: document
  characteristics: [master]

fields:
  - name: customer_code
    type: string
    natural_key: true      # otomatis generate lookup find_by_customer_code
    unique: true
```

**Rekomendasi, bukan wajib:** tanpa `natural_key`, user selalu mencari lewat UUID atau field non-indexed biasa — sering jadi UX buruk untuk dropdown/search di admin panel.

### 9.3 Soft Deactivation — Sembunyikan Tanpa Menghapus

Kebutuhan umum: Customer lama/Product discontinued tidak boleh muncul di dropdown pembuatan transaksi baru, tapi tidak boleh dihapus (masih direferensi transaksi lama). Ini **beda** dari guard `delete` (§3) yang soal integritas data — ini soal *visibility*, keputusan bisnis murni. Bukan bagian dari `doc_status` (sudah terkunci 3 nilai) — pola yang direkomendasikan adalah field bisnis biasa:

```yaml
resource:
  name: product
  type: document
  characteristics: [master]

fields:
  - name: is_active
    type: boolean
    default: true

actions:
  - name: deactivate
    conditions:
      - script: "resource.is_active == true"
  - name: reactivate
    conditions:
      - script: "resource.is_active == false"
```

Admin panel generator otomatis filter `is_active: true` di dropdown pembuatan transaksi baru, tapi tetap tampilkan Master nonaktif di daftar lengkap dengan indikator visual.

### 9.4 Denormalisasi Wajib untuk Field yang Mempengaruhi Perhitungan Historis

**Prinsip kritis, sudah ada presedennya di contoh Order yang eksis di spec:** field Master yang nilainya **mempengaruhi perhitungan finansial** pada transaksi (harga, diskon, tier, tarif pajak) **wajib** didenormalisasi (disalin) ke transaksi saat dibuat — bukan dibiarkan sebagai live join ke Master.

```yaml
# Field di TRANSAKSI (bukan di master) — snapshot nilai SAAT transaksi dibuat
- name: customer_tier_at_transaction
  type: string
  # Sengaja didenormalisasi: kalau customer naik/turun tier bulan depan,
  # nota order bulan lalu tidak boleh ikut berubah retroaktif.
```

**Kenapa ini tetap penting terlepas dari mekanisme snapshot arsip (§11):** snapshot Master di archive hanya terjadi **saat archive run** (mis. 3 tahun kemudian) — bukan setiap transaksi harian. Kalau tier Customer berubah bulan ini, Invoice bulan lalu yang masih live (belum diarsipkan) akan **tetap** menunjukkan tier baru kalau field itu live-joined, bukan didenormalisasi — retroactive drift yang tidak diinginkan. Field non-finansial (nama, alamat kontak) aman live-join — koreksi typo pada Customer boleh "menjalar" ke tampilan invoice lama. Field finansial tidak boleh.

**Aturan praktis:** setiap field Master yang dipakai dalam `conditions`/kalkulasi total pada action `create`/`submit` transaksi wajib disalin sebagai field transaksi sendiri (mis. suffix `_at_transaction`), bukan dibaca ulang dari Master saat transaksi ditampilkan nanti.

### 9.5 Interaksi dengan Archiving

- Master **tidak pernah** diarsipkan langsung — hanya **snapshot** kalau direferensi transaksi yang diarsipkan (detail §11.1).
- `locked_for_deletion` otomatis `true` selama ada transaksi archived yang mereferensi (§11.4).
- Master boleh dihapus (`delete` action, guard §3) kalau benar-benar tidak ada referensi apa pun — live maupun archived.

---

## 10. Transaction Date vs System Date

| | `created_at` (tanggal sistem) | `transaction_date` (tanggal bisnis) |
|---|---|---|
| Fungsi | Urutan kejadian sebenarnya, audit | Periode akuntansi/pelaporan mana yang mengakui |
| Bisa dimanipulasi? | Tidak | Ya, tunduk `backdate_policy`/`forward_date_policy` |
| Dipakai untuk sequencing/audit? | **Ya, selalu** | **Tidak pernah** |

**Prinsip kunci (pelajaran dari kasus ERPNext):** sequencing/audit selalu berdasarkan `created_at`. `transaction_date` murni menentukan periode laporan mana yang mengakuinya. Memakai `transaction_date` untuk sequencing menyebabkan backdate memicu cascading recompute mahal ke semua periode berikutnya — masalah nyata yang pernah dialami ERPNext bahkan setelah mereka pindah ke model "immutable ledger".

**Validasi wajib di `forma apply`, bukan auto-inject:**

```
forma apply -f journal-entry.resource.yaml

✗ Missing required field for characteristics:[transaction]

  journal-entry punya characteristics: [transaction] tapi tidak ada field
  bernama "transaction_date" (type: date/datetime) di daftar fields.
```

Berbeda dengan `doc_status` (selalu auto-inject, tidak pernah ditulis manual), `transaction_date` **wajib ditulis eksplisit** oleh developer di `fields:` dan divalidasi keberadaannya saat `forma apply` — supaya developer sadar penuh field ini ada dan dari mana nilainya berasal, bukan magic tersembunyi.

**Tidak ada auto-partitioning DB berdasarkan `transaction_date`.** Physical DB partitioning (native Postgres partition, dsb.) sengaja **tidak** dijadikan efek otomatis dari `characteristics: [transaction]` — ini kompleksitas operasional yang tidak seharusnya dipindahkan ke developer app hanya karena satu baris YAML. Didefer sebagai advanced concept, dibahas terpisah di versi mendatang kalau memang dibutuhkan.

### 10.1 Backdate & Forward-date Policy

```yaml
# Global default
transaction_defaults:
  backdate_policy:
    max_days_back: 3
    override_permission: null       # null = tidak ada yang boleh override
  forward_date_policy:
    max_days_forward: 0             # default paling konservatif
    override_permission: accounting.post_forward_dated
  period_guard:
    enabled: true

# Per-resource override
resource:
  name: journal-entry
backdate_policy:
  max_days_back: 7
  override_permission: accounting.post_backdated
```

### 10.2 Period Lock — Reuse Summary Finalize, Jangan Bikin Baru

Penutupan periode dimodelkan sebagai **Document sendiri** (`period-closing`), bukan cuma CLI command — supaya dapat gratis semua yang sudah dibangun: `doc_status`, guard referensi, audit trail, permission model.

```yaml
resource:
  name: period-closing
  type: document
  characteristics: [transaction]

fields:
  - name: transaction_date
    type: date
  - name: period_ref
    type: relation
    relation: gl/gl-monthly-balance

actions:
  - name: submit                 # submit = trigger forma summary finalize
    conditions:
      - script: "all_reconciliations_done(period_ref)"
  - name: cancel                 # cancel = trigger forma summary unfinalize
    required_permission: accounting.reopen_period
    conditions:
      - script: "reason != ''"
```

Document transaksi lain yang mau menembus periode closed butuh `period_guard.override_permission` eksplisit.

### 10.3 `today`/`current` — Resolve dari Business Calendar

Period shortcut (`today`, `current`) di Summary Spec **harus** resolve dari kalender bisnis (tanggal EOD terakhir yang closed + 1), **bukan** `date.Now()` sistem operasi — supaya EOD yang tertunda (skenario "sistem sudah tanggal 5, bisnis masih akui tanggal 4") tetap compute periode yang benar.

## 11. Retensi & Arsip Data Historis

**Hanya transaksi yang diarsipkan; master di-snapshot untuk menjaga konsistensi temporal.**

Prinsip: view-only app archive harus mandiri dan konsisten — tidak boleh query live DB. Saat transaksi lama diarsipkan, master yang direferensi di-snapshot "as-of" tanggal archive dan disimpan bersama di archive Parquet — supaya customer/product data sesuai dengan state saat transaksi itu valid secara historis, bukan state terkini di production.

### 11.1 Apa yang Diarsipkan

**Transaksi (characteristics: [transaction])** — selalu diarsipkan kalau umur ≥ `max_age`:
- Invoices, Payments, Journal Entries, Purchase Orders, Stock Movements, dsb.

**Master (characteristics: [master]) — HANYA snapshot, jika direferensi transaksi yang diarsipkan:**

Kalau transaksi T diarsipkan dan mereferensi master M (via field `relation`):
- Snapshot M (as-of archive_date) disimpan ke archive
- Row M asli di production tetap utuh (tidak dihapus)
- M di-flag `locked_for_deletion = true` selama ada transaksi archived mereferensi

**Sub-case: Master direferensi transaksi mix (archived + live)**

```
Customer ABC:
  - 156 Invoices diarsipkan (2021-2023) ← mereferensi ABC
  - 24 Invoices masih live (2024) ← juga mereferensi ABC

Result:
  - Snapshot ABC (as-of 2023-07-08) → stored di archive
  - Row ABC asli → tetap live di production
  - locked_for_deletion(ABC) = true ← tidak boleh dihapus selama ada archived ref
```

**Sub-case: Master direferensi HANYA transaksi archived**

```
Product XYZ:
  - 45 Invoices diarsipkan (2021-2023) ← mereferensi XYZ
  - 0 Invoices masih live

Result:
  - Snapshot XYZ (as-of 2023-07-08) → stored di archive
  - Row XYZ asli → boleh dihapus dari production (dengan owner approval)
  - locked_for_deletion(XYZ) = false ← aman dihapus, tidak ada live ref
```

### 11.2 Framework Logic: `forma archive run`

```bash
forma archive run --max-age 3y --dry-run

Scanning for transactions older than 2023-07-08...

Found 1,247 Journal Entries
Found 892 Sales Invoices → referenced Customers: [ABC, XYZ, ...] (24 unique)
                        → referenced Products: [PROD-001, ...] (67 unique)
Found 156 Payments      → referenced GL Accounts: [1000, 2000, ...] (89 unique)

Analyzing master references:
  Customer ABC: referenced by 156 archived invoices + 24 live invoices
    → Snapshot to archive, keep live (locked_for_deletion = true)
  Customer DEF: referenced by 12 archived invoices only
    → Snapshot to archive, DELETE candidate from live (locked_for_deletion = false)
  Product PROD-001: referenced by 34 archived invoices only
    → Snapshot to archive, DELETE candidate from live (locked_for_deletion = false)
  GL Account 1000: referenced by 892 archived journal entries only
    → Snapshot to archive, DELETE candidate (locked_for_deletion = false)

Archive batch: 2,295 transaction records + 180 master snapshot records
Total size: ~847 MB (compressed Parquet)
Masters to delete from production (if approved): DEF, PROD-001, ... (24 records)

Ready to proceed? (y/n)
```

**Keputusan operator di prompt:**
- Proceed dengan default: archive transaksi, snapshot master, set `locked_for_deletion` flag. Master tetap live.
- `--delete-masters`: setelah archive, DELETE master yang tidak ada live reference. Requires confirmation + audit ticket.

### 11.3 Archive Storage Format

```
archive-2021-2023.parquet/
  manifest.yaml
    archive_date: 2023-07-08
    max_age: 3y
    record_count: 2,475 (2,295 transactions + 180 masters)
    masters_snapshot_date: 2023-07-08
    
  transactions/
    journal_entries.parquet (1,247 rows)
    invoices.parquet (892 rows)
    payments.parquet (156 rows)
  
  masters/
    customers.parquet (24 rows, snapshot as-of 2023-07-08)
    products.parquet (67 rows, snapshot as-of 2023-07-08)
    gl_accounts.parquet (89 rows, snapshot as-of 2023-07-08)
```

Setiap archive batch adalah self-contained time capsule — view-only app tidak perlu live DB.

### 11.4 Master Locking & Deletion

Field otomatis di semua `type: document` dengan `characteristics: [master]`:

```yaml
fields:
  - name: locked_for_deletion
    type: boolean
    default: false
    immutable: true
    # set true otomatis saat ada transaksi archived mereferensi
    # set false otomatis saat semua archived reference di-restore atau dihapus

  - name: archived_reference_count
    type: integer
    default: 0
    audited: true
    # berapa transaksi archived yang mereferensi master ini

actions:
  - name: delete
    conditions:
      - script: "!resource.locked_for_deletion"
        message: "Master ini tidak bisa dihapus: masih direferensi {archived_reference_count} transaksi archived (periode {archive_dates}). 
                   Opsi: 1) restore batch terkait, 2) hubungi audit untuk override."
```

### 11.5 View-Only App — Mandiri

Akses ke archived data tidak butuh production DB. App ini bisa berjalan di environment terpisah, read-only.

```bash
forma archive view --batch-id archive-2021-2023 \
  --query "SELECT i.number, i.total, c.name, p.amount 
           FROM invoices i 
           JOIN customers c ON i.customer_id = c.id 
           JOIN payments p ON i.id = p.invoice_id"
```

Queries baca dari Parquet archive, tidak ada SQL ke live DB. Hasil konsisten temporal — customer name adalah nama valid saat invoice dibuat (snapshot as-of 2023-07-08).

### 11.6 Restore — View-Only Atau Staging Only

**Tidak ada restore ke production DB.** Dua opsi legal:

1. **View-only access** (default) — read Parquet, filter, export CSV/PDF. Kalau butuh operational akses (reprocessing invoice, create adjustment, dsb), tidak bisa di sini.

2. **Batch restore ke staging environment** (Advanced, needs approval) — full batch direstored ke PostgreSQL staging instance yang dedicated, atomic, dengan dependency urut. User bisa jalankan analysis, reconciliation, custom query. Hasil di-export, tidak ada sync balik ke production.

Selective restore per dokumen **tidak didukung** — terlalu berisiko corrupt state (invoice tanpa customer, payment tanpa invoice).

```bash
# View-only (default)
forma archive view --batch-id archive-2021-2023 \
  --filter "customer_id=ABC" --export-format csv

# Staging restore (requires approval ticket)
forma archive restore-batch \
  --batch-id archive-2021-2023 \
  --target-environment staging-reconcile-xyz \
  --reason "Deep audit Customer ABC 2021-2023" \
  --approval-ticket TICKET-12345
```

### 11.7 Opsi Retensi: Default Global

```yaml
# forma.yaml
retention:
  archive_after: "3y"           # dihitung dari transaction_date
  strategy: cold_storage        # cold_storage | delete
  destination: s3://archive-bucket

  master_delete_allowed: true   # boleh hapus master orphan dari live (requires approval)
  max_days_in_pending: 180      # max hari dokumen menunggu karena dependency belum ready
```

Resource yang mau opt-out:

```yaml
resource:
  name: audit-log
  type: document
  characteristics: [transaction]
  
  retention:
    disabled: true   # tidak boleh diarsipkan
```

### 11.8 Opsi 1a Revisited: Dependency Blocking

Saat `forma archive run --max-age 3y`:

```
Payment C (3.5y old, mereferensi Invoice B yang 2.5y old) 
  → EXCLUDED dari batch ini (dependency not ready)
  → Recorded di "pending-archive" log
  → Nanti saat run ulang 6 bulan kemudian, C dan B siap bersama,
    diarsipkan dalam batch yang sama

Master:
  Invoice B still live in production (mereferensi vendor, line items)
  → tidak dihapus, tidak dikunci (masih ada live reference)
  → Nanti saat batch C+B diarsipkan, B baru di-snapshot
```

---



## 12. Characteristics: Summary — Spec Lengkap (Laporan)

### 12.1 Karakteristik Dasar: Bypass Total Reserved Actions (§3)

`characteristics: [summary]` bukan sekadar hint dokumentasi — dia **mengubah total** cara Document ini ditulis. Standard actions `create`/`update`/`delete` otomatis **disabled via API** (sudah di conformance checklist Core Basic) — hanya `list`/`find` yang tersedia untuk caller biasa. Penulisan nilai **hanya** lewat compute engine internal (trigger-driven, §12.4), bukan lewat action call biasa dari luar.

**Konsekuensi penting: `doc_status` tidak berlaku secara bermakna untuk Summary.** Seluruh mekanisme lifecycle draft→submit→cancel (§3) berjalan lewat action call, dan Summary tidak pernah menerima action call dari luar — jadi field `doc_status` yang biasanya auto-inject untuk semua `type: document` **tidak punya makna operasional** di sini. Implementasi boleh tetap menyertakannya untuk konsistensi struktural, tapi nilainya harus tetap/diabaikan, bukan sesuatu yang berubah lewat submit/cancel seperti Document biasa.

```yaml
resource:
  name: gl-balance
  type: document
  characteristics: [summary]
  # Standard actions create/update/delete otomatis disabled — hanya list/find
  # doc_status: TIDAK relevan, tidak pernah berubah lewat submit/cancel
```

### 12.2 Summary Selamanya Aktif, Tidak Pernah Diarsipkan

Rasionalnya sederhana: Summary adalah proyeksi derived dari source transaction. Selama source transaction tetap queryable (live atau via archive access), Summary bisa di-rebuild. Kalau Summary sendiri diarsipkan, dia menjadi "snapshot statis" yang tidak lagi reflect realitas data — bertentangan dengan purpose Summary sebagai "terkini projection."

**Asumsi operasional:** Ukuran Summary tetap terkendali. Jika Summary growth-nya ternyata exponential (mis. GL Balance untuk ratusan tahun dengan jutaan account), itu pattern yang salah — harus didesain ulang (partition, aggregate strategy, dsb). Summary yang benar didefinisikan dengan `rebuild.strategy: partial` (hanya rebuild periode dengan source data baru), bukan full rebuild setiap kali — ini menjaga size manageable.

### 12.3 Live vs Static — Mengikuti Periode Finalization

**Live vs Static bukan pilihan mode eksplisit** — otomatis mengikuti status periode:
- Periode belum `finalize` → **Live**, terus dihitung ulang tiap ada event sumber baru.
- Periode sudah `finalize` (via Document `period-closing` §10.2) → **Static**, terkunci sampai ada `unfinalize` eksplisit.

### 12.4 Realtime vs Scheduled Trigger

```yaml
resource:
  name: ship-arrival-board
  type: document
  characteristics: [summary]      # BUKAN type: summary — lihat koreksi §1

triggers:
  - on: ship-status.updated       # realtime — dorong instan begitu ada kejadian nyata
    period: none
  - on: cron("*/2 * * * *")       # scheduled — untuk nilai turunan (mis. ETA countdown)
    period: none                   # yang berubah murni sebagai fungsi waktu
```

**Refresh logic tidak boleh hidup di UI** — harus jadi trigger di level resource (`cron`/event), lalu didorong ke client via `ctx.pubsub`/WebSocket (sudah otomatis di-generate dari definisi resource). Kalau logika refresh cuma di client, setiap client menebak ulang interval sendiri-sendiri dan tidak konsisten — bertentangan dengan prinsip "one definition, many protocols".

### 12.5 Compute, Sources, dan Rebuild Strategy — Rujuk Core Extended Spec

Sintaks lengkap `compute:`, `group_by:`, `sources:` (termasuk multi-source join dengan anchor, lihat Core Extended Spec §26.6), `rebuild.strategy` (`full`/`partial`/`none`), dan mekanisme `finalize`/`unfinalize` **sudah dispesifikasikan lengkap di Forma Core Extended Spec v0.1.4 §26**. Technical Note ini sengaja tidak mengulang sintaksnya — hanya menambahkan klarifikasi yang belum eksplisit di sana: bypass `doc_status` (§12.1), exemption dari archiving (§12.2), dan pemetaan live/static ke period finalization (§12.3).

---

## 13. Reserved Word — Daftar Lengkap Saat Ini

| Kategori | Nama | Catatan |
|---|---|---|
| Field | `name`, `owner`, `created_at`, `modified`, `doc_status` | Tidak boleh dipakai ulang di `fields:` |
| Field (kondisional) | `transaction_date` | Wajib ada kalau `characteristics: [transaction]` |
| Action | `create`, `update`, `submit`, `cancel`, `delete`, `amend` | Guard baku otomatis aktif |
| Event | `before_*` (sync, terkunci), `on_*` (async, terkunci) | Berlaku otomatis untuk semua action baku; custom event di luar pola ini wajib `type` eksplisit |
| Type resource | `document`, `service` | Rename `entity`→`document`. Hanya dua type — `summary` BUKAN type terpisah |
| Characteristics (opsional) | `master`, `transaction`, `reference`, `summary` | List hint pada `type: document` saja (service tidak punya). Tidak wajib diisi. Spec lengkap: `master` §9, `transaction` §10-11, `summary` §12 |
| Field (Master, rekomendasi) | `is_active` | Bukan reserved secara ketat, tapi pola standar untuk soft deactivation (§9.3) |
| Field (Master, auto-inject saat archiving aktif) | `locked_for_deletion`, `archived_reference_count` | §11.4 — hanya relevan kalau resource pernah terlibat archive run |

## 14. Error Glossary — Satu Sumber Kebenaran untuk i18n

Untuk error dari mekanisme baku framework (guard reserved action, period lock, saga, dll), pesan tidak boleh di-hardcode inline per guard — supaya konsisten begitu multi-bahasa diperlukan. Satu file (`forma-error-glossary.yaml`), didistribusikan sebagai bagian Core Basic Spec, versinya ikut versi Core Basic.

**Satu field (`code`) untuk matching program maupun lookup terjemahan** — tidak perlu field `key` terpisah. Keduanya sama-sama harus permanen setelah rilis (kode tidak boleh berubah makna, key terjemahan juga tidak boleh berubah), jadi memisahkan keduanya tidak menyelesaikan masalah nyata, cuma menambah field redundan. Library i18n modern tidak peduli huruf besar/kecil pada key lookup.

```yaml
- code: FORMA.DOC.UPDATE_NOT_DRAFT
  params: [resource_name, doc_status]
  default_message: "{resource_name} tidak bisa diubah karena berstatus {doc_status}, bukan draft"

- code: FORMA.DOC.DELETE_REFERENCED
  params: [resource_name, blocking_resource, blocking_id]
  default_message: "{resource_name} tidak bisa dihapus karena masih direferensi oleh {blocking_resource} #{blocking_id}"

- code: FORMA.DOC.CANCEL_REFERENCED
  params: [resource_name, blocking_resource, blocking_id]
  default_message: "{resource_name} tidak bisa dibatalkan karena masih direferensi oleh {blocking_resource} #{blocking_id}"

- code: FORMA.PERIOD.CLOSED
  params: [transaction_date, period_ref]
  default_message: "Tidak bisa posting ke {transaction_date}, periode {period_ref} sudah ditutup"

- code: FORMA.TXN.BACKDATE_EXCEEDED
  params: [transaction_date, max_days_back]
  default_message: "Tanggal transaksi melebihi batas backdate {max_days_back} hari"

- code: FORMA.SAGA.OUTCOME_UNKNOWN
  params: [event_name, target_resource]
  default_message: "Hasil pemanggilan {target_resource} tidak dapat dipastikan setelah retry habis — butuh verifikasi manual"
```

**Aturan yang dikunci:**

- `code` tidak pernah diubah/dipakai ulang artinya setelah dirilis — integrasi pihak ketiga yang `switch(error.code)` tidak boleh rusak diam-diam. Kalau ada situasi baru, tambah entri baru.
- Hanya mencakup error dari mekanisme baku framework. Error dari `conditions:` custom milik developer tetap bebas menulis pesan sendiri, tidak wajib masuk glossary ini — tapi dianjurkan ikut format `code`+`params` yang sama, dengan namespace App sendiri (bukan `FORMA.*`).

---

## 15. Pola UI Admin Panel: Draft/Submit vs CRUD Polos

**Sinyal penentu: apakah action `submit` di-disable atau aktif — bukan `type: document` atau `characteristics: [transaction]` semata.** Dua flag itu independen: `characteristics: [transaction]` murni soal tanggal/periode akuntansi (§10), sementara pola UI murni soal apakah lifecycle draft→submit bermakna secara bisnis. Ada resource yang butuh lifecycle tanpa transaction_date (mis. pengajuan cuti karyawan — perlu approval, bukan transaksi akuntansi), dan resource master data yang butuh transaction_date tapi tidak punya lifecycle (jarang, tapi mungkin) — jangan digabung jadi satu flag.

```
Action "submit" di-disable eksplisit (lihat §3.4)
  → CRUD polos: satu tombol "Simpan", tidak ada tombol Submit,
    tidak ada konsep draft yang ditampilkan ke user
    (doc_status tetap "draft" selamanya secara teknis, tapi tersembunyi dari UI)

Action "submit" AKTIF (default kalau tidak ditulis)
  → Pilih salah satu dari tiga pola di bawah, lewat manifest `ui:` hint.
    Default kalau tidak dideklarasikan: 2-step + auto-save.
```

**Tiga pola untuk resource dengan lifecycle aktif:**

| Pola | Kapan dipakai | UI yang muncul |
|---|---|---|
| **2-step + auto-save** (default) | Dokumen kompleks, butuh review (Invoice, Order, Contract) | Auto-save senyap saat draft (debounced `update`), satu tombol eksplisit "Submit" |
| **2-step manual** | Draft sengaja dibagi ke orang lain untuk direview dulu | Tombol "Simpan Draft" + "Submit" terpisah |
| **1-step (composite)** | Entry cepat volume tinggi (POS, antrian klinik) | Satu tombol, reuse Composite Action (§3.2) `create-and-submit` — tidak ada konsep draft terlihat di UI meski `doc_status` tetap lewat `draft` sesaat secara teknis, atomik (all-or-nothing) |

```yaml
resource:
  name: invoice
  type: document
  characteristics: [transaction]

actions:
  - name: create-and-submit      # opt-in eksplisit untuk pola 1-step
    composite: true
    calls: [create, submit]
    ui:
      button_label: "Simpan & Submit"
      style: primary
      show_when: "quick_entry_mode"
```

Dua tombol standar (**Simpan Draft**/auto-save, **Submit**) selalu otomatis tersedia dari model tanpa perlu dideklarasikan. Composite Action seperti `create-and-submit` di atas menambah tombol ketiga sebagai jalur cepat opsional — tidak menggantikan dua tombol dasar.

---

## 16. Open Questions

- Ini adalah revisi cukup dalam ke Core Basic v0.1.9 Section 12 (State Machine Spec) dan Section 3.1 (Resource Types) — perlu diputuskan apakah masuk sebagai v0.2.0 dengan breaking change eksplisit, atau strategi migrasi bertahap.

---

*Dokumen ini adalah rangkuman kerja dari sesi diskusi panjang. Tujuannya menyimpan alur penalaran dan keputusan agar tidak hilang. Bukan keputusan final — perlu direview dan diformalkan sebagai revisi resmi Forma Core Basic Spec.*

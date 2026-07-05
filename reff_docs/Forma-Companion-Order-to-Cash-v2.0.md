# Forma Technical Companion: Order-to-Cash (v2.0)

**Status:** Draft — ditulis dengan **Forma Core Basic Spec v0.2.0** secara harfiah.
**Fungsi ganda:** (1) contoh kanonik Forma (Foundation D16), (2) *test drive* spec v0.2.0 — gesekan yang ditemukan dicatat di bagian 7.
**Menggantikan:** companion lama (arsip), yang ditulis untuk spec v0.1.9.

---

## 1. Kebutuhan Bisnis: Mini Order-to-Cash

**Alur:** customer checkout → bayar via payment gateway → setelah bayar sukses: nota PDF dibuat, email nota dikirim, notifikasi WA dikirim, jurnal akuntansi otomatis terbentuk, dashboard admin menampilkan pembayaran masuk real-time.

| # | Requirement | Kenapa sulit tanpa konvensi |
|---|---|---|
| FR1 | Nomor order berurutan (`ORD-2026-000123`), tak duplikat saat checkout bersamaan | Race condition klasik |
| FR2 | Payment gateway (Midtrans/Xendit) dengan mode simulasi saat dev | Integrasi eksternal tanpa pola baku |
| FR3 | Webhook bisa terkirim berkali-kali — tak boleh diproses dua kali | Idempotency persisten, bukan cache |
| FR4 | Setelah bayar, **wajib** ada jurnal akuntansi — tak boleh hilang/dobel | Transactional outbox, bukan fire-and-forget |
| FR5 | Email + WA boleh telat, tak boleh gagal diam-diam | Background job reliable (retry) |
| FR6 | Dashboard live ticker pembayaran masuk | Boleh hilang jika admin offline — kelas beda dari FR4/FR5 |
| FR7 | Diskon per tier membership di-cache, bisa di-invalidate saat aturan berubah | Cache yang boleh hilang + invalidasi manual |
| FR8 | Prefix nomor & template notifikasi per workspace tanpa redeploy | Config dinamis |
| FR9 | Semua langkah tercatat log terstruktur, satu correlation ID | Observability konsisten |
| FR10 | PDF nota tersimpan, bisa diunduh ulang | Object storage, bukan filesystem container |

FR1, FR3, FR4 paling kritis — salah di sini berarti uang salah hitung atau jurnal hilang. **Cakupan perbandingan yang adil:** Skenario A (bagian 2) hanya pernah diminta menjawab FR1, FR3, FR4; FR lainnya didemonstrasikan hanya di Skenario B sebagai bukti kelengkapan arsitektur — bukan klaim "AI tanpa Forma tidak bisa".

---

## 2. Skenario A — Tanpa Forma (Plain Go/Gin, tiga putaran review)

Ringkasan tiga putaran prompt → kode → review (kode lengkap di arsip companion lama):

**Putaran 1** — *"Buatkan fungsi generate nomor order berurutan dan handler webhook pembayaran."*
Hasil: `SELECT MAX(seq)+1` (race condition), goroutine fire-and-forget untuk jurnal (hilang saat restart), tanpa idempotency.

**Putaran 2** — *"Perbaiki race condition-nya."*
Hasil: advisory lock — race teratasi. Tapi AI *sekalian* mengganti goroutine menjadi Redis Pub/Sub — regresi yang tidak diminta: event finansial kini lewat kanal non-durable.

**Putaran 3** — *"Tambahkan idempotency dan pastikan event jurnal tidak hilang saat restart."*
Hasil terlihat benar — queue dipakai, idempotency ada. Dua bug subtil lolos: (1) idempotency key disimpan di Redis dengan TTL 60 detik — jendela balapan tersembunyi; (2) `UPDATE` status dan `Enqueue` jurnal tetap **dua operasi tanpa transaksi bersama** — order bisa "paid" tanpa job jurnal pernah terkirim. *Persis requirement yang diminta diperbaiki, hanya berpindah bentuk.*

| Aspek | P1 | P2 | P3 |
|---|---|---|---|
| Race nomor order | ❌ | ✅ | ✅ |
| Event finansial reliable | ❌ | ❌ (regresi) | ⚠️ non-atomic |
| Idempotency webhook | ❌ | ❌ | ⚠️ TTL 60s |
| Audit trail | ❌ | ❌ | ❌ (tak pernah diminta) |

Masalah yang *masih* belum tersentuh setelah tiga putaran: tanpa auth/permission, `MAX(seq)` tak difilter per tenant, webhook tanpa verifikasi signature, tanpa validasi item/total, lock global lintas tenant, tanpa audit log. **Intinya:** AI memperbaiki tepat apa yang diminta, tidak lebih — kualitas akhir dibatasi kelengkapan pengetahuan reviewer tentang kelas-kelas bug, bukan kemampuan AI. Jumlah putaran yang dibutuhkan tidak punya batas yang jelas.

---

## 3. Dari FR ke Konstruksi Forma

### 3.1 Struktur project (tiga jenis file — Foundation D14)

```
tokoku/
  forma.yaml                          # kind: App
  modules/
    billing/
      module.yaml                     # kind: Module
      entities/order.yaml             # kind: Entity
      services/payment-gateway.yaml   # kind: Service (wrapper Midtrans/Xendit)
      services/notify-wa.yaml         # kind: Service (wrapper WA Business API)
      scripts/order_checkout.star
    notifications/
      module.yaml
      subscriptions/wa-on-order-paid.yaml   # kind: Subscription (§4.7)
    gl/
      module.yaml
      entities/journal-entry.yaml
  mockups/payment-gateway/            # kind: Mockup (Extended) — simulasi saat dev
```

### 3.2 Setiap FR punya rumah yang spesifik

| FR | Konstruksi | Kenapa di situ |
|---|---|---|
| FR1 | `natural_key_rule` di field + `ctx.next_key()` di script | Rule di skema, locking dibungkus helper — tak pernah ditulis manual |
| FR2 | `kind: Service` `payment-gateway` + deklarasi di `uses.resources` | Integrasi eksternal WAJIB dibungkus Service (§4.2) |
| FR3 | `idempotent: true` + `idempotency_key` — ditegakkan framework | Dideklarasikan di kontrak, handler bersih (D32) |
| FR4 | `publish.durable: true` + `deliver.reliable_event` | Kolom wajib kontrak — bukan keputusan yang bisa lupa |
| FR5 | email: `deliver` publisher (janji billing); WA: `kind: Subscription` (D35) — ditambah belakangan tanpa menyentuh order.yaml | Konsekuensi event = deklaratif; garis pembagi = kepemilikan jaminan |
| FR6 | `deliver: {channel: websocket}` | Boleh hilang — tak mungkin tertukar dengan FR5 |
| FR7 | `ctx.cache` di job `generate-receipt` + invalidasi di `update-discount-rule` | |
| FR8 | `kind: Config` + `ctx.config(...)` | |
| FR9 | `ctx.log` — tenant/request/user ID otomatis | |
| FR10 | `ctx.storage.write(...)` | |

Urutan kerja (sama untuk manusia maupun AI dengan Agent Skill): manifest dulu (kontrak) → putuskan `impl` per action → tulis isi impl → `forma generate`.

---

## 4. Skenario B — Dengan Forma Core Basic v0.2.0

### 4.1 App manifest

```yaml
apiVersion: forma.dev/v1alpha1
kind: App
metadata:
  name: tokoku
  description: Mini order-to-cash
spec:
  version: 1.0.0
  vendor: acme-corp
  modules: [billing, gl]
  # publishes: []  — tidak ada; app ini tidak menawarkan interface lintas-app
  # consumes:  []  — payment gateway adalah integrasi eksternal (Service wrapper),
  #                  bukan provider app Forma — jadi bukan grant lintas-app
```

### 4.2 Entity `order`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: order
  module: billing
  description: Order pelanggan dari checkout sampai lunas
spec:
  version: v1
  characteristics: [transaction]
  # tenant: TIDAK ADA — workspace-isolated by default, tanpa opsi lain (§8)

  auth: { required: true, strategies: [token] }

  fields:
    - name: number
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      natural_key_rule:
        strategy: sequence
        format: "{prefix}-{year}-{seq:06d}"
        prefix: { config: billing.order_number_prefix, default: "ORD" }
        reset: yearly
    - name: customer_id
      type: relation
      relation: { type: belongs_to, resource: customer }
    - name: items
      type: child
      child:
        storage: jsonb
        sequence_field: line_number
        fields:
          - { name: line_number, type: integer, immutable: true }
          - { name: product_id,  type: uuid,    rules: [required, {exists: product}] }
          - { name: quantity,    type: integer, rules: [required, positive] }
          - { name: price,       type: decimal, rules: [required, positive] }
    - name: total
      type: decimal
      rules: [required, positive]
    - name: member_tier
      type: enum
      enum_values: [regular, silver, gold]
      # Snapshot tier SAAT checkout — sengaja didenormalisasi: kalau customer
      # naik tier bulan depan, nota bulan lalu tidak berubah retroaktif.
    - name: status
      type: enum
      enum_values: [draft, awaiting_payment, paid, fulfilled, void]
      index: true
    - name: gateway_reference
      type: string
    - name: paid_at
      type: datetime

  state_machine:
    field: status
    initial: draft
    transitions:
      - { from: draft,            to: awaiting_payment, via: checkout,
          guard: "len(resource.items) > 0 and resource.total > 0" }
      - { from: awaiting_payment, to: paid,             via: mark-paid }
      - { from: [draft, awaiting_payment], to: void,    via: void }

  actions:
    # Override standard "update" — TANPA ini, PATCH tersedia tanpa restriksi
    # bahkan untuk order "paid". State machine hanya mengatur transisi kolom
    # status via action bernama — TIDAK otomatis membatasi update standar.
    - name: update
      conditions:
        - script: "resource.status == 'draft'"
          message: "Order yang sudah checkout tidak bisa diedit — gunakan 'void'"

    # Hard delete tidak masuk akal untuk transaction — jejak audit harus hidup.
    - name: delete
      disabled: true

    - name: checkout
      description: Generate nomor order & buat sesi pembayaran
      required_permission: orders.checkout       # auto-prefix → billing.orders.checkout
      audit: true
      conditions:
        # D33 — "declared location, scripted expression": aturan bisnisnya TERLIHAT
        # di kontrak, ekspresinya tetap Starlark. Bukan sintaks YAML baru.
        - script: "not customer.load(resource.field.customer_id).field.is_blacklisted"
          message: "Customer diblokir, tidak bisa checkout"
      uses:                                       # D20 — eksplisit, semua impl type
        resources: [payment-gateway.create-session, customer.find]
        config: { read: [billing.order_number_prefix] }
        primitives: [lock]                        # dipakai ctx.next_key di balik layar
      impl: { type: script_ref, ref: billing/order_checkout }

    - name: mark-paid
      description: Transisi ke paid — dipanggil oleh payment-gateway.webhook (§4.4)
      required_permission: orders.mark-paid
      idempotent: true          # ditegakkan FRAMEWORK (§11.8/D32)
      idempotency_key: { from: param, field: event_id }
      audit: true
      emits: paid
      params:
        validate:
          - { field: gateway_reference, rules: [required] }
      impl: { type: script_ref, ref: billing/order_mark_paid }
      # Handler-nya kini 3 baris (§4.4) — semua konsekuensi pembayaran pindah
      # ke blok deliver di bawah (D33): langkah vs konsekuensi.

    - name: update-discount-rule
      description: Admin mengubah persen diskon satu tier — invalidasi cache
      required_permission: orders.manage-discount
      audit: true
      uses:
        db: { write: [billing] }
        primitives: [cache]
      impl: { type: native, ref: "OrderResource.UpdateDiscountRule" }

  events:
    - name: paid
      description: Order berhasil dibayar
      publish:
        durable: true           # WAJIB untuk event finansial — syarat spec, bukan selera
      payload:
        fields: [id, number, total, customer_id, paid_at]
      deliver:
        - channel: audit_log
        - channel: websocket
          target: { scope: tenant }               # FR6 — ticker admin, boleh hilang
        - { channel: queue, job: generate-receipt }        # FR10 + FR7 (D33)
        - { channel: queue, job: send-receipt-email }      # FR5 — nota = janji billing
        # send-wa-notification TIDAK di sini — dia bukan kontrak billing.
        # Rumahnya kind: Subscription di modul notifications (§4.7, D35).
        - channel: reliable_event                  # FR4 — jurnal, tak boleh hilang
          target: { resource: gl.journal-entry, action: create }
          retry: { max: 10, backoff: exponential, initial_delay_ms: 1000 }
          dead_letter: { resource: failed-event, action: create }
          idempotency_key: "order.paid.{id}"
```

Perhatikan blok `deliver` ini sekarang adalah **peta lengkap konsekuensi pembayaran** — empat kelas jaminan berbeda terbaca dalam satu tempat: audit, realtime-boleh-hilang, background-reliable, dan transaksional. Di Skenario A, informasi yang sama tersebar di goroutine, Pub/Sub, dan queue call yang masing-masing bisa salah pilih.

**Tabel masalah Skenario A → jawaban baris per baris:** auth wajib default + `required_permission` per action (tanpa auth = mustahil, bukan terlupakan) · nomor order otomatis per-tenant (counter `forma_natural_key_counters` ber-key tenant — lock tidak pernah global lintas tenant) · validasi item/total = guard state machine + field rules · audit = `audit: true` · hilangnya event = mustahil karena `mark-paid` mengubah status DAN menulis outbox dalam satu transaksi DB. **Yang jujur belum otomatis:** verifikasi signature webhook — rumahnya `kind: Webhook` (Extended, strategi hmac/rsa); bedanya dengan Skenario A: tempatnya sudah pasti, bukan lubang yang harus ditemukan dulu.

### 4.3 Script `checkout` (FR1, FR2) — script_ref, editable dari admin panel

```python
# modules/billing/scripts/order_checkout.star

def execute(resource, params, ctx):
    # Blacklist check TIDAK di sini lagi — dia precondition, rumahnya di
    # blok `conditions` action (lihat §4.2) sehingga terbaca di kontrak.
    # Yang tersisa di script murni PROSEDUR: langkah berurutan yang hasilnya
    # dipakai langkah berikutnya (litmus test D33).

    # FR1 — rule ada di field (natural_key_rule); ctx.next_key membaca rule itu,
    # menangani lock + reset period + format. Tidak pernah MAX()+1 manual.
    number = ctx.next_key("number")
    resource.set("number", number).save()

    # FR2 — gateway dipanggil HANYA lewat Service wrapper yang dideklarasikan
    # di uses.resources. Saat dev: otomatis ke mockup; saat prod: konektor asli.
    session = payment_gateway.call("create-session", {
        "order_id": resource.id,
        "amount":   resource.field.total,
    })

    ctx.log.info("order.checkout", {"order_id": resource.id, "number": number})
    return ok({"payment_url": session.payment_url})
```

### 4.4 Alur webhook yang benar: `payment-gateway.webhook` → `order.mark-paid` → `deliver`

Companion lama punya inkonsistensi laten: handler mark-paid memanggil
`order.Call("mark-paid")` dari dalam impl mark-paid sendiri — rekursif.
Alur yang benar tiga lapis, masing-masing kecil:

**Lapis 1 — penerima webhook** (action di Service `payment-gateway`, §4.5):
verifikasi payload (signature = Extended `kind: Webhook`), lalu panggil
`order.mark-paid` meneruskan `event_id` + `gateway_reference`.

**Lapis 2 — action `mark-paid`** kini hanya melakukan *langkahnya sendiri*:

```python
# modules/billing/scripts/order_mark_paid.star
def execute(resource, params, ctx):
    # FR3 sudah beres SEBELUM baris ini jalan: framework menolak duplikat
    # event_id dan me-replay response asli (idempotent: true, §11.8/D32).
    resource.set("gateway_reference", params.gateway_reference)
    resource.set("paid_at", ctx.now())
    resource.save()   # transisi awaiting_payment→paid + tulis outbox event
                      # "paid" — SATU transaksi DB (jawaban Bug #2 Skenario A)
    return ok()
```

**Lapis 3 — semua konsekuensi = blok `deliver`** (lihat §4.2): jurnal
(reliable), ticker (websocket), dan tiga job queue. Satu-satunya kode Go
yang tersisa adalah handler job:

```go
// Job "generate-receipt" — dipicu deliver channel: queue (D33)
func GenerateReceiptHandler(ctx forma.Context, evt PaidEvent) error {
    order, _ := forma.Load[Order](ctx, evt.ID)

    // FR7 — cache diskon + fallback; invalidasi eksplisit di update-discount-rule
    pct, found := ctx.Cache().Get("member-discount:" + order.MemberTier)
    if !found { pct = fetchMemberDiscount(order.MemberTier); ctx.Cache().Set(..., 3600) }

    // FR10 — PDF ke object storage; FR9 — correlation ID otomatis di semua log
    ctx.Storage().Write("receipts/"+order.Number+".pdf",
        renderReceiptPDF(order, pct), forma.StorageOpts{Visibility: "private"})
    ctx.Log().Info("receipt.generated", map[string]any{"order_id": order.ID})
    return nil   // gagal → retry otomatis oleh queue (FR5-class guarantee)
}
```

Bandingkan dengan `HandlePaymentWebhook` Skenario A: logika yang sama kini
terpecah menjadi kontrak yang bisa dibaca (manifest + deliver) dan dua
handler kecil yang masing-masing hanya tahu satu hal.

### 4.5 Integrasi eksternal = Service, tidak ada jalur lain

```yaml
apiVersion: forma.dev/v1alpha1
kind: Service
metadata: { name: payment-gateway, module: billing,
            description: Wrapper Midtrans/Xendit (mockup saat dev) }
spec:
  version: v1
  actions:
    - name: create-session
      required_permission: payment-gateway.create-session
      uses: { primitives: [] }        # panggilan HTTP keluar dikelola konektor/mockup
      impl: { type: native, ref: "PaymentGateway.CreateSession" }
    - name: webhook
      description: Callback dari gateway — verifikasi lalu teruskan ke order
      required_permission: payment-gateway.webhook   # dipegang api-key khusus gateway
      idempotent: true
      idempotency_key: { from: param, field: event_id }
      uses:
        resources: [order.mark-paid]
      impl: { type: native, ref: "PaymentGateway.Webhook" }
      # Verifikasi signature payload = Extended (kind: Webhook, hmac/rsa) —
      # gap yang diakui, tempatnya sudah pasti (temuan #4).
---
apiVersion: forma.dev/v1alpha1
kind: Service
metadata: { name: notify-wa, module: billing,
            description: Wrapper WhatsApp Business API }
spec:
  version: v1
  actions:
    - name: send
      call: async
      required_permission: notify-wa.send
      impl: { type: native, ref: "NotifyWA.Send" }
```

Karena `kind: Service`, keduanya otomatis tunduk auth, permission, audit, dan isolasi workspace — tidak ada cara "lupa mengamankan" karena tidak ada jalur lain memanggil API eksternal. Email memakai modul resmi `forma/mail` (D12) di belakang job `send-receipt-email`.

### 4.6 Sisi penerima: `gl.journal-entry`

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata: { name: journal-entry, module: gl }
spec:
  version: v1
  characteristics: [transaction]
  fields:
    - { name: source,     type: string,  immutable: true, index: true }
    - { name: source_id,  type: uuid,    immutable: true, index: true }
    - { name: amount,     type: decimal, rules: [required] }
    - { name: entry_date, type: date,    rules: [required] }
  actions:
    - name: create
      required_permission: journal-entries.create   # → gl.journal-entries.create
      idempotent: true      # outbox worker boleh retry — duplikat tertolak di sini juga
      audit: true
```

Worker outbox memanggil `create` secara **sync** dengan idempotency check dua sisi (`idempotency_key` di deliver + `idempotent: true` di target) — jurnal tidak hilang dan tidak dobel, bahkan saat retry.

### 4.7 Reaksi yang lahir belakangan: `kind: Subscription` (D35)

Skenario nyata: sistem sudah jalan berbulan-bulan, lalu muncul ide
notifikasi WA saat order dibayar. **`order.yaml` tidak disentuh sama
sekali** — apalagi kalau `billing` adalah modul marketplace ter-sign yang
memang tidak bisa diedit. Cukup satu manifest baru di modul pelanggan:

```yaml
# modules/notifications/subscriptions/wa-on-order-paid.yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata:
  name: wa-on-order-paid
  module: notifications
spec:
  on: { resource: billing.order, event: paid }
  deliver:
    - { channel: queue, job: send-wa-notification }   # via Service notify-wa
```

Garis pembaginya (D35): jurnal tetap di `deliver` order karena dia *janji
billing* (FR4); WA adalah kepentingan modul notifications — Subscription.
Kekhawatiran "deliver tersebar tak disadari" dijawab struktural:
`forma describe entity billing.order` menampilkan **fan-out gabungan**
(deliver publisher + semua Subscription penunjuknya), Subscription masuk
footprint consent saat install modulnya, dan kontrak durability dua sisi
tetap berlaku.

---

## 5. Skenario C — Forma + Agent Skill

Agent Skill membuat AI menulis Skenario B secara konsisten. Aturan wajibnya (ringkas): manifest dulu, `impl` belakangan · setiap action wajib `required_permission` + `uses` — tidak ada, tolak generate · event finansial wajib `durable: true` + reliable_event · nomor berurutan wajib `natural_key_rule` + `ctx.next_key` — `MAX()+1` = tolak · integrasi eksternal wajib `kind: Service` · webhook wajib `idempotent: true`. Bedanya dengan Skenario A: aturan-aturan ini bukan pengetahuan yang harus dimiliki reviewer — mereka syarat struktural yang tidak bisa dilewati.

| | A (plain) | B (Forma) | C (Forma + Skill) |
|---|---|---|---|
| Putaran sampai benar | 3+, tanpa batas jelas | 1 (struktur memaksa) | 1 (AI dipagari) |
| Bug subtil lolos | 2 terbukti | titik rawannya terdeklarasi | idem |
| Konsistensi antar developer | tergantung orang | konvensi | konvensi |

---

## 6. Pemetaan ke Primitif (closed set — semua terpakai natural)

| Primitif | Dipakai di | FR |
|---|---|---|
| `ctx.db` | `update-discount-rule` (write [billing]); laporan via read tier | FR7 |
| `ctx.cache` | diskon membership + invalidasi eksplisit | FR7 |
| `ctx.lock` | via `ctx.next_key` (tenant-scoped otomatis) | FR1 |
| `ctx.queue` | via `deliver channel: queue` (D33) — email, WA, receipt jobs | FR5 |
| `ctx.pubsub` | via `deliver channel: websocket` — ticker dashboard | FR6 |
| `ctx.storage` | PDF nota | FR10 |
| `ctx.kvstore` | dipakai framework untuk idempotency store (§11.8) — handler tidak menyentuhnya | FR3 |
| `ctx.config` | prefix nomor per workspace | FR8 |
| `ctx.log` | semua titik penting | FR9 |

Outbox (FR4) bukan primitif ke-10 — dia perilaku `publish.durable` di atas DB.

---

## 7. Temuan Test Drive Spec v0.2.0

Menulis dokumen ini dengan v0.2.0 secara harfiah menemukan empat hal:

1. **`idempotent: true` belum punya semantik mekanis di spec.** → **SELESAI: D32 + §11.8.** Kini ditegakkan framework: idempotency store `(tenant, action, key)` dengan **response replay**, key dari client (header/param — webhook) atau server-issued via prepare-step (double-submit form); ditambah optimistic concurrency via `version` CAS, dan `updated_at` diturunkan jadi metadata audit murni. Manifest §4.2 dan handler §4.4 dokumen ini sudah memakai bentuk finalnya — handler bersih dari cek manual.
2. **Validasi cross-resource (blacklist customer) tidak punya rumah deklaratif di Core Basic** — level 4–6 memang Extended; di Core Basic rumahnya di dalam script action. Bukan bug, tapi harus dikatakan eksplisit di spec §13 agar tidak dicari-cari. → perbaikan redaksi §13.
3. **Target reliable_event lintas modul perlu bentuk qualified** — `resource: gl.journal-entry` dipakai di sini; §12.3 baru mencontohkan nama polos. → perbaikan redaksi §12.3.
4. **Verifikasi signature webhook** tetap gap yang diakui (Extended `kind: Webhook`) — konsisten dengan companion lama, tempatnya sudah pasti.
5. **Garis deklaratif vs imperatif dinormatifkan (D33)** — pertanyaan "blacklist/gateway/queue di YAML atau script?" terjawab litmus test: fakta/jaminan → YAML, prosedur → handler, konsekuensi event → `deliver`, dan vocabulary YAML = closed set (escape hatch selalu ekspresi Starlark di konstruksi yang ada). Penerapannya: handler mark-paid menyusut dari ±30 baris jadi script 3 baris + blok `deliver` yang menjadi peta lengkap konsekuensi pembayaran; sekaligus menemukan `channel: queue` yang hilang di §12.3 dan memperbaiki alur webhook rekursif dari companion lama (kini: `payment-gateway.webhook` → `order.mark-paid` → deliver).
6. **`kind: Subscription` lahir dari skenario "ide muncul setelah sistem jalan" (D35/§12.5)** — WA-notif ternyata salah rumah di deliver order (bukan janji billing); kini jadi Subscription di modul notifications (§4.7). Sekaligus prasyarat ekosistem: modul ter-sign tidak bisa diedit, jadi reaksi pihak lain wajib bisa dideklarasikan dari luar — dengan fan-out yang selalu terkompilasi di `forma describe`.

Selebihnya: seluruh skenario terekspresikan tanpa satu pun konstruksi di luar spec — format manifest, `uses`, workspace-default, dan permission auto-prefix terasa *lebih ringkas* daripada v0.1.9 (blok `tenant:` dan blok `permissions:` per-resource hilang sama sekali dari YAML).

# GAP #40–#43 — Integrasi Lintas-App

Ditemukan saat menulis Tahap 3e: menyambungkan penjualan kafe ke vertical `gl`
(jurnal akuntansi). Ini bagian yang paling lama diminta pemilik kafe —
*"untuk sistem akuntansi penuh seharusnya ada modul vertical yang tersedia dan
bisa diintegrasikan"* — dan justru di sinilah FormSpec paling banyak menunjukkan
kesenjangan **kontrak**, bukan kesenjangan kode.

Ringkasan: vertical akuntansinya **ada dan lengkap**; yang belum ada adalah cara
**menyatakan integrasi** ke sana secara bermakna.

---

## Gap #40 — Tidak jelas apakah transisi state machine memancarkan event ✅ Terverifikasi (inkonsistensi)

### Bukti

Dua sumber yang saling bertentangan:

| Sumber | Bentuk nama event |
| --- | --- |
| Dokumentasi vertical FormSpec (`verticles/README.md`, `07-vertical-modules.md`) | `billing.order.**paid**` — nama **state** |
| `ValidateEvents()` di `pkg/spec/entity.go` | mewajibkan prefix: `before_*` = sync, `on_*` = async; nama lain butuh `type` eksplisit |
| Skill `entity-authoring` | *"Events named `before_*` are sync gates… Events named `on_*` are async notifications… Custom event names (no before_/on_ prefix) require an explicit type field"* |

Ditambah lagi validator mengungkap aturan ketiga yang tidak disebut di mana pun:
untuk keperluan integritas, nama yang dikenali adalah **`on_cancel` /
`before_cancel`** (lihat Gap #43).

Yang **tidak** jelas: apakah transisi state machine memancarkan event
**otomatis** (mis. transisi ke state `paid` → event `paid`), atau event harus
dipancarkan eksplisit dari script action. Dokumentasi vertical menyiratkan
otomatis; konvensi validator menyiratkan eksplisit.

### Dampak ke aplikasi kafe: **HIGH**

Seluruh integrasi stok & jurnal bergantung pada `on_paid` benar-benar terpancar.
Spec ini mendeklarasikan event secara eksplisit (`on_paid`, `on_cancel`) supaya
konsisten dengan validator — tetapi **belum bisa dipastikan event itu dipancarkan
saat transisi `awaiting_payment -> paid` terjadi**, karena verifikasi runtime
terkunci **GAP-24**.

Kalau ternyata tidak otomatis, integrasi akuntansi diam-diam tidak pernah jalan:
tidak ada error, tidak ada jurnal, dan tidak ada yang tahu sampai tutup buku.

### Usulan

1. Nyatakan tegas di dokumentasi: apakah transisi memancarkan event otomatis,
   dan bila ya — apakah namanya mengikuti state atau mengikuti aksi transisi.
2. Bila eksplisit: dokumentasikan **di mana** pemancaran itu ditulis (script
   action? hook?) dan tambahkan contohnya.
3. Selaraskan dokumentasi vertical yang memakai `entity.state` sebagai nama event.

---

## Gap #41 — `Integrator` tidak punya pemetaan payload ⚠️ Verifikasi

### Bukti

`IntegratorSpec` (`schemas/v1/kinds/Integrator.schema.json`):

```json
"IntegratorSpec": {
  "properties": {
    "listen":     { "$ref": "#/$defs/IntegratorListen" },
    "call":       { "$ref": "#/$defs/IntegratorCall" },
    "compensate": { "type": "string" }
  },
  "required": ["listen", "call"],
  "additionalProperties": false
}
```

`IntegratorCall` hanya `{resource, action}`. **Tidak ada tempat** mendeklarasikan
bagaimana data sumber dipetakan ke data target.

Masalahnya nyata untuk kafe, karena kedua sisi tidak akan pernah punya nama field
yang sama:

| Sisi kafe (`order`) | Sisi akuntansi (`gl.journal-entry`) |
| --- | --- |
| `total_amount` (money) | `debit_account`, `credit_account` |
| `tax_amount` (money) | `entry_date` |
| `branch_id` (relation) | `lines[]` dengan akun COA |
| `subtotal` | `memo` |

Pemetaan *"omzet → kredit 4-1000, pajak → kredit 2-2000, kas → debit 1-1000"*
adalah **pengetahuan akuntansi**, bukan penamaan field. Ia tidak bisa disimpulkan.

### Dampak ke aplikasi kafe: **HIGH** — menyerang langsung permintaan pemilik

Tanpa blok pemetaan, integrasi harus hidup di salah satu sisi, dan keduanya buruk:

1. **Di script milik `gl`** — vertical pihak ketiga. Mengubahnya berarti
   mem-fork module vendor (ada mekanisme `overrides/`, tapi artinya kita
   mengambil alih tanggung jawab pemeliharaannya), dan logika khas kafe
   (PB1, service charge) menumpang di module akuntansi generik.
2. **Di action kustom sisi kafe** yang memanggil `gl` — integrasinya kembali
   menempel dan tidak bisa diganti vendor. `kind: Integrator` kehilangan gunanya.

Bentuk yang dibutuhkan kira-kira:

```yaml
call:
  resource: gl.journal-entry
  action: create
  map:
    entry_date: "{order.paid_at}"
    memo: "Penjualan {order.number}"
    lines:
      - { account: "1-1000", debit: "{order.total_amount}" }
      - { account: "4-1000", credit: "{order.subtotal}" }
      - { account: "2-2000", credit: "{order.tax_amount}" }
```

Catatan: seluruh nilai yang dipetakan adalah `money` (objek `{amount, currency}`)
— jadi **GAP-28** juga berlaku di sini. Aritmetika money yang belum terverifikasi
berdampak paling mahal pada jurnal akuntansi.

### Usulan

1. Tambah blok pemetaan/transform eksplisit pada `IntegratorCall`
   (mis. `map:` dengan FormSpecExpr), atau
2. Tegaskan bahwa pemetaan memang tanggung jawab sisi penerima, dan sediakan
   **cara untuk menyatakannya** di module yang meng-consume — mis.
   `uses.resources` + action adapter yang dideklarasikan, bukan script tersembunyi.

---

## Gap #42 — Dua App meng-mount module yang sama: siapa pemilik antarmuka? ✅ Terverifikasi (secara desain)

### Bukti

Di aplikasi kafe ini, module `cafe-order` di-mount oleh **dua** App:

```yaml
# spec/apps/kafe-qr.yaml        (public, pelanggan memesan sendiri)
modules: [cafe-master, cafe-order]

# spec/apps/kafe-pos.yaml       (private, kasir)
modules: [cafe-order, cafe-stock, cafe-master, cafe-loyalty, cafe-report]
```

Keduanya memproduksi event `on_paid` — karena event itu milik **module**, bukan
App. Sementara `publishes` dideklarasikan **per-App**, dan grant lintas-app
menyasar **App** (`AppConsume.app`).

Jadi: bila `kafe-qr` juga memesan dari pelanggan (memang itu tujuannya), pesanan
yang lunas lewat jalur QR tidak tercakup oleh `publishes` milik `kafe-pos`.

### Dampak ke aplikasi kafe: **MEDIUM–HIGH**

Ini bukan kasus tepi di kafe — QR ordering **memang** menghasilkan pesanan
sendiri. Kalau antarmuka `order-events` dianggap milik `kafe-pos` saja, maka:

- Integrator yang meng-consume `kafe-pos:order-events` akan **kehilangan
  separuh penjualan** (yang datang lewat QR).
- Tidak ada aturan yang menentukan apakah dua App yang sama-sama meng-mount satu
  module harus mendeklarasikan `publishes` yang sama, atau apakah itu dua
  antarmuka berbeda.
- `formspec validate` **hijau** — tidak ada yang menangkap ketidakjelasan ini.

### Usulan

1. Nyatakan bahwa **pemilik antarmuka adalah MODULE**, bukan App — karena event
   dan entity memang hidup di module. `publishes` di App kemudian berarti
   "App ini menyajikan antarmuka module X", dan grant menyasar module pemilik.
2. Atau, bila memang per-App: nyatakan bagaimana dua App yang meng-mount module
   sama memperlakukan `publishes` — dan tambahkan peringatan validasi bila salah
   satu tidak mendeklarasikannya padahal module-nya sama.

---

## Gap #43 — Aturan simetri cancel ditegakkan tapi tidak terdokumentasi ✅ Terverifikasi — **dan ini aturan yang bagus**

### Bukti

Menulis Integrator satu arah (`on_paid` → jurnal) **ditolak** validator:

```
[FAIL] spec\modules\cafe-gl-integrator\integrators\order-paid-to-journal.yaml#0
       integrator: integrator listens to cafe-order.order.on_paid but has no
       symmetric cancel handler for cafe-order.order (on_cancel/before_cancel)
       — cancel on the source would be permanently blocked (7.7.2)
```

Setelah menambahkan manifest pasangan yang mendengarkan `on_cancel` →
`gl.journal-entry.cancel`, validasi hijau (69 manifest, 0 problem).

### Dampak: **positif** (dengan catatan dokumentasi)

Aturan ini **sangat baik** dan layak dipuji:

> Ia memaksa penulis spec memikirkan **jalur pembalikan sejak awal**. Tanpa itu,
> integrasi akuntansi yang sudah berjalan ribuan transaksi tidak akan bisa
> dibatalkan lagi di sisi sumber — dan itu baru ketahuan saat pembatalan
> pertama diminta.

Ini juga contoh **kegagalan yang benar**: validator menolak, **dan menjelaskan
alasannya** ("cancel on the source would be permanently blocked") plus kode
aturan (7.7.2). Ini standar pesan error yang saya usulkan di Gap #29/#37.

Yang menjadi masalah hanya **penemuan**-nya: aturan ini tidak ada di deskripsi
`IntegratorSpec`, tidak di atribut `kind: Integrator`, tidak di
`ai_skills/formspec-kinds/SKILL.md`, dan tidak di `02-core-extended.md §5`
(yang mendefinisikan Integrator). Satu-satunya cara tahu adalah **mencoba dan
gagal**.

### Usulan

1. Dokumentasikan aturan 7.7.2 di halaman `kind: Integrator` — sertakan contoh
   pasangan manifest maju + balik, karena itu bentuk yang diwajibkan.
2. Sebutkan nama event yang dikenali validator (`on_cancel` / `before_cancel`)
   di panduan penamaan event (Gap #40) — saat ini daftar resminya hanya
   `before_*`/`on_*` generik.

---

## Status Gap #15 setelah pekerjaan ini

**#15 tetap BLOCKER**, dan sekarang terbukti dari sisi spec: manifest
`cafe-gl-integrator` **valid** dan menyatakan integrasi yang benar
(`consumes: gl:journal-entries` + dua Integrator), tetapi menurut catatan gap
resmi FormSpec:

> - Cross-app grant enforcement **unimplemented** — "zero runtime implementation"
> - SyncAgent registry sync **not wired** ke HTTP router — "a real multi-App
>   workspace can accept manifests per App but **can't yet serve them together
>   end-to-end**"

Jadi: **spec-nya siap, jalanannya belum tersambung.** Ini persis nilai dari test
case ini — spec ideal yang menuntut engine menyusul.

**Cadangan bila harus jalan hari ini** (dicatat juga di manifest App): tanam
`kind: Subscription` di dalam module `cafe-order`, satu App saja. Lebih buruk —
logika akuntansi menempel pada pemiliknya, tidak bisa diganti vendor, dan tidak
ada batas konsen — tapi berjalan.

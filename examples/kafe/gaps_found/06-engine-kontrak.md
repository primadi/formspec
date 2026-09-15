# Gap #10, #11, #12, #16, #17 — Engine & Kontrak

Gap pada lapisan engine/kontrak yang menghambat aplikasi kafe, tapi tidak masuk
kategori widget/POS/vertical.

---

## Gap #10 — Print `thermal` & `dotmatrix` belum diimplementasi (selalu PDF) ✅ Pasti

### Bukti

Kontrak menjanjikan 4 format — `docs/kind/ui/Print.md` / `06-page-kinds.md` §8:

| Format | Pipeline | Kegunaan |
| --- | --- | --- |
| `pdf` | Generate PDF server-side | Invoice, surat jalan |
| **`thermal`** | **Server-side ESC/POS byte stream → printer mentah** | **Struk POS** |
| `dotmatrix` | Teks polos + escape code | Pick list gudang |
| `html` | `window.print()` client-side | Print browser |

`06-page-kinds.md` §8 bahkan menyatakan:

> **semua format kecuali `html` render server-side**, hasil ke download tray

Tapi implementasi server hanya punya **satu** fungsi render —
`internal/api/print.go`:

```go
// HandlePrint returns a GET /_ui/print/{module}/{name}/{id}?format=pdf
// handler that renders a kind: Print document server-side (todo 5.13.2).
//
// The frontend PrintRenderer handles `format: html` via window.print(); this
// endpoint covers `format: pdf` — the same declarative Print manifest
// (header/body/footer) rendered to a PDF with the go-pdf/fpdf library
func (b *RouterBuilder) HandlePrint() http.HandlerFunc { ... }

func renderPrintPDF(ps *spec.PrintSpec, record map[string]any) ([]byte, error) { ... }
```

Handler **selalu** mengembalikan PDF:

```go
w.Header().Set("Content-Type", "application/pdf")
w.Header().Set("Content-Disposition",
    fmt.Sprintf("attachment; filename=%q", name+".pdf"))
```

**Tidak ada percabangan pada `ps.Output.Format`.** Jadi manifest dengan
`output: { format: thermal, paper: { size: thermal_58mm } }` —
yang **lolos validasi** (`ui_test.go` memuat fixture persis seperti itu) —
akan menghasilkan **file PDF bernama `.pdf`**, bukan byte stream ESC/POS.

Dikuatkan oleh `docs/renderers/shadcn-shell/03-kind-renderers.md` §3:

> `Print` | Fungsional untuk `format: html` saja | ... `pdf`/`thermal`/`dotmatrix`
> **belum ada kode sama sekali** (butuh pipeline server, di luar cakupan renderer ini)

### Dampak ke aplikasi kafe: **BLOCKER** untuk struk

Kasir kafe butuh **struk thermal 58mm**. Hari ini:

1. `formspec validate` **hijau** untuk `format: thermal` → memberi kesan
   didukung.
2. Saat dijalankan, hasilnya PDF A4/A5 (atau `thermal_58mm` sebagai "paper size"
   PDF — `fpdf.New("P", "mm", paper, "")` menerima string apa pun).
3. Tidak ada byte ESC/POS, jadi **tidak bisa dikirim ke printer thermal**.

Ini kasus "spec ahead of engine" yang paling berbahaya untuk test case:
**validasi bilang OK, runtime diam-diam salah format.** Kafe tidak akan bisa
mencetak struk.

### Usulan

1. Implementasi `renderPrintThermal()` (ESC/POS) + `renderPrintDotmatrix()`.
2. **Sampai itu land: `formspec validate` harus MENOLAK `format: thermal`
   dan `dotmatrix`** (atau memberi warning tegas "belum diimplementasi").
   Ini prinsip "gagal keras, bukan gagal senyap" yang sama dengan Gap #1.
3. Tambah tes yang memverifikasi `Content-Type` sesuai `output.format`.

---

## Gap #11 — Relasi lintas `persist.category` diblokir senyap ✅ Pasti

### Bukti

`renderers/jsonb-persist/crud.go` → `resolveRelations()`:

```go
// Cross-category JOIN block (4.4.2): a relation may not resolve across
// persist categories (FORMSPEC.PERSIST.CROSS_CATEGORY). Skipped when no
// category resolver is wired.
if s.targetCategoryResolver != nil {
    targetCat := s.targetCategoryResolver(targetModule, targetEntity)
    if targetCat != "" && s.category != "" && targetCat != s.category {
        log.Printf("[WARN] resolve relation %s: cross-category JOIN blocked (%s vs %s) — FORMSPEC.PERSIST.CROSS_CATEGORY",
            f.Name, s.category, targetCat)
        continue
    }
}
```

Changelog `2026-08-17-031-cross-category-join-block.md` mengonfirmasi:

> Catatan: blokir saat ini **log warning + skip resolusi (bukan hard error)**
> agar tidak memutus list yang sudah jalan.

Test-nya (`cross_category_test.go`) memvalidasi bahwa alias relasi **tidak
dipopulasi**.

Kategori yang tersedia: `operational | financial | compliance | analytics |
master | archive` (`spec.PersistSpec.Category`).

### Dampak ke aplikasi kafe: **HIGH**

- Anda tidak akan mendapat error apa pun. Yang terjadi: **label relasi hilang**.
- Kasus nyata: kalau `payment`/`journal-entry` diberi
  `persist.category: financial` sementara `order` tetap default
  (`operational`), maka saat menampilkan `payment` yang menunjuk ke `order`,
  **objek `order` tidak terisi** → kolom "Nomor Pesanan" kosong, `RelationPicker`
  tidak bisa menampilkan label.
- Untuk kafe yang ingin memisahkan data operasional dari pembukuan — hal yang
  **wajar dan direkomendasikan** — ini menjadi jebakan senyap.
- Default `category` kosong (`""`), dan blokir hanya aktif kalau **kedua** sisi
  punya kategori dan berbeda. Jadi risiko muncul justru setelah seseorang
  "merapikan" kategori.

### Usulan

1. Naikkan dari `log.Printf` + `continue` menjadi error terlihat
   (`FORMSPEC.PERSIST.CROSS_CATEGORY` sudah ada di `pkg/spec/errors.go`) —
   minimal saat `formspec validate`, ideal saat request.
2. `formspec validate` sebaiknya **mendeteksi** relasi lintas kategori saat
   load spec dan melaporkannya sebagai problem, bukan menemukannya saat runtime.
3. Dokumentasikan eksplisit: "kategori memisahkan schema; relasi lintas
   kategori tidak di-resolve".

---

## Gap #12 — Resolusi tabel target relasi naif (`+s`) ✅ Pasti

### Bukti

`docs/renderers/jsonb-persist/03-migration-engine.md` §2:

> **Gap implementasi:** resolusi nama tabel target (`ValidateRelationTargets`)
> memakai **module milik entity itu sendiri + pluralisasi naif (tambah `s`)** —
> belum benar-benar resolve module/plural asli entity target. Relasi lintas
> module atau entity ber-plural tidak beraturan bisa **diam-diam lolos** dari
> guard referenceability (§1.2 core-basic) **alih-alih ditolak**.

### Dampak ke aplikasi kafe: **HIGH**

Kafe punya banyak relasi lintas module:

| Relasi | Dari → ke |
| --- | --- |
| `order.menu_item_id` | `cafe-order` → `cafe-master` |
| `order.branch_id` | `cafe-order` → `cafe-master` (cabang) |
| `stock-movement.product_id` | `inventory` → `cafe-master` |
| `journal-entry.order_id` | `gl` → `billing`/`cafe-order` |

Karena module target `cafe-master` bukan module entity sumber `cafe-order`,
resolusi naif akan memakai module sumber → nama tabel salah → guard
**referenceability** bisa lolos (relasi menunjuk ke record yang tidak ada tanpa
ditolak). Untuk aplikasi yang menuntut integritas data keuangan, ini serius.

### Usulan

- Gunakan resolver yang sudah ada: `SetTargetTableResolver`
  (`internal/entity/registry.go` sudah me-wire-nya, dan komentarnya menyebut
  "When nil, the naive `{module}_{plural}` convention is used") — pastikan
  jalur `ValidateRelationTargets` memakainya, bukan konvensi naif.
- Tambah test untuk entity dengan plural tidak beraturan (mis. `menu-item` →
  plural kustom).

---

## Gap #16 — `Dashboard widget.ref` dicocokkan dengan nama polos ✅ Pasti

### Bukti

`docs/architecture/07-vertical-modules.md` §8:

> Dashboard `widget.ref` is matched by **bare `metadata.name`, not
> module-qualified**. `internal/ui/registry.go`'s `Widgets` map is keyed by name
> only. Confirmed while building `reference-app/spec/dashboards/erp-overview.yaml`
> — a cross-module dashboard must use each widget's bare name
> (`today-revenue`, not `billing.today-revenue`); harmless here since names are
> unique across the composed set, but **two modules reusing the same widget name
> would silently collide**.

Dan gotcha yang sama berulang di skill `formspec-kinds`:

> Dashboard widget `ref` uses just the widget name — NOT `module.name` format.

### Dampak ke aplikasi kafe: **MEDIUM**

Dashboard pemilik kafe akan menggabungkan widget dari beberapa module:
`cafe-order` (omzet hari ini), `cafe-master` (menu terlaris),
`inventory` (stok kritis), `gl` (laba). Dua module **tidak boleh** memakai nama
widget yang sama, mis. keduanya punya `today-sales` atau `low-stock` — kalau
tidak, dashboard akan diam-diam menampilkan widget yang salah.

Ini masalah **konvensi penamaan lintas-module**, dan tidak ada validasi yang
menangkapnya. Untuk vertical pihak ketiga (mis. `gl` versi vendor), tabrakan
nama hampir pasti terjadi.

### Usulan

- Kunci registry widget dengan `{module}.{name}` (atau dukung **keduanya**:
  bare name untuk kompatibilitas + module-qualified sebagai bentuk yang
  disarankan).
- `formspec validate` melaporkan tabrakan nama widget lintas-module sebagai
  problem.

---

## Gap #17 — Realtime hanya di Table/Kanban/Dashboard ✅ Pasti

### Bukti

`docs/renderers/realtime.md` §5 — "Sudah berjalan (renderer memakai
`useRealtime`)":

| Kind | Flag | Perilaku |
| --- | --- | --- |
| **Table** | `realtime: true` | Silent refetch baris pada event entity |
| **Kanban** | `realtime: true` (default) | Kartu muncul/pindah/berubah status dari klien lain |
| **Dashboard** | `realtime: true` | Widget metric & chart refetch |

Dan §7 "Gap & Pekerjaan ke Depan":

> **Timeline realtime belum di-wire** (renderer sudah ada).
> Heartbeat ping/pong belum ada — deteksi putus bergantung browser/OS.

> **Catatan:** dokumen yang sama juga menyebut Calendar/ApprovalInbox/
> NotificationCenter "renderer belum ada" — padahal
> `renderers/react-shadcn/src/shell/router.tsx` sudah lazy-import
> `CalendarRenderer`, `ApprovalInboxRenderer`, dan
> `NotificationCenterRenderer`. Jadi bagian itu **kedaluwarsa** (lihat
> `07-dokumen-dan-skill.md`).

### Dampak ke aplikasi kafe: **MEDIUM**

- ✅ **KDS (layar dapur) bisa jalan.** `kind: Kanban` + `realtime: true`
  sudah: kartu pindah kolom saat status berubah dari kasir. Ini kebutuhan
  terbesar kafe dan **sudah terpenuhi**.
- ⚠️ **Layar riwayat/aktivitas** (`kind: Timeline`) tidak akan auto-refresh.
  Untuk feed pesanan berjalan, tidak bisa diandalkan tanpa reload manual.
- ⚠️ **`kind: Listing` publik tidak punya realtime** — kalau status pesanan
  pelanggan berubah ("sedang dibuat" → "siap"), halaman QR pelanggan tidak akan
  memperbaruinya. Untuk layar status pesanan pelanggan ini cukup penting.
- ⚠️ `NotificationCenter` punya `realtime` di spec
  (`NotificationCenterSpec.Realtime`) — perlu verifikasi apakah sudah dipakai
  renderer-nya (dokumentasi bertentangan).
- ⚠️ **Tidak ada heartbeat** — di WiFi kafe yang tidak stabil, deteksi putus
  bergantung OS/browser. Untuk KDS yang ditinggal berjam-jam, ini berisiko
  (tampilan basi tanpa peringatan).

### Usulan

1. Wire `useRealtime` di `TimelineRenderer` (pola sudah jelas — tinggal salin
   dari Kanban/Table).
2. Tambah `realtime` pada `ListingSpec` (public order status).
3. Tambah ping interval eksplisit agar putus terdeteksi cepat + indikator
   "connection lost" di KDS.

---

## Gap #29 — Kontrak `Report` berbeda dari `Table`, dan sebagian tipe dibiarkan bebas ⚠️ Terverifikasi

Ditemukan saat menulis 6 report untuk module `cafe-report`: keenamnya gagal
validasi. **Kesalahan awalnya milik saya** — saya memakai bentuk kolom `Table`
untuk `Report`. Tapi penyebabnya adalah kontrak yang tidak konsisten, dan itu
layak dicatat.

### Bukti 1 — dua "kolom" dengan kemampuan berbeda

Konsepnya sama (kolom tabel output), tetapi himpunan properti yang diizinkan
berbeda — dan keduanya `additionalProperties: false`:

| | `TableColumn` | `ReportColumn` |
| --- | --- | --- |
| `field` | ✅ | ✅ (wajib) |
| `label` | ✅ | ✅ (wajib) |
| `format` | ✅ | ✅ |
| `widget` | ✅ | ❌ **ditolak** |
| `sortable` / `width` / `align` / `link` | ✅ | ❌ |
| `aggregate` | ❌ | ✅ |

Jadi menulis `{ field: status, label: "Status", widget: badge }` di report →
`schema: /spec/columns/4: validation failed`. Tidak ada pesan yang menjelaskan
"`widget` hanya ada di Table", dan tidak ada dokumen yang menaruh kedua bentuk
berdampingan.

### Bukti 2 — `totals` memakai kunci yang berbeda lagi

`ReportTotal` (`formspec.schema.json` → `$defs`):

```json
"ReportTotal": {
  "properties": {
    "field": { "type": "string" },
    "fn":    { "description": "sum | avg | count | min | max", "type": "string" },
    "label": { "type": "string" }
  },
  "required": ["label", "field", "fn"],
  "additionalProperties": false
}
```

Bentuk `{ field: total, format: currency }` — yang intuitif dan muncul di
contoh dokumen lama — **ditolak**: `missing properties 'label', 'fn'` +
`additional properties 'format' not allowed`. Bentuk benar:
`{ label: "Total", field: total_amount, fn: sum }`.

### Bukti 3 — sebagian properti adalah string bebas (tidak ada enum)

| Properti | Deklarasi schema | Akibat |
| --- | --- | --- |
| `ReportParam.type` | `{ "type": "string" }` — tanpa enum | `relation`, `date`, `select` semuanya lolos; **salah ketik seperti `reltion` juga lolos** |
| `ReportColumn.aggregate` | `{ "type": "string" }` | `sum`/`avg`/`count` tidak divalidasi; `sumn` lolos |
| `ReportColumn.format` | `{ "type": "string" }` | `currency`/`date` tidak divalidasi |

Ini kelas yang sama dengan Gap #21: **kesalahan penulisan yang seharusnya
tertangkap, lolos**. Bedanya, di sini ada di dalam kind yang sudah punya
schema ketat untuk properti lain — jadi terasa tidak konsisten.

### Dampak ke aplikasi kafe: **MEDIUM**

- Biaya authoring: 6 report gagal validasi sekaligus, dan pesan errornya tidak
  menyebut solusinya. Agent/developer harus membaca `$defs` untuk tahu bentuk
  yang benar.
- Risiko runtime: `ReportParam.type: reltion` lolos validasi lalu diam-diam
  jadi input teks biasa, bukan picker relasi.

### ✅ Sisi baiknya — pesan errornya justru bagus di sini

Berbeda dari kasus `target:` (Gap #18) yang hanya bilang
`/spec/fields/0: validation failed`, error report menyebut **properti yang
hilang dan yang tidak diizinkan**:

```
/spec/totals/0: missing properties 'label', 'fn'
/spec/totals/0: additional properties 'format' not allowed
/spec/columns/4: validation failed
```

Itu cukup untuk memperbaiki tanpa menebak. **Jadikan ini standar**: sebutkan
nama properti di semua error, bukan hanya "validation failed".

### Usulan

1. **Satukan konsep kolom.** Idealnya `ReportColumn` menerima subset
   `TableColumn` (setidaknya `widget`, `align`, `width`) — perbedaan
   kemampuan antar-kind dengan nama sama adalah jebakan.
2. **Berikan enum** pada `ReportParam.type`, `ReportColumn.aggregate`, dan
   `format` — tipe/format yang didukung adalah himpunan tertutup, dan
   mendokumentasikannya di schema membuat editor menyarankan nilainya.
3. Taruh `ReportColumn`/`ReportTotal` berdampingan dengan `TableColumn` di satu
   halaman referensi kind.

---

## GAP #38 — Workflow tidak bisa mengawal transisi dengan banyak state asal ✅ Terverifikasi

### Bukti

Dua tipe di schema yang sama, dengan kemampuan berbeda:

| Tipe | Field `from` | Bukti |
| --- | --- | --- |
| `TransitionDecl` (state machine entity) | **daftar boleh** — `StateList` menerima skalar **dan** array | `UnmarshalYAML` di `pkg/spec/entity.go`: *"YAML accepts either a scalar (\"draft\") or a sequence ([draft, awaiting_payment])"* |
| `WorkflowTransitionRef` (pemicu workflow) | **string tunggal** — `"from": { "type": "string" }` | `$defs.WorkflowTransitionRef`: *"identifies the intercepted transition by its from/to states"* |

Aplikasi kafe ini memakai state asal ganda pada transisi void:

```yaml
# entity `order`
- { from: [paid, in_kitchen, ready, served], to: cancelled, via: void-order }
```

Workflow approval hanya bisa menempel pada **satu** state asal.

### Dampak ke aplikasi kafe: **HIGH** — ini lubang penegakan, bukan sekadar ketidaknyamanan

Kalau workflow ditulis `from: paid`, maka void dari `in_kitchen`, `ready`, atau
`served` **tidak melewati approval sama sekali**. Dan justru ketiga state itulah
kasus yang paling sering terjadi — makanan sudah dibuat, lalu harus dibatalkan.

Artinya: **persetujuan supervisor bisa dilewati hanya dengan membatalkan dari
state yang berbeda, dan tidak ada error, log, atau peringatan apa pun.** Aturan
bisnis #8 (D5) bocor tanpa gejala.

Catatan tambahan: `formspec validate` **hijau** untuk manifest yang menulis
`from: paid` — validator tidak bisa tahu bahwa transisi aslinya punya empat
state asal. Jadi tidak ada gerbang otomatis yang menangkap kebocoran ini.

### Usulan

1. `WorkflowTransitionRef.from` harus menerima daftar — samakan dengan
   `TransitionDecl.from` (`StateList`). Ini yang paling langsung.
2. **Alternatif yang lebih kuat:** pemicu workflow merujuk **transisi**, bukan
   pasangan state — mis. `on: { transition: cafe-order.order.void-order }`.
   Identitas transisi sudah ada dan unik (namanya `via`). Merujuk state berarti
   menduplikasi identitas yang sudah dimiliki transisi, dan mudah basi bila
   state machine diubah.
3. **Peringatan validasi** (perbaikan sementara yang murah): bila ada
   `TransitionDecl` dengan lebih dari satu state asal DAN ada `kind: Workflow`
   yang menunjuk ke salah satu state asalnya, `formspec validate` harus
   memperingatkan "transisi ini punya N state asal; workflow hanya mengawal 1".
   Ini menutup kebocoran senyap tanpa menunggu perubahan tipe.

---

## GAP #39 — `WorkflowStep` tidak punya label ✅ Terverifikasi

### Bukti

`$defs.WorkflowStep`:

```json
"WorkflowStep": {
  "properties": {
    "approvers":  { "description": "quorum, default 1", "type": "integer" },
    "escalation": { "$ref": "#/$defs/StepEscalation" },
    "mode":       { "description": "all | any | sequential", "type": "string" },
    "roles":      { "type": "array", "items": { "type": "string" } },
    "when":       { "type": "string" }
  }
}
```

Tidak ada `title`, tidak ada `description`. Ditulis `title:` →
`schema: /spec/steps/0: validation failed`.

### Dampak ke aplikasi kafe: **MEDIUM**

ApprovalInbox bersifat zero-config: sumbernya langkah workflow yang menunggu
untuk pemanggil. Karena langkah tidak punya label, **tidak ada apa pun yang bisa
ditampilkan sebagai deskripsi tugas.**

Untuk kafe, ini persis kasus yang butuh konteks: supervisor diminta menyetujui
pembatalan pesanan yang sudah dibayar, tetapi tidak ada tempat untuk menulis
"Persetujuan Void Pesanan" — dan juga tidak ada tempat mendeklarasikan field
pendukung yang seharusnya ikut tampil (nomor pesanan, total, alasan void) agar ia
bisa mengambil keputusan.

Perhatikan bedanya dengan `WizardStep` yang **punya** `title` + `description`.
Jadi ini inkonsistensi antar-kind, bukan keterbatasan yang disengaja.

### Usulan

1. Tambah `title` + `description` ke `WorkflowStep` (samakan dengan `WizardStep`).
2. Tambah cara mendeklarasikan **field pendukung** yang ditampilkan pada tugas
   approval — mis. `display_fields: [number, total_amount, void_reason]`.
   Tanpa ini, approver harus membuka record secara manual untuk memahami konteks.
3. Kalau langkah punya `when`, tampilkan evaluasinya juga di inbox — supaya
   approver tahu kondisi apa yang sedang dinilai.

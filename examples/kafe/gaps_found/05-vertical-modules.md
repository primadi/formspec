# Gap #13, #14, #15 — Vertical Modules & Integrasi Akuntansi

Permintaan pemilik proyek:

> catat juga, untuk sistem akuntansi penuh seharusnya ada modul vertical yg
> tersedia dan bisa diintegrasikan.

**Kabar baiknya: vertical akuntansi sudah ada.** Kabar buruknya: jaringan
integrasinya belum bisa dijalankan end-to-end.

---

## Yang sudah ada — jangan dibangun ulang

`verticals/README.md` — 9 App mandiri:

| App | Publishes | Consumes | Isi |
| --- | --- | --- | --- |
| `company` | `branch-directory` | — | Struktur organisasi, directory cabang |
| `billing` | `order-events` | — | Customer, order, checkout, payment gateway |
| **`gl`** | `journal-entries` | — | **Double-entry accounting core** |
| `inventory` | `stock-movements` | `company` | Multi-warehouse stock tracking |
| `notifications` | — | `billing` | Reaksi notifikasi WhatsApp |
| `sales-inventory-integrator` | — | `billing`, `inventory` | `billing.order.paid` → stock movement |
| `sales-gl-integrator` | — | `billing`, `gl` | `billing.order.paid` → sales journal entry |
| `registry` | — | — | Registry module (dogfooding) |
| `reference-app` | — | semua | Komposisi dev-mode |

**`gl` (General Ledger) sudah lengkap sebagai vertical:** `account` [reference]
= chart of accounts, `journal-entry` [transaction] = double-entry journal,
`gl-balance` [summary] = saldo per akun, plus script `journal_post.star`,
`journal_reverse.star`, `gl_balance_update.star`.

Jadi **sistem akuntansi penuh sebagai modul vertical memang sudah tersedia dan
dirancang untuk diintegrasikan.** Yang perlu diperiksa adalah apakah
integrasinya benar-benar bisa dijalankan — dan di situ ada gap.

---

## Gap #15 — Cross-app grant & SyncAgent belum jalan ✅ Pasti

Ini gap **paling penting** untuk "modul vertical yang bisa diintegrasikan".

### Bukti

`docs/architecture/07-vertical-modules.md` §8 — daftar gap resmi:

| Gap | Bukti | Status |
| --- | --- | --- |
| **Cross-app grant enforcement unimplemented** | "No App-scoped grant-checking anywhere in `internal/permission`/`internal/action`" | "Spec'd (D25, §15.3), **zero runtime implementation** — not attempted here" |
| **SyncAgent registry sync not wired to the live HTTP router** | `docs/runtimes/02-formspec-resource.md` | "A real multi-App workspace can accept manifests via `formspec apply` per App but **can't yet serve them together end-to-end**" |
| `App.consumes/publishes` only demonstrated for `kind: Service` | Spec §3 contohnya hanya `service: icd-lookup` | Reorg ini "approximates it for plain entity events ... with an **invented** `service:` name" |

Dan solusi pengganti yang dipakai FormSpec sendiri jujur disebut sebagai
shortcut:

> `verticals/reference-app/` ... explicitly the same shortcut
> `spec/platform/02-workspace-app-module.md` §6 calls **"non-conformant"** for
> direct filesystem loading — tolerated here because the conformant path
> (per-App `formspec apply` into one workspace) can't yet serve a converged
> result end-to-end.

### Dampak ke aplikasi kafe: **BLOCKER** untuk integrasi akuntansi

Skenario yang diinginkan pemilik: **kafe + akuntansi + inventory** dalam satu
workspace, dengan pesanan yang sudah lunas otomatis menjadi jurnal.

Yang terjadi:

1. **Komposisi multi-App belum bisa disajikan bersama.** Setiap vertical bisa
   di-boot mandiri (`go run ./examples/reference-app --spec
   ./verticals/inventory/spec` jalan), tapi menggabungkan beberapa **App** ke
   satu workspace dan melayani hasil gabungan itu **belum** jalan (§9 above).
   Jalan pintas `compose.sh` menyalin `spec/modules/` ke satu pohon — itu
   **dev-only** dan secara eksplisit disebut non-conformant.
2. **Grant lintas-App tidak ditegakkan.** Spec-nya ada, runtime-nya nol. Jadi
   "vertical ini boleh membaca interface vertical itu" **belum bergigi**.
3. **`publishes`/`consumes` belum punya sintaks resmi untuk event entity.**
   Integrasinya bergantung pada `billing.order.paid` (event entity biasa),
   sementara spec hanya membesarkan `kind: Service`. Nama service-nya
   "invented".

### Catatan tambahan: dua mekanisme integrasi hidup bersamaan

> `billing.order`'s `paid` event has **two integration mechanisms live
> simultaneously** — an inline `deliver: reliable_event` targeting
> `gl.journal-entry` directly ..., and a separate `sales-gl-integrator`
> Subscription achieving roughly the same outcome via a queued job. ...
> **Which pattern should be preferred for future integrations is an open
> question this document doesn't resolve**.

Untuk kafe, ini berarti: **belum ada satu jawaban kanonik** tentang cara
menyambungkan penjualan ke jurnal. Dua-duanya jalan, dua-duanya berbeda sifat
(inline andal vs. integrator app yang bisa diganti vendor).

### Usulan

1. Wire SyncAgent ke router → komposisi multi-App jadi jalur normal.
2. Tegakkan cross-app grant (spec sudah ada; tinggal implementasi).
3. Tentukan satu pola integrasi kanonik (rekomendasi: `Subscription` di
   integrator App terpisah — lebih swappable dan sesuai arah reorg).
4. Tambah sintaks resmi `publishes`/`consumes` untuk event entity, bukan hanya
   `kind: Service`.

---

## Gap #13 — Inventory belum bisa menghitung HPP / margin ✅ Pasti

Ini gap yang **langsung memblokir laporan "menu terlaris & profitabilitas"**
yang diminta pemilik kafe.

### Bukti

`docs/architecture/07-vertical-modules.md` §4 (ERPNext comparison):

| Area | FormSpec |
| --- | --- |
| Stock | "ERPNext: warehouses, **valuation (FIFO/LIFO/moving-avg/standard)**. FormSpec: **simpler today, no valuation method yet**." |

§10 (Roadmap, explicitly deferred):

> The Inventory features that originally motivated this reorg:
> **stock opname (physical count)**, a **`movement-type` master-data entity**
> for transaction categorization, **adjustment handling**, and fixing
> `stock-movement`'s `transfer` type (today `movement_apply.star`/
> `stock_level_update.star` only branch on `in`/`out` — a `transfer` movement
> is **accepted by the state machine but never actually moves stock** between
> warehouses).

### Dampak ke aplikasi kafe: **BLOCKER** untuk margin

| Kebutuhan kafe | Status |
| --- | --- |
| Resep/BOM (menu → bahan baku) | ✅ Bisa dimodelkan: entity `recipe` + `child` item, script potong stok |
| Stok berkurang otomatis saat terjual | ✅ `Subscription`/`Integrator` + script, dengan `ctx.lock` anti-race |
| Stok opname (hitung fisik) | ❌ Belum ada pola; harus dibangun sendiri |
| Klasifikasi pergerakan stok | ❌ Tidak ada master `movement-type` |
| **HPP per menu (biaya bahan)** | ❌ **Tidak ada metode valuasi** — tidak ada FIFO/moving-average, jadi biaya bahan tidak bisa dihitung benar |
| **Margin per menu** | ❌ Turunan dari HPP → tidak bisa |
| Transfer stok antar outlet | ❌ `transfer` diterima state machine tapi **tidak memindahkan stok** |

**Untuk kafe multi-outlet, ini berarti:** laporan "menu terlaris" bisa dibuat
(jumlah terjual & omzet), tapi **"profitabilitas" tidak** — karena tidak ada
cara sah menghitung biaya bahan yang terjual.

Tambahan untuk kafe: **penjualan stok tidak sama dengan konsumsi bahan.**
Yang berkurang saat menjual Latte adalah susu + kopi + cup, bukan "1 Latte".
Itu ada di jalur resep, dan **snapshot harga bahan** harus dibekukan agar
margin historis tidak berubah retroaktif.

> **Catatan:** FormSpec sudah punya mekanisme yang tepat untuk itu —
> `snapshot:` pada relation `belongs_to` (`docs/spec/backend/02-core-extended.md`
> §1.1: field master yang memengaruhi perhitungan finansial **wajib** disalin ke
> transaksi, tidak boleh live-join). Jadi aturannya sudah ada; yang belum ada
> adalah dukungan valuasi untuk menghitung biayanya.

### Usulan

| Tingkat | Usulan |
| --- | --- |
| Kecil | Pola `stock-opname` + `movement-type` sebagai contoh terdokumentasi di vertical `inventory` (tidak butuh engine baru) |
| Sedang | Perbaiki `transfer` agar benar-benar memindahkan stok antar warehouse/outlet |
| Besar | Metode valuasi (FIFO / moving average) + `hpp` pada `stock-movement`, dan `summary` biaya per menu — prasyarat laporan margin |

---

## Gap #14 — Vertical `purchase` belum ada ✅ Pasti

### Bukti

§7 (ERPNext comparison):

| Area | FormSpec |
| --- | --- |
| Buying | **"(not built — deferred, see §7)"** — ERPNext: purchase order → purchase receipt → landed cost |

§10 (Roadmap):

> A real `purchase` vertical (purchase order → goods receipt).
> `purchase-inventory-integrator`, `inventory-gl-integrator`.

### Dampak ke aplikasi kafe: **HIGH**

Pemilik memilih **"Supplier & pembelian bahan"** sebagai salah satu kebutuhan.
Hari ini:

- Entity supplier & pembelian **bisa dibuat sendiri** di module kafe
  (`supplier` [master], `purchase-order` [transaction], `goods-receipt`).
- Tapi **tidak ada vertical yang menyediakannya**, jadi:
  - tidak ada integrasi otomatis purchase → stock movement
    (`purchase-inventory-integrator` belum ada),
  - tidak ada integrasi purchase → jurnal/hutang
    (`purchase-gl-integrator` belum ada),
  - tidak ada *landed cost* (biaya kirim masuk ke HPP bahan).

Untuk kafe, landed cost berpengaruh langsung ke HPP — jadi gap ini
berhubungan dengan Gap #13.

---

## Kesimpulan Bagian Ini

| Pertanyaan pemilik | Jawaban |
| --- | --- |
| Apakah modul akuntansi penuh tersedia sebagai vertical? | **Ya** — `verticals/gl`: chart of accounts, double-entry journal, saldo (summary), script post/reverse |
| Apakah modul inventory tersedia? | **Ya** — `verticals/inventory`: product, warehouse, stock-movement, stock-level |
| Apakah modul purchase/supplier tersedia? | **Belum** — deferred di roadmap |
| Apakah "bisa diintegrasikan" sudah benar-benar jalan? | **Belum sepenuhnya** — komposisi multi-App belum disajikan end-to-end, cross-app grant belum ditegakkan (Gap #15) |
| Apakah kafe bisa dapat laporan margin per menu? | **Belum** — tidak ada metode valuasi/HPP (Gap #13) |

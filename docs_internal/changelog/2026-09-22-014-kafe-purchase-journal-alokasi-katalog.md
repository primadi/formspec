# 2026-09-22-014 — Kafe 10.6/10.7/10.8: jurnal pembelian, alokasi by-weight, katalog publik

**Tanggal:** 2026-09-22 · **Trigger:** "lanjut" (melanjutkan item kafe yang masih
terbuka) · **Item:** kafe `10.6 ⏸️`, `10.7 ⏸️`, `10.8 ⏸️` → ketiganya **✅**

## 10.7 — Pembelian → jurnal (event, bukan panggilan)

`cafe-stock/purchase-order` **memancarkan** `on_po_received`/`on_po_cancelled`;
`gl/subscriptions/purchase-to-journal.yaml` mendengarkan dan
`gl/scripts/journalize_purchase.star` membangun jurnalnya:

```
Persediaan Bahan (1-2000)   debit   500.000
Utang Dagang     (2-3000)   kredit  500.000
```

Mengapa persediaan, bukan beban: bahan yang dibeli belum menjadi beban — ia
menjadi aset sampai dipakai. Mengapa utang, bukan kas: PO menyatakan barang
**diterima**, bukan dibayar.

Baris `line_type: cost` **dilewati** saat menghitung nilai barang: biaya kirim
sudah masuk `unit_cost` pergerakan stok lewat alokasi landed cost, jadi
menjumlahkannya lagi di sini akan menghitung dua kali.

**Terukur** (PO 200 × 2.500): `JRN-2026-000062`, `status=posted`, `source_id`
terisi, debit = kredit = **500.000**, akun tepat.

### Dua kegagalan SENYAP yang ikut ketemu

1. **`emit:` pada transisi tanpa `emits:` pada action.** State machine terbaca
   benar, `formspec validate` hijau, dan **tidak ada event yang pernah dikirim**
   — subscription menunggu sesuatu yang tidak pernah datang, penerimaan barang
   tidak menghasilkan akuntansi apa pun, tanpa satu pun error atau log.
2. **Script `.star` memanggil fungsi dari file `.star` lain.** Starlark
   mengompilasi tiap file sebagai unit tersendiri; ini gagal di **runtime**
   (`undefined: journalize_sale`, outbox retry 5×) sementara `formspec validate`
   tetap hijau — ia mengompilasi tiap file terpisah, jadi tidak ada cara melihat
   rujukan lintas-file.

Keduanya kini ber-test pengunci, dan keduanya dicatat sebagai item bernomor
(**7.8.10 ✅**, **7.8.11 ⏸️** validator, **7.8.12 ✅**, **7.8.13 ⏸️** berbagi
fungsi antar-script).

## 10.6 — Alokasi landed cost bisa dipilih

`ingredient.weight_per_unit` (gram per satuan dasar) + `allocation_strategy`
pada Config module `cafe-stock` (`weight` | `value` | `quantity`). Default
`weight` bila berat lengkap, selain itu `value`.

Terukur pada PO yang **sama** (beras 10.000 unit/150.000 ; kopi 2.000
unit/240.000 ; biaya kirim 300.000):

| strategi | beras | kopi |
| --- | --- | --- |
| `weight` (89,3% vs 10,7% berat) | **42** | **136** |
| `value` (38,5% vs 61,5% nilai) | **27** | **212** |

**Menolak, bukan menebak:** `weight` diminta eksplisit tetapi ada bahan tanpa
berat → gagal `FORMSPEC.PURCHASE.WEIGHT_MISSING` yang menyebut bahan mana,
bukan diam-diam beralih ke `value`. Beralih diam-diam menghasilkan angka yang
salah tanpa satu pun tanda — kelas kegagalan yang paling mahal, karena tidak ada
yang tahu harus memeriksa apa.

**Bug ikut ketemu:** `unit_cost` hasil alokasi tersimpan dengan 15 desimal
(`41.785714285714285`). Itu bukan nilai uang yang sah — IDR tidak punya pecahan
(skala 0) — dan laporan keuangan akan menampilkan angka yang tidak pernah bisa
dibayar siapa pun. `money_like` kini membulatkan half-up ke skala mata uang (42,
bukan 41.79).

## 10.8 — Katalog publik diverifikasi; dua bug nyata

Verifikasi dijalankan pada jalur yang **benar-benar dipakai browser** (SPA route
+ endpoint yang sama, **anonim**), dan menemukan:

1. **Foto menu 403 untuk tamu.** `menu-item.photo` tidak menyatakan
   `visibility`; default-nya `private`, yang menuntut permission `view` — yang
   tidak dimiliki tamu. Katalog publik bisa membaca daftar menu (200) tetapi
   **setiap fotonya 403**: "menu bergambar" tidak pernah bergambar di permukaan
   pelanggan (kelas gap #4). Foto menu memang data publik — ia tampil di papan
   menu dan di QR meja tanpa login. Fix: `visibility: public`.
2. **Storage hilang setelah hot-reload.** `ReloadSpec` membangun
   `api.NewRouterBuilder` **baru**, sementara resolver object store dan link
   store hidup di App (tidak diturunkan dari spec, jadi reload tidak punya apa
   pun untuk di-resolve ulang) — builder baru mulai kosong. Akibatnya **setiap**
   upload/download (entity mana pun) membalas
   `STORAGE_UNAVAILABLE: storage not configured` **setelah reload pertama**.
   Terukur di dev server: foto `200` → ubah file spec (memicu watcher) → foto
   **`500`**. Gejala dan penyebabnya tidak berhubungan — yang diedit adalah spec,
   yang rusak adalah storage — dan hanya muncul pada alur yang justru jadi alasan
   dev server ada. Fix: App menyimpan `storageFn` + `linkStore` dan reload
   me-wire ulang keduanya; dicatat sebagai **2.10.7 ✅** dengan sisa audit untuk
   pola yang sama di registri lain.

Plus widget `recent-journals` yang dirujuk `gl-dashboard` tanpa manifest (ditutup
lebih awal hari ini).

**Bukti akhir:** `GET /app/menu/x` → **200 text/html** · `GET menu-item` anonim →
**200, 7 menu** dengan `photo` terisi · `GET menu-category` anonim → **200, 4
kategori** · `GET menu-item/{id}/photo` anonim → **200 `image/jpeg` 83.189 byte**
(magic `FF D8 FF E2`) — **juga sesudah hot-reload**.

## Bukti & gerbang

`go test ./...` hijau · `go vet ./...` bersih · `gofmt` bersih · kafe `validate`
→ **85 manifest, 0 problem** · `check` → **0 error/0 warning** · seed dari DB
kosong → **70 inserted**, rerun → 0 inserted/70 skipped.

**Test baru** (semua dibuktikan gagal saat fix-nya dinonaktifkan):
`TestKafe_PurchaseReceivedCreatesJournal` · `TestKafe_LandedCostAllocationStrategy`
(weight + value) · `TestKafe_WeightStrategyRefusesUnknownWeight` ·
`TestKafe_PublicMenuPhotoIsReadableAnonymously` · `TestReloadSpecKeepsFileStorage`.

## Sisa (bernomor)

Kafe **10.9 ⏸️** (kurva UI katalog — kebenaran data sudah terbukti, kualitas
tampilan belum dinilai dengan mata) · **7.8.11 ⏸️** (validator belum memeriksa
`emit:` ↔ `emits:`) · **7.8.13 ⏸️** (berbagi kode antar-script) · **2.10.7**
sisa audit registri lain · kafe **2.15 / 6.3 / 6.4 / 9.4** tetap deferred
beralasan.

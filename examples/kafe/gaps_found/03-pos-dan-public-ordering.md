# Gap #5, #6, #7 — POS, QR Order Pelanggan, dan Akses Anonim

Kasus kafe di sini: **pelanggan duduk → scan QR di meja → pesan → bayar
(online atau tunai di kasir) → setelah lunas masuk dapur.**

Pola dua App (publik + privat) **sudah ada** (`examples/storefront/`), dan
anonim `create` **sudah didukung**. Yang tidak ada adalah seluruh lapisan
**keranjang/checkout** dan **pembatasan akses anonim**.

---

## Gap #5 — Tidak ada blok cart/pesanan; `Listing` read-only tanpa aksi baris ✅ `CLOSED` untuk sisi pelanggan (2026-09-15)

> **Ditutup oleh TODO 1.5 (S1).** `PageBlock` kini punya blok transaksional
> `order_builder` (katalog + keranjang + checkout), dipakai halaman QR kafe
> (`cafe-order/pages/menu-catalog.yaml`). Poin 1–3 di bawah (katalog bergambar +
> tombol tambah, keranjang, kirim pesanan) **tertutup dan terverifikasi runtime**.
> Poin 4–5 (layar POS kasir, numpad uang, pembayaran gabungan) **tetap terbuka**:
> widget uang = item **2.14**, POS sebagai kind tersendiri belum ada.
> Normatif: `docs/spec/frontend/06-page-kinds.md` §1 "Blok `order_builder:`".
>
> Catatan asli di bawah dipertahankan sebagai jejak temuan.

### Bukti

`renderers/react-shadcn/src/types/manifest.ts` — `PageBlock` adalah himpunan
tertutup:

```ts
export interface PageBlock {
  form?: BlockRef
  table?: BlockRef
  component?: BlockRef
  widget?: BlockRef
  html?: string
  section?: SectionBlock
}
```

Tidak ada `cart`, `product_grid`, `checkout`, atau blok transaksional lain.

`kind: Listing` secara desain **hanya baca** —
`docs/kind/ui/Listing.md`:

> `kind: Listing` adalah **katalog publik** ... **Kapan TIDAK pakai Listing:**
> Operasi tulis terautentikasi → `kind: Table`

Dan `renderers/react-shadcn/src/kinds/listing/ListingRenderer.tsx` mengambil
baris → `navigate(...)` ke detail. **Tidak ada `row_actions`** (memang tidak ada
di `ListingSpec`: hanya `entity`, `columns`, `filters`, `search`).

`formWidget()` (`derive.ts`) memetakan `child` → `"child-grid"`, jadi
`ChildTable` bisa menjadi _daftar item_ di dalam Form — tapi itu **baris tabel
yang bisa diedit**, bukan keranjang: tidak ada pemilih produk berbentuk ubin,
tidak ada penambah cepat, tidak ada penjumlah.

### Dampak ke aplikasi kafe: **BLOCKER**

Alur pelanggan dari QR tidak bisa dinyatakan:

1. Halaman QR meja butuh **katalog bergambar dengan tombol "Tambah"**.
2. Butuh **keranjang** (badge jumlah, ubah qty, hapus item, subtotal real-time).
3. Butuh **pemilihan metode bayar** lalu penyerahan ke kasir/dapur.
4. Kasir butuh **layar POS**: grid menu, tombol cepat, numpad uang, hitung
   kembalian, pilih meja/takeaway.
5. Kasir butuh **pembayaran gabungan/terpisah** (tunai + QRIS + kartu).

Poin 1–3 bisa _dipaksakan_ dengan `Form` + `ChildTable` + `SectionBlock`
(hero/feature_grid) + script Starlark — tapi hasilnya adalah form admin, bukan
UX pemesanan. Poin 4–5 tidak mungkin: `money` tidak punya widget (lihat
`01-widget-money-time.md`), jadi tidak ada numpad uang maupun kembalian.

### Usulan

| Tingkat | Usulan                                                                                                           | Untuk siapa                         |
| ------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| Kecil   | Opsi `widget: image` + dukungan `money` → katalog pakai `Listing` lebih hidup                                    | Katalog publik                      |
| Sedang  | Blok Page baru `checkout` / tipe `SectionBlock` `product_grid` dengan slot aksi (`add-to-cart`)                  | Halaman QR                          |
| Besar   | `kind: Pos` / `kind: Cart` — VisualSpecKind transaksional: grid produk, keranjang, diskon, pembayaran, kembalian | Layanan cepat, retail, klinik kasir |

Untuk test case ini, **kombinasi `money` widget + `widget: image` di Listing +
satu blok cart** sudah cukup membuat alur QR order layak.

---

## Gap #6 — Akses anonim bersifat per-module, bukan per-entitas ✅ Pasti

### Bukti

`internal/api/router.go`:

```go
// publicEntities returns the set of "module/entity" keys mounted by any
// `access: public` App (frontend/05-app-kinds.md §1). A public App's surface
// is served anonymously, so the entities it mounts get anonymous read +
// create on the UI surface.
func (b *RouterBuilder) publicEntities() map[string]bool
```

`docs/spec/frontend/05-app-kinds.md` §1:

> `access: public` memicu: bundle anonim (`alwaysVisible`) dan data seam anonim
> (**list/find/create** di `/_ui/entity/`).

`ai_skills/formspec-kinds/SKILL.md` bahkan memberi peringatan:

> `access: public` grants anonymous read + create on **every mounted module** —
> never mount write-heavy transactional modules into a public App unless
> anonymous intake is intended.

### Dampak ke aplikasi kafe: **HIGH** (privasi)

Granularitasnya adalah **module**, bukan entity dan bukan record:

- Untuk mengizinkan pelanggan melihat **menu** (entity `menu-item` di module
  `cafe-master`) dan membuat **pesanan** (entity `order`), module yang
  di-mount ke App publik harus berisi baik `menu-item` **maupun** `order`.
- Konsekuensinya: **anonim bisa `list` semua `order`** — termasuk pesanan
  meja lain, pesanan orang lain, beserta isinya.
- Juga **anonim bisa `list` semua `member`** (nomor HP pelanggan!) dan
  `employee` kalau module itu sama.
- Tidak ada konsep "hanya pesanan milik sesi/device saya". `list` bersifat
  global untuk entity yang publik.

Kerja manual (memisahkan entity ke module berbeda, menambah filter wajib) bisa
mengurangi risiko, tapi itu **bukan** pengamanan: filter `fixed_filters` adalah
UI-level, bukan otorisasi.

### Usulan

- Tambah **allowlist per-entity** pada App publik, mis.
  `spec.public_entities: [cafe-master.menu-item]` + deklarasi aksi
  (`read`, `create`) — bukan seluruh module.
- Tambah konsep **capability/token sesi anonim** (mis. `order.token` acak yang
  menjadi kunci akses `find`) sehingga pelanggan hanya bisa membaca pesanannya
  sendiri. Ini pola "public order tracking" yang sangat umum.
- Sampai itu ada: **jangan** mount entity transaksional ke App publik; taruh
  `order` di App privat, dan buat jalur khusus untuk intake anonim.

---

## Gap #7 — `exclude: [public_api]` belum ditegakkan ✅ Pasti

### Bukti

`pkg/spec/entity.go` — `Field.Exclude []string` ada, dengan komentar
"per-surface field exclusion". Tapi di `docs_internal/plan/landing-page.md`:

> Data seam publik + mitigasi (rate limit/honeypot **disiapkan**; enforcement
> field `exclude: [public_api]` = **concern security audit Fase 6**).

Artinya field-level exclusion untuk surface publik **belum dijalankan**.

### Dampak ke aplikasi kafe: **HIGH**

Entity yang terekspos di App publik akan mengirim **seluruh field**-nya,
termasuk yang sensitif. Contoh nyata untuk kafe:

- `member.phone`, `member.email`, `member.total_spent`
- `order.notes`, `order.internal_notes`
- `employee.salary` (kalau salah module)

Tanpa `exclude: [public_api]` yang berfungsi, satu-satunya cara memisahkan
field sensitif adalah memindahkannya ke entity/module lain — solusi struktural
yang mahal.

### Usulan

- Tegakkan `exclude` di jalur response: `internal/api` + `internal/ui` harus
  menyaring field per surface (public vs authenticated vs admin).
- Ini juga menyelesaikan sebagian Gap #6 tanpa perlu token sesi.

---

## Gap #7b — Limitasi anonim bersifat global, bukan per-workspace ✅ Pasti

### Bukti

`docs/spec/platform/02-workspace-app-module.md` §3.1:

> **Limitasi**: anonymous surface App `access: public` (registrasi permission
> entitas publiknya) masih berlaku **global, tidak per-workspace** — deferred.

### Dampak ke aplikasi kafe: **MEDIUM**

Multi-outlet direncanakan memakai satu workspace dengan dimensi outlet
(Gap #8). Selama registrasi publik masih global, outlet mana pun akan
"mewarisi" permukaan anonim yang sama — tidak bisa membuat katalog publik
berbeda per outlet.

---

## Realitas POS di luar gap di atas

Sebagai catatan agar tidak salah harap: **beberapa hal POS sudah bisa berjalan
hari ini.**

| Kebutuhan kafe                   | Status FormSpec                                                                                                                      |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| KDS (layar dapur)                | ✅ **Bisa** — `kind: Kanban` + `realtime: true` (default Kanban), silent refetch saat event. Kartu pindah kolom saat status berubah. |
| Struk/nota cetak                 | ⚠️ Hanya PDF/html — lihat `06-engine-kontrak.md` Gap #10                                                                             |
| Riwayat transaksi & status       | ✅ Table + state machine + custom action                                                                                             |
| Nomor pesanan berurutan          | ✅ `natural_key_rule` (`strategy`, `format`, `prefix`, `reset`)                                                                      |
| Shift kasir (buka/tutup/selisih) | ✅ Bisa di-author: entity + state machine + script + `Wizard`                                                                        |
| Void butuh approval supervisor   | ✅ `kind: Workflow` (multi-approver, `on_reject`, `escalation`) + `ApprovalInbox`                                                    |
| Loyalty poin                     | ✅ Entity ledger + summary entity                                                                                                    |
| Harga "dibekukan" saat transaksi | ✅ `snapshot:` pada relation `belongs_to` (denormalisasi finansial)                                                                  |

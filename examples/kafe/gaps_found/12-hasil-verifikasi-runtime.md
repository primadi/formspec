# Hasil Verifikasi Runtime — 2026-09-14

Verifikasi pertama yang benar-benar berjalan: `formspec dev` di port **18100**
(port bebas yang diverifikasi dengan mencoba bind, bukan dengan membaca netstat).

```
[formspec] engine loaded: 155 routes
[formspec] SPA embedded — open http://localhost:18100/default/_admin
[formspec] REST API on :18100
[outbox-worker] started · [workflow-escalation] started
[subscription-stream] started · [subscription-dynamic] started
```

Server **jalan normal**. Semua dugaan "engine tidak bisa dijalankan" gugur.

---

## ⛔ KOREKSI GAP #24 — `formspec dev` TIDAK rusak; saya yang salah diagnosis

Severity turun dari **HIGH (DX)** menjadi **LOW (pesan error)**.

**Yang saya lakukan salah:**

1. Mencoba port 18080 → ditolak dengan _"port in use but cannot identify the owner"_.
2. Memeriksa dengan `Get-NetTCPConnection -LocalPort 18080` → kosong.
3. **Menyimpulkan dari nol hasil itu bahwa port bebas, lalu menuduh CLI-nya salah.**

Kesalahannya: nol hasil dari **satu** perintah bukan bukti port bebas. Pemeriksaan
itu hanya melihat listener IPv4 lokal pada momen itu, dan tidak menangkap pemilik
yang tidak bisa di-query tanpa elevasi. Bukti cukup untuk berkata "saya tidak
tahu siapa pemiliknya" — bukan untuk berkata "tidak ada pemiliknya".

**Yang seharusnya saya lakukan** (dan yang diingatkan user): **coba port lain**,
karena port memang bisa terpakai oleh proses lain. Cara yang benar adalah
memverifikasi kebebasan port dengan **mencoba bind**, bukan dengan membaca tabel:

```powershell
foreach ($p in 18100..18120) {
  try { $l = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Any, $p)
        $l.Start(); $l.Stop(); return $p } catch { }
}
```

Port 18100 lolos cara itu, dan dev server langsung naik tanpa keluhan.

**Yang tetap berlaku dari #24** (dan hanya ini): pesan errornya tidak memberi
tahu apa yang harus dilakukan. _"cannot identify the owner"_ tidak menyebut
"coba port lain" maupun menyertakan flag. Itu masalah DX kecil, bukan blocker.

**Pelajaran yang saya tulis besar-besaran:** ini kali **ketiga** saya mengklaim
bug framework dari bukti yang terlalu tipis (Gap #2 storage money, Gap #18
`target:`, sekarang Gap #24). Pola yang sama: membaca kode/dokumen atau menjalan-
kan satu perintah, lalu melompat ke kesimpulan struktural. Yang membedakan
temuan yang bertahan (#21, #22, #23, #38, #39, #43) adalah temuan yang **diuji
dengan percobaan yang bisa gagal**, bukan yang disimpulkan.

---

## Gap #44 — Record baru duduk di `doc_status: draft` dan karena itu TIDAK BISA DIREFERENSIKAN 🔴 BLOCKER

Ini temuan runtime paling penting, dan ia membuat aplikasi **tidak bisa berjalan
end-to-end** apa adanya.

### Bukti

Membuat `menu-category` berhasil (`doc_status: "draft"`, `is_active: true`).
Lalu membuat `menu-item` yang mereferensikannya:

```
POST /default/_ui/entity/cafe-master/menu-item
{ code: "LAT-001", name: "Latte", menu_category_id: "<uuid>" }

→ 400 VALIDATION_ERROR
  "menu-item insert: field validation failed: relation target
   cafe-master.menu-category[<uuid>] is draft (must be submitted or lifecycle-free)"
```

Percobaan menyelesaikannya:

```
POST /default/_ui/entity/cafe-master/menu-category/<uuid>/submit

→ 403 FORBIDDEN
  "missing permission: cafe-master.menu-category.update"
```

Jadi rantainya:

| Langkah                       | Hasil                              |
| ----------------------------- | ---------------------------------- |
| `create` menu-category        | ✅ berhasil, `doc_status: draft`   |
| `submit` menu-category        | ❌ 403 — butuh permission `update` |
| referensikan dari entity lain | ❌ ditolak selama masih `draft`    |

### Dampak ke aplikasi kafe: **BLOCKER**

Setiap entity yang jadi **target relasi** harus `submitted` dulu. Di spec ini
hampir semua entity saling mereferensikan:

- `menu-item` → `menu-category`
- `menu-item-price` → `branch` + `menu-item`
- `order` → `branch`, `dining-table`, `member`, `employee`, `promo`, `table-session`, `shift`
- `payment` → `order`, `branch`, `shift`, `employee`

Artinya **seluruh master data harus lewat dua langkah** (create lalu submit)
sebelum satu transaksi pun bisa dibuat. Dan untuk permukaan publik (lihat #45),
langkah kedua itu **tidak mungkin dilakukan**.

Konsekuensi desain yang belum saya sadari saat menulis spec: `lifecycle:
plain_crud` + `{name: submit, disabled: true}` — yang saya pakai di semua entity
berstate-machine — berarti record itu **tidak akan pernah bisa direferensikan**,
karena satu-satunya jalan keluar dari `draft` sudah dimatikan.

Untuk `order` khususnya ini fatal: `payment.order_id` menunjuk ke `order`, jadi
pembayaran **tidak akan pernah bisa dicatat**.

### Usulan

1. Tegaskan di dokumentasi bahwa `create` **selalu** menghasilkan `doc_status:
draft`, dan bahwa `submit` adalah langkah wajib sebelum bisa direferensikan —
   saat ini itu hanya tersirat di komentar kode dan pesan error.
2. Sediakan cara menyatakan "entity ini tidak ber-lifecycle" secara tegas
   (mis. `lifecycle: none`) supaya record-nya langsung referenceable —
   karena `lifecycle: plain_crud` ternyata **masih** memakai `doc_status`.
3. `formspec validate` sebaiknya memperingatkan kombinasi berbahaya:
   **`submit` disabled + entity dipakai sebagai target relasi** → "target relasi
   ini tidak akan pernah bisa direferensikan".

---

## Gap #45 — Permukaan publik bisa `create` tapi tidak bisa `submit` ✅ CLOSED (2026-09-15, TODO 2.2)

> **Ditutup 2026-09-15 (TODO 2.2).** Tiga bagiannya: (1) `submit` tidak lagi
> relevan karena katalog & `order` **lifecycle-free** (#44/2.1) sehingga record
> anonim langsung referenceable oleh `payment`, dan rute aksi lifecycle sudah ada
> di surface UI (#52/2.12) — kasir memang bisa melanjutkan; (2) allowlist publik
> per-entity (#6/1.2) menghentikan "anonim boleh apa saja di module"; (3)
> **kepemilikan token tamu** kini dinyatakan: `public_entities[].scope` membatasi
> `list` anonim ke baris yang tokennya cocok, dibaca server dari parameter request
> (tanpa token → 403, dan klien tidak bisa melebarkannya). Bukti runtime ada di
> TODO 2.2; adopsi kafe: `order.guest_token` + grant `create,list` di `kafe-qr`.
> **Sisa:** alur scan QR masih dua langkah (token → sesi → ID) karena `find`
> tidak bisa di-scope.

### Bukti

Terhadap workspace yang sama, sebagai anonim:

| Entity                      | Module di-mount App publik? | Hasil anonim                            |
| --------------------------- | --------------------------- | --------------------------------------- |
| `cafe-master/branch`        | ✅ (`kafe-qr`)              | `create` **berhasil**                   |
| `cafe-master/menu-category` | ✅                          | `create` **berhasil**, `submit` **403** |
| `cafe-master/promo`         | ✅                          | `create` **berhasil**                   |
| `cafe-stock/ingredient`     | ❌ (hanya `kafe-pos`)       | `create` **401 Unauthorized**           |

Ini memperlihatkan **Gap #06 persis seperti yang didokumentasikan** — dan lebih
tajam dari perkiraan: persoalannya bukan hanya "anonim bisa membaca lebih banyak
dari yang seharusnya", tetapi **anonim bisa membuat data yang tidak bisa
diselesaikan siapa pun**.

### Dampak ke aplikasi kafe: **BLOCKER**

Alur QR order yang diinginkan:

1. Pelanggan (anonim) memesan → butuh `create` pada `order` → **bisa**
2. Agar `order` bisa dibayar, `payment.order_id` harus menunjuk ke `order` →
   butuh `order` sudah `submitted` → **tidak bisa dari sisi anonim**
3. Dan kasir pun tidak bisa menyelesaikan record yang dibuat anonim bila
   kepemilikannya/izinnya tidak dirancang untuk itu

Jadi permukaan publik dapat **membuat sampah**: record `draft` yang tidak
referenceable, tidak bisa di-submit dari sisi publik, dan menumpuk.

### Usulan

`access: public` sebaiknya **tidak** memberikan `create` secara default. Yang
lebih aman: izin eksplisit per-entity **plus** pernyataan langkah penyelesaiannya
— mis. "anonim boleh create `order`, dan `order` yang dibuat anonim otomatis
`doc_status: submitted` dengan kepemilikan token tamu". Ini juga menyelesaikan
#06 dan #07 sekaligus.

---

## Gap #46 — `money` TIDAK divalidasi dan TIDAK dinormalisasi; `settings.currency` tidak diterapkan 🔴 HIGH

### Bukti

Tiga bentuk dikirim ke field `money`, **ketiganya diterima apa adanya**:

| Dikirim                                | Tersimpan                                          |
| -------------------------------------- | -------------------------------------------------- |
| `{ amount: "50000", currency: "IDR" }` | `{ "amount": "50000", "currency": "IDR" }`         |
| `{ amount: "15000" }` (tanpa currency) | `{ "amount": "15000" }` — **currency tidak diisi** |
| `25000` (angka polos)                  | `25000` — **angka, bukan objek**                   |

Padahal kontraknya tegas. `pkg/spec/money.go`:

```go
type Money struct {
    Amount   string `json:"amount"`
    Currency string `json:"currency"`
}
```

Dan `ResolveMoneyCurrency` didokumentasikan menyelesaikan dengan urutan
_"explicit field currency -> settings.currency.code -> **error (never guess)**"_.

**Yang diamati: tidak ada error, dan `settings.currency` tidak dipakai.**
Kemungkinan: jalur API ini tidak melewati resolusi itu, atau `spec/config/app.yaml`
tidak dimuat. Keduanya perlu dipastikan — tetapi yang **pasti** adalah tiga
bentuk data kini hidup di satu kolom yang sama.

### Dampak ke aplikasi kafe: **HIGH**

- **Tidak ada jaminan bentuk** bagi konsumen. Renderer, report, agregasi, dan
  script akan menerima minimal tiga bentuk berbeda.
- `{ "amount": "15000" }` **kehilangan informasi mata uang** — untuk aplikasi
  yang mencatat uang, itu data yang tidak bisa dipulihkan.
- Angka polos `25000` bukan `Money`, sehingga `json_extract(data,'$.value.amount')`
  menghasilkan `null` — dan **Gap #23** (kolom turunan money bertipe `text`)
  menjadi lebih buruk: kolomnya bukan hanya salah urut, tapi bisa kosong.
- Ini juga menjelaskan mengapa `renderCellValue` akan gagal: nilainya objek,
  bukan angka, dan salah satu bentuknya bahkan bukan objek.

### Koreksi Gap #26

Gap #26 menyatakan `settings.currency` **WAJIB** dan tanpa itu aplikasi gagal di
runtime. **Verifikasi membantah separuh:** aplikasi tidak gagal — ia justru
menerima money tanpa currency secara diam-diam. Jadi:

- ❌ "WAJIB, kalau tidak gagal" → **salah** untuk jalur ini.
- ✅ "tidak diterapkan, dan data tanpa currency masuk tanpa keluhan" → **benar**.

Yang lebih berbahaya adalah yang kedua: kegagalan keras bisa diperbaiki, data
tanpa mata uang tidak.

### Usulan

1. Tetapkan **satu** bentuk kanonik di batas API dan normalisasi di sana:
   terima angka/string/objek, simpan selalu sebagai `{amount, currency}`.
2. Isi `currency` dari `settings.currency` bila kosong — sesuai yang sudah
   didokumentasikan, tapi belum terjadi.
3. **Tolak** dengan error bila currency tetap tidak bisa ditentukan. Diam-diam
   menyimpan tanpa mata uang adalah pilihan terburuk untuk data finansial.
4. `formspec validate` memperingatkan bila ada field `money` tanpa
   `settings.currency` di spec — pemeriksaan statis yang murah.

---

## Yang Terverifikasi **Bekerja**

| Aspek                                     | Bukti                                                                                                 |
| ----------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `formspec dev` jalan (port bebas)         | 155 route, SPA embedded, 4 worker aktif                                                               |
| UI surface entity                         | `GET/POST /{ws}/_ui/entity/{module}/{entity}` berfungsi                                               |
| Bentuk request create                     | **flat** (bukan dibungkus `data`); error menuntun: `unknown field: "data"`                            |
| Envelope respons                          | `{ data, meta: { request_id, timestamp } }`; list: `{ data, meta: { page, per_page, total }, links }` |
| `soft_deactivate` bekerja                 | `is_active: true` otomatis ada di record baru                                                         |
| Guard referenceability relasi **bekerja** | Menolak target `draft` dengan pesan jelas (lihat #44)                                                 |
| Permission ditolak dengan jelas           | `403 missing permission: cafe-master.menu-category.update`                                            |
| Module non-publik terlindungi             | `401 authentication required` untuk `cafe-stock`                                                      |
| Pesan error informatif                    | `unknown field`, `is draft (must be submitted or lifecycle-free)`                                     |

> **Catatan #46 soal penamaan permission:** engine menghasilkan
> `cafe-master.menu-category.update` — memakai **nama entity (singular)**.
> Sementara contoh di dokumen Core memakai **plural** (`orders.checkout` →
> `billing.orders.checkout`), dan itu yang saya salin ke
> `required_permission` di seluruh spec ini. Bila konvensi sebenarnya singular,
> maka **semua `required_permission` di aplikasi ini salah** dan tidak akan
> cocok dengan permission yang diperiksa engine.

---

## Gap #47 — Kontrak REST surface `/_ui/` tidak terdokumentasi ✅ Terverifikasi

### Bukti

Satu-satunya cara saya mengetahui bentuk request adalah **gagal berkali-kali**:

| Yang saya coba             | Hasil                                                   | Pelajaran           |
| -------------------------- | ------------------------------------------------------- | ------------------- |
| `POST { "data": { ... } }` | `400 unknown field: "data"`                             | body harus **flat** |
| `POST { code, name, ... }` | ✅                                                      | bentuk yang benar   |
| aksi                       | `POST /{ws}/_ui/entity/{module}/{entity}/{id}/{action}` | saya coba-coba      |

Bentuk yang akhirnya diketahui:

```jsonc
// list
{ "data": [ ... ], "meta": { "page":1, "per_page":20, "total":N, "total_pages":N },
  "links": { "first": "...", "last": "..." } }

// satu record (create/read)
{ "data": { ... }, "meta": { "request_id": "...", "timestamp": "..." } }

// error
{ "error": { "code": "VALIDATION_ERROR", "message": "..." }, "meta": { "timestamp": "..." } }
```

### Kenapa ini gap

Surface `_ui` adalah **kontrak publik** bagi setiap klien — SPA bawaan pun
memakainya. Tetapi tidak ada satu halaman dokumentasi pun yang menjelaskan
bentuk request, envelope respons, atau endpoint aksi. Setiap klien baru harus
menemukannya dengan trial-and-error.

Ironisnya, untuk project yang menjanjikan _"manifest sebagai satu-satunya sumber
kebenaran"_, kontrak HTTP-nya justru permukaan yang paling tidak terdokumentasi.

### Usulan

1. Halaman `docs/api/ui-surface.md` — bentuk request/response + daftar endpoint
   per kind (entity, list, action, print, report).
2. Idealnya **digenerate** dari `pkg/spec` (sumber yang sama dengan JSON Schema),
   supaya tidak bisa basi.
3. Minimal: `formspec describe <module/entity>` mencetak kontrak HTTP entity itu
   (field, aksi, bentuk body), bukan hanya ringkasan manifest.

---

## Gap #48 — `kind: Workspace` tidak menentukan workspace aktif ⚠️ Kemungkinan BY DESIGN

### Bukti

| Pengamatan                      | Hasil                                                           |
| ------------------------------- | --------------------------------------------------------------- |
| `spec/workspaces/kafe.yaml` ada | **lulus validasi** (`[OK] spec\workspaces\kafe.yaml#0`)         |
| Pengumuman `formspec dev`       | `SPA embedded — open http://localhost:18100/**default**/_admin` |
| `POST /default/...`             | tersimpan dengan `tenant_id: "default"`                         |
| `GET /kafe/...`                 | **200 OK tapi kosong** (`total=0`)                              |
| `formspec dev --help`           | `-workspace-id string` · _Workspace ID (default: default)_      |

### ⚠️ Bukan bug — tapi ekspektasinya menyesatkan

Flag `--workspace-id` **ada** dan defaultnya memang `default`. Jadi perilakunya
sesuai desain. Yang menjadi gap adalah **apa yang pembaca dokumen simpulkan**:

> _"Workspace manifests are **seed declarations**: the slug becomes (**and must
> equal**) the workspace ID used in URLs"_

Kalimat itu mudah dibaca sebagai "mendeklarasikan workspace berarti memakainya".
Akibat praktisnya:

1. Developer mendeklarasikan `kafe`, menjalankan dev, dan **datanya masuk ke
   `default`** — tanpa peringatan apa pun.
2. `GET /kafe/...` merespons **200** (bukan 404), sehingga terlihat benar —
   hanya saja kosong.

Jadi datanya tidak hilang, tapi berada di tenant yang tidak diduga, dan satu
permukaan tambahan tampak sehat padahal berbeda. Ini kelas "berbeda dari yang
dibaca" yang sama dengan banyak gap lain di catatan ini.

### Usulan

1. Peringatkan saat startup bila ada `spec/workspaces/*` tetapi workspace aktif
   berbeda — murah, dan langsung menutup kebingungannya.
2. Atau: bila **hanya satu** workspace dideklarasikan, jadikan ia default.
3. Perjelas di dokumen bahwa workspace aktif ditentukan **flag saat menjalankan**,
   bukan oleh manifest — manifest hanya mendaftarkannya.

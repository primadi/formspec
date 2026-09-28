# Plan — Test skenario kafe end-to-end (QR → QRIS → dapur → kasir)

**Tanggal:** 2026-09-27. **Status:** ✅ MENDARAT (semua §1–§6 selesai).
**Skenario sumber:** permintaan pemilik — _"tes lengkap mulai pelanggan datang,
scan qrcode meja → pilih nasi goreng, bayar QRIS, order masuk dapur → dapur
selesai, update status, meja `served` → tambah es teh, bayar, meja kembali
`occupied` → dapur selesai, meja `served` → kasir update meja `available`"_.

## Hasil

| Skenario pemilik                  | Cara diverifikasi                                                         | Status |
| --------------------------------- | ------------------------------------------------------------------------- | ------ |
| Pelanggan datang, scan QR meja    | klik `/kafe/t/JKT-A01-DEMO` → isi nama → "Lihat Menu"                     | ✅     |
| Pilih nasi goreng                 | panel picker "Pilih Nasi Goreng Spesial"                                  | ✅     |
| Bayar QRIS → order ke dapur       | PATCH `awaiting_payment` → `paid` (kanal `qris`); KDS menampilkan pesanan | ✅     |
| Dapur selesai → meja `served`     | `dapur`: `in_kitchen` → `ready`; `pelayan`: `served`                      | ✅     |
| Tambah es teh, bayar → `occupied` | pesanan kedua di sesi sama → meja kembali `occupied`                      | ✅     |
| Dapur selesai es teh → `served`   | ulangan langkah yang sama                                                 | ✅     |
| Kasir `release` → `available`     | PATCH `table_status: available`                                           | ✅     |

**Jalankan:** `make e2e-deps` (sekali per mesin) → `make e2e-kafe`.
Go: `go test ./resource/ -run TestKafe -count=1`.

## Arti "test e2e" di sini: dua lapis, dua pertanyaan

Permintaan awal menyebut "jalankan tes e2e menggunakan frontend yang ada".
Dijawab dengan **dua** lapis, karena sebuah klik di browser dan sebuah invariant
di database adalah dua hal yang bisa rusak secara independen:

| Lapis         | File                                                | Menjawab pertanyaan                                                             |
| ------------- | --------------------------------------------------- | ------------------------------------------------------------------------------- |
| Go in-process | `resource/kafe_table_lifecycle_e2e_test.go`         | apakah **semantik**-nya benar (status meja, idempotensi, keunikan sesi, jurnal) |
| Playwright    | `renderers/react-shadcn/e2e/kafe-lifecycle.spec.ts` | apakah **tombol yang diklik manusia** benar-benar bekerja                       |

Lapis Go memakai harness yang sudah ada (`bootKafe` di
`resource/o2c_e2e_test.go`) sehingga yang di-boot adalah aplikasi kafe
**sungguhan** — manifest, engine, outbox, subscription, Starlark — bukan tiruan.

### Test Go (3)

| Test                                               | Mengunci                                                                                                                                                                                   |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `TestKafe_TableLifecycle_FullScenario`             | 8 langkah skenario; setiap peralihan status di-poll (subscription async), bukan diasumsikan. Termasuk resolve meja dari token QR **dan** resolve sesi dari `guest_token` secara **anonim** |
| `TestKafe_TableLifecycle_CancelReturnsToAvailable` | simetri: pesanan batal tidak meninggalkan meja terisi selamanya                                                                                                                            |
| `TestKafe_SessionOpenUnique`                       | 10.34c di lapisan storage: sesi terbuka kedua ditolak; sesi `closed` tidak memblokir                                                                                                       |

### Skenario Playwright (1, 8 langkah)

Empat konteks browser: **tamu** (anonim), **kasir**, **dapur** (KDS), **pelayan**
(POS). Setiap langkah dilakukan oleh peran yang memang **memegang grant**-nya —
ini bukan detail gaya:

- `mark-served` dipegang **pelayan**, bukan dapur. Versi pertama menjalankannya
  dari sesi KDS → **403** `missing permission: cafe-order.orders.mark-served`.
  Test lalu menghormati pembagian peran itu, alih-alih memakai token serba-bisa
  (yang akan menyembunyikan regresi otorisasi).

**Pembagian sadar antara klik dan API** (alasan teknis, bukan kemudahan):

- Langkah 1–3 dan 5–8 adalah **klik** — jalur renderer yang bisa rusak oleh
  refactor, dan itulah alasan file ini ada.
- Langkah 4 (drag KDS) lewat API: board `@dnd-kit` butuh gesture pointer nyata,
  dan klik kartunya **bernavigasi** (tidak membuka dialog), jadi tidak ada
  affordance kedua. Yang distabilkan adalah setengah yang jujur — board
  **menampilkan** pesanan yang sudah lunas (aturan bisnis #1).
- Perubahan status meja lewat API: tombolnya ada di halaman detail **derived**
  yang segmen route-nya bergantung identitas record; mengkliknya berarti menguji
  derivasi route, bukan transisi. Semantik transisinya sudah dipin test Go; di
  sini yang diassert adalah **hasil** yang dilihat kasir.

## Prasyarat yang menghalangi (dan ditutup) sebelum test bisa ditulis

Skenario ini tidak bisa "diuji" apa adanya karena tiga langkahnya belum bisa
dinyatakan. Itu sebabnya plan ini punya fase spec, bukan hanya fase test.
Enam gap ditutup — daftar lengkap + bukti ada di changelog `2026-09-27-017` dan
ledger kafe (10.34b, 10.34c, 10.35/2.15, 10.36, 10.37, 10.40b).

Urutannya tidak bebas: **10.34b (natural key) adalah prasyarat 10.35** (halaman
masuk resolve meja dari token), dan **10.37 (redirect) prasyarat tanpa
`guest_token` yang bisa di-URL**. Rencana awal menyebut 10.34b "bukan blocker" —
itu keliru, dan koreksinya sudah dicatat di
`docs_internal/plan/kafe-qr-table-session-flow.md`.

## Yang ditemukan justru karena menguji di browser

Nilainya terbukti pada run pertama: **dua bug renderer nyata** membuat alur tamu
**mustahil diklik** sebelum diperbaiki, dan keduanya tidak terlihat oleh 40 test
jsdom maupun `go test`:

1. **Tombol Create tamu hilang.** `canDoEntityAction` memeriksa identitas
   sebelum `authorized_actions`; permukaan publik boot anonim **tanpa** identitas
   (desain `session.ts`), sehingga setiap tombol create disembunyikan. Form
   hanya merender dua field + "Cancel".
2. **`default_from` mengirim placeholder mentah.** Form yang `spec.context`-nya
   mengambil record (`source: entity`) menyelesaikannya **async**, sedangkan
   seeding berjalan sekali → `{table.branch_id}` terkirim literal → **422**
   `relation branch_id points to cafe-branch[{table.branch_id}]`.

Detail + mekanisme: changelog `2026-09-27-018`.

**Pelajaran metodologis yang dicatat:** kedua bug itu **gagal-tertutup** dan
**senyap** — tombol hilang, atau klik berhasil tanpa apa-apa terjadi. Karena
tombolnya tidak ada, tidak ada toast error yang bisa memancing kecurigaan. Test
browser bukan "tambahan" untuk kelas bug ini; ia satu-satunya yang menangkapnya.

## Isolasi: kenapa bukan dev DB (pembalikan dari rencana awal)

Rencana awal memakai dev DB `examples/kafe/.formspec/kafe.db`. Saat
implementasi, itu **terbukti menghasilkan false green**:

- Dev DB adalah fixture yang bermutasi; sejak 10.34c mendarat, run kedua gagal
  karena alasan yang tidak berhubungan dengan kode.
- `reuseExistingServer` mengambil **server yang sudah listen**. Terukur: server
  debug dengan DB lama dipakai ulang, sehingga daftar meja memuat **dua baris
  `A-01` di satu cabang** (satu `available`, satu `occupied`) — assertion membaca
  baris yang tak pernah disentuh dan melaporkan meja yang **sebenarnya sudah
  terisi** sebagai `available`. Kegagalan yang tampak seperti bug produk.

Penutupnya: DB sementara yang di-seed ulang per run (`mkdtempSync` +
`formspec seed`), dan **`reuseExistingServer: false`** pada kedua `webServer`.
Sekalian ditemukan bahwa pencarian meja lewat `code` juga salah **terlepas dari
harness** (`code` hanya unik per cabang) → assertion kini me-resolve lewat
`qr_token` yang unik, sekaligus menguji 10.34b.

## Instalasi (didokumentasikan karena tidak trivial)

`~/.cache` dimiliki root di dev container ini, jadi Chromium dipasang ke lokasi
yang bisa ditulis, dan **pustaka sistemnya** (`libglib`/`libatk`/`libgbm`/
`libpango`/…) belum ada — Chromium gagal start tanpa pesan yang jelas.

```bash
make e2e-deps   # chromium + install-deps (butuh sudo)
make e2e-kafe
```

`sudo` mereset `PATH`, jadi target `e2e-deps` memakai path `node` absolut.

## Sisa yang bernomor (tidak ditutup di sini, dan alasannya)

| Item         | Sisa                                                                                                                                             |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **10.41** 🟡 | `served` = **semua** pesanan sesi disajikan. Yang landing: aturan longgar. Butuh agregat lintas-record (`ctx.db().query` atau `Rebuild.Trigger`) |
| **10.52** ⏸️ | pembatalan mengosongkan meja yang masih berisi pesanan lain (akar sama dengan 10.41)                                                             |
| **10.55** ⏸️ | tidak ada kompensasi bila subscription occupancy gagal — meja tetap `available` padahal lunas, tanpa pemberitahuan                               |
| **10.38** ⏸️ | PIN tamu kedua + `guest_token`/PIN acak server-side + hashing                                                                                    |
| **10.39** ⏸️ | Service publik `table-status`/`verify-pin`                                                                                                       |
| **10.20** ⏸️ | landing `/kafe` → petunjuk scan QR                                                                                                               |
| **10.53** ⏸️ | `Materialize` menolak seluruh role bila satu grant gagal (senyap)                                                                                |
| **10.54** ⏸️ | lookup natural key tanpa `LIMIT 1` yang diekspresikan                                                                                            |
| **10.42** ⏸️ | baris `dining-table` lama tanpa `table_status` masih bisa 500 pada transisi                                                                      |

## Bukti

- `formspec validate` kafe → **89 manifest, 0 problem**.
- `go test ./...` → hijau; kafe: **16 test** (4 baru).
- `make e2e-kafe` → **1 passed** (8 langkah, Chromium).
- `npx vitest run` → **536 passed** (40 file); `npx tsc --noEmit` bersih.
- oxlint: **41 warning**, sama dengan baseline (tidak ada yang ditambah).

## Tindak lanjut setelah laporan pertama (changelog `-020`)

Gelombang kedua menutup apa yang laporan pertama tinggalkan, dan dua di antaranya
menemukan bahwa **test ini sendiri yang membongkarnya**:

| Item                                               | Hasil                                                                                                                                                                                                                         |
| -------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **10.42** baris lama → transisi state 500          | ✅ diperbaiki **di engine** (`validateStateTransition` salah membaca `!oldExists` sebagai "record baru")                                                                                                                      |
| **10.35a** sesi tidak pernah ditutup               | ✅ — dan ternyata **wajib**: gabungan 10.34c + halaman masuk QR membuat meja bisa dipakai **sekali**. Test reproduksi ditulis lebih dulu (`status="open"` → gagal), lalu `release` → `on_cleared` → `cafe-order` menutup sesi |
| **10.53** satu grant jelek melemahkan seluruh role | ✅ `MaterializePartial` + pelaporan per-grant                                                                                                                                                                                 |
| **10.43** "gate bukan batas keamanan"              | ✅ **superseded** — `PATCH` memang menghormati gate (403 `dapur` vs 200 `pelayan`, dari E2E ini)                                                                                                                              |
| **10.54** `LIMIT 1` "tidak ada"                    | ⛔ **ditarik — klaim saya salah**; `FindByField`/`FindByFields` memilikinya                                                                                                                                                   |
| **10.56 / 10.57**                                  | temuan baru bernomor (subscription tak bisa `uses`; sesi telantar mengunci meja)                                                                                                                                              |

**Pelajaran metodologis kedua:** 10.42 dan 10.35a baru terlihat setelah alur
ini benar-benar **dijalankan** berulang. 10.42 menuntut baris DB lama (seed fresh
tak pernah memperlihatkannya), dan 10.35a menuntut **tamu kedua di meja yang
sama** — sesuatu yang tidak akan pernah muncul pada satu run yang bersih. Itu
argumen terkuat mengapa test ini memakai meja yang sama dua kali, bukan sekali.

## Rujukan

- Changelog: `docs_internal/changelog/2026-09-27-017`, `-018`, `-019`.
- Plan desain alur: `docs_internal/plan/kafe-qr-table-session-flow.md`.
- Ledger kafe: `examples/kafe/gaps_found/TODO.md`.

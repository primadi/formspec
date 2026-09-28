# Plan — Perbaikan bug yang timbul dari alur sesi meja kafe

**Tanggal:** 2026-09-28. **Status:** B1 ✅ · B3 ✅ · B2 ⏸️ MENUNGGU KEPUTUSAN
PEMILIK · B4 ⏸️ belum ada bukti. **Pemicu:** permintaan pemilik "perbaiki bug
yang timbul".
**Rujukan:** changelog `2026-09-27-017`…`-020`, `2026-09-28-001`, ledger
`examples/kafe/gaps_found/TODO.md` (10.56, 10.57).

## Hasil

| #      | Bug                                    | Status                                                          |
| ------ | -------------------------------------- | --------------------------------------------------------------- |
| **B1** | Pelanggaran constraint dijawab 500     | ✅ **selesai** (changelog `2026-09-28-001`)                     |
| **B3** | Handler subscription tidak bisa `uses` | ✅ **selesai** (changelog `2026-09-28-001`)                     |
| **B2** | Sesi ditinggalkan mengunci meja        | ⏸️ **butuh keputusan pemilik** (opsi di bawah)                  |
| **B4** | vitest flaky                           | ⏸️ tidak ter reproduksi dalam 9 run — tidak dikejar tanpa bukti |

## Ringkasan

Empat bug, tiga bisa diperbaiki tanpa keputusan produk dan satu **butuh
keputusan pemilik**. Urutan pengerjaan disusun supaya B1 (klasifikasi error)
lebih dulu: ia membuat B2 terlihat sebagai masalah _produk_, bukan _server rusak_.

| #      | Bug                                                                 | Sifat                 | Effort       | Butuh keputusan? |
| ------ | ------------------------------------------------------------------- | --------------------- | ------------ | ---------------- |
| **B1** | Pelanggaran constraint dijawab **500 INTERNAL_ERROR**               | platform, luas        | small–medium | tidak            |
| **B2** | Sesi dibuat lalu ditinggalkan **mengunci meja** (kafe 10.57)        | perilaku + produk     | medium       | **YA**           |
| **B3** | Handler subscription tidak bisa mendeklarasikan `uses` (kafe 10.56) | jebakan mode produksi | small–medium | tidak            |
| **B4** | vitest flaky (3 gagal di 1 dari 9 run)                              | tak ter reproduksi    | ?            | tidak            |

---

## B1 — Pelanggaran constraint dijawab 500 (platform-wide)

**Bukti (diukur, bukan disimpulkan).** Probe tiga bentuk pelanggaran unik
terhadap server in-process:

| Bentuk                                                                      | Hasil terukur            |
| --------------------------------------------------------------------------- | ------------------------ |
| Partial unique index — sesi TERBUKA kedua di satu meja (10.34c)             | **500 `INTERNAL_ERROR`** |
| Composite unique — `dining-table` dengan `(branch_id, code)` yang sudah ada | **500 `INTERNAL_ERROR`** |
| Field `unique: true` + `natural_key` — `qr_token` duplikat                  | **500 `INTERNAL_ERROR`** |

**Rantai akar masalahnya (3 mata rantai, semuanya perlu):**

1. `renderers/jsonb-persist` **tidak punya sentinel** untuk pelanggaran
   constraint. Yang ada: `ErrNotFound`, `ErrValidationRequired`,
   `ErrImmutableFieldChanged`, `ErrValidationRule`, `ErrUnknownField`. Error
   driver naik apa adanya sebagai string.
2. `internal/api/handler.go` `isConflictError` hanya mencocokkan substring
   `"version conflict"` dan `"not found"` — tidak ada cabang untuk
   `UNIQUE constraint failed` (SQLite) maupun
   `duplicate key value violates unique constraint` (PostgreSQL, SQLSTATE 23505).
3. `writeStoreError` jatuh ke `default:` → `500 INTERNAL_ERROR`.

**Konsekuensi yang membuat ini prioritas:**

- **Tamu/dashboard melihat "server rusak"** untuk kesalahan yang bisa
  diperbaiki pengguna (kode duplikat, token duplikat). Di jalur QR ini
  user-facing: halaman masuk tamu menampilkan 500.
- **Sinyal monitoring rusak.** 500 adalah kelas yang di-page; duplicate value
  bukan. Setiap duplicate di produksi menjadi alarm palsu sekaligus
  menyembunyikan bug nyata di antara mereka.
- **Klien tidak bisa membedakan** "kirim ulang dengan nilai lain" dari "coba
  lagi nanti" — tidak ada pesan yang bisa ditampilkan.
- **Field `unique: true` tidak punya pre-check.** `validateUnique`
  (`crud.go:3197`) hanya jalan untuk rule `{name: unique}`, **bukan** untuk
  deklarasi `unique: true`/`natural_key`. Jadi constraint DB adalah
  satu-satunya penegak — dan itulah jalur yang bocor ke 500.

**Perbaikan yang direncanakan:**

- `renderers/jsonb-persist`: sentinel `ErrUniqueViolation` + helper pengklasifikasi
  (`isUniqueViolation(driver, err)`) yang mengenali bentuk SQLite & PostgreSQL.
  Bungkus di **satu tempat** — titik tulis (`Insert`/`Update`) — supaya semua
  jalur (API, script, seed) mendapat kelas error yang sama, bukan hanya jalur HTTP.
  Sekalian tangkap `FOREIGN KEY constraint failed` sebagai kelas terpisah
  (guard relasi sudah pre-check, jadi ini jaring pengaman).
- `internal/api/handler.go`: `isConflictError` mengenali `ErrUniqueViolation`
  (via `errors.Is`, bukan substring) → **409 `CONFLICT`** dengan pesan yang
  menyebut field/constraint-nya, bukan teks driver mentah.
- **Regression test** untuk ketiga bentuk di atas (probe yang sudah ada
  dijadikan test permanen, di `resource/` karena lewat HTTP).
- Test unit untuk kedua bentuk teks driver, supaya dukungan PostgreSQL tidak
  bergantung pada ingatan.

**✅ Yang benar-benar mendarat:**

- `renderers/jsonb-persist/constraint.go` (baru): `ErrUniqueViolation`,
  `UniqueViolationError{Detail, Err}`, `classifyConstraintError`. Frase driver
  yang dikenali: `UNIQUE constraint failed:` (SQLite, termasuk suffix kode
  perluasan `(2067)`), `duplicate key value violates unique constraint` +
  `DETAIL: Key (...)` (PostgreSQL/pgx), dan SQLSTATE `23505` telanjang.
- Detail dinormalisasi ke **nama field logis**: prefix tabel dibuang
  (`cafe_order_table_sessions._dining_table_id` → `dining_table_id`) dan kolom
  scope milik framework (`tenant_id`) disaring — kalau tidak, pesan menunjuk
  field yang tidak pernah dikirim pemanggil.
- Klasifikasi dipasang di **titik tulis**: baris induk (insert), insert children,
  dan update children. Jadi bukan hanya jalur HTTP yang mendapat kelas error
  yang benar.
- `internal/api`: cabang `errors.Is(err, db.ErrUniqueViolation)` **sebelum**
  cabang conflict generik → `writeUniqueViolationError` → **409 `CONFLICT`**
  dengan `error.message = "a record with this value already exists: <field>"`.
- Test: `renderers/jsonb-persist/constraint_test.go` (8 bentuk teks driver,
  termasuk **negatif** — FK/NOT NULL/error lain tidak boleh diklasifikasi —
  plus idempotensi dan nil) dan `resource/constraint_status_e2e_test.go`
  (3 bentuk nyata lewat HTTP, asert status **dan** nama field, plus test
  negatif bahwa relasi rusak tetap bukan 409).

**Bukti sebelum → sesudah (probe terhadap server in-process):**

| Bentuk                                      | Sebelum              | Sesudah                                   |
| ------------------------------------------- | -------------------- | ----------------------------------------- |
| Sesi TERBUKA kedua di satu meja             | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: dining_table_id` |
| `dining-table` `(branch_id, code)` duplikat | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: branch_id, code` |
| `qr_token` duplikat                         | 500 `INTERNAL_ERROR` | **409** `CONFLICT` — `…: qr_token`        |

**Catatan desain:** 409 bukan 422. Nilainya **valid**; yang salah adalah
keadaannya (sudah ada). Klien membutuhkannya untuk memilih pesan ("kode ini
sudah dipakai") dan tidak mengulang. Ini juga konsisten dengan `CONFLICT` yang
sudah dipakai untuk version conflict (`not found` sengaja tetap di jalur lama).

## B2 — Sesi ditinggalkan mengunci meja (kafe 10.57) — **BUTUH KEPUTUSAN**

**Bukti.** Tamu memindai kartu → sesi dibuat (`open`), meja **tetap
`available`** (status meja hanya berubah saat BAYAR, 10.40b). Tamu batal pergi.
Sesi itu tidak punya jalur keluar: penutupnya adalah `release`, dan `release`
hanya boleh dari `occupied`/`served`. Tamu berikutnya di meja yang sama →
**500** (probe B1 bentuk 1).

**Ketidaksesuaian yang harus diakui:** `examples/kafe/docs/overview.md` (yang
saya tulis kemarin) menyatakan _"tamu yang memindai kartu meja yang sedang
terisi tetap masuk ke kunjungan yang sama"_. **Perilaku hari ini tidak begitu** —
form selalu MEMBUAT sesi baru, jadi tamu kedua tidak "masuk", ia gagal. Jadi ini
bukan hanya celah: dokumen dan program saling bertentangan.

**Kenapa tidak bisa saya putuskan sendiri** — setiap opsi punya konsekuensi
produk/keamanan, dan tiga di antaranya terhalang gap lain:

| Opsi                                                               | Bisa dikerjakan sekarang?                                   | Konsekuensi                                                                    |
| ------------------------------------------------------------------ | ----------------------------------------------------------- | ------------------------------------------------------------------------------ |
| **(a)** Tamu masuk ke sesi terbuka meja itu (sesuai dokumen)       | ✗ butuh `kind: Service` publik (10.39) untuk find-or-create | Perilaku yang sudah didokumentasikan; menyatukan bill satu meja                |
| **(b)** Sesi kedaluwarsa (`expires_at`) + penyapu                  | ✗ v1 tidak punya timer/`Schedule`                           | Meja bebas otomatis; butuh adamya lifecycle waktu                              |
| **(c)** Drop partial unique index 10.34c                           | ✓                                                           | Kehilangan penjagaan "satu sesi terbuka per meja"; kembali ke 4 sesi/4 request |
| **(d)** Tutup sesi yang menggantung saat meja di-`release`         | sebagian sudah                                              | Tidak menolong: meja `available` tidak bisa di-`release`                       |
| **(e)** Landing/check-in memberi tombol "sudah selesai" untuk tamu | ✗ route publik + identitas tamu                             | Menyerahkan pembersihan ke tamu                                                |

**Rekomendasi saya: (a)**, karena ia (i) memenuhi perilaku yang sudah
didokumentasikan, (ii) menghapus celah ini **dan** menjawab kebutuhan "tamu
kedua masuk ke bill yang sama" yang sudah ada di rencana pemilik, dan
(iii) tidak melemahkan penjagaan 10.34c. Harganya: **10.39 harus lebih dulu**
(Service publik), dan itu perubahan `pkg/spec` + validator.

**Kalau (a) dipilih**, urutannya: 10.39 (Service publik + deklarasi izin
anonim) → Service `table-open` find-or-create yang mengembalikan `guest_token`
→ Form `table-open-form` memanggilnya lewat `context.source: api` → redirect.
Meja tetap `available` sampai bayar (keputusan 2026-09-27 tidak berubah).

**Sementara belum diputuskan:** dengan B1 selesai, tamu kedua menerima
**409 dengan pesan yang benar** ("meja ini sedang punya kunjungan terbuka")
alih-alih 500. Itu jujur dan tidak menyesatkan, tetapi tetap berarti tamu tidak
bisa memesan — jadi B2 tidak boleh ditutup oleh B1.

## B3 — Handler subscription tidak bisa mendeklarasikan `uses` (kafe 10.56)

**Bukti (baca kode).**

- `SubscriptionSpec` (`pkg/spec/resources.go:651`) **tidak punya field `Uses`**.
- Dispatcher membuat `spec.Action` sintetis yang hanya mengisi `Name` + `Impl`
  (`internal/subscription/dispatch.go`), jadi `actionSpec.Uses` selalu nil.
- `checkPrimitive` dipanggil untuk **tepat 7** primitive: `db`, `cache`, `lock`,
  `queue`, `pubsub`, `storage`, `kvstore`. Di `ProdMode`/`StrictMode`, `uses
== nil` → `USES_VIOLATION` dengan pesan
  _"add uses.primitives: [db]"_ — **instruksi yang mustahil dituruti** pada
  subscription.

**Kenapa belum menggigit, dan kenapa tetap harus ditutup:** handler subscription
kafe memakai `resource.find`/`set`/`save` (bukan `ctx.*`), jadi tidak ada yang
terpengaruh. Tetapi `ctx.now()` — yang **saya pakai** di
`close_session_on_clear.star` — lolos **bukan karena aman**, melainkan karena
`builtinNow` tidak memanggil `checkPrimitive`. Jadi yang kita punya adalah
pemeriksaan yang **tidak konsisten antar-primitive**; begitu ada handler
subscription yang butuh `ctx.db`/`ctx.config`/`ctx.lock`, ia gagal di produksi
dengan pesan yang menunjuk konfigurasi yang tidak ada.

**Perbaikan yang direncanakan:**

1. `Uses *UsesDecl` pada `SubscriptionSpec`; teruskan ke `actionSpec` di
   `dispatchOne` (mengikuti pola action biasa).
2. `formspec check` memvalidasi `uses` subscription (agar typo tertangkap saat
   deploy, bukan saat produksi).
3. Putuskan **satu** aturan yang konsisten untuk primitive non-datastore
   (`now`/`today`/`config`/`log`): diperiksa juga, atau dinyatakan eksplisit
   "bebas". Hari ini statusnya tidak dinyatakan di mana pun — dan itulah bagian
   yang menipu. Rekomendasi: **tidak diperiksa, dan ditulis begitu di
   `docs/reference/primitives.md`**, karena ketiganya tidak menyentuh
   infrastruktur ber-batas (`now` hanya jam).
4. `make generate-schema` (perubahan `pkg/spec` → test
   `TestGeneratedSchemas_MatchOnDisk` gagal kalau lupa).

**✅ Yang benar-benar mendarat:**

1. `SubscriptionSpec.Uses *UsesDecl`, diteruskan ke `actionSpec` di `dispatchOne`
   dengan komentar yang menjelaskan mengapa nil membuat instruksi itu mustahil
   dituruti. Test: `TestDispatcher_CarriesUsesToTheHandler` (menegaskan `uses`
   sampai ke executor **lewat action**, bukan lewat params).
2. **Tidak dibangun, dan itu keputusan sadar.** Setelah diperiksa: skema
   `Subscription` yang digenerate sudah membawa `uses` bertipe (`$ref:
#/$defs/UsesDecl`), jadi **struktur** tervalidasi. Yang tidak bisa ditangkap
   schema adalah **typo nama primitive** (`uses.primitives: [dbb]`) — dan itu
   tetap tertangkap di runtime oleh `checkPrimitive` yang sama dengan action,
   karena ia membandingkan dengan daftar yang dideklarasikan. Menambah whitelist
   di `check` berarti **duplikasi set tertutup** yang bisa menyimpang dari
   `context.go` — persis kelas yang berkali-kali dicatat ledger. Dibutuhkan
   untuk memperbaiki, tidak.
3. **Asimetri didokumentasikan, bukan diubah.** Terverifikasi dari kode:
   `checkPrimitive` dipanggil untuk tepat 7 primitive datastore; `config`,
   `log`, `now`, `today`, `next_key`, `unit` tidak melewatinya. Ditulis sebagai
   tabel di `docs/reference/primitives.md`, dengan alasannya (yang diperiksa
   adalah yang menyentuh infrastruktur ber-batas) dan tabel "di mana `uses`
   boleh ditulis". Sekalian menambahkan `uses` deklaratif pada
   `cafe-master/table-occupancy` sebagai contoh nyata.
4. `make generate-schema` dijalankan; `schemas/kinds/Subscription.schema.json` +
   `schemas/formspec.schema.json` ikut berubah.

**Koreksi saat implementasi:** `uses` adalah **saudara** `handler`, bukan
anaknya. Percobaan pertama menaruhnya di dalam blok `handler` dan langsung
tertangkap validator (`schema: /spec/handler: validation failed`), karena
`handler` bertipe `ImplDecl` (type/ref saja). Dokumen di sini juga ikut
diperbaiki.

## B4 — vitest flaky: catat, jangan tebak

Diukur: 3 test gagal (`533/536`, 2 file) pada **satu** run, lalu **536/536
sembilan kali berturut-turut** (3 + 5 + 1 sesudah perubahan). Saya tidak tahu
test mana yang gagal — outputnya tidak tertangkap.

**Status: tidak dikejar tanpa bukti.** Menambal test yang tidak terbukti rusak
hanya menyembunyikan masalah. Yang akan dilakukan **kalau terulang**: jalankan
vitest dengan reporter yang menyimpan nama test (`--reporter=json
--outputFile`), lalu periksa test yang bergantung waktu (`setTimeout`, debounce
auto-save, `Date.now`) atau urutan. **Tidak ada perubahan kode untuk B4** di
gelombang ini.

**Catatan kejujuran:** karena gagalnya hanya 1 dari 10 run, suite ini **tidak
bisa** diklaim hijau secara deterministik. Yang bisa diklaim: sembilan run
terakhir hijau, penyebab run yang gagal tidak diketahui.

## Urutan kerja & dependensi

| #   | Langkah                                                                     | Depends   | Effort       |
| --- | --------------------------------------------------------------------------- | --------- | ------------ |
| 1   | B1: sentinel + klasifikasi di `jsonb-persist`                               | —         | small        |
| 2   | B1: mapping 409 + pesan berguna di `internal/api`                           | 1         | small        |
| 3   | B1: jadikan probe sebagai regression test (3 bentuk + 2 bentuk teks driver) | 2         | small        |
| 4   | B3: `Uses` pada `SubscriptionSpec` + teruskan + `check` + doc               | —         | small–medium |
| 5   | B3: `make generate-schema` + regenerate                                     | 4         | small        |
| 6   | **B2: keputusan pemilik dulu**                                              | keputusan | —            |
| 7   | B2 (bila (a)): 10.39 Service publik → Service find-or-create → form         | 6         | medium       |
| 8   | B4: instrumentasi reporter                                                  | —         | small        |

1–3 dan 4–5 bisa paralel. **6 menghentikan 7.**

## Verifikasi yang harus lulus

- `go test ./...` hijau; `formspec validate --spec examples/kafe/spec` 0 problem.
- Probe B1: ketiga bentuk → **409** (bukan 500), dengan kode `CONFLICT` dan pesan
  yang menyebut field/constraint.
- Test unit klasifikasi untuk teks driver SQLite **dan** PostgreSQL.
- `make e2e-kafe` tetap hijau (B1 menyentuh jalur yang dilewatinya).
- `gofmt -l` bersih; `npx tsc --noEmit` bersih; oxlint tetap 41 warning (baseline).
- Setelah B1: **perbarui ledger 10.57** supaya tidak lagi menyebut 500 — sisa
  B2 murni "meja tidak bisa dipakai", bukan "server rusak".

## Yang TIDAK dikerjakan di sini

- **10.41 / 10.52** (agregat lintas-record): tetap terhalang konstruk
  (`ctx.db` butuh SQL mentah ber-tenant; subscription tidak bisa `uses`).
  B3 memperbaiki satu dari dua penghalang itu, tetapi agregatnya sendiri belum.
- **10.38 / 10.39 / 10.20**: gap lain, bukan bug yang timbul. 10.39 menjadi
  prasyarat bila B2 memilih opsi (a).
- **Perubahan spec kafe**: tidak ada. Bug-bug ini di engine dan di perilaku,
  bukan di manifest.

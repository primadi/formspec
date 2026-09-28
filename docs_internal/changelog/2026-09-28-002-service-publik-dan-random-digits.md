# 2026-09-28-002 — Service action publik + `ctx.random_digits` (prasyarat kode gabung sesi)

**Apa:** dua kemampuan engine yang menjadi prasyarat fitur "tamu kedua JOIN ke
sesi meja lewat kode 4–6 digit" (keputusan pemilik; plan
`docs_internal/plan/kafe-join-session-kode.md`). Menutup dua gap yang selama ini
menghalangi: kafe **10.39** (Service tidak bisa publik) dan **10.38** (tidak ada
sumber acak). Fitur kafe-nya sendiri (P3–P6) belum dikerjakan.

Rencana disetujui sebelum implementasi, dan risetnya mengubah ukuran pekerjaan:
dua dari tiga bagian ternyata **sudah ada**, dan satu komentar di kode terbukti
**bertentangan dengan perilaku sebenarnya**.

## P1 — `public: true` pada Service action (kafe 10.39)

**Temuan yang mengecilkan pekerjaannya:** jalur route anonim sudah lengkap.
`registerRouteWithPattern` sudah punya cabang `rd.Public && rd.RequiredPermission`
→ `RequirePermissionOrAnonymous`, dan route Service (`/_ui/service/...`) sudah
lewat fungsi itu. Yang hilang hanya **mengisi flag-nya**: `GenerateUIServiceRoutes`
mengisi `RequiredPermission` (default `{module}.{service}.{action}`) tetapi tidak
pernah `Public`, sehingga setiap route Service session-only. Ledger menyebut
10.39 "medium"; kenyataannya small.

`RateLimitSpec` pada Service juga sudah ditegakkan (`HandleServiceAction`
memanggil `rateLimitFor`), jadi tidak ada pekerjaan rate limit yang baru.

**Yang dikerjakan:** `Action.Public bool` + `GenerateUIServiceRoutes` meneruskan
`Public` + validasi. Dua aturan keras, keduanya error (bukan peringatan) karena
kegagalannya adalah pintu terbuka, bukan ketidaknyamanan:

| Aturan                                        | Kenapa                                                                                                                                                                    |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `public: true` **wajib** `rate_limit`         | Endpoint anonim tanpa rate limit adalah vektor penyalahgunaan; hanya manifest yang bisa menyatakannya                                                                     |
| `public: true` **ditolak** pada entity action | Entity sudah punya jalur anonim sendiri (`public_entities` di App) — satu-satunya tempat operator meninjau keputusan itu. Flag per-action akan memutari tinjauan tersebut |

`public: true` pada entity diarahkan ke `public_entities` lewat pesan errornya.
Validasi dipasang di `Loader.Validate` (gate deploy `formspec validate`), tempat
`spec.ValidateEntitySpec` juga berada.

**Bukti:** `TestServiceAction_PublicActionIsAnonymousCallable` — pemanggil anonim
mendapat **200 dan handler-nya benar-benar jalan** (payload `code` kembali di
`data`), sementara `internal-only` pada Service yang **sama** ditolak untuk
anonim. Pasangan itu yang membuat flag-nya bermakna: tanpa yang kedua, solusi yang
sekadar melepas pemeriksaan untuk semua route Service akan lolos test pertama.

## Koreksi: komentar route bertentangan dengan perilaku middleware

Komentar di `registerRouteWithPattern` (dua tempat) berbunyi _"A public grant
authorizes anonymous callers; a signed-in caller on the same route still needs
the permission (#45)"_. Itu **salah**, dan
`RequirePermissionOrAnonymous`'s dokumen sendiri menyatakan sebaliknya: public
grant adalah **LANTAI, bukan jalur khusus anonim** — pemanggil yang sudah login
**tanpa** permission juga diizinkan, supaya login tidak membuat orang **lebih
buruk** daripada tamu. Perilaku itu sengaja dan terukur (kafe: `menu-category`
menjawab 200 untuk anonim dan **404** untuk permintaan yang sama dengan sesi).

Komentar itu menyesatkan saya sampai menulis assertion `403` yang salah. Sudah
diperbaiki di kedua situs, dan perilakunya kini dikunci
`TestServiceAction_PublicRouteDoesNotInvertSignedInCallers` — arah yang salah
justru yang tampak "lebih aman", jadi perlu dipin.

## P2 — `ctx.random_digits(n)` (kafe 10.38)

Starlark tidak punya sumber acak; satu-satunya generator bernilai adalah
`ctx.next_key`, yang merupakan **deret** — bisa ditebak. Memakainya untuk kode
gabung akan tampak benar dan bisa dipalsukan.

`ctx.random_digits(n)` memakai `crypto/rand` dengan **rejection sampling**
(bukan `byte % 10`, yang membiaskan digit 0–5 karena 256 bukan kelipatan 10 dan
menyusutkan ruang pencarian diam-diam). Batas 4–12: nilai 1–2 digit bukan rahasia
yang berarti, dan `n` tanpa batas berarti satu manifest bisa meminta string
sepanjang apa pun per panggilan.

Sengaja **tidak** diperiksa `uses.primitives` — konsisten dengan
`now`/`today`/`next_key`: yang dijaga pemeriksaan itu adalah akses ke
infrastruktur ber-batas, dan entropi OS bukan dependensi yang bisa dideklarasikan
manifest. Asimetri itu kini didokumentasikan sebagai tabel di
`docs/reference/primitives.md` (yang juga membedakan `random_digits` dari
`next_key` secara eksplisit).

**Test:** bentuk (panjang + hanya digit), tidak konstan (50 undian),
**distribusi rata** (5.000 digit, toleransi ±25% — inilah yang akan menangkap
regresi ke `% 10`), dan batas ditegakkan.

## Pelajaran yang dicatat

- **Script action harus `return ok({...})`.** `echo.star` pertama mengembalikan
  dict biasa dan hasilnya **200 OK dengan `data: nil`** — sukses yang tak
  membawa apa-apa. Tidak ada error, hanya nilai yang hilang.
- **Periksa apakah mesinnya sudah ada sebelum mempercayai estimasi ledger.**
  Dua dari tiga bagian P1 ternyata sudah terpasang; yang kurang hanya satu
  baris meneruskan flag.
- **Komentar yang bertentangan dengan kode harus diperbaiki, bukan diabaikan.**
  Komentar route di atas membuat saya menulis test yang menegaskan perilaku
  yang salah.

## Verifikasi

`go test ./...` hijau · `formspec validate` kafe 89 manifest/0 problem ·
`make e2e-kafe` 1 passed · `gofmt` bersih · `make generate-schema` dijalankan
(`Action.public` masuk skema).

## File

`pkg/spec/entity.go` (Action.Public) · `internal/api/generator.go` ·
`internal/api/router.go` (komentar) · `internal/permission/validator.go` ·
`internal/manifest/loader.go` · `internal/starlark/context.go` ·
`resource/service_public_e2e_test.go` · `internal/starlark/random_test.go` ·
`internal/permission/permission_test.go` · `docs/reference/primitives.md` ·
`schemas/`

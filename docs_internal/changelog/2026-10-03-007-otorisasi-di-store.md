# 2026-10-03-007 — Penegakan otorisasi di store: principal eksplisit + guard field-level berlaku di jalur script (kafe 10.71)

**Keputusan yang diambil.** Pemilik memilih: **store = lapisan penegakan
otoritatif, HTTP tetap sebagai lapisan gagal-cepat** (plan
`docs_internal/plan/lapisan-otorisasi.md`). Changelog ini mengerjakan
**langkah 1–2** dari plan itu: principal eksplisit, lalu penegakan field-level di
store — yang menutup **10.71** dan menyiapkan **10.46**.

**Bukti yang membuat keputusan ini murah:** pipanya sudah ada, dan **kedua** jalur
ber-identitas sudah mengopernya — HTTP (`handler.go`) dan script
(`resource/formspec.go`, `auth.PermissionsFromContext(ctx)`). Store bahkan sudah
membaca `Permissions` untuk override kebijakan tanggal. Yang hilang hanya
penegakan.

## 1. Principal eksplisit: `SystemCaller`

`InsertParams`/`UpdateParams` mendapat `SystemCaller bool` — **eksplisit, tidak
pernah disimpulkan**.

**Kenapa bukan "Permissions kosong = sistem".** Inferensi itu membuat setiap
situs yang lupa mengisi permissions menjadi **bypass istimewa tanpa suara** —
persis bentuk bug yang melahirkan 10.46/10.71. Dengan penanda eksplisit,
`grep SystemCaller` menjawab pertanyaan yang dibutuhkan audit: *"siapa menulis
tanpa pengguna?"*

**Semua penulis produksi kini menyatakan identitasnya** (diverifikasi dengan
skrip yang membandingkan jumlah literal `InsertParams{`/`UpdateParams{` terhadap
jumlah penanda — hasil: nol situs tak bertanda):
seed · backup restore · archive · registry vendor-activation · auth
(apikey/role/session/user/workspace) · `setActive` internal.

## 2. Guard field-level di store

`checkFieldWritePermissions` + `ErrForbidden`/`ForbiddenError`
(`renderers/jsonb-persist/crud.go`), dipanggil di awal `Insert` dan `Update` —
**sebelum** default diterapkan, karena default menambahkan field yang tidak
dikirim pemanggil dan itu nilai framework, bukan niat mereka.

**Hanya NIAT yang dinilai:** field yang ada di payload = field yang coba diset.
Nilai yang sudah tersimpan tidak dihitung, sehingga PATCH yang tidak menyebut
field tersebut tetap lolos — diuji.

### ⚠️ Dua cacat nyata yang ikut terangkat

1. **`resource.create` adalah satu-satunya jalur tulis script TANPA identitas.**
   `SetSaveHandler` mengoper `PermissionsFromContext`, tetapi `SetCreateHandler`
   tidak — sehingga (setelah guard ini) sebuah field bergerbang akan **ditolak
   salah** (false denial) bagi pemanggil yang justru memegang permission. Kini
   mengoper permissions yang sama. Ditemukan bukan dengan membaca, melainkan
   dengan skrip audit di atas.
2. **Sentinel tidak selamat melintasi batas script.** Script gagal dikirim
   sebagai **string** (`internal/action/script.go` — `result.Error`), jadi
   `errors.Is(err, db.ErrForbidden)` **false** saat sampai di lapisan API, dan
   penolakan dari `resource.save()` dijawab **500 `ACTION_ERROR`** padahal
   seharusnya 403. Terukur sebelum diperbaiki:
   `500 ACTION_ERROR … script failed: resource.save: permission required to set
   field(s): secret`. Perbaikan: marker pesan yang **dibagi kedua lapisan**
   (`db.ForbiddenMarker`) + `isStoreForbidden` yang memeriksa sentinel **atau**
   marker. Kalau klasifikasi meleset, tulisannya **tetap ditolak** — hanya
   dilaporkan sebagai 500 — jadi mode gagalnya tetap fail-closed.

## Bukti

- **Store (unit)** `renderers/jsonb-persist/field_write_permission_test.go` — 7
  subtest: insert tanpa permission ditolak (pesan menyebut field) · dengan
  permission lolos · wildcard `*` lolos · tidak mengirim field bukan pelanggaran
  · update tanpa permission ditolak **dan nilai lama utuh** · update field lain
  tidak tersandung · **`SystemCaller` mem-bypass, dan payload yang sama TANPA
  penanda tetap ditolak** (bypass adalah keputusan, bukan kecerobohan).
- **Script (e2e)** `resource/field_permission_script_e2e_test.go` — aksi Starlark
  nyata (`resource.save` → Update, `resource.create` → Insert) atas fixture
  ber-`required_permission`: **403 tanpa permission**, sukses dengan permission,
  nilai tersimpan diperiksa sebagai admin.
- **HTTP (e2e)** `resource/field_permission_write_e2e_test.go` (dari
  `2026-10-03-006`) tetap hijau — jalur HTTP masih 403.
- **Kalibrasi:** guard `Insert` dihapus → test store gagal (`expected
  ErrForbidden, got <nil>`); guard `Update` dihapus → **kedua** subtest script
  gagal dengan **200** (tulisannya benar-benar lolos, bukan sekadar salah
  status). Dipulihkan dari backup `/tmp` (bukan `git checkout`).
- `go test -count=1 ./...` hijau · `gofmt` bersih.

## Dampak & sisa

**Dampak ke kafe: nol**, dan itu diperiksa: seluruh `required_permission` kafe ada
di action, tidak satu pun di field. Verifikasi dev server dijalankan ulang pada DB
bersih (create QR → derivasi nilai, jurnal, sortir) tanpa regresi.

**10.71 ✅ ditutup.** **10.46 tetap ⏸️** dan analisisnya diperbarui: mekanismenya
kini ada dan terbukti (guard di store + principal eksplisit), tetapi gerbang
**transisi** belum dipindah — dan hambatannya spesifik: `SetSaveHandler` dipakai
**bersama** oleh script aksi HTTP **dan** oleh subscription/worker, jadi menandai
"sistem" harus memisahkan konteks keduanya lebih dulu. Itu pekerjaan tersendiri,
bukan satu baris.

**Dampak.** `renderers/jsonb-persist/crud.go` · `internal/api/handler.go` ·
`resource/formspec.go` · `cmd/formspec/{seed,backup,archive}.go` ·
`cmd/formspec-registry/main.go` · `internal/auth/{apikey,role,session,user,workspace}.go`
· test baru `renderers/jsonb-persist/field_write_permission_test.go`,
`resource/field_permission_script_e2e_test.go` · plan
`docs_internal/plan/lapisan-otorisasi.md` · ledger kafe.

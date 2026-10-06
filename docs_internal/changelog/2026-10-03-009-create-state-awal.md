# 2026-10-03-009 — Create harus lahir di state awal (kafe 10.72)

**Cacat yang diukur.** `POST` (create) menerima **state apa pun yang sah**,
bukan hanya state awal. Pada dev server kafe (kasir memegang `orders.create`):

| `POST order` | Sebelum | Sesudah |
| --- | --- | --- |
| tanpa `status` | 201, `status=draft` | 201, `draft` |
| `status: "paid"` | **201, tersimpan `paid`** | **422** |
| `status: "ready"` | **201, tersimpan `ready`** | **422** |
| `status: "ngawur"` | 422 — oleh **CHECK enum**, bukan state machine | **422** dengan pesan state yang benar |

**Kenapa baris kedua berbahaya, bukan sekadar tidak rapi.** Sebuah record yang
lahir di tengah siklus menembus **dua** hal sekaligus, dan keduanya senyap:

1. **Gerbang permission per-transisi** — hanya dijalankan pada `Update`.
2. **`emit:` milik transisi** — emission diselesaikan dari **perubahan state**
   pada jalur update. Order yang lahir `paid` karena itu **tidak menerbitkan
   `on_paid`**, sehingga jurnal GL, jembatan okupansi meja, dan setiap konsumen
   lain tidak pernah mendengarnya — padahal record-nya tampak sah sepenuhnya.

Sebabnya: `applyDefaults` mengisi state awal hanya bila field **kosong**, dan
`validateStateTransition` hanya dipanggil di `Update`. Jadi `Insert` tidak pernah
memeriksa state.

## Perbaikan

Aturan di `Insert` (`renderers/jsonb-persist/crud.go`), setelah `applyDefaults`
(yang membuat "field absen" berarti "state awal", bukan "entah"):

- `s.stateMachine != nil && !SystemCaller` → state **wajib** sama dengan
  `state_machine.initial`, kalau tidak `422` dengan pesan yang menyebut **kedua**
  state.
- **`SystemCaller` boleh state apa pun** — seed, restore, dan migrasi
  mereproduksi baris tersimpan apa adanya, operasi yang berbeda dari pemanggil
  yang membuat record baru. Marker ini sudah ada dari `2026-10-03-007/008`, jadi
  tidak ada mekanisme baru.

## Dampak ke test (dan mengapa itu bagus)

Menegakkan aturan ini membuat **5 test di `internal/api` gagal** — semuanya
fixture yang menyemai record di state akhir (`{"status": "posted"}`). Perbaikannya
adalah **menandai situs itu `SystemCaller: true`**, bukan melonggarkan aturannya.
Itu memang tujuannya: sebelum ini "menulis state akhir saat seeding" adalah
kebetulan yang tak terlihat; sekarang ia pernyataan.

## Bukti

- **Store (unit)** `renderers/jsonb-persist/create_initial_state_test.go` —
  6 subtest: tanpa state → state awal · mengirim state awal → boleh · **state sah
  tapi bukan awal → ditolak** (pesan menyebut kedua state) · state berikutnya
  tetap terjangkau lewat update · **`SystemCaller` boleh state apa pun** · entity
  tanpa state machine tidak terpengaruh.
- **Kafe (e2e)** `resource/create_initial_state_e2e_test.go` — create tanpa
  status → `draft` · **`paid` ditolak dan dibuktikan TIDAK ada order ber-status
  bukan-draft tertulis** (penolakan terjadi sebelum tulis, bukan dilaporkan
  sesudah) · jalur normal `draft → awaiting_payment → paid` tetap bekerja.
- **Kalibrasi:** aturan di-`if false`-kan → test store gagal (`expected a
  validation error … got <nil>`) **dan** e2e kafe gagal dengan **201** plus
  record lengkap ber-`status:paid`. Dipulihkan dari backup `/tmp`.
- **Dev server (DB bersih):** ketiga varian (`paid`, `ready`, `ngawur`) → **422**
  dengan pesan `create must start in the initial state "draft", got …`; tanpa
  status → `draft`. Rantai nilai/jurnal/sortir tidak berubah; `make seed-kafe`
  tetap bekerja (seed menulis state akhir dengan benar).
- `go test -count=1 ./...` **39 paket ok** · `gofmt` bersih · `validate` 89/0.

## Catatan dokumen

`docs/spec/backend/01-core-basic.md` §1.6 mendefinisikan blok `state_machine`
tetapi **tidak pernah menyatakan** bahwa create harus mulai dari `initial`.
Aturan itu kini ditegakkan, jadi ia ditambahkan ke §1.6 — spec adalah kontrak
normatif dan sebuah penegakan baru tanpa kalimat kontrak akan menjadi aturan
yang hanya bisa ditemukan dengan membaca kode.

**Dampak.** `renderers/jsonb-persist/crud.go` · `docs/spec/backend/01-core-basic.md`
· test baru `renderers/jsonb-persist/create_initial_state_test.go`,
`resource/create_initial_state_e2e_test.go` · fixture `internal/api/*_test.go`
(5 situs ditandai sistem) · ledger kafe (10.72 ✅).

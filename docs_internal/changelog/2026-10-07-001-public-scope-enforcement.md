# 2026-10-07-001 — Penegakan scope permukaan publik: `scope.enforced` + `create_scope`

**Plan:** `docs_internal/plan/public-scope-enforcement.md`
**Konteks:** pertanyaan "untuk page tanpa login (kafe-qr), kode cabang harusnya
ditentukan admin?" — jawabannya **tidak**: cabang sudah datang dari rantai
`qr_token → dining-table.branch_id → table-session.branch_id`. Yang diperbaiki
adalah **penegakannya** atas dua lubang nyata.

## Yang ditemukan

1. **Create anonim tanpa write scope** (baru, tidak terlacak). `HandleCreate`
   (`internal/api/handler.go`) memanggil `store.Insert(db.InsertParams{…})` tanpa
   predikat apa pun — `InsertParams` memang tidak punya field-nya. Penjaga yang
   ADA hanya `denyForbiddenFieldWrites` (field ber-`required_permission`) dan
   state machine (`initial`). `order.branch_id` tidak punya
   `required_permission`, jadi **tamu anonim dari QR bisa mem-POST pesanan
   dengan `branch_id` cabang lain** (dan `table-session` di cabang lain).
   Dampak: polusi data antar-cabang, dibatasi rate limit `30/1m per ip`.
2. **`scope` deklaratif tanpa pemilik penegakan.** Dimensi dideklarasikan
   (`scope: {dimension: branch, field: branch_id}`) sementara penegakannya hidup
   di tempat lain — atau tidak sama sekali. Membaca manifest tidak bisa
   menjawab "siapa yang menjaga pembaca tetap di cabangnya?", sehingga
   satu-satunya jawaban yang bisa dipercaya adalah "audit kodenya". Ini kelas
   yang sama dengan **kafe 10.76 ⏸️**.
3. **Bom waktu test** (ditemukan saat menjalankan suite):
   `TestKafe_CreateCannotBeBornPaid` (`resource/create_initial_state_e2e_test.go`)
   memakai tanggal literal `2026-10-03`, sehingga sejak **2026-10-07** gagal
   permanen dengan `FORMSPEC.TXN.BACKDATE_EXCEEDED` (batas default 3 hari).
   Sudah ada helper `recentDate()` di paket yang sama.

## Yang dikerjakan

### A. `scope.enforced` — siapa yang menegakkan dimensi

`ScopeDecl.Enforced` (`pkg/spec/entity.go`) dengan enum
`session` | `route` | `none` | `external`; absen = `session`.
`ValidateScopeEnforcement` mengikat nilai itu ke deklarasi yang diklaimnya:

| nilai               | validator                                                                 |
| ------------------- | ------------------------------------------------------------------------- |
| `session` (default) | wajib ada `row_scope` pada field itu dengan `from: session`               |
| `route`             | wajib `from: route` — `from: session` ditolak                             |
| `none`              | pengecualian eksplisit; `row_scope` yang ada justru ditolak (kontradiksi) |
| `external`          | pengecualian eksplisit (ditegakkan grant publik / `create_scope`)         |

Literal (`value:` tanpa `from`) juga ditolak untuk `session`/`route`: konstanta
tidak bisa membawa dimensi milik pemanggil. Pesan errornya menyebut nama field
DAN nilai yang diharapkan, karena inilah aturan yang paling mudah salah tulis.

Divalidasi **hanya di manifest** (bukan runtime): tidak ada flag day dan tidak
ada perubahan perilaku diam-diam.

Adopsi kafe (4 entity yang sah tanpa `row_scope`) — masing-masing diberi alasan
di manifest: `table-session`/`dining-table`/`menu-item-price` → `external`,
`promo` → `none` (promo global memang lintas cabang).

### B. `create_scope` — pasangan `row_scope` di jalur tulis

`EntitySpec.CreateScope` (`pkg/spec/entity.go`), `CreateScopeSpec`:

```yaml
create_scope:
  - {
      field: branch_id,
      from: record,
      ref_field: table_session_id,
      via: cafe-order.table-session,
      via_field: branch_id,
    }
```

- Aturannya **kondisional pada rujukannya**: diperiksa hanya bila payload
  membawa `ref_field`. Pesanan kasir (walk-in/takeaway) tidak punya sesi meja,
  jadi tidak ada record untuk menurunkan cabangnya — aturan ini tidak boleh
  berubah menjadi "setiap create wajib punya sesi meja".
- **Mismatch ditolak (403)**, tidak ditimpa: menimpa diam-diam membuat
  permintaan dan baris tersimpan tidak sepakat tanpa cara bagi pemanggil untuk
  mengetahuinya.
- **Referensi tak ter-resolve = 422** (`VALIDATION_ERROR`), bukan 403 — lihat
  "Regresi yang ditemukan" di bawah.
- `to = field` diambil dari `via.via_field`; `via` menerima `module.entity`
  maupun `module/entity`.

Pengecualian `via: session` (baca lewat lookup `session_id`) **belum** dikerjakan
— itu fase C plan, dan **10.76 masih terbuka** karena anonim masih bisa membaca
`menu-item-price` semua cabang. `create_scope` memakai resolver yang sama, jadi
fase C tinggal memasang deklarasinya.

Adopsi kafe: `order` (dari `table_session_id`) dan `table-session` (dari
`dining_table_id`) — dua langkah alur QR, keduanya lubang yang sama.

## Bukti

**Unit (`internal/api/create_scope_test.go`, 7 test)** — fixture memakai tree
kafe NYATA + spec dari registry (spec buatan tangan bisa lulus sementara manifest
tetap salah, dan itu mode gagal yang penting di sini):

- deklarasi manifest terbaca benar untuk `order` dan `table-session`
- branch B pada sesi cabang A → **403**, pesan menyebut KEDUA nilai
- branch yang cocok → diterima
- dimensi yang di-OMIT → diterima (aturan soal pertentangan, bukan ketiadaan)
- tanpa rujukan (`channel: cashier`) → diterima (pengecualian sengaja)
- referensi menggantung → **422**, fail closed

**E2E HTTP (`resource/create_scope_e2e_test.go`, 3 test)** — menguji WIRING,
bukan hanya aturannya:

| langkah                                | hasil                                                |
| -------------------------------------- | ---------------------------------------------------- |
| `POST table-session` klaim cabang lain | **403**, dan dibuktikan **tidak ada** baris tertulis |
| `POST table-session` cabang mejanya    | **201**                                              |
| `POST order` klaim cabang lain         | **403**                                              |
| `POST order` cabang sesinya            | **201**                                              |
| rujukan menggantung                    | **422** (bukan 403)                                  |

**Kalibrasi (tiga lapis, semuanya dibuktikan MERAH lebih dulu):**

1. hapus `create_scope:` dari `order/entity.yaml` →
   `TestCreateScope_ManifestDeclaresItForOrder` gagal
2. `return nil` di awal `enforceCreateScope` → test mismatch + dangling gagal
3. ganti pemanggilan guard di handler dengan lambda no-op → **e2e** gagal
   (membuktikan test e2e menguji sambungan, bukan cuma aturannya)

**Suite:** `go test ./...` hijau · `golangci-lint run ./...` **0 issues** ·
`bin/formspec validate --spec examples/kafe/spec --schema schemas` **88/0** ·
`make generate-schema` + `make generate-kind-docs` (173 shared defs; `CreateScopeSpec`
ditambahkan ke `sharedTypes` — tanpa itu setiap Entity schema menunjuk definisi
yang tidak ada).

## Regresi yang ditemukan & diperbaiki

`TestKafe_ClientFaults_BrokenRelationIsStillValidationError` (`resource/`) MERAH
setelah `create_scope` dipasang: rujukan rusak (`dining_table_id` nol) mulai
dijawab **403** oleh guard, padahal kelasnya adalah **input buruk** — 422.
Perbaikannya bukan melonggarkan test, melainkan **memisahkan dua kelas**:

- **mismatch** = penolakan otorisasi → **403** (pemanggil minta menulis di luar
  dimensinya)
- **rujukan tak ter-resolve** = input buruk → **422** (pemanggil boleh, payload
  yang salah)

`enforceCreateScope` kini mengembalikan `(status, error)`, dan handler memilih
kode error dari status itu. Test e2e #3 mengunci klasifikasi ini.

## Sisa ⏸️

- **`via` untuk `row_scope` (fase C)** — penutup **kafe 10.76**; anonim masih
  bisa membaca harga semua cabang.
- **Validasi harga baris pesanan** — `order.lines[].unit_price_snapshot` diterima
  apa adanya dari klien (`line_total` adalah `computed`), jadi pesanan bisa
  dikarang nilainya. Satu keluarga dengan 10.76, butir baru.
- **Pengalih konteks di `UserMenu`** — tetap todo **6.5.10 ⏸️**.

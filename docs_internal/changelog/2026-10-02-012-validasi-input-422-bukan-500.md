# 2026-10-02-012 — Kesalahan input pemanggil dijawab 422 + nama field, bukan 500 (kafe 10.61)

**Apa yang diubah.** Tiga kelas kesalahan yang sepenuhnya milik pemanggil
dipetakan ke `422 VALIDATION_ERROR` dengan `details[].field`, dari sebelumnya
`500 INTERNAL_ERROR` yang membawa teks driver mentah:

| Masukan                               | Sebelum                                               | Sesudah                              |
| ------------------------------------- | ----------------------------------------------------- | ------------------------------------ |
| field tak dikenal (`zzz`)             | 500 `unknown field: "zzz"`                            | 422, `details[].field=zzz`           |
| tanggal tak bisa diurai (`"kemarin"`) | 500 `cannot parse … as date`                          | 422, `field=transaction_date`        |
| nilai enum di luar `enum_values`      | 500 `CHECK constraint failed: json_extract(…) IN (…)` | 422, `field=status`, allowed disebut |

Perubahan:

- `renderers/jsonb-persist/constraint.go` — `classifyConstraintError` kini
  mengenali **dua** kelas klien, bukan hanya uniqueness:
  `EnumViolationError` (`ErrInvalidEnumValue`) mem-parse ekspresi CHECK yang
  dihasilkan `GenerateDDL`, mengekstrak nama field + himpunan `allowed`; bentuk
  PostgreSQL (`violates check constraint "<name>"`) tetap terklasifikasi tanpa
  field. Stripping extended result code hanya menerima sufiks **numerik**
  (` (275)`) supaya tanda kurung milik ekspresi `IN ('a', 'b')` tidak memotong
  himpunan.
- `renderers/jsonb-persist/crud.go` — `ErrUnknownField`/`ErrInvalidFieldValue`
  jadi bertipe (`UnknownFieldError`, `InvalidFieldValueError`) dan membawa nama
  field. **`validateKnownFields` kini deterministik:** dulu ia mengembalikan key
  pertama hasil iterasi map Go, sehingga dengan dua typo field yang dilaporkan
  **berubah antar-panggilan** (teramati saat memverifikasi fix ini). Sekarang
  seluruh key tak dikenal dikumpulkan, diurutkan, dan dilaporkan semuanya.
- `renderers/jsonb-persist/transaction_date.go` — parse gagal →
  `InvalidFieldValueError{Field: "transaction_date"}`.
- `internal/api/handler.go` — `writeStoreError` menambah satu braket untuk
  ketiga sentinel → `writeInvalidFieldError` (422; satu entri `details[]` per
  field tak dikenal).

**Kenapa.** Kafe 10.61, ditemukan saat walkthrough 9.4 skenario 5 pada DB seed
baru. Dua dari tiga kelas itu **melanggar kontrak tertulis**:
`docs/spec/backend/01-core-basic.md` §"Unknown field = rejection (normatif)"
mewajibkan `VALIDATION_ERROR` (422), dan `docs/spec/backend/05-field-types.md`
§`enum` mewajibkan 422 untuk nilai di luar himpunan. 500 adalah kelas yang
memanggil operator, jadi setiap typo pengguna menaikkan alarm palsu dan
menenggelamkan kegagalan nyata. Ini kelas yang sama dengan perbaikan
`2026-09-28-001` (uniqueness → 409), dengan argumen yang sama: klasifikasi hidup
di **batas penyimpanan** supaya HTTP, script, seed, dan operator mendapat kelas
yang sama.

**Koreksi klaim.** Entri todo semula menulis SPA menampilkan toast "Internal
server error". Itu **salah** — `FormRenderer` mem-`toast.error(err.message)`,
dan `message` berasal dari server, jadi yang dilihat pengguna untuk kasus enum
adalah teks driver mentah (`CHECK constraint failed: json_extract(…) IN (…)`).
Koreksinya masuk ke entri 10.61.

**Bukti.**

- Unit `renderers/jsonb-persist/constraint_enum_test.go` — 7 test: bentuk SQLite
  dengan/tanpa extended code, bentuk PostgreSQL, uniqueness ≠ enum, FK bukan
  keduanya, pesan menyebut field+allowed tanpa kebocoran teks driver, idempoten,
  dan pass-through tak berubah untuk error tak terkait.
- e2e `resource/validation_status_e2e_test.go` — 4 test lewat HTTP nyata: tiga
  kelas → 422 + `details[].field` benar; permintaan **valid tetap 201**;
  **determinisme** (8 percobaan dengan dua typo → himpunan sama, terurut);
  relasi rusak tetap 422 `VALIDATION_ERROR`.
- **Guard terkalibrasi:** braket pemetaan dihapus dari `writeStoreError` →
  ketiga subtest gagal **500** (pesan sudah benar, kelas salah), hijau setelah
  dipulihkan dari backup `/tmp` (bukan `git checkout`).
- `go test ./...` hijau · `gofmt -l`/`go vet` bersih · verifikasi ulang pada dev
  server yang di-restart dari `bin/formspec` hasil build: `zzz`→422/`zzz`,
  `area: kolong`→422/`area`, `kemarin`→422, valid→201, duplikat tetap 409.

**Dampak.** `renderers/jsonb-persist/constraint.go`, `crud.go`,
`transaction_date.go` · `internal/api/handler.go` · test baru
`renderers/jsonb-persist/constraint_enum_test.go`,
`resource/validation_status_e2e_test.go` · `examples/kafe/gaps_found/TODO.md`
(10.61 ✅; sisa PostgreSQL tanpa field dicatat) · temuan klien ikut tercatat
sebagai **10.64 ⏸️** (tipe `ErrorDetail` di TS tidak cocok dengan wire:
`code` wajib padahal tidak dikirim, `level` selalu dikirim tetapi tidak ada di
tipe). Tidak ada perubahan pada klien.

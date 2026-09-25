# 2026-09-22-002 — Detail error validasi terstruktur (level + field) (todo 7.9.5)

**Plan/Todo**: item **7.9.5** (`docs_internal/plan/todo.md`).

Envelope error normatif mensyaratkan `details: [{level, field?, message}]`
(`01-core-basic.md` §8.5; `02-core-extended.md` §14 untuk makna `level`).
Kenyataannya `Level` **hardcoded `"error"`** dan `Field` tidak pernah diisi,
dan jalur validasi record (`writeStoreError` → `isValidationError`) memakai
`writeError` polos — jadi **tidak mengirim array `details` sama sekali**, padahal
di situlah mayoritas kegagalan rule field (`required`, `min_length`, `pattern`,
`after:*`, `unique`, …) muncul. Klien tidak punya cara memetakan error ke field.

Ditutup dengan dua bagian:

1. **Tipe error terstruktur** — `validation.ValidationError{Level, Field,
   Message, Cause}` (`internal/validation/error.go`) plus konstruktor
   `fieldError`/`crossFieldError` dan kosakata level dari `02-core-extended.md`
   §14 (`field`, `cross_field`, `business_rules`, `cross_validate`,
   `consistency` — L4–L6 didefinisikan sekarang agar envelope tidak berubah saat
   item 7.9.1–7.9.4 mendarat). `ValidateCrossField` (+`compareDateTime`),
   `checkExists`, `applyInlineRule` (27 cabang), dan rule `required` kini
   mengembalikan tipe ini alih-alih `fmt.Errorf` telanjang.
   `Unwrap()` memaparkan `Cause` agar `errors.Is` tetap mencapai error dasar
   (mis. lookup `exists` yang gagal) tanpa memakai `%w` di `Message`.
2. **Propagasi ke envelope** — `writeValidationErrors` membaca tipe itu lewat
   `errors.As` dan mengisi `details[].level/field`; error tak-terstruktur tetap
   menghasilkan entri dengan `level:"error"` (degradasi hormat, bukan entri
   hilang). `writeStoreError` memanggil `writeStoreValidationError` baru untuk
   jalur validasi, sehingga 422 record kini membawa `details`.

**File terkena dampak**: `internal/validation/error.go` (baru),
`internal/validation/validator.go`, `internal/api/handler.go`,
`internal/validation/validation_test.go`, `internal/api/validation_details_test.go`
(baru).

**Bukti**: `TestTypedErrors_CarryLevelAndField`; `TestWriteValidationErrors_TypedLevelAndField`,
`TestWriteValidationErrors_UnstructuredDegradesGracefully`, `TestStoreValidationDetail_Levels`
(4 sub-kasus), `TestWriteStoreValidationError_HasDetails`. `go test ./...` hijau; `go vet ./...` bersih.

**Sisa** → item **7.9.5** baris `⏸️` di todo: `Field` untuk
`db.ErrValidationRequired`/`db.ErrImmutableFieldChanged` belum terisi — pesannya
menyebut nama field tetapi tidak membawanya secara struktural, jadi detailnya
level-only (tidak menebak field dengan mem-parsing string).

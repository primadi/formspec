# Fix: Backdate Policy Violation Returns 500 Instead of 422

## Perubahan
`internal/api/handler.go` — `writeStoreError()` sekarang mengenali `TransactionDatePolicyError` sebagai validation error dan mengembalikan HTTP 422 (VALIDATION_ERROR) bukan 500 (INTERNAL_ERROR).

## Penyebab
* `validateTransactionDatePolicy()` di `renderers/jsonbpersist/transaction_date.go` mengembalikan `*TransactionDatePolicyError`
* Error handler `writeStoreError()` hanya mengecek `ErrValidationRule`, `ErrValidationRequired`, dan `ErrImmutableFieldChanged` via `isValidationError()` — ketiganya tidak mencakup `TransactionDatePolicyError`
* Akibatnya error jatuh ke `default` case → HTTP 500

## Dampak
* Auto-save pada form (two_step_autosave) yang mentrigger PATCH pada record dengan `transaction_date` melampaui backdate limit menghasilkan 500 error di browser
* Sekarang mengembalikan 422 dengan pesan jelas: `[FORMSPEC.TXN.BACKDATE_EXCEEDED] transaction_date ... exceeds backdate limit`

## File Terkena Dampak
- `internal/api/handler.go` — tambahan `errors.As` untuk `*db.TransactionDatePolicyError`

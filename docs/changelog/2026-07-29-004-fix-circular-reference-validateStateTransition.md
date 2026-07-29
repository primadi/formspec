# Fix: circular reference in `validateStateTransition` menyebabkan timeout pada custom action

## Perubahan

- **File**: `renderers/jsonbpersist/crud.go`
- **Bug**: `validateStateTransition` membuat circular reference dengan `env["resource"] = env` dan `env["data"] = env`, menyebabkan infinite recursion di `toStarlark()`.
- **Fix**: Pisahkan `combined` map (data lama+baru) dari `env` (yang dikirim ke Starlark). `resource`/`data` sekarang mengarah ke `combined`, bukan ke `env` sendiri.
- **Pola yang benar** sudah ada di `internal/entity/state_machine.go:evaluateGuard`.

## Dampak

Setiap custom action yang memiliki state machine guard expression (seperti `recall` pada Visit) akan hang selamanya — bukan timeout jaringan, tapi infinite recursion di Go stack.

## Files affected
- `renderers/jsonbpersist/crud.go` — `validateStateTransition()`: env map structure fix

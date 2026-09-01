# 2026-08-31-012 — Username validation + camelCase initials

## Apa

Dua perbaikan lanjutan identitas user:

1. **Validasi format username di Register** (`internal/auth/service.go`):
   - Fungsi baru `ValidateUsername` + error `ErrInvalidUsername` (wrapped,
     dicek via `errors.Is`).
   - Aturan: 3–32 karakter, huruf (a-z, A-Z), digit, `. _ -`. Tanpa spasi
     atau simbol lain — username muncul di URL, CLI flag (`--vendor`), dan
     lookup-by-username.
   - Hanya pendaftaran baru yang divalidasi; akun existing tidak terdampak.
   - `internal/api/auth_handler.go`: `ErrInvalidUsername` → HTTP 400
     `INVALID_USERNAME`.
2. **Initials camelCase-aware** (`renderers/react-shadcn/src/shell/UserMenu.tsx`):
   `initialsOf` kini menyisipkan pemisah di boundary camelCase
   (`TestUser` → `T U`) sebelum split, sehingga avatar menampilkan `TU`
   bukan `TE`.

## Verifikasi

- `go build ./...` bersih; `go test ./internal/auth ./internal/api` 194 pass.
- `tsc --noEmit` bersih; `make build-registry` sukses.
- curl register `"Budi Santoso"` → 400 `INVALID_USERNAME` (ditolak).
- curl register `TestUser` → 201 created (diterima).
- Browser: login TestUser → avatar **TU**.

## File terdampak

- `internal/auth/service.go`
- `internal/api/auth_handler.go`
- `renderers/react-shadcn/src/shell/UserMenu.tsx`

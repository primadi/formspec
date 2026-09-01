# 2026-08-31-011 — UserMenu: username asli, cursor pointer, menu Profile

## Apa

Tiga perbaikan lanjutan UserMenu (lanjutan changelog 010):

1. **Avatar menampilkan username asli** (bukan "DE" dev-bypass):
   - `internal/auth/token.go`: access claim baru `username` (di-issue dari
     `User.Username` saat login).
   - `internal/auth/jwt.go`: JWT validator membaca claim `username` →
     `Identity.Username`.
   - `internal/auth/auth.go`: field `Username` di struct `Identity`.
   - `internal/api/meta.go`: `/_meta/me` expose `username`.
   - `renderers/react-shadcn/src/types/manifest.ts`: `MeResponse.username?`.
   - `UserMenu.tsx`: label = `me.username || me.user_id`.
2. **Cursor pointer** pada trigger avatar (`cursor-pointer` class) —
   sebelumnya default arrow sehingga tidak terlihat clickable.
3. **Menu item Profile** — klik → navigasi ke `/{ws}/_admin` (surface
   admin tempat user profile dikelola), di atas Sign out.

## Akar masalah "DE / admin developer"

Registry binary (`cmd/formspec-registry`) tidak punya flag `--dev-auth`,
sehingga dev mode selalu memakai `DevValidator` — identitas sintetis
`user_id: "developer", roles: ["admin"], permissions: ["*"]` — apapun
yang di-login user. Fix:

- `cmd/formspec-registry/main.go`: flag `--dev-auth` baru, di-wire ke
  `formspec.Config.DevAuth`.
- Jalankan: `formspec-registry --dev-auth` (opsional `--jwt-secret` agar
  token survive restart).

## Verifikasi

- `go build ./...` bersih; `go test ./internal/auth ./internal/api` 194 pass.
- `tsc --noEmit` bersih; `make build-registry` sukses.
- Browser (login testuser): avatar "TE", cursor pointer, dropdown
  "testuser / Profile / Sign out", Profile → `/default/_admin`.

## File terdampak

- `internal/auth/token.go`, `internal/auth/jwt.go`, `internal/auth/auth.go`
- `internal/api/meta.go`
- `cmd/formspec-registry/main.go`
- `renderers/react-shadcn/src/types/manifest.ts`
- `renderers/react-shadcn/src/shell/UserMenu.tsx`

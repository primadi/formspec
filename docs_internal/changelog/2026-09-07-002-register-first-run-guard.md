# 2026-09-07-002 — Guard First-Run: Register Ditolak Sebelum Setup Selesai

**Plan**: session plan (unify dev/prod auth — follow-up bug loop setup)

## Apa

Bug: di workspace **kosong** (belum ada user), halaman login menampilkan link
"Sign up" → register membuat user **non-admin** → `SetupRequired` menjadi
false → setup wizard terkunci (409 `SETUP_COMPLETE`) dan tidak ada admin →
`_meta/ui?admin=true` selalu 403 → SPA bolak-balik login/Access Denied +
websocket `_ws` reconnect terus ("layar error looping"). Workspace bricked.

Perbaikan:

- Backend: `auth.Register` menolak saat workspace belum punya user — error
  baru `ErrSetupRequired` → handler memetakan ke 403 `SETUP_REQUIRED`
  (`internal/auth/service.go`, `internal/api/auth_handler.go`). Akun pertama
  hanya bisa dibuat lewat setup wizard.
- Frontend `LoginScreen`: submit register yang kena 403 `SETUP_REQUIRED`
  mengarahkan ke `/{ws}/_admin/setup` (bukan error mati).
- Frontend `SurfaceShell` (App.tsx): route `/register` saat
  `setup_required && !token` → redirect ke setup; route register/login saat
  sudah ter-autentikasi → bounce ke surface root (sebelumnya register route
  menampilkan form lagi ke user yang sudah login — kasus returnTo
  `.../register` setelah setup).

## Kenapa

User melaporkan "create admin user, layar error looping" di DB baru: yang
terjadi adalah register (bukan setup) membuat user pertama non-admin sehingga
workspace terkunci tanpa admin.

## File terdampak

- `internal/auth/service.go` — `ErrSetupRequired` + guard di `Register`
- `internal/api/auth_handler.go` — mapping 403 `SETUP_REQUIRED`
- `renderers/react-shadcn/src/shell/LoginScreen.tsx` — redirect ke setup,
  sembunyi-kan "Sign up" saat `setup_required` diketahui
- `renderers/react-shadcn/src/App.tsx` — guard route register + bounce
  authenticated
- Tests: seed admin di test yang register (`oauth_login_test.go`,
  `oauth_handler_test.go`), test baru `TestService_Register_SetupRequired`

## Verifikasi

E2E browser (Clinic example, DB fresh): register → 403 → redirect setup →
buat admin → login → `/default/_admin` render normal. `go test ./...` hijau.

# 2026-09-07-001 — Auth Seragam Dev/Prod, Hapus `--dev-auth`, Persist Secret Dev

**Plan**: `docs_internal/plan/` (session plan — unify dev/prod auth)

## Apa

Perilaku auth `formspec dev` disamakan dengan prod: selalu JWT asli — bypass
`DevValidator` (identity sintetis `developer`/`*`) dan auto-seed `admin/admin`
dihapus. Flag `--dev-auth` (dan key config `dev-auth:`) dihapus. First-run di
semua mode: setup wizard (`/{ws}/_admin/setup`) membuat admin pertama via
`POST /{ws}/_ui/setup` → login → redirect ke app.

JWT secret: prod tetap wajib secret/public-key eksplisit (fail-fast). Dev
tanpa `jwt-secret` → generate sekali dan persist ke
`<state-dir>/dev-jwt-secret` (chmod 0600) sehingga sesi bertahan antar
restart — sebelumnya secret random per proses membuat sesi invalid tiap
restart.

Frontend: `SetupScreen` kini cek `GET /{ws}/_ui/setup` saat mount — bila
setup tidak diperlukan (workspace sudah punya user), wizard langsung
mengarahkan ke login alih-alih menampilkan form yang pasti gagal 409
`SETUP_COMPLETE`. Submit yang kena 409 juga redirect ke login.

## Kenapa

User melaporkan `/_admin/setup` selalu error `auth: setup already complete`
pada DB baru: `SeedDevUser` jalan di semua dev boot (gate hanya
`!ProdMode`), sehingga workspace tidak pernah kosong user dan setup wizard
tidak pernah relevan. Keputusan user: samakan behaviour dev/prod dan hapus
`--dev-auth` — satu model auth, first-run via setup wizard di semua mode.

## File terdampak

- `resource/formspec.go` — hapus `Config.DevAuth`, branch `DevValidator`,
  blok seed; auto-secret gate `!ProdMode`
- `internal/auth/service.go` — `SeedDevUser` pindah ke `testutil.go`
  (test-only)
- `internal/devsecret/` (baru) — resolve/generate/persist secret dev
- `cmd/formspec/dev.go`, `dev_config.go`, `cmd/formspec-registry/main.go` —
  hapus flag `--dev-auth`, wire persist secret
- `renderers/react-shadcn/src/shell/SetupScreen.tsx` — redirect UX
- `formspec-app.yaml`, `examples/{cafe,service-demo}/formspec-app.yaml` —
  hapus `dev-auth:`
- Tests: `resource/dev_auth_test.go` (rewrite), 6 file e2e yang tadinya
  mengandalkan bypass anonim kini auth eksplisit (`seedAdminToken` helper)
- Docs: `docs/cli-tools/01-formspec-dev.md`, `docs/guides/authentication.md`,
  `docs/guides/how-to-run.md`

## Konsekuensi

- Demo lama yang mengandalkan anonymous bypass kini harus login (by design).
- Request tanpa token di dev → 401, konsisten dengan prod.

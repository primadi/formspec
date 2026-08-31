# Dev-Auth: Real JWT Auth di Dev Mode + Redirect Login

**Tanggal**: 2026-08-21 · **Sequence**: 016
**Plan**: `docs/plan/` (session plan — dev-auth + login redirect)

## Apa yang diubah

Menambahkan kemampuan menguji **authorization di dev mode** tanpa kehilangan
kenyamanan dev bypass. Sebelumnya dev mode selalu memakai `DevValidator`
(auth bypass total) sehingga `/app/...` bisa diakses tanpa login; sekarang ada
flag opt-in `--dev-auth` yang mengaktifkan JWT auth sungguhan di dev, plus
frontend me-redirect user yang belum login ke `/login?returnTo=...`.

### Backend

- `resource/formspec.go` — tambah `Config.DevAuth`. Di `New()`, jika `DevAuth`
  tanpa `JWTSecret`, generate secret acak (shared validator + token issuer).
  `configureAuth()` memakai `JWTValidator` saat `DevAuth` (bukan `DevValidator`).
  `SeedDevUser` (admin/admin) tetap jalan karena `!ProdMode`.
- `cmd/formspec/dev.go` — flag baru `--dev-auth` dan `--jwt-secret` (persist
  token antar restart), di-wire ke `formspec.Config`.
- `cmd/formspec/dev_config.go` — key `dev-auth` dan `jwt-secret` di
  `formspec-app.yaml` (config file; CLI flag menang jika di-set).
- `examples/cafe/formspec-app.yaml` — contoh key `dev-auth`/`jwt-secret`
  (commented).
- `resource/dev_auth_test.go` (baru) — `TestDevAuth_ValidatorSelection`
  (DevValidator vs JWTValidator) dan `TestDevAuth_LoginFlow` (anonim →
  identitas `anonymous`; login admin/admin → identitas user nyata).

### Frontend

- `renderers/react-shadcn/src/lib/api/meta.ts` — `fetchMe` return `null` saat
  error. `_meta/me` selalu 200; anonim → `user_id: "anonymous"`.
- `renderers/react-shadcn/src/stores/session.ts` — state `unauthenticated`;
  `boot()` mendeteksi identitas `anonymous` → `unauthenticated: true` (hapus
  fallback identitas sintetis yang sebelumnya membuka akses tanpa login).
- `renderers/react-shadcn/src/App.tsx` — `SurfaceShell` auth guard: jika
  `!isPublic && unauthenticated` → redirect ke `/login?returnTo=<path>`.
  Fix effect: saat session sudah loaded (mis. setelah redirect login), tetap
  panggil `loadMeta` jika bundle belum ada (sebelumnya "Loading manifests..."
  selamanya setelah login).
- `renderers/react-shadcn/src/lib/api/auth.ts` (baru) — `loginWithPassword`
  memanggil `POST /{ws}/api/v1/auth/login`.
- `renderers/react-shadcn/src/shell/LoginScreen.tsx` — dukung login
  username/password (mode default) + toggle "Use API token instead".

### Docs

- `docs/cli-tools/01-formspec-dev.md` dan `docs/guides/how-to-run.md` — tabel
  flag + config file untuk `--dev-auth` / `--jwt-secret`.

## Kenapa

User ingin menguji perilaku authorization di dev mode. Dev bypass tetap default
(`--dev` tanpa `--dev-auth`); auth bisa diaktifkan eksplisit untuk testing.
Secret persist via `jwt-secret` agar token survive restart.

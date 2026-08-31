# Fase 6 Dogfooding — Auth per-App `auth_config_ref` (Fase F)

**Tanggal**: 2026-08-20 · **Sequence**: 008
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase F)

## Apa yang diubah

Menghidupkan auth per-App (todo 6.1.4): `App.spec.auth_config_ref` kini
di-resolve ke strategy auth + entity overrides.

### Fase F — selesai

- **F1** `internal/auth/appauth.go` (baru) — `ResolveAppAuth(apps, configs)`:
  untuk tiap App, baca `auth_config_ref` → Config; tentukan `strategy`
  (default `basic-auth`; `sso`/`social-sso`/`passwordless`/`passkey` = open set,
  dikenali tapi belum diimplementasikan); baca entity overrides
  (`user_entity`/`session_entity`/`role_entity`) → `RoleResolver.SetOverride`.
  Error untuk config hilang / strategy tak dikenal.
- **F2** Wiring di `resource/formspec.go` — ekstrak Config manifests dari
  `specManifests`, panggil `ResolveAppAuth`, warn untuk strategy non-basic-auth
  (fallback ke basic-auth), terapkan overrides ke `authRoles`. Workspace context
  per-request sudah ada (identity/permission/ws ctx).

## Kenapa

Auth strategy per-App adalah kontrak `platform/02 §3` — App bisa memilih
strategi auth sendiri. Single-server mengimplementasi `basic-auth`; strategy
lain dideklarasikan (open set) dan di-warn saat dipakai, dengan seam
`RoleResolver.SetOverride` untuk entity override.

## File yang terkena dampak

- `internal/auth/appauth.go` + `appauth_test.go` (baru)
- `resource/formspec.go` — ekstrak Config + `ResolveAppAuth` + wire overrides

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `ResolveAppAuth`: default basic-auth, custom strategy + overrides,
  error (config hilang, strategy tak dikenal) — hijau.

# 2026-08-23-002 — Fix Hot-Reload: Embedded Core Entities Survive ReloadSpec

## Apa yang diubah

Bug hot-reload diperbaiki: `ReloadSpec()` sebelumnya membangun entity registry
baru hanya dari filesystem spec (`LoadEntities()`), sehingga entity embedded
module `formspec.core` (`user`, `role`, `session`, `api-key`, `app-membership`,
`workspace`) **hilang saat reload** — sementara UI registry tetap me-load
embedded manifests (`user-table`, `role-table`, `role-form`). Akibatnya
validasi gagal ("entity not found") dan tab Access Management merender
fallback "Table: user-table".

### Fix

`resource/formspec.go` `ReloadSpec()` — mirror startup ordering (`NewApp`):

- `newReg.AddManifestRoot(a.cfg.ExternalDir)` — external/ overrides
  di-register sebelum user manifests (agar bisa replace embedded defaults).
- `auth.RegisterCoreEntities(newReg)` — embedded framework-owned auth
  entities di-register sebelum `LoadEntities()`, sehingga survive reload.

### Test

`resource/reload_spec_test.go` (baru) — `TestReloadSpec_PreservesEmbeddedCoreEntities`:
memverifikasi `formspec.core.user/role/session/api-key` + `acme.customer`
terdaftar sebelum dan sesudah `ReloadSpec()`.

## Kenapa diubah

Hot-reload (`formspec dev` watcher) adalah alur dev utama; menjatuhkan entity
auth saat reload membuat Access Management rusak sampai server di-restart.
Fix menyelaraskan `ReloadSpec()` dengan urutan startup `NewApp()`.

## File yang terkena dampak

- `resource/formspec.go` — `ReloadSpec()`: register external dir + embedded
  core entities sebelum `LoadEntities()`
- `resource/reload_spec_test.go` — test regresi (baru)

## Referensi

- Changelog: `2026-08-23-001` (catatan bug hot-reload)
- Todo: 6.1 (auth module dogfooding)

## Verifikasi

- `go build ./...` hijau
- `go test ./...` hijau (734 pass, +1 test baru)
- E2E: `touch`/edit `module.yaml` → reload log `80 routes, 12 entities`
  (sebelumnya `60 routes, 6 entities` + "entity not found"); tab Users/Roles
  di Access Management tetap merender tabel setelah reload.

# Fase 6 Dogfooding — Permission Model (Fase C)

**Tanggal**: 2026-08-20 · **Sequence**: 005
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase C)

## Apa yang diubah

Melengkapi permission model (todo 6.2). C1 (format) dan C2 (wildcard) sudah
terimplementasi di `internal/permission` + `Identity.HasPermission`; gap utama
adalah C3 (resolusi role→perms + cache per-session).

### Fase C — selesai

- **C1** (6.2.1) Format `{module}.{entity}.{action}` — sudah ada:
  `permission.ValidatePermissionFormat`, `ParseResourceTarget`,
  `AutoPrefixPermission`. Diverifikasi, tidak ada perubahan.
- **C2** (6.2.2) Wildcard — sudah ada di `Identity.HasPermission`: `*`
  (super-wildcard), `{module}.{entity}.*`, `public` (anonymous). Diverifikasi.
- **C3** (6.2.4) **`PermissionResolver`** (baru, `internal/auth/resolver.go`) —
  resolve user's effective permissions (direct + role grants materialized)
  dengan **cache per-session** (key `workspace/userID`), `Invalidate(userID)`,
  `InvalidateAll()`. Di-wire ke `auth.Service` via `ensureResolver()` (lazy,
  setelah `SetRoleStore`+`SetMaterializer`); `permissionsForUser` kini memakai
  resolver; `Service.InvalidatePermissions(userID)` untuk invalidasi saat
  role/role-assignment berubah.

## Kenapa

Menghindari re-materialisasi role grants pada setiap issuance token (login/
refresh) — cache per-session mempercepat resolusi permission, dan menyediakan
seam invalidasi untuk Fase G (roles & delegation) dan Fase E (middleware).

## File yang terkena dampak

- `internal/auth/resolver.go` + `resolver_test.go` (baru)
- `internal/auth/service.go` — field `resolver`, `ensureResolver`,
  `InvalidatePermissions`, `permissionsForUser` pakai resolver

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `PermissionResolver` (resolve direct+role grants, cache hit, invalidate)
  hijau.

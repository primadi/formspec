# Fase 6 Dogfooding — Roles & Delegation (Fase G)

**Tanggal**: 2026-08-20 · **Sequence**: 009
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase G)

## Apa yang diubah

Menghidupkan 4 symmetric owner roles (todo 6.3.4) + fondasi delegation chain
(todo 6.3.3).

### Fase G — selesai

- **G1** (6.3.4) 4 symmetric owner roles:
  - Konstanta `RoleWorkspaceOwner`/`RoleAppOwner`/`RoleModuleOwner`/`RoleCloudOwner`
    (`internal/auth/core.go`).
  - `RoleStore.CreateRole` + `SeedOwnerRoles` (idempotent) — seed 4 role.
  - `ownerRolePermission(role)` — workspace/cloud/app-owner → `*`; module-owner →
    `{module}.*` (scoped). Di-recognize di `PermissionResolver.resolveUncached`.
  - `Identity.HasPermission` kini mendukung **module-level wildcard** `{module}.*`
    (match `{module}.{entity}.{action}`), selain entity-level `{module}.{entity}.*`.
  - Field `module` ditambahkan ke `Role` + entity `formspec.core.role`.
  - `Service.SeedOwnerRoles` + wiring di `resource/formspec.go` (dev mode).
- **G2** (6.3.3) Admin delegation chain — fondasi: owner roles di-seed + di-recognize
  (workspace owner → app owner → module owner). Enforcement penuh rantai delegasi
  (siapa boleh assign role apa) bergantung role-assignment management UI + per-App
  routing — di-defer, dicatat di todo.

## Kenapa

Owner roles adalah baseline RBAC: role-assignment punya role untuk di-grant, dan
permission resolver mengenali scope owner (workspace/app/module/cloud).

## File yang terkena dampak

- `internal/auth/core.go` — konstanta owner roles
- `internal/auth/role.go` — `Module` field, `CreateRole`, `SeedOwnerRoles`,
  `ownerRolePermission`
- `internal/auth/auth.go` — module-level wildcard di `HasPermission`
- `internal/auth/resolver.go` — recognize owner roles
- `internal/auth/service.go` — `SeedOwnerRoles`
- `internal/auth/module/master/role/entity.yaml` — field `module`
- `internal/auth/owner_test.go` (baru)
- `resource/formspec.go` — seed owner roles (dev)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test: module wildcard, `ownerRolePermission`, resolver owner-role → `*`,
  `SeedOwnerRoles` idempotent — hijau.

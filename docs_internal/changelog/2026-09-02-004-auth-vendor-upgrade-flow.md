# 2026-09-02-004 — Fase 6 Auth Redesign: Vendor Upgrade Flow

## Apa

Flow upgrade user biasa → vendor (auth redesign Fase 6): user apply jadi
vendor → aplikasi pending → admin approve → user dapat role `vendor` +
permissions registry.

### Vendor entity (`registry/spec/modules/registry/entities/vendor.yaml`)

- Field `status` (pending|active|rejected), default `pending`.
- Action custom `approve` (native, `required_permission:
registry.vendors.update`) — route `POST /_ui/entity/registry/vendor/{id}/approve`.

### Native handler (`cmd/formspec-registry/main.go`)

- `registry.vendor.approve` — 1) aktifkan vendor (status → active, CAS via
  entity store), 2) grant role `vendor` + permissions
  `registry.vendor.*`/`registry.module.*` ke owner user via auth service.

### Auth service (`internal/auth/service.go`)

- `GrantRoles(ctx, workspaceID, username, roles, permissions)` — tambah role +
  permission idempotent (merge tanpa duplikat), invalidasi cache permission.
- `GetAuthService()` getter di `internal/api/auth_handler.go`.

### UI

- `vendor-signup` form: label "Ajukan Vendor", message status pending.
- `vendor-signup` page: subtitle 4 langkah (buat akun → keygen → ajukan →
  tunggu approval).

## Kenapa

User poin 6: user harus sign-up dulu sebagai user biasa, kemudian upgrade jadi
vendor. Sebelumnya vendor langsung aktif tanpa approval dan tanpa grant role.

## Verifikasi

- Create vendor (tester1) → status `pending`. ✅
- Approve tanpa `registry.vendors.update` → 403. ✅
- Approve (dengan permission) → vendor `active` + tester1 dapat role
  `["vendor"]` + perms `registry.vendor.*`, `registry.module.*`. ✅
- `go test ./internal/auth ./internal/api`: 195 passed.

## File terdampak

- `registry/spec/modules/registry/entities/vendor.yaml`
- `registry/spec/modules/portal/{forms/vendor-signup.yaml,pages/vendor-signup.yaml}`
- `cmd/formspec-registry/main.go`
- `internal/auth/service.go`, `internal/api/auth_handler.go`

## Referensi

- Plan: `/memories/session/plan.md` (Fase 6)
- Todo: 5.2.15

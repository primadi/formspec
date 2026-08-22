# Fase 6 — E2E Test Auth + Authorization

**Tanggal**: 2026-08-21 · **Sequence**: 015
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (verifikasi)

## Apa yang diubah

Menambahkan **e2e test komprehensif** auth + authorization yang mem-boot app
penuh (`resource.New`) dan mengalirkan seluruh alur melalui HTTP API.

### `resource/auth_e2e_test.go` (baru)

`TestAuthAuthz_E2E` — mem-boot app dalam **ProdMode** (JWT validation) dengan
spec minimal (entity `acme.customer` exposed + field `secret` masked + field
`salary` dengan `required_permission`), lalu menguji:

1. **Authentication** — login `admin/admin` → token; password salah → 401.
2. **Authorization** — create customer dengan token admin → 201.
3. **Field-level security (masked)** — field `secret` (masked) tidak bocor di response.
4. **Authentication** — tanpa token → 401.
5. **Authorization (RBAC)** — user `limited` (list+view saja): view → 200,
   delete → 403 (tanpa `acme.customers.delete`).
6. **Field-level required_permission** — admin (has `*`) melihat `salary`;
   user `limited` (tanpa `acme.customers.salary.view`) TIDAK melihat `salary`.
7. **API key** — `X-FormSpec-Key` di surface external `/api/v1/` → 200.
8. **API key revoked** — setelah revoke, key yang sama → 401.

`TestAuthRefresh_Rotation_E2E` — refresh token rotation (todo 6.1.3):
login → refresh → pair baru; **replay refresh token lama → 401**; refresh token
baru masih berfungsi → 200.

`TestAuthRoleGrants_E2E` — **role-based via grant materialization** (todo 6.3.1):
role `viewer` dengan grant page `customer-list` → action `view`; user pemegang
role login → grant ter-materialisasi jadi `acme.customers.view` → view 200,
delete 403 (tanpa grant delete).

`TestAuthSessionRevoke_E2E` — **session revocation** (todo 6.5.4): setelah
session user dihapus (logout), refresh token session itu → 401.

`TestAuthConcurrentSessionLimit_E2E` — **concurrent session limit** (todo 6.5.3):
`Config.MaxSessionsPerUser` (baru, di-wire ke `authSvc.SetMaxSessionsPerUser`);
login #2 meng-evict session #1 (cap=1) → refresh session #1 → 401, refresh
session #2 → 200.

`TestAuthRateLimit_E2E` — **rate limiting** (todo 6.6.3): burst 5 login sukses,
login ke-6 → 429.

`TestAuthWildcardPermission_E2E` — **wildcard permission** (todo 6.2.2): user
dengan `acme.customers.*` bisa delete → 204.

`TestAuthOwnerRole_E2E` — **owner role** (todo 6.3.4): user dengan role
`workspace-owner` → `*` super-wildcard → delete → 204.

`TestAuthAuditLog_E2E` — **audit log** (todo 6.6.4): login sukses + gagal
tercatat di `api.RecentAuthAudit` (accessor baru di-export).

Helper: `buildAuthSpecDir` (kini + Table/Page manifest utk materialisasi),
`seedUser`, `login`, `loginPair`, `doAuthed`.

> Catatan: rate limiter auth global (burst 5) di-reset antar test via
> `api.ResetAuthRateLimiters()` (di-export dari `internal/api/ratelimit.go`).
> DELETE mengembalikan 204 No Content (bukan 200).

> Catatan: di **dev mode**, `AuthMiddleware` memakai `DevValidator` yang memberi
> identitas sintetis `*` ke semua request — jadi 401/403 hanya teruji benar di
> **ProdMode** (JWT validation). Test ini memakai ProdMode.

## Kenapa

Memberi satu titik verifikasi end-to-end yang membuktikan seluruh auth +
authorization (login, RBAC 401/403, field security, API key) bekerja bersama
melalui HTTP — bukan hanya unit test per komponen.

## File yang terkena dampak

- `resource/auth_e2e_test.go` (baru)

## Verifikasi

- `go test ./resource/ -run TestAuthAuthz_E2E` hijau.
- `go build ./...` + `go test ./...` hijau (tidak ada regresi).

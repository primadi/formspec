# Plan: Role Grants — App Scope Sync (Frontend ↔ Backend)

**Tanggal**: 2026-08-26
**Status**: In Progress
**Scope**: C (grants editor unfiltered + app-scoped auth)

## Latar Belakang

Dua temuan dari audit role form (`/app/{app}/formspec.core/roles/{id}/edit`):

1. **GrantsEditor dibatasi permission caller.** Tree grants dibangun dari
   `useMetaStore((s) => s.bundle)` — bundle `/_meta/ui?app={app}` yang
   di-permission-filter per caller (`BuildBundle`). Akibatnya user dengan hak
   terbatas (mis. module-owner) hanya melihat page/entity yang bisa dia lihat,
   padahal grants editor adalah tool admin yang harus menampilkan **semua**
   page di app.

2. **Role tidak benar-benar app-scoped.** Role punya field `app`, tapi
   `PermissionResolver.resolveUncached` me-resolve **semua** role user tanpa
   memfilter `app`. Login juga workspace-level (tanpa app). Jadi permission
   check tidak pernah mempertimbangkan app.

## Tujuan

Frontend dan backend sinkron:

1. **UI**: GrantsEditor menampilkan semua module di app tertentu (unfiltered),
   tidak peduli permission user yang login.
2. **Auth**: karena level pengaturan role di app, login membawa app, dan
   permission check mempertimbangkan app (role di-filter per-app).

## Keputusan Desain

- **`?grants=true`** pada `/_meta/ui`: bundle app-scoped tapi **tanpa**
  permission filtering. Digerbangi permission `formspec.core.roles.update`
  **atau** `formspec.core.roles.create` (caller harus bisa mengelola role).
  `_admin` (`?admin=true`) tetap workspace-level & unfiltered (existing).
- **Login membawa `app` opsional**: `POST /_ui/auth/login` body `{ username,
password, app }`. `app` kosong = workspace-level (backward compatible,
  dipakai `_admin`). `app` terisi = permission di-resolve per-app.
- **Permission resolution per-app**: `PermissionResolver.Resolve` menerima
  `app`; cache key `{ws}/{app}/{user}`; role berkontribusi hanya jika
  `role.App == ""` (workspace-global, mis. owner roles seed) atau
  `role.App == app`.
- **App dibawa di session/JWT/Identity**: refresh token membawa `app` agar
  refresh me-resolve ulang untuk app yang sama; `Identity.App` tersedia di
  context untuk audit/context app-specific.
- **Frontend**: login mengirim `app` (dideteksi dari URL); session store
  menyimpan `app`; GrantsEditor fetch bundle `grants=true` saat mount.

## File yang Terkena Dampak

### Backend

- `internal/api/meta.go` — `HandleMetaUI` dukung `?grants=true` (app-scoped,
  unfiltered, gate role manage).
- `internal/auth/auth.go` — `Identity.App`.
- `internal/auth/token.go` — `accessClaims.App` + `refreshClaims.App`.
- `internal/auth/jwt.go` — `JWTValidator.Validate` ekstrak `app`.
- `internal/auth/session.go` — `Session.App`.
- `internal/auth/service.go` — `Login(ctx, ws, app, user, pass)`; `issuePair`
  resolve per-app; `permissionsForUser` terima `app`.
- `internal/auth/resolver.go` — `Resolve(ctx, ws, app, user)`; cache key
  include app; filter role by `app`.
- `internal/api/auth_handler.go` — `loginRequest.App`; pass ke `Login`.
- `internal/api/middleware.go` — ekstrak `app` dari URL/identity ke context.
- Test: `internal/auth/*_test.go`, `internal/api/*_test.go`.

### Frontend

- `renderers/react-shadcn/src/lib/api/auth.ts` — login kirim `app`.
- `renderers/react-shadcn/src/stores/session.ts` — simpan `app`.
- `renderers/react-shadcn/src/App.tsx` — LoginPage kirim `app`.
- `renderers/react-shadcn/src/lib/api/meta.ts` — `fetchMetaBundle` dukung
  `grants: true`.
- `renderers/react-shadcn/src/widgets/GrantsEditor.tsx` — fetch bundle
  unfiltered saat mount.

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- `cd renderers/react-shadcn && npx tsc --noEmit` + `vitest`.
- E2E: login app → buka role form → GrantsEditor menampilkan semua page app
  (termasuk yang di luar permission caller).

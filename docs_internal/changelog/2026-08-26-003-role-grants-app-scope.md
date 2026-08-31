# 2026-08-26-003 — Role Grants: App Scope Sync (Frontend ↔ Backend)

## Apa yang diubah

Menyinkronkan frontend dan backend untuk role management yang benar-benar
per-App (plan: `docs/plan/role-grants-app-scope.md`).

### 1. GrantsEditor menampilkan SEMUA module di app (unfiltered)

Sebelumnya `GrantsEditor` membangun tree dari bundle `/_meta/ui?app={app}`
yang di-permission-filter per caller — user dengan hak terbatas hanya melihat
page/entity yang bisa dia lihat. Sekarang:

- **Backend**: `GET /_ui/_meta/ui?app={app}&grants=true` — bundle app-scoped
  tapi **tanpa** permission filtering (semua entity/page di app), digerbangi
  permission `formspec.core.roles.create` **atau** `formspec.core.roles.update`
  (`internal/api/meta.go`).
- **Frontend**: `GrantsEditor` fetch bundle `grants=true` saat mount
  (`widgets/GrantsEditor.tsx`), fallback ke bundle filtered jika gagal.

### 2. Auth app-scoped: login membawa app + permission check per-app

Role punya field `app`, tapi sebelumnya `PermissionResolver` me-resolve semua
role tanpa memfilter app, dan login workspace-level. Sekarang:

- **Login membawa `app`** (opsional): `POST /_ui/auth/login` body
  `{ username, password, app }`. `app` kosong = workspace-level (backward
  compatible, dipakai `_admin`).
- **Permission resolution per-app**: `PermissionResolver.Resolve(ctx, ws, app,
user)` — cache key `{ws}/{app}/{user}`; role berkontribusi hanya jika
  `role.App == ""` (workspace-global, mis. owner roles seed) atau
  `role.App == app`.
- **App dibawa di session/JWT/Identity**: `accessClaims`/`refreshClaims` +
  `Session` + `Identity` punya field `App`; refresh token membawa app agar
  refresh me-resolve ulang untuk app yang sama; `_meta/me` mengembalikan `app`.
- **Frontend**: login mengirim `app` (dideteksi dari URL `/{ws}/app/{app}/...`
  di `LoginPage`); session store menyimpan `app` (persisted di sessionStorage);
  `loginWithPassword` menerima `app`.

## Kenapa

Role management adalah per-App. Grants editor adalah tool admin — harus
menampilkan semua page di app agar admin bisa memberikan akses ke hal yang
belum dia pegang sendiri. Dan karena role per-App, login + permission check
harus mempertimbangkan app (role di app lain tidak boleh berkontribusi).

## File yang terkena dampak

- `internal/api/meta.go` — `?grants=true` + `metaIdentity.App`.
- `internal/auth/{auth,user,token,jwt,session,service,resolver}.go` — app scope.
- `internal/auth/module/transaction/session/entity.yaml` — field `app`.
- `internal/api/{auth_handler,middleware,handler}.go` — login app + context.
- `renderers/react-shadcn/src/{lib/api/auth.ts,lib/api/meta.ts,stores/session.ts,App.tsx,shell/LoginScreen.tsx,widgets/GrantsEditor.tsx,types/manifest.ts}`.
- Test: `internal/auth/resolver_test.go` (app-scoped), `internal/api/meta_test.go` (grants mode).

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- `cd renderers/react-shadcn && npx tsc --noEmit` + `vitest` hijau.
- E2E: login dengan `app` → claim `app` di JWT; `?grants=true` → 200; login
  tanpa app (workspace-level) tetap jalan.

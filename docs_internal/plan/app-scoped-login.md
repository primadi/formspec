# Plan — Login per-App & pensiun panel admin `_admin`

Sumber: usulan pemilik proyek 2026-10-02 — _"user bukan login ke workspace, tapi
login ke App, kalau pindah App, user harus login ulang"_ + _"bagaimana kalau
\_admin/ dihapus saja?"_

Status: **In Progress** (2026-10-02). Breaking change disetujui pemilik proyek.

## Keputusan

| #   | Keputusan                                                                                                                                                             | Alasan                                                                                                                                                                      |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | **App = batas sesi.** `POST /_ui/auth/login` wajib membawa `app`; rute `/{ws}/_admin/login` dan `/login` dihapus.                                                     | Yang dimaksud pemilik proyek: jalan masuk adalah App, bukan workspace.                                                                                                      |
| D2  | **Pindah App tidak mengetik password ulang** bila sesi App itu masih hidup: sesi disimpan **per `(workspace, App)`**.                                                 | Identitas tetap level workspace (`spec/platform/02` §8 — identitas workspace-level, membership per-App); yang di-scope adalah **sesi**.                                     |
| D3  | **App publik tidak boleh login.** App `access: public` tidak merender form login.                                                                                     | Login di App publik menghasilkan sesi 0-role → inversi "login lebih lemah dari tamu" (`public-grant-signed-in-floor.md` sisa #3).                                           |
| D4  | **Panel entity `_admin` dihapus** (bundle unscoped `?admin=true` + gate biner `_admin.access`).                                                                       | Gate-nya terlalu kasar: satu permission membuka **semua** module (kafe 5.22.7); role `app-owner` wildcard `"*"` otomatis memegangnya. Motif pemilik proyek: attack surface. |
| D5  | **Rute framework di `_admin/*` DIPERTAHANKAN** (setup wizard, oauth callback/link, change-password) — tanpa entity browsing. **Tidak** ada App baru bernama `_admin`. | "ada aplikasi baru dengan nama \_admin, ini agak aneh dan tidak diharapkan"; rute framework dibutuhkan bootstrap.                                                           |
| D6  | Gate masuk App = **validasi saat login** (`app` tak dikenal → 400, publik → 400, 0 permission → 403). Bukan field `access_permission` baru.                           | Menutup kafe 10.21/10.22 tanpa skema baru; `access_permission` (`app-entry-gate.md`) tidak dipakai.                                                                         |
| D7  | Token **mengikat** `_meta/ui`: token ber-`app` yang meminta `?app=` berbeda → 403.                                                                                    | Hari ini App dipilih murni dari query param (`internal/api/meta.go` `resolveAppContext`) → sesi App A bisa merender bundle App B.                                           |
| D8  | `_admin.access` dipensiunkan.                                                                                                                                         | Surface-nya sudah tidak ada.                                                                                                                                                |

## Fase & file

### Fase 0 — Dokumen (wajib, AGENTS.md §1–§2)

- Plan ini + changelog `2026-10-02-006-*`.

### Fase 1 — Backend: App sebagai batas sesi

- `internal/auth/service.go` — `ErrAppRequired`, `ErrUnknownApp`,
  `ErrPublicAppNoLogin`, `ErrNoAppAccess`; `LoginWithContext` menolak 0-permission
  saat `app != ""` (via `issuePair`); `enforceSessionLimit` menghitung per
  `(user, app)` sehingga login App B tidak meng-evict sesi App A.
- `internal/api/auth_handler.go` — `app` wajib; validasi App (ada, tidak publik)
  lewat `b.apps`; petakan error → 400 `APP_REQUIRED`/`UNKNOWN_APP`/`APP_PUBLIC_NO_LOGIN`,
  403 `NO_APP_ACCESS`.
- `internal/api/meta.go` — buang cabang `?admin=true` + `adminAccessPermission`
  (→ 400 `ADMIN_BUNDLE_REMOVED`); `resolveAppContext` menolak `?app=` yang
  berbeda dari `Identity.App`.
- `internal/api/oauth_handler.go` — bawa `app` lewat OAuth `state`; callback
  boot sesi untuk App itu (redirect URI **tidak** berubah).
- `internal/vendor/registry.go` — URL `.../default/_admin` diperbarui.

### Fase 2 — Frontend: sesi per-App + hapus panel

- `src/stores/session.ts` — storage multi-slot `formspec-session:{ws}:{app}`;
  sesi aktif dipilih dari App yang di-resolve URL.
- `src/App.tsx` — hapus `<Route>` surface `admin`, `_admin/login`, `/login`,
  `/register`; `RootSurface` fallback → 404 (bukan `_admin`); setup chain ke
  login App privat pertama.
- `src/shell/LoginPage.tsx`, `LoginScreen.tsx` — `app` wajib; App publik tanpa
  form login; buang tautan `_admin/setup` & `_admin/login`.
- `src/shell/SetupScreen.tsx`, `OAuthCallback.tsx`, `OAuthLinkCallback.tsx`,
  `SwitchContextScreen.tsx`, `ChangePasswordPage.tsx` — buang tujuan `_admin`.
- `src/stores/meta.ts`, `src/hooks/useResolvedMenu.ts`, `src/shell/useSurface.ts`,
  `src/shell/router.tsx` — buang cabang admin.

### Fase 3 — Schema & validator

- `pkg/spec` — invarian `access: public` + chrome ber-auth ditolak; lalu
  `make generate-schema` + `make generate-kind-docs`.

### Fase 4 — Docs, test, todo

- `docs/architecture/02-admin-surfaces.md` §4 + D-ARCH-2; `docs/spec/frontend/04-spec-resolution-api.md` §2
  (hapus `?admin=true`); `docs/spec/platform/02-workspace-app-module.md` §8;
  `docs/spec/backend/01-core-basic.md` §8.7; `docs/renderers/shadcn-shell/01|05`;
  `docs/guides/authentication.md` §2/§4; `docs/kind/curation/App.md`;
  `docs/runtimes/02-formspec-resource.md`.
- Todo: 5.22.7 ✅ (panel dihapus), kafe 10.21 ✅, 10.22 ✅, 6.6.5; 14.c.1–14.c.4.

## Verifikasi

- `go build ./...`, `go test ./...`, `gofmt -l`.
- `cd renderers/react-shadcn && npx tsc -b && npx vitest run`.
- Test baru: login tanpa `app` → 400; `app` ngawur → 400; App publik → 400;
  0 permission → 403; `?app=` ≠ token → 403; `?admin=true` → 400.
- Manual kafe: `/kafe/app/pos/login` → sesi pos; `/kafe/app/kds` → sesi KDS
  terpisah; `/kafe/_admin` → tanpa panel; `/kafe/_admin/setup`,
  `change-password` tetap jalan; `kafe-qr` tanpa form login.

## Sisa / pertanyaan terbuka

1. Workspace **tanpa App privat** → tidak ada jalan login (by design D3).
2. Lokasi rute framework dipertahankan di `_admin/*` (D5). Rename ke
   `/{ws}/auth/*` ditunda.
3. `_meta/ui` tanpa query `app` + workspace >1 App masih 400 (perilaku lama).

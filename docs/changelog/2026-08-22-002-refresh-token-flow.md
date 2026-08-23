# Refresh Token Flow (Frontend) + Fix Session Workspace (Backend)

**Tanggal**: 2026-08-22 · **Sequence**: 002
**Plan**: follow-up Fase 6.5 (session management) — `docs/plan/todo.md`

## Apa yang diubah

Access token JWT ber-TTL 15 menit. Sebelumnya frontend membuang refresh token
(7 hari) sehingga sesi selalu berakhir setelah 15 menit → 401 → redirect login.
Kini frontend menyimpan refresh token dan **auto-refresh** access token saat
expired, sehingga sesi bertahan tanpa login ulang.

### Frontend (`renderers/react-shadcn`)

- `lib/api/authHooks.ts` (baru) — shared ky hooks untuk refresh-token flow:
  `beforeRequest` attach token live dari store; `afterResponse` 401 pertama
  memaksa retry (`ky.retry()`); `beforeRetry` refresh access token
  (single-flight via `onUnauthorized`), jika gagal → session expired + abort.
  `retry: { limit: 1, shouldRetry: () => false }` — hanya forced retry 401,
  network/5xx tidak di-retry (mempertahankan perilaku `retry: 0`).
- `lib/api/client.ts` — `createApiClient` menerima `getToken`/`onUnauthorized`;
  memakai `createAuthHooks`.
- `lib/api/meta.ts` — `createMetaClient` + semua fetcher menerima callback yang
  sama; `fetchMe` mengembalikan null pada 401 (HTTPError/FormaApiError) dan
  `SessionExpiredError`.
- `lib/api/sessionEvents.ts` — tambah `SessionExpiredError` (sentinel).
- `stores/session.ts` — state `refreshToken`; aksi `refreshSession()`
  (single-flight, `POST /{ws}/api/v1/auth/refresh`, update token pair);
  `setSession`/`boot` menerima refresh token; `getClient` meneruskan
  `getToken`/`onUnauthorized`.
- `stores/meta.ts` — `load`/`refresh` meneruskan callback dari session store.
- `shell/LoginScreen.tsx` + `App.tsx` — login meneruskan `refreshToken` ke
  `boot`.
- `stores/session.test.ts` — test `refreshToken` state.

### Backend fix (`internal/auth`)

Bug pre-existing: `sessWorkspaceForJTI` hardcode `"demo"`, padahal session
disimpan di workspace sebenarnya (mis. `"default"`). Akibatnya `POST
/auth/refresh` selalu gagal ("invalid or expired refresh token") untuk
workspace non-`demo`. Fix: thread workspace melalui `SessionStore`:

- `internal/auth/session.go` — `Get`/`Delete`/`DeleteForUser`/`CountForUser`/
  `ListForUser` menerima `workspace`; hapus `sessWorkspaceForJTI`.
- `internal/auth/service.go` — `Refresh` memakai `claims.Workspace`;
  `enforceSessionLimit`/`LogoutAll` menerima workspace.
- `internal/auth/session_test.go` — update pemanggil (workspace `"demo"`).

## File yang terkena dampak

- `renderers/react-shadcn/src/lib/api/authHooks.ts` (baru)
- `renderers/react-shadcn/src/lib/api/client.ts`, `meta.ts`, `sessionEvents.ts`
- `renderers/react-shadcn/src/stores/session.ts`, `meta.ts`
- `renderers/react-shadcn/src/shell/LoginScreen.tsx`, `App.tsx`
- `renderers/react-shadcn/src/stores/session.test.ts`
- `internal/auth/session.go`, `service.go`, `session_test.go`

## Verifikasi

- `go build ./...` + `go test ./...` hijau (auth/api pass).
- `npm run build` (tsc + vite) hijau; `vitest run` — 100 test pass.
- E2E browser (cafe, dev-auth): login → korup access token pada request entity
  pertama (route intercept) → server balas 401 → app **auto-refresh** (session
  baru di-rotate di DB, `POST /auth/refresh`) → retry sukses → halaman tetap
  tampil, **tanpa redirect ke login**.
- Backend refresh contract via curl: login → refresh → token baru valid (200),
  token lama tetap 401.

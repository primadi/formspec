# Plan — UI "Link {provider}" (todo 5.2.21)

Status: In Progress (2026-09-04)
Date: 2026-09-04
Referensi spec: `docs/spec/` auth redesign Fase 5 (OAuth); plan
`account-pre-hijacking-fix.md` (explicit linking backend).

## Masalah

Endpoint backend `POST /_ui/auth/oauth/{provider}/link` sudah ada (todo
5.2.19), tapi tidak ada UI untuk memicunya. User yang kena
`ErrAccountLinkRequired` (email verified + akun punya password) hanya
diberitahu "sign in with your password to link this account" — tanpa tombol
untuk benar-benar link.

## Solusi

Alur explicit linking (full-page navigation, konsisten dengan login flow):

1. User signed-in (password) buka "Linked accounts" di user menu.
2. Klik "Link {provider}" → redirect ke
   `GET /_ui/auth/oauth/{provider}/authorize?mode=link`.
3. Backend menyimpan `mode` di CSRF state; callback mendeteksi `mode=link`
   → redirect ke SPA `/{ws}/_admin/oauth/link-callback#code=...&provider=...`
   (TANPA menjalankan `OAuthLogin` — itu akan gagal `ErrAccountLinkRequired`).
4. `OAuthLinkCallback` restore session dari sessionStorage, POST `{code}` ke
   `POST /_ui/auth/oauth/{provider}/link` (Bearer token), lalu kembali ke
   `/{ws}/_admin`.

## Perubahan

### Backend

| File                            | Perubahan                                                                                                                                                                 |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/api/oauth_handler.go` | `oauthState.Mode` ("" = login, "link"); `newOAuthState` terima mode; `HandleOAuthAuthorize` baca `?mode=`; `HandleOAuthCallback` redirect ke link-callback saat mode=link |
| `internal/api/meta.go`          | `metaIdentity.OAuthProvider` + populate di `HandleMetaMe` (dari `User.OAuthProvider`)                                                                                     |

### Frontend

| File                                                        | Perubahan                                                                                        |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `renderers/react-shadcn/src/types/manifest.ts`              | `MeResponse.oauth_provider` (+ `email_verified`)                                                 |
| `renderers/react-shadcn/src/shell/OAuthLinkCallback.tsx`    | Baru — POST code ke link endpoint, toast, kembali ke \_admin                                     |
| `renderers/react-shadcn/src/shell/LinkedAccountsDialog.tsx` | Baru — daftar provider + status linked/link; tombol Unlink (two-step confirm) + refresh identity |
| `renderers/react-shadcn/src/shell/UserMenu.tsx`             | Item "Linked accounts" → buka dialog                                                             |
| `renderers/react-shadcn/src/shell/index.ts`                 | Export `OAuthLinkCallback`                                                                       |
| `renderers/react-shadcn/src/App.tsx`                        | Route `/:workspace/_admin/oauth/link-callback`                                                   |

### Unlink (changelog `2026-09-04-002`)

- `internal/auth/user.go`: `EntityUserStore.UnlinkOAuthIdentity` (clear identity)
- `internal/auth/service.go`: `Service.UnlinkOAuthIdentity` — `ErrNotLinked`
  (provider tidak ter-link), `ErrUnlinkRequiresPassword` (akun pure-OAuth
  tanpa password — cegah lockout), notifikasi email
- `internal/api/oauth_handler.go` + `router.go`: `POST /_ui/auth/oauth/{provider}/unlink`
- `LinkedAccountsDialog.tsx`: tombol Unlink + refresh `me` via `fetchMe`

### Tests

- `internal/api/oauth_handler_test.go`: authorize `?mode=link` → callback
  redirect ke link-callback (bukan token pair).
- `internal/auth/oauth_login_test.go`: 3 test unlink (sukses, `ErrNotLinked`,
  `ErrUnlinkRequiresPassword`).

## Dependensi

Backend (mode=link + meta) → frontend (type + callback + dialog).

## Deferred

- Multi-identity per user (todo 5.2.20) — dialog hanya menampilkan satu
  `oauth_provider` saat ini.

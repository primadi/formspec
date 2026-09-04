# Plan — Fix Account Pre-Hijacking (OAuth auto-link by unverified email)

Status: ✅ Done (2026-09-03, changelog `2026-09-03-004`)
Date: 2026-09-03
Referensi spec: `docs/spec/` auth redesign Fase 4 (registration) & Fase 5 (OAuth);
OWASP Account Pre-Hijacking.

## Masalah

`OAuthLogin` (`internal/auth/service.go`) auto-link by email ke user mana pun
yang punya email itu, tanpa verifikasi kepemilikan email. Skenario serangan:

1. Hacker register dengan email X + password P (email tidak diverifikasi).
2. User asli login via Google OIDC dengan email X.
3. `GetByEmail(X)` menemukan akun hacker → user asli masuk ke akun itu.
4. User asli tidak sadar; hacker tetap bisa login dengan password P.

Gap pendukung:

- `oauth.UserInfo` tidak menyimpan klaim `email_verified` dari OIDC.
- Tidak ada field `email_verified` / identitas provider (`oauth_provider`+`oauth_sub`)
  di user record.
- `Register` tidak menerima email → tidak ada jalur verifikasi email.

## Solusi (defense in depth)

| #   | Fix                                                                                                                                     | File                                                                                  | Effort |
| --- | --------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | ------ |
| 1   | Capture `email_verified` dari OIDC provider                                                                                             | `internal/auth/oauth/oauth.go`                                                        | small  |
| 2   | Simpan `email_verified`, `oauth_provider`, `oauth_sub` di user                                                                          | `internal/auth/user.go`, `internal/auth/module/master/user/entity.yaml`               | small  |
| 3   | `Register` terima email → user unverified + kirim email verifikasi                                                                      | `internal/auth/service.go`, `internal/api/auth_handler.go`                            | medium |
| 4   | `VerifyEmail` + endpoint verify/resend                                                                                                  | `internal/auth/service.go`, `internal/api/auth_handler.go`, `internal/api/router.go`  | medium |
| 5   | `OAuthLogin` rewrite: identity-first `(provider,sub)`, gate email-verified, takeover akun unverified, explicit-link untuk akun password | `internal/auth/service.go`                                                            | medium |
| 6   | `LinkOAuthIdentity` (explicit linking, authenticated) + endpoint                                                                        | `internal/auth/service.go`, `internal/api/oauth_handler.go`, `internal/api/router.go` | medium |
| 7   | Notifikasi email saat akun di-link via OAuth                                                                                            | `internal/auth/service.go`                                                            | small  |
| 8   | Frontend: register form + email field; OAuth callback handle error baru                                                                 | `renderers/react-shadcn/src/shell/LoginScreen.tsx`, `OAuthCallback.tsx`               | small  |

## Kebijakan linking OAuth (hasil desain)

Urutan pencarian di `OAuthLogin`:

1. **Identity match** `(provider, sub)` → login langsung (identitas sama).
2. **Email match**:
   - Email **unverified**:
     - Provider `email_verified=true` → **takeover**: tandai verified, hapus
       password lama (password hacker berhenti bekerja), link identity, login.
     - Provider `email_verified=false` → blokir `ErrEmailUnverified`.
   - Email **verified** + akun **punya password** → `ErrAccountLinkRequired`
     (tidak pernah merge diam-diam; user login password lalu link eksplisit).
   - Email **verified** + akun **tanpa password** (pure OAuth) → attach identity,
     login, kirim notifikasi.
3. **User baru** → create dengan `EmailVerified` dari provider.

## Dependensi antar task

1 → 2 → 3 → 4 → 5 → 6 → 7 (backend), lalu 8 (frontend), lalu tests, lalu docs.

## File yang dibuat/diubah

- `internal/auth/oauth/oauth.go` (UserInfo.EmailVerified)
- `internal/auth/user.go` (fields + store methods)
- `internal/auth/service.go` (Register, VerifyEmail, OAuthLogin, LinkOAuthIdentity, notify)
- `internal/auth/module/master/user/entity.yaml` (3 field baru)
- `internal/api/auth_handler.go` (register+email, verify-email, resend-verification)
- `internal/api/oauth_handler.go` (callback error handling, oauth link)
- `internal/api/router.go` (route baru)
- `internal/api/meta.go` (expose email_verified di /meta/me)
- `renderers/react-shadcn/src/shell/LoginScreen.tsx`, `OAuthCallback.tsx`
- Tests: `internal/auth/oauth_login_test.go`, `internal/auth/password_test.go`,
  `internal/api/oauth_handler_test.go`, `internal/api/auth_handler_test.go`
- Docs: changelog + todo.md

## Deferred

- Multi-identity per user (array `oauth_identities`) — saat ini satu
  `oauth_provider`/`oauth_sub` per user (identity lookup best-effort, email
  sebagai fallback).
- Tombol "Link {provider}" di UI profile — endpoint backend sudah ada
  (`POST /_ui/auth/oauth/{provider}/link`).

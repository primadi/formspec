# 2026-09-03-004 — Fix Account Pre-Hijacking (OAuth auto-link by unverified email)

## Apa yang diubah

Menutup celah **account pre-hijacking**: `OAuthLogin` sebelumnya auto-link by
email ke user mana pun yang punya email itu, tanpa verifikasi kepemilikan —
seorang hacker bisa register dengan email X + password, lalu user asli yang
login via Google OIDC dengan email X masuk ke akun hacker tanpa sadar, dan
hacker tetap bisa login dengan password buatannya.

Perubahan (plan `docs_internal/plan/account-pre-hijacking-fix.md`):

1. **`oauth.UserInfo.EmailVerified`** — parse klaim `email_verified` dari
   provider OIDC (absent → true, karena provider hanya expose email
   primary/verified; explicit false → false).
2. **Field user baru** `email_verified`, `oauth_provider`, `oauth_sub` di
   `formspec.core.user` + `User` struct + store methods
   (`GetByOAuthIdentity`, `LinkOAuthIdentity`, `SetEmailVerified`,
   `TakeoverUnverifiedEmail`).
3. **`Register` terima email** → akun dibuat unverified + email verifikasi
   (token single-use TTL 24 jam). Endpoint `POST /_ui/auth/verify-email`
   (publik) + `POST /_ui/auth/resend-verification` (authenticated).
4. **`OAuthLogin` rewrite** — kebijakan linking:
   - Identity match `(provider, sub)` → login langsung.
   - Email match: email unverified + provider verified → **takeover** (tandai
     verified, ganti password lama dengan dead hash, link identity, login);
     email unverified + provider unverified → `ErrEmailUnverified`; email
     verified + akun punya password → `ErrAccountLinkRequired` (tidak pernah
     merge diam-diam); email verified + akun tanpa password → attach identity.
   - User baru → create dengan `EmailVerified` dari provider.
5. **Explicit linking** `POST /_ui/auth/oauth/{provider}/link`
   (authenticated) — user login password lalu link provider; email provider
   harus cocok dengan email akun yang verified.
6. **Notifikasi email** saat akun di-link via OAuth (takeover/attach).
7. **`/meta/me`** expose `email_verified`; frontend register + email field,
   OAuth callback handle fragment `#oauth=email_unverified` /
   `#oauth=link_required`.

## Kenapa

Email adalah kunci linking OAuth, tapi tanpa verifikasi email itu hanya
"klaim", bukan bukti kepemilikan. Fix ini memastikan OAuth hanya link ke email
yang terbukti dimiliki (verified), dan akun unverified yang di-claim hacker
bisa di-takeover oleh pemilik asli (provider-verified).

## File terdampak

- `internal/auth/oauth/oauth.go`
- `internal/auth/user.go`, `internal/auth/service.go`
- `internal/auth/module/master/user/entity.yaml`
- `internal/api/auth_handler.go`, `internal/api/oauth_handler.go`,
  `internal/api/router.go`, `internal/api/meta.go`
- `renderers/react-shadcn/src/shell/LoginScreen.tsx`,
  `renderers/react-shadcn/src/shell/OAuthCallback.tsx`
- Tests: `internal/auth/oauth_login_test.go`,
  `internal/api/oauth_handler_test.go`, `internal/api/auth_handler_test.go`

## Referensi

- Plan: `docs_internal/plan/account-pre-hijacking-fix.md`
- Todo: `docs_internal/plan/todo.md` 5.2.19
- OWASP Account Pre-Hijacking

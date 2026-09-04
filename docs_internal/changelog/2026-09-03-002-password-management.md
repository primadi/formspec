# 2026-09-03-002 — Password Management (empty-password guard, change password, reset via email)

## Apa yang diubah

Menutup asimetri auth OAuth vs password dari diskusi sebelumnya, dengan tiga fitur:

1. **Backend tolak password kosong** — `auth.Service.Login` kini menolak `password == ""`
   (defense in depth; sebelumnya hanya dicek di handler/frontend). Ini juga memblokir
   user OAuth (yang `password_hash`-nya bcrypt("")) dari login password kosong.
2. **Change password (self-service)** — endpoint `POST /{ws}/_ui/auth/change-password`
   (authenticated) + dialog "Change Password" di user menu. Verifikasi current password
   hanya jika user punya password (user OAuth tanpa hash bisa langsung set).
3. **Reset password via email** — endpoint `POST /{ws}/_ui/auth/forgot-password`
   (public, selalu 200, tidak bocorkan keberadaan email) + `POST /{ws}/_ui/auth/reset-password`
   (single-use token, TTL 15 menit). Email dikirim via SMTP mailer baru
   (`internal/mail`) — dev default ke Mailpit (`mailpit:1025`). Link memakai
   `?reset_token=` (bukan `?token=`) karena middleware auth membaca `?token=` sebagai JWT.

## Kenapa

- User OAuth dibuat tanpa password → tidak bisa login password, dan tidak punya cara set password.
- Password kosong hanya dicek di frontend → perlu guard di service layer.
- Tidak ada infrastruktur email sama sekali untuk reset password.

## File yang terpengaruh

- `internal/auth/service.go` — guard login, `ChangePassword`, `RequestPasswordReset`,
  `ResetPassword`, reset-token store, `Mailer` interface
- `internal/auth/user.go` — `SetPassword` (update hanya password_hash)
- `internal/mail/mailer.go` — BARU, SMTP mailer (net/smtp)
- `internal/api/auth_handler.go` — 3 handler baru
- `internal/api/router.go` — 3 route baru
- `resource/formspec.go` — Config SMTP + wire mailer
- `renderers/react-shadcn/src/shell/UserMenu.tsx` — item "Change Password"
- `renderers/react-shadcn/src/shell/ChangePasswordDialog.tsx` — BARU
- `renderers/react-shadcn/src/shell/LoginScreen.tsx` — link "Forgot password?" + mode forgot
- `renderers/react-shadcn/src/shell/ResetPasswordScreen.tsx` — BARU
- `renderers/react-shadcn/src/App.tsx` — route `/{ws}/reset-password`
- Test: `internal/auth/password_test.go`, `internal/api/password_handler_test.go` (BARU)

Referensi plan: `docs_internal/plan/password-management-plan.md`

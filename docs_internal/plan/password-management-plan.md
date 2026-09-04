# Plan — Password Management (empty-password guard, change password, reset via email)

Status: ✅ Complete (2026-09-03)
Date: 2026-09-03
Referensi spec: `docs/spec/backend/01-core-basic.md` (auth §15), `docs/spec/frontend/05-app-kinds.md` §4.1 (user menu)

## Latar belakang

Dari diskusi OAuth vs user biasa:

- User OAuth dibuat dengan `password_hash` = bcrypt("") — bisa "lolos" login password kosong
  di layer service (frontend sudah blokir, handler backend sudah blokir, tapi service `Login`
  belum).
- User OAuth tidak punya cara set password (tidak ada flow change/reset password).
- Tidak ada infrastruktur email sama sekali (reset password via email belum ada).

## Scope

### 1. Backend — tolak password kosong (defense in depth)

- `internal/auth/service.go` `Login`: tolak `password == ""` → `ErrInvalidCredentials`.
- `Register` sudah menolak password kosong — tidak diubah.

Effort: small.

### 2. Backend — change password (self-service, profile)

- `internal/auth/user.go`: tambah `SetPassword(ctx, workspaceID, userID, plain)` —
  hash bcrypt + update `password_hash` saja (preserve field lain; `UpdateUser` saat ini
  selalu preserve hash lama).
- `internal/auth/service.go`: tambah `ErrWeakPassword` + `ChangePassword(ctx, workspaceID,
userID, currentPassword, newPassword)`:
  - user by ID; jika `password_hash` non-kosong → verifikasi `currentPassword`.
  - validasi `newPassword` (non-empty, min 8).
  - hash + simpan.
- `internal/api/auth_handler.go`: `HandleChangePassword` — authenticated (identity dari
  context), body `{current_password, new_password}`.
- `internal/api/router.go`: `POST /{ws}/_ui/auth/change-password`.

Effort: medium.

### 3. Backend — reset password via email

- Baru: `internal/mail/mailer.go` — SMTP via `net/smtp` (host/port/user/pass/from).
  Mailpit dev: SMTP `mailpit:1025` (devcontainer) / `localhost:11025` (host), UI `:18025`.
- `internal/auth/service.go`:
  - field `mailer` + `SetMailer`.
  - reset token store in-memory (map, mutex, TTL 15 menit, single-use).
  - `RequestPasswordReset(ctx, workspaceID, email)` — find user by email; tidak bocorkan
    keberadaan email (selalu sukses); generate token; kirim email berisi link
    `{base_url}/{ws}/reset-password?token=...`.
  - `ResetPassword(ctx, workspaceID, token, newPassword)` — validasi token + expiry,
    single-use, set password baru.
- `internal/api/auth_handler.go`: `HandleForgotPassword` (public, rate-limited, selalu 200),
  `HandleResetPassword` (public, rate-limited).
- `internal/api/router.go`: `POST /{ws}/_ui/auth/forgot-password`,
  `POST /{ws}/_ui/auth/reset-password`.
- `resource/formspec.go` Config: tambah `SMTPHost/Port/User/Pass/MailFrom/MailBaseURL`
  (env-driven, default mailpit dev) + wire `authSvc.SetMailer`.
- `cmd/formspec/dev.go` + `serve.go`: baca env `FORMSPEC_SMTP_*` / `FORMSPEC_MAIL_*`.

Effort: large.

### 4. Frontend — change password di profile

- `renderers/react-shadcn/src/shell/UserMenu.tsx`: tambah item "Change Password" →
  buka dialog.
- Baru: `renderers/react-shadcn/src/shell/ChangePasswordDialog.tsx` — form
  current + new + confirm, panggil `POST /{ws}/_ui/auth/change-password`.

Effort: medium.

### 5. Frontend — reset password di login

- `renderers/react-shadcn/src/shell/LoginScreen.tsx`: tambah link "Forgot password?" →
  mode forgot (input email → submit → pesan sukses).
- Baru: `renderers/react-shadcn/src/shell/ResetPasswordScreen.tsx` + route
  `/{ws}/reset-password?token=...` di `App.tsx` — form password baru + konfirmasi.

Effort: medium.

## Dependensi

1 → 2 → 3 (backend), lalu 4 & 5 (frontend) paralel setelah backend siap.

## File yang terpengaruh

| File                                                        | Perubahan                                        |
| ----------------------------------------------------------- | ------------------------------------------------ |
| `internal/auth/service.go`                                  | Login guard, ChangePassword, reset token, mailer |
| `internal/auth/user.go`                                     | SetPassword                                      |
| `internal/mail/mailer.go`                                   | BARU — SMTP mailer                               |
| `internal/api/auth_handler.go`                              | 3 handler baru                                   |
| `internal/api/router.go`                                    | 3 route baru                                     |
| `resource/formspec.go`                                      | Config SMTP + wire mailer                        |
| `cmd/formspec/dev.go`, `serve.go`                           | env SMTP                                         |
| `renderers/react-shadcn/src/shell/UserMenu.tsx`             | item change password                             |
| `renderers/react-shadcn/src/shell/ChangePasswordDialog.tsx` | BARU                                             |
| `renderers/react-shadcn/src/shell/LoginScreen.tsx`          | forgot password                                  |
| `renderers/react-shadcn/src/shell/ResetPasswordScreen.tsx`  | BARU                                             |
| `renderers/react-shadcn/src/App.tsx`                        | route reset-password                             |

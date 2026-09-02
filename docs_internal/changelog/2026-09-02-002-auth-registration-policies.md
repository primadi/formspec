# 2026-09-02-002 — Fase 4 Auth Redesign: Registration Policies

## Apa

Implementasi policy registrasi per-workspace (auth redesign Fase 4) — 3
skenario: open / approval / closed.

### Spec (`pkg/spec/resources.go`)

- `Settings.Registration *RegistrationSettings` — `policy` (open|approval|
  closed, default open) + `default_role` (dipakai saat open).
- `ResolveSettings` overlay + `DefaultSettings` (policy open).

### Auth service (`internal/auth/service.go`)

- `RegistrationPolicy` struct + `SetRegistrationPolicy` (default open).
- `Register` mengikuti policy:
  - open → user active + default_role
  - approval → user pending (tidak bisa login)
  - closed → `ErrRegistrationClosed`
- `ApproveUser` — approve user pending (status → active + assign roles);
  invalidasi cache permission.
- `Login` sudah memblokir user pending (Fase 1).

### User store (`internal/auth/user.go`)

- `UpdateUser` — update status/roles/permissions/dll dengan optimistic
  concurrency (fetch version + preserve required fields username/password_hash).

### API

- `internal/api/approve_handler.go` (baru) — `POST /{ws}/_ui/auth/approve`,
  admin-only (`formspec.core.users.update`).
- `auth_handler.go` — register error `REGISTRATION_CLOSED` (403).
- `resource/formspec.go` — wire policy dari resolved settings saat boot DAN
  hot-reload (field `App.authSvc` dipertahankan).

## Kenapa

User poin 4: semua skenario registrasi harus bisa — (1) sign-up langsung
aktif dengan role default, (2) sign-up tunggu approval admin, (3) semua user
dibuat admin (tanpa sign-up).

## Verifikasi (via hot-reload Config manifest)

| Policy          | Register                | Status user       | Login |
| --------------- | ----------------------- | ----------------- | ----- |
| approval        | created                 | pending           | 401   |
| approve (admin) | —                       | active + roles    | 200   |
| closed          | 403 REGISTRATION_CLOSED | —                 | —     |
| open            | created                 | active + ["user"] | 200   |

- Approve tanpa permission `formspec.core.users.update` → 403.
- `go test ./internal/auth ./internal/api ./pkg/spec ./resource`: hijau.

## File terdampak

- `pkg/spec/resources.go`
- `internal/auth/{service,user}.go`
- `internal/api/{approve_handler.go (baru),auth_handler.go,router.go}`
- `resource/formspec.go`

## Referensi

- Plan: `/memories/session/plan.md` (Fase 4)
- Todo: 5.2.13

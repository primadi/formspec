# Fase 6 Dogfooding — Session Management (Fase D)

**Tanggal**: 2026-08-20 · **Sequence**: 006
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase D)

## Apa yang diubah

Melengkapi session management (todo 6.5). Session entity (6.5.1) dan refresh
rotation (6.5.2) sudah ada; gap: concurrent limit, global revoke, expiry cleanup.

### Fase D — selesai

- **D1** (6.5.3) Concurrent session limit per user — `Service.SetMaxSessionsPerUser(n)`
  (0 = unlimited); `issuePair` memanggil `enforceSessionLimit` → evict session
  tertua saat melebihi cap. `SessionStore.CountForUser` + `ListForUser` (oldest
  first) ditambahkan.
- **D2** (6.5.4) Global revoke / logout all — `Service.LogoutAll(userID)` →
  `SessionStore.DeleteForUser` (sudah ada); plus `Service.Logout(jti)` untuk
  logout satu device.
- **D3** (6.5.5) Session expiry + cleanup job — `SessionStore.PurgeExpired` +
  `Service.PurgeExpiredSessions`; `Service.StartSessionCleanup(interval)` (goroutine
  background, idempotent) + `StopSessionCleanup()`.

## Kenapa

Menutup siklus hidup session: batasi jumlah device aktif, dukung logout global,
dan bersihkan session kedaluwarsa — prasyarat keamanan untuk Fase E (middleware
pipeline) dan Fase F (auth per-App).

## File yang terkena dampak

- `internal/auth/session.go` — interface + `EntitySessionStore`: `CountForUser`,
  `ListForUser`, `PurgeExpired`
- `internal/auth/service.go` — field `maxSessions`/`cleanupStop`/`cleanupOnce`,
  `SetMaxSessionsPerUser`, `enforceSessionLimit`, `Logout`, `LogoutAll`,
  `PurgeExpiredSessions`, `StartSessionCleanup`, `StopSessionCleanup`
- `internal/auth/session_test.go` (baru)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test: concurrent limit (cap 1 → 1 session), unlimited default (3 sessions),
  LogoutAll (0 sessions), PurgeExpired (expired session dihapus) — semua hijau.

# Session Expiry → Login Redirect + Auto-Logout Timer (Frontend)

**Tanggal**: 2026-08-22 · **Sequence**: 001
**Plan**: follow-up Fase 6.5 (session management) — `docs/plan/todo.md`

## Apa yang diubah

Dua perbaikan UX autentikasi di renderer `react-shadcn`:

1. **Token invalid → redirect ke form login** (sebelumnya langsung error).
   Semua API client (entity CRUD + meta) kini mendeteksi respons `401` dan
   menandai session sebagai unauthenticated via event bus baru
   (`lib/api/sessionEvents.ts`), sehingga auth guard mengarahkan ke
   `{surfacePath}/login?returnTo=...` alih-alih menampilkan error. Boot flow
   (`fetchMe`) juga membedakan 401 (→ login) dari network error (→ connection
   error screen), dan `meta.load/refresh` memperlakukan 401 sebagai session
   expired.

2. **Auto-logout timer yang bisa diatur**. Preferensi `sessionTimeoutMinutes`
   (default 30, 0 = never) di `stores/prefs.ts`, di-set dari form login
   (selector "Auto logout after": 5/15/30/60 menit atau Never). Hook baru
   `hooks/useAutoLogout.ts` memantau aktivitas user (mouse/keyboard/scroll/
   touch/visibility) dan mengekspirasi session setelah idle timeout — dipasang
   di `SurfaceShell` hanya untuk session terautentikasi.

## File yang terkena dampak

- `renderers/react-shadcn/src/lib/api/sessionEvents.ts` (baru — event bus)
- `renderers/react-shadcn/src/lib/api/client.ts` (401 → notifySessionExpired)
- `renderers/react-shadcn/src/lib/api/meta.ts` (`fetchMe` 401 → null)
- `renderers/react-shadcn/src/stores/session.ts` (`expireSession` + handler)
- `renderers/react-shadcn/src/stores/meta.ts` (401 di load/refresh)
- `renderers/react-shadcn/src/stores/prefs.ts` (`sessionTimeoutMinutes`)
- `renderers/react-shadcn/src/hooks/useAutoLogout.ts` (baru — idle timer)
- `renderers/react-shadcn/src/App.tsx` (mount `useAutoLogout`)
- `renderers/react-shadcn/src/shell/LoginScreen.tsx` (selector timeout)
- `renderers/react-shadcn/src/stores/session.test.ts` (baru — 3 test)

## Verifikasi

- `npm run build` (tsc + vite) hijau.
- `vitest run` — 99 test pass (96 lama + 3 baru).
- E2E di browser (cafe example, dev-auth): login `admin/admin`; restart server
  dengan `--jwt-secret` berbeda → API call balas 401 → app redirect ke form
  login dengan `returnTo` yang benar. Selector "Auto logout after" tampil dan
  persist di localStorage.

# Session Persistence ke sessionStorage (Browser Refresh)

**Tanggal**: 2026-08-22 · **Sequence**: 003
**Plan**: follow-up Fase 6.5 (session management) — `docs/plan/todo.md`

## Apa yang diubah

Sebelumnya session (access + refresh token) hanya in-memory — browser refresh
(F5) selalu kembali ke form login. Kini access + refresh token dipersist ke
**sessionStorage** sehingga refresh halaman me-restore session tanpa login
ulang.

`sessionStorage` dipilih (bukan `localStorage`) karena per-tab dan dibersihkan
saat tab ditutup — token tidak pernah bertahan melewati restart browser.

### `renderers/react-shadcn/src/stores/session.ts`

- Helper `readStoredSession` / `writeStoredSession` / `clearStoredSession`
  (key `formspec-session`, dibungkus try/catch untuk private mode).
- `boot` — restore session dari sessionStorage saat tidak ada token eksplisit
  (hanya jika workspace cocok); setelah autentikasi berhasil, persist session;
  saat token invalid/anonymous, clear sessionStorage.
- `setSession` — tulis sessionStorage saat token ada; clear saat anonymous
  (public surface) di workspace yang sama.
- `clearSession` / `expireSession` — clear sessionStorage.
- `refreshSession` — update sessionStorage dengan token pair baru setelah
  refresh.

## File yang terkena dampak

- `renderers/react-shadcn/src/stores/session.ts`

## Verifikasi

- `npm run build` (tsc + vite) hijau; `vitest run` — 100 test pass.
- E2E browser (cafe, dev-auth): login → sessionStorage berisi access + refresh
  token → hard-reload (F5) → halaman tetap tampil (menu-items), **tanpa
  redirect ke login form**.

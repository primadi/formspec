# Logout Button di Shell

**Tanggal**: 2026-08-21 · **Sequence**: 017
**Plan**: session plan (dev-auth + login redirect) — follow-up

## Apa yang diubah

Menambahkan tombol **Logout** di header shell sehingga user yang login bisa
keluar dan kembali ke halaman login. Sebelumnya `clearSession()` sudah ada di
session store tapi tidak pernah dipanggil dari UI.

### `renderers/react-shadcn/src/shell/LogoutButton.tsx` (baru)

Komponen reusable: tombol icon `LogOut` yang memanggil `clearSession()` lalu
`navigate("/login", { replace: true })`. Hanya tampil saat ada sesi asli
(`token` non-kosong) — disembunyikan untuk anonim dan dev-bypass (identitas
developer) yang tidak punya sesi untuk di-logout.

### Shell integration

- `AppShell.tsx` — tombol di header, di samping avatar user.
- `TopNavShell.tsx` — tombol di header, di samping avatar user.
- `NoNavShell.tsx` — tombol di brand bar (kanan), untuk no-nav App privat.

## Kenapa

User ingin bisa logout setelah login (alur dev-auth). Sebelumnya tidak ada cara
keluar dari sesi; reload halaman adalah satu-satunya cara (session in-memory).

## Verifikasi

- `tsc --noEmit` + `vitest` (96 tests) lulus.
- E2E browser (dev-auth): login → tombol Log out muncul di header → klik →
  kembali ke `/login` → akses app lagi di-redirect ke login (sesi bersih).

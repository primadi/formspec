# Hapus Mode "Use API Token" dari Form Login

**Tanggal**: 2026-08-21 · **Sequence**: 019
**Plan**: session plan (dev-auth + login redirect) — follow-up

## Apa yang diubah

Menghapus mode **"Use API Token instead"** dari form login. Login sekarang
hanya via **username/password**. API token tetap didukung untuk akses
programatik dari app lain (via header `Authorization` / `X-FormSpec-Key`),
bukan lewat form login.

### `renderers/react-shadcn/src/shell/LoginScreen.tsx`

- Hapus state `mode` (`LoginMode`) dan `token`.
- `handleSubmit` hanya memanggil `loginWithPassword` (username/password).
- Hapus field "API Token" dan tombol toggle "Use API token instead" /
  "Use username & password instead".
- Update komentar header.

## Kenapa

User menilai mode paste token tidak relevan di form login — token dipakai saat
app diakses dari app lain (programatik), bukan oleh manusia lewat UI.

## Verifikasi

- `tsc --noEmit` + `vitest` (96 tests) lulus.
- E2E browser (dev-auth): form login hanya Username + Password + Sign In
  (toggle hilang); login admin/admin → dashboard render; logout tetap
  berfungsi.

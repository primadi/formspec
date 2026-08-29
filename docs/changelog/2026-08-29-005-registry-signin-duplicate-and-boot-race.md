# 2026-08-29-005 — Fix Sign in Registry: Duplikat, Dead Link, dan Boot Race

## Apa yang diubah

Tiga perbaikan terkait auth di App Registry (public no-nav App):

1. **Duplikat "Sign in" di header** (`registry/spec/apps/registry.yaml`) —
   menu App mendeklarasikan entry `Sign in` yang duplikat dengan kontrol
   bawaan `NoNavShell`. Entry menu dihapus; `NoNavShell` kini juga skip
   entry menu `/login` & `/register` saat derivasi nav (defensive).
2. **Sign in dead link saat unauthenticated** (`renderers/react-shadcn/src/App.tsx`) —
   auth guard me-redirect ke `/login` level workspace alih-alih in-app
   `{surfacePath}/login`, menyebabkan bounce balik. Guard kini mengarahkan
   ke path login in-app yang benar.
3. **Boot race: header tetap "Sign in" setelah login** (`renderers/react-shadcn/src/stores/session.ts`) —
   efek boot `AppSurface` (branch public) bisa memulai boot anonim kedua
   saat boot login masih in-flight; boot anonim selesai terakhir dan
   menimpa token (`me` dev auto-auth bukan `anonymous`, jadi tidak
   terdeteksi guard). `boot()` kini single-flight (boot anonim dedupe ke
   boot in-flight) + generation guard (boot dengan token eksplisit
   meng-invalidate boot lama).

## Kenapa

Login Registry tampak "tidak jalan": klik Sign in tidak bereaksi, dan
setelah login sukses header tetap menampilkan Sign in sampai reload manual.

## File terkena dampak

- `registry/spec/apps/registry.yaml`
- `renderers/react-shadcn/src/App.tsx`
- `renderers/react-shadcn/src/shell/NoNavShell.tsx`
- `renderers/react-shadcn/src/stores/session.ts`

## Verifikasi

- `tsc --noEmit` bersih.
- Siklus logout → login → header langsung "Log out" tanpa reload (browser test).
- Reload halaman tetap restore sesi dari sessionStorage.

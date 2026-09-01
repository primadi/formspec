# 2026-08-31-010 — UserMenu: avatar + dropdown Profile/Sign out di auth area

## Apa

Mengganti tombol logout polos di kanan atas dengan **avatar user menu**:

- `renderers/react-shadcn/src/shell/UserMenu.tsx` (baru): avatar bulat
  (inisial dari `me.user_id`) yang saat diklik membuka dropdown berisi
  identitas user (user_id + roles) dan aksi **Sign out** (clearSession →
  redirect `{surface}/login`).
- `renderers/react-shadcn/src/components/ui/dropdown-menu.tsx` (baru):
  komponen shadcn dropdown-menu (via `shadcn add dropdown-menu`).
- `renderers/react-shadcn/src/shell/AuthArea.tsx`: signed-in state kini
  merender `<UserMenu />` menggantikan `<LogoutButton />` (untuk mode
  `links` maupun `button`).
- `renderers/react-shadcn/src/shell/AuthArea.test.tsx`: test signed-in
  di-update — assert `User menu` (aria-label trigger) bukan `Log out`.

## Kenapa

UX standar aplikasi web: setelah sign-in, kanan atas menampilkan avatar
identitas; klik → menu dengan profil + sign out. LogoutButton lama hanya
ikon logout tanpa identitas.

## Fix blank screen (Base UI error #31)

Klik avatar pertama kali melempar `Base UI error #31` → layar blank.
Akar masalah: `DropdownMenuLabel` di base-ui adalah `Menu.GroupLabel`
yang **wajib** berada di dalam `<Menu.Group>` — tanpa wrapper
`DropdownMenuGroup`, `MenuGroupContext` hilang dan menu melempar error.
Fix: bungkus label dengan `<DropdownMenuGroup>` di `UserMenu.tsx`.
Juga ganti `onSelect` → `onClick` pada `DropdownMenuItem` (API base-ui
`Menu.Item` memakai `onClick`, bukan `onSelect` ala Radix).

## Verifikasi

- `tsc --noEmit` bersih; `vitest run src/shell` 11 pass.
- `make build-registry` sukses (SPA embed di-rebuild).
- Verifikasi manual via browser: klik avatar → dropdown terbuka
  (identitas + roles + Sign out); klik Sign out → redirect ke login.

## File terdampak

- `renderers/react-shadcn/src/shell/UserMenu.tsx` (baru)
- `renderers/react-shadcn/src/components/ui/dropdown-menu.tsx` (baru)
- `renderers/react-shadcn/src/shell/AuthArea.tsx`
- `renderers/react-shadcn/src/shell/AuthArea.test.tsx`

## Catatan

`LogoutButton.tsx` dipertahankan (masih dipakai UserMenu sebagai pola; bisa
dihapus bila tidak ada pemakaian lain). Field `MeResponse` belum punya
`display_name`/`username` — label menu sementara memakai `user_id`; bisa
ditingkatkan saat meta API mengekspos nama tampilan.

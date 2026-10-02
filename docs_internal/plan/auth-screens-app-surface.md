# Plan — Auth screen di permukaan App (`change-password` 404)

**Status:** selesai · **Tanggal:** 2026-10-02 · **Todo:** 14.c.5

## Masalah

`UserMenu` menavigasi ke `surfacePath("change-password")`
(`renderers/react-shadcn/src/shell/UserMenu.tsx:109`). Untuk permukaan App
(`/{ws}{root_url}`, mis. `/kafe/app/pos`) path itu **tidak punya route**:

- Route `change-password` hanya didaftarkan **top-level** di `App.tsx`, dan
  hanya untuk `_admin`: `/:workspace/_admin/change-password`.
- Permukaan App di-mount lewat splat `/:workspace/app/*`, dan `<Routes>`
  bersarang di `SurfaceShell` hanya berisi `bundle.pages` + route turunan CRUD +
  root App + catch-all. Auth slot (`AuthPage slot="change_password_page"`) tidak
  pernah terdaftar di sana.

Akibatnya item menu "Change Password" di App mana pun berakhir di catch-all
`Page not found`.

**Bukti observasi:** `http://localhost:5174/kafe/app/pos/change-password` →
heading "Page not found"; URL yang sama di `_admin`
(`/kafe/_admin/change-password`) merender form Change Password.

**Regresi dari mana:** commit `27266f9` ("auth screens & custom screens
spec-driven") mengubah UserMenu dari membuka `ChangePasswordDialog` (bekerja di
semua permukaan) menjadi `navigate(surfacePath("change-password"))` — dan hanya
mendaftarkan route admin.

## Kenapa bukan sekadar balik ke dialog

Kembali ke dialog mematikan jalur yang justru jadi tujuan refactor spec-driven:
`App.spec.auth.change_password_page` (override Page) hanya bisa dihormati kalau
permukaan App benar-benar merender `AuthPage slot="change_password_page"`.
Selain itu halaman penuh memberi chrome (sidebar) dan bisa di-bookmark.

## Perubahan

| File                                                      | Perubahan                                                                                                                                                                                                                                                                          | Effort |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| `renderers/react-shadcn/src/App.tsx`                      | Hoist `surfaceRelative` (= `surfacePath` − `mountPrefix`, logika yang sudah dipakai route root) dan daftarkan `<Route path={surfaceRelative + "/change-password"} element={<AuthPage slot="change_password_page" />} />` di `<Routes>` bersarang `SurfaceShell`, sebelum catch-all | small  |
| `renderers/react-shadcn/src/shell/ChangePasswordPage.tsx` | Sukses → `navigate(surfacePath())` (root permukaan aktif) menggantikan `/${workspace}/_admin` yang hardcoded — di permukaan App dulu melempar pengguna ke `_admin` (kandidat 403)                                                                                                  | small  |
| `renderers/react-shadcn/src/stores/*`                     | — (tidak berubah)                                                                                                                                                                                                                                                                  | —      |

`_admin` tidak terpengaruh: route statis top-level `/:workspace/_admin/change-password`
menang atas `/:workspace/_admin/*`, sehingga `SurfaceShell` (dan route bersarang
baru) tidak pernah ter-mount untuk path itu.

## Kenapa path relatif, bukan literal

`mountPrefix` = `/{ws}/app`, sedangkan `surfacePath` = `/{ws}{root_url}`. Untuk
root_url `/app/pos`, `<Routes>` bersarang melihat sisa `app/pos/...` (lihat
`docs/renderers/shadcn-shell/05-routing.md` §1 "Cara prefix dipasang"). Karena
itu path route wajib dihitung sebagai `surfacePath − mountPrefix`, sama seperti
route root App — menghindari bug "double `/app`".

## Verifikasi

1. `http://localhost:5174/kafe/app/pos/change-password` → form Change Password.
2. `http://localhost:5174/kafe/cafe-master/menu-categories` (App `kafe-qr`,
   root_url `/`) → `change-password` juga tersedia.
3. `http://localhost:5174/kafe/_admin/change-password` → tetap seperti sekarang
   (tanpa regresi).
4. `vitest` di `renderers/react-shadcn` (uji `router.shapes` + auth autofill).

# Menu: `MenuItem.Permissions` (RBAC di server) — cek mati di klien dihapus

**Tanggal**: 2026-09-25 · **Plan**: `docs_internal/plan/routing-docs-and-menu-visibility.md`
**Todo**: 5.22.2 · **Changelog terkait**: `2026-09-25-003` (`when` + gate)

## Apa yang diubah

- `pkg/spec/resources.go` — `MenuItem.Permissions []string` (baru), dengan
  komentar yang memisahkan dua sumbu visibilitas menu.
- `internal/ui/meta.go` — `filterMenu` menerima `PermissionChecker` dan membuang
  item yang `permissions`-nya tidak dipegang pemanggil (any-of, helper
  `menuItemAllowed`). `BuildBundle` meneruskan `can` yang sama dengan yang
  memutuskan entity ikut bundle.
- `renderers/react-shadcn/src/types/manifest.ts` — `MenuItem.permissions`.
- `renderers/react-shadcn/src/hooks/useResolvedMenu.ts` — cek `permissions` di
  klien **dihapus**.

## Kenapa

`filterMenuItem` (klien) sudah memeriksa `item.permissions`, dan
`internal/api/meta.go` menyebut "menu.permissions" — tetapi **field-nya tidak
pernah ada** di `pkg/spec` maupun tipe TS. Jadi satu-satunya cek RBAC menu di
sistem ini adalah cek mati yang (a) tidak pernah menerima data dan (b) berada di
tempat yang bisa di-bypass.

RBAC di renderer salah bukan hanya karena bisa di-bypass: ia akan **menyimpang
diam-diam** dari server begitu keduanya tidak lagi sama, dan tidak ada yang akan
gagal. Karena itu keputusannya satu arah — cek di klien dihapus, bukan
diduplikasi.

Konsekuensi yang diterima: `?admin=true` dan `?grants=true` memakai checker
always-true (`internal/api/meta.go`), sehingga kedua varian itu mengabaikan
`permissions` — memang dikehendaki (grants editor harus menawarkan semua
page/action agar admin bisa memberi yang tidak ia pegang sendiri).

## Bukti

`TestFilterMenuHonoursItemPermissions` (`internal/ui/registry_test.go`), 4
sub-test: tanpa permission → item tidak terkirim; any-of → satu cukup; checker
always-true → semua item tetap ada; grup dengan satu anak ter-gerbang → grup ikut
hilang. `go test ./internal/ui/` hijau. Cek klien dipin dari sisi sebaliknya oleh
`filterMenuItem — permissions is NOT a client concern`
(`src/hooks/useResolvedMenu.test.ts`), sehingga menambahkan kembali cek klien akan
gagal test.

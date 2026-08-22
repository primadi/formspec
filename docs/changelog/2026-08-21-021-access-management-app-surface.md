# Access Management di App Surface (/app)

**Tanggal**: 2026-08-21 · **Sequence**: 021
**Plan**: `docs/plan/access-management-ui.md` (follow-up)

## Apa yang diubah

Menambahkan **Access Management** ke app surface (`/app`) sehingga bisa diakses
dari menu app, tidak hanya admin surface (`/_admin`).

### `examples/cafe/spec/apps/cafe.yaml`

- Tambah `formspec.core` ke `spec.modules` (agar UI auth termuat di bundle app).
- Tambah menu item "Access Management" (`module: formspec.core`,
  `route: /access-management`).

### `examples/cafe/spec/modules/formspec.core/module.yaml` (baru)

Deklarasi module `formspec.core` di spec user. Tanpa ini, `Resolve` menolak
App yang mereferensikan module yang tidak ada di map `modules` user (module
auth adalah embedded, tidak otomatis masuk map tersebut).

### `internal/auth/module/module.yaml`

Ubah nama module dari `auth` → `formspec.core` agar konsisten dengan namespace
entities/UI (`formspec.core`) — sehingga App bisa mereferensikan module ini dan
`BuildBundle` (filter `appCtx.allows(module)`) mencocokkan module UI auth.

## Kenapa

User ingin manajemen user/role/hak akses bisa diakses dari menu app (`/app`),
tidak hanya admin.

## Verifikasi

- `go test ./...` + `tsc --noEmit` + `vitest` (96 tests) lulus.
- Cafe app bundle: menu `['Access Management', 'Dashboard', 'Transaksi',
  'Laporan', 'Master']`, pages termasuk `access-management`.
- SPA `/default/app/kafe` dan `/default/app/kafe/access-management` → 200.
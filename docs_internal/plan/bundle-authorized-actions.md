# Plan — Authorized actions di bundle (kafe 10.23 + 10.19)

**Tanggal**: 2026-09-24 · **Status**: In Progress
**Sumber**: `examples/kafe/gaps_found/TODO.md` 10.23 (baru) + 10.19, dan
investigasi `docs_internal/plan/app-entry-gate.md` §Lampiran 2.

## Masalah

Lapisan UI (route + tombol) **buta permission**. Terukur di sesi `kafe-qr`
(grant `{entity: cafe-master.menu-category, actions: [list, find]}`):

| Yang terjadi                                                                      | Bukti                                    |
| --------------------------------------------------------------------------------- | ---------------------------------------- |
| `/kafe/cafe-master/menu-categories/new` merender **modal "Create Menu Category"** | DOM yang dirender                        |
| `POST` entity yang sama                                                           | **401**                                  |
| Tombol "New" di POS untuk kasir (grant `[list, view]`)                            | tampil, lalu modal **tanpa tombol Save** |

Akar: `shell/router.tsx` mendaftarkan 4 route turunan per entity tanpa melihat
izin, dan `getLifecycle(entity)` hanya membaca `characteristic`+`lifecycle`.
Klien **tidak bisa** memperbaiki sendiri karena bundle tidak mengirim data izin
(`AppSummary` tanpa `public_entities`; `entity.actions: null`).

## Konstruksi

Satu field baru pada `EntitySchema` di bundle:

```jsonc
{
  "module": "cafe-master",
  "name": "menu-category",
  "authorized_actions": ["list", "find"],
}
```

- **Dihitung server, untuk PEMANGGIL INI**, memakai `can` yang sudah dipakai
  filter bundle (identitas atau grant publik — satu sumber kebenaran, jadi
  bundle & endpoint tak bisa berbeda).
- **Absen** (`null`/tidak ada) = klien jatuh ke jalur lama (`me.permissions`) —
  kompatibel, dan renderer lama tetap jalan.
- Cakupan aksi = aksi kanonik UI (`list, find, create, update, delete`), aksi
  lifecycle (`submit, cancel, amend, deactivate, reactivate`) bila entity
  memilikinya, dan aksi kustom dengan permission sendiri.

### Pemetaan aksi → permission

Memakai aturan yang sudah ada agar tidak ada kosakata ketiga:
`PermissionAction` rute (`find` → `view`, lihat `internal/api/descriptor.go`
`StandardRESTActions`) dan `entityActionPermission` di klien.

## Perubahan

| File                                           | Perubahan                                                                                                                                                                 |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/ui/meta.go`                          | `EntitySchema.AuthorizedActions []string` + diisi di loop `BuildBundle` (tempat `can` tersedia)                                                                           |
| `renderers/react-shadcn/src/types/manifest.ts` | `authorized_actions?: string[]`                                                                                                                                           |
| `.../src/engine/permissions.ts`                | `canDoEntityAction` membaca `authorized_actions` lebih dulu; `me.permissions` sebagai fallback                                                                            |
| `.../src/shell/router.tsx`                     | daftarkan route turunan hanya bila aksinya diizinkan (`list`→list, `create`→`/new`, `find`→`:id`, `update`→`:id/edit`); sisanya jatuh ke catch-all → **"Page not found"** |
| `.../src/kinds/table/TableRenderer.tsx`        | tombol "New" = `lifecycle.hasCreate && canDoEntityAction(me, entity, "create")`                                                                                           |

## Bukti yang harus ada setelah perbaikan

| Kasus                                                       | Sebelum                                | Sesudah                        |
| ----------------------------------------------------------- | -------------------------------------- | ------------------------------ |
| Sesi `kafe-qr` buka `/kafe/cafe-master/menu-categories/new` | modal create                           | **Page not found**             |
| Sesi `kafe-qr` buka `/{id}` (grant `find`)                  | daftar + toast "Failed to load record" | **terisi** (find diizinkan)    |
| Sesi `kafe-qr` list                                         | 200, tombol "New" tampil               | daftar tampil, **tanpa "New"** |
| kasir (POS) grant `[list, view]` → "New"                    | tampil (modal tanpa Save)              | **tidak tampil**               |
| kasir (POS) grant penuh → "New"                             | tampil                                 | **tetap tampil**               |
| anonim `POST`                                               | 401                                    | 401 (tidak berubah)            |

## Test pengunci

- Go (`internal/ui`): entity publik → `authorized_actions` **persis** himpunan
  grant; entity privat → mengikuti `can`; pemanggil tanpa izin → tidak ada.
- Vitest: `canDoEntityAction` menghormati `authorized_actions` (termasuk
  `authorized_actions: []` = tidak ada izin) dan fallback ke `permissions` bila
  field absen; `buildRoutes` tidak mendaftarkan `/new` tanpa `create`.

## Dependensi

Spec Resolution API §2 (bentuk bundle) menyebut bundle berisi entity schema —
penambahan field ini perlu diselaraskan di
`docs/spec/frontend/04-spec-resolution-api.md` §2 (bentuk `EntitySchema`).

## Temuan ketiga — kosakata aksi klien ≠ kosakata permission (ikut diperbaiki)

Diperiksa saat memetakan aksi → permission: klien memakai **`edit`** untuk baris
tabel (`engine/derive.ts:100-108`), sedangkan permission rutenya **`update`**
(`StandardRESTActions`: `find`→`view`, `update`→`update`). Terukur pada
`manajer` (punya `cafe-master.menu-categories.update`):

|                                                  |                                                                          |
| ------------------------------------------------ | ------------------------------------------------------------------------ |
| permission yang dihitung klien untuk aksi `edit` | `cafe-master.menu-categories.edit`                                       |
| permission yang sebenarnya diminta rute `update` | `cafe-master.menu-categories.update`                                     |
| hasil                                            | `.edit` tidak pernah ada → **tombol Edit hilang** padahal boleh mengubah |

Ini cacat yang **berlawanan arah** dengan 10.19/10.23 (yang menampilkan terlalu
banyak): di sini klien menyembunyikan kemampuan yang sah. Akarnya sama —
kosakata aksi klien tidak dipetakan ke permission resource. Karena itu
perbaikan ini memuat **peta aksi kanonik** (`view`→`find`, `edit`→`update`,
`delete`→`delete`) di satu tempat, dan `authorized_actions` dikirim memakai
kosakata permission (`find`, `update`) sehingga tak ada lagi nama aksi yang
tidak pernah punya permission.

## Sisa / temuan baru (dicatat, tidak dikerjakan di sini)

Ditemukan saat verifikasi, **di luar** 10.23/10.19 → item kafe **10.24**:

1. **Grant `find` tidak pernah cocok di bundle.** `publicEntityChecker`
   memetakan permission `…​.view` ke `grants[ref][action]` dengan `action` =
   `view`, sedangkan manifest menulis `find`. Terukur: `dining-table`
   (grant `[find]`) **tidak ikut bundle**, padahal
   `GET /_ui/entity/cafe-master/dining-table/{id}` anonim = **200**.
2. **Entity ber-grant `create` saja ikut hilang dari bundle.** Visibilitas
   hanya dihitung dari `list`/`view`; `table-session` (grant `[create, find]`)
   tidak ikut bundle walau `POST`-nya publik di rute.

Keduanya kelas **bundle ↔ endpoint tidak sepakat** (kafe 10.13 ⏸️).

## Estimasi

**medium** — 1 field backend + plumbing `can`, 3 titik klien (permissions,
router, table), plus test di kedua sisi.

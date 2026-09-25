# 2026-09-24-001 — Bundle mengirim `authorized_actions`; route & tombol berhenti menebak

**Plan**: `docs_internal/plan/bundle-authorized-actions.md`
**Todo**: item kafe **10.23 ⏸️** dan **10.19 ⏸️** (dua-duanya ditutup), **10.24 ⏸️**
(ditemukan & ditutup di jalur yang sama).

## Konteks

Lapisan UI buta permission. Terukur di sesi `kafe-qr` (grant
`{entity: cafe-master.menu-category, actions: [list, find]}`):
`/kafe/cafe-master/menu-categories/new` **merender modal "Create Menu Category"**
sementara `POST`-nya **401**. Klien tidak bisa memperbaikinya sendiri karena
bundle tidak mengirim data izin (`app` tanpa `public_entities`,
`entity.actions: null`).

## Yang diubah

`EntitySchema` di bundle kini membawa **`authorized_actions`** — himpunan aksi
yang boleh dilakukan **pemanggil ini**, dihitung server dengan checker yang
sama yang memutuskan entity itu ikut bundle atau tidak
(`internal/ui/meta.go:authorizedActions`, mengikuti aturan rute
`internal/api/generator.go`: summary tanpa CUD, lifecycle-free tanpa
submit/cancel/amend, soft-deactivate hanya bila dinyalakan).

Konsumen klien:

- `shell/router.tsx` — route turunan didaftarkan sesuai
  `authorized_actions`. Route yang tidak diizinkan **tetap didaftarkan** dengan
  elemen not-found, bukan dihilangkan: menghilangkannya membuat `/new`
  ditangkap `:id` (id literal `"new"`), yang lalu merender daftar dan **tampak
  berhasil** — lebih buruk daripada 404.
- `kinds/table/TableRenderer.tsx` — tombol "New" = `hasCreate &&
canDoEntityAction(…, "create")`.
- `kinds/page/DetailPage.tsx` — tombol "Edit" kini digerbangi `update`
  (sebelumnya hanya `hasSave`, sementara targetnya rute yang digerbangi).

**Kosakata aksi diselaraskan.** Klien memakai `view`/`edit`, resource meminta
`.view`/`.update`. Klien menghitung `.edit` — yang **tidak pernah ada** —
sehingga `manajer` yang memegang `.update` **kehilangan tombol Edit** yang
justru boleh ia pakai. Kini ada `resourceAction()` dan
`entityActionPermission()` yang memetakan `view→find`, `edit→update` di satu
tempat.

## Temuan yang ikut tertutup (10.24)

`publicEntityChecker` membandingkan aksi grant (`find`) dengan aksi permission
(`view`), jadi grant `[find]` **tidak pernah cocok**. Terukur sebelum
perbaikan: `cafe-master.dining-table` (grant `[find]`) **tidak ikut bundle**,
padahal `GET /_ui/entity/cafe-master/dining-table/{id}` anonim → **200**;
`cafe-order.table-session` (grant `[create, find]`) juga hilang karena
visibilitas hanya dihitung dari `list`/`view`. Setelah perbaikan keduanya ikut:
`dining-table → ['find']`, `table-session → ['find', 'create']`.

## File

`internal/ui/meta.go` (+`authorizedActions`, `entityActionPermission`,
`grantActionForPermission`), `internal/ui/authorized_actions_test.go` (baru),
`pkg/spec` (tidak berubah), `renderers/react-shadcn/src/types/manifest.ts`,
`.../src/engine/permissions.ts`, `.../src/shell/router.tsx`,
`.../src/shell/router.gating.test.tsx` (baru),
`.../src/kinds/table/TableRenderer.tsx`, `.../src/kinds/page/DetailPage.tsx`.

## Bukti

Bundle (sesi nyata): anonim `kafe-qr` → `menu-category ['list','find']`,
`order ['list','create']`, `dining-table ['find']`; kasir (POS) →
`menu-category ['list','find']`; manajer (POS) →
`['list','find','create','update']`.

Browser, sesi `kafe-qr` + grant `[list, find]`:

| Route                   | Sebelum                          | Sesudah                            |
| ----------------------- | -------------------------------- | ---------------------------------- |
| `…/menu-categories`     | daftar + tombol "New"            | daftar, **tanpa** "New"            |
| `…/menu-categories/new` | **modal "Create Menu Category"** | **Page not found**                 |
| `…/{id}/edit`           | daftar (route tertangkap)        | **Page not found**                 |
| `…/{id}`                | daftar + toast gagal             | **detail terisi** (find diizinkan) |

Tanpa regresi: **manajer** (POS) → "New" ada + modal create terbuka, aksi baris
`["View","Edit"]`; **kasir** (POS, hanya `list`+`view`) → "New" **tidak ada**,
aksi baris `["View"]` saja.

Test: `go test ./...` hijau; vitest **320 lulus** (22 file, +14 dari 306).
`router.gating.test.tsx` dan `permissions.test.ts` **dibuktikan gagal** saat
gate-nya dinonaktifkan (2 test gagal), jadi keduanya mengunci perilaku, bukan
sekadar menemani. `formspec validate` kafe: 85 manifest, 0 problem.

**Catatan jujur.** Route yang tidak diizinkan kini merender blok "Page not
found" **di dalam chrome App**, bukan 404 HTTP — karena SPA memang menyajikan
seluruh permukaan dari satu `index.html` (`/kafe/<apa pun>` → 200). Yang berubah
adalah apa yang **dirender**, dan itu yang diperiksa. Menjadikannya 404 HTTP
sesungguhnya menuntut server tahu rute klien — di luar cakupan item ini.

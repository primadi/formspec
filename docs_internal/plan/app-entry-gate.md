# Plan — Gerbang masuk App (App entry gate)

Sumber: usulan pemilik proyek 2026-09-24 — _"urutannya bukannya shell → page
utama → page entity? jadi kalau punya hak akses ke page entity, tetapi tidak
punya akses ke page utama, seharusnya auth gagal."_

## Usulan yang dinilai

Hierarki berjenjang: **Shell → Page utama → Page entity**, dengan otorisasi
**top-down** — akses ke page entity tidak sah bila tidak ada akses ke page
utama; ketiadaan akses ke page utama = **login gagal**.

## Yang sudah normative hari ini (harus disadari sebelum mengubah)

**1. Otorisasi berbasis halaman sudah ditolak secara eksplisit** —
`docs/spec/frontend/04-spec-resolution-api.md` §4:

> Model "bisa lihat halaman → implisit bisa simpan entity-nya" ditolak sebagai
> mekanisme enforcement: asal UI […] tidak bisa diverifikasi server — masalah
> _confused deputy_ klasik — dan client unmanaged (Flutter, API mentah) tidak
> pernah melewati "halaman" sama sekali.

Karena itu enforcement **selalu** di resource (`required_permission`). Grant
per-halaman tetap ada sebagai **UX granting**, tetapi **wajib dimaterialisasi**
menjadi permission resource konkret — bukan flag opaque "boleh lihat halaman".

**2. Hierarki resmi adalah Shell → App renderer → Page renderer → Component**
(`01-visual-hierarchy.md` §1) — **bukan** Shell → page utama → page entity.
Tidak ada konsep "page utama" di spec; yang ada halaman ber-`route: /` (opsional)
dan `Page.spec.permissions` (filter bundle, any-of —
`internal/ui/meta.go:allowedPage`). Page juga **datar**: tidak ada relasi
parent/child antar Page.

## Fakta terukur (kafe, dev server, 2026-09-24)

| Fakta                                                                   | Bukti                                                                                                               |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `kafe-pos` & `kafe-kds` **tidak punya** halaman `route: /`              | bundle `?app=kafe-pos` → `home: []`; kafe-kds → `home: []`                                                          |
| Keduanya tetap **berfungsi**                                            | kasir → `/kafe/app/pos/cafe-order/orders` 21 records; barista → KDS                                                 |
| Gerbang setara untuk App **tidak ada**                                  | `grep AccessPermission` → hanya `_admin.access`; mount SPA App digerbangi allowlist workspace, **bukan** permission |
| `_admin.access` **tidak dimiliki siapa pun** di kafe (termasuk manajer) | 5 akun seed → `_admin.access: TIDAK`; tidak ada seed `workspace-owner`/`app-owner`                                  |
| `app-owner` = wildcard `"*"`, **belum per-App**                         | `internal/auth/role.go:82` ("single-server approximation; per-App scoping deferred")                                |

**Kesimpulan fakta:** kalau aturan "tanpa page utama → auth gagal" diterapkan
apa adanya hari ini, **kasir dan barista tidak akan bisa login** — karena App
kerja mereka memang tidak punya halaman `route: /`. Aturan itu mematikan dua
dari tiga App kafe.

## Koreksi yang disarankan

Pisahkan **dua hal** yang di usulan tercampur:

| Pertanyaan                                          | Jawaban yang disarankan                                                                                                                                                             |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Apakah check-nya top-down (butuh "page utama")?** | **Tidak.** Yang bermakna bukan "punya halaman route `/`", melainkan **"punya sesuatu di App ini"** — entity/route yang bisa dibuka. Menuntut route `/` akan mematikan App yang sah. |
| **Apakah enforcement-nya di halaman?**              | **Tidak.** Spec sudah menolaknya (§4, _confused deputy_). Gerbang = **satu permission per App**, sama persis pola `_admin.access` yang sudah berjalan.                              |

Jadi bentuknya: **`_admin.access` digeneralisasi menjadi gerbang per-App**, bukan
hierarki halaman. Usulan user tetap terpenuhi maksudnya ("tidak punya akses ke
App ⇒ gagal"), tanpa melanggar §4 dan tanpa mematikan App tanpa route `/`.

## Konstruksi yang diusulkan

```yaml
kind: App
spec:
  access: private
  # Permission yang harus dipegang caller untuk MASUK App ini.
  # Absen = perilaku lama (siapa pun yang terautentikasi boleh masuk).
  # Nama permission mengikuti konvensi resource biasa → auditable & revocable.
  access_permission: cafe-pos.access
```

Aturan:

1. **Enforcement di bundle, bukan di login.** `GET /_meta/ui?app=X` dijawab
   **403** bila caller tidak memegang `X.access_permission`. Di login, token
   tetap diterbitkan (sama seperti `_admin`: token ada, bundle-nya yang
   bergerbang) — supaya _switch context_ dan multi-App tetap mungkin.
2. **Absen = perilaku lama.** App tanpa `access_permission` tidak bergerbang —
   kompatibel penuh, tidak ada migration.
3. **`_admin.access` menjadi kasus khusus** dari mekanisme yang sama (App
   `_admin`), bukan kode terpisah.
4. **Grant-nya per-halaman** (UX yang sudah ada): role boleh menulis
   `{ page: "pos-workbench", actions: [view] }`, dan materializer
   (`internal/auth/materialize.go`) **menurunkan** `cafe-pos.access` dari
   halaman yang di-grant. Jadi penulis manifest tetap menulis halaman; yang
   ditegakkan tetap permission resource.
   _Kelayakan sudah diperiksa:_ `entityFootprint`/`navigationFootprint` memang
   menghasilkan permission per entitas/navigasi (`{module}.{plural}.{action}`),
   dan `permission.ValidatePermissionFormat` menerima bentuk 2+ segmen
   (`{module}.{key}`) — jadi `cafe-pos.access` sah sebagai string permission
   biasa, bukan bentuk baru.
5. **Lubang yang sekaligus tertutup:** App privat berisi 0 entity karena 0
   permission akan **ditolak saat bundle** (kafe **10.22 ⏸️**), dan
   `POST /_ui/auth/login` dengan `app` tak dikenal jadi **400**, bukan 200
   dengan sesi hampa (kafe **10.21 ⏸️**).

## Sisa yang tetap terbuka sesudah ini

Gerbang App **tidak** menyelesaikan App publik tanpa kontrol auth (kafe
**10.18 ⏸️**): App publik _memang_ tak bertuan, jadi "gagal login" bukan jawaban
yang benar di sana — yang kurang adalah jalan masuk/keluar (chrome region,
`docs_internal/plan/chrome-composition-spec.md` §Opsi region+attach).

## Pertanyaan yang menunggu keputusan

1. **Bentuk deklarasi**: `access_permission` (satu string di App) vs
   `entry: {page: pos-workbench}` (merujuk halaman, diturunkan ke permission).
   Yang kedua lebih dekat ke UX granting; yang pertama lebih eksplisit/auditable.
2. **Apakah pemeriksaannya juga menutup _mount SPA_** (mis. `GET
/kafe/app/pos` → 403, bukan menyerahkan SPA lalu bundle-nya 403), atau cukup
   bundle-nya saja.
3. **Apakah App publik boleh mendeklarasikan `access_permission`** (gerbang
   untuk sesi terautentikasi, sementara anonim tetap boleh) — atau field ini
   hanya sah untuk `access: private`.

## Estimasi

**medium** — satu field + validasi di `pkg/spec`, pemeriksaan di `internal/api/meta.go`
(bundle) + penurunan di `internal/auth/materialize.go`, 400 untuk `app` tak
dikenal di `HandleLogin`, plus test pengunci per kasus di atas.

## Lampiran — kenapa menu kosong itu BUKAN gejala permission (diukur 2026-09-24)

Pertanyaan pemilik proyek: _"user punya permission ke entity menu-categories,
tapi tidak punya permission ke `/kafe`, akibatnya page menu-categories muncul
tapi tanpa menu — benar begitu?"_ **Tidak** — dan sebabnya penting, karena
tampak seperti konfirmasi bahwa otorisasi halaman bekerja padahal bukan.

**Bukti pemisah: user SAMA (`kasir`), permission entity SAMA, App berbeda.**

| App        | `chrome.nav` | `/cafe-master/menu-categories` di menu?  |
| ---------- | ------------ | ---------------------------------------- |
| `kafe-pos` | `menu`       | **ADA** (`/cafe-master/menu-categories`) |
| `kafe-qr`  | `none`       | **TIDAK ADA**                            |

Kalau menunya difilter oleh kurangnya akses, baris pertama tidak mungkin
"ADA" untuk user yang sama. Yang berbeda hanya **App**, bukan permission.

**Tiga sebab terpisah — jangan dicampur:**

1. **`no-nav` mengosongkan nav secara archetype, bukan karena permission.**
   `resolveChrome()` memberi `nav: none` untuk `no-nav`; `kafe-qr` adalah
   `no-nav`. Bukti: `barista` → `kafe-kds` (juga `no-nav`) **punya 5
   permission** tetapi `nav=none`, `menu=0` — permission ada, menu tetap tidak
   dirender. Jadi `nav: none` murni keputusan chrome.
2. **`kafe-qr` memang tidak punya menu untuk diresolusi.** Manifestnya
   mengosongkan menu (`apps/kafe-qr.yaml:61` — _"menu dikosongkan: `no-nav`
   default tidak merender nav sama sekali"_), sedangkan `kafe-pos` memakai
   **adopt node** lima kali (`- type: module`). Menu App adalah _curated cart_
   yang **harus diadopsi/ditulis**; tidak ada menu turunan otomatis di
   permukaan App (`useResolvedMenu`: _"Entities not wired into the menu do not
   appear here at all — no derived fallback"_).
3. **Filter menu yang ada memang permission-aware, tetapi lewat mekanisme lain.**
   Server (`filterMenu`, `internal/ui/meta.go:781`) menyaring entri menu
   terhadap **isi bundle** (route yang tidak ada di bundle ikut dibuang), dan
   klien menyaring lagi lewat `menu[].permissions` / `when` (`filterMenuItem`).
   Jadi kalau sebuah menu entri dibuang, penyebabnya bisa: (a) entity-nya tidak
   ikut bundle (0 permission), atau (b) halaman/nav-nya memang tidak ada.

**Diukur lintas role di `kafe-pos`** (semuanya `nav=menu`): entity di bundle
12/6/11/23 untuk kasir/pelayan/supervisor/manajer, menu 6/5/6/9 node — dan
**tidak ada satu pun entity yang ikut bundle tapi route-nya hilang dari menu**.
Jadi pada surface ber-nav, menu dan bundle konsisten; yang kosong adalah App
tanpa nav.

**Konsekuensi untuk rencana ini:** kondisi yang dilaporkan user **bukan**
bukti bahwa "akses page entity tanpa akses page utama" sedang terjadi. Yang
benar-benar terjadi di `/kafe` adalah `no-nav` + menu kosong + fallback
`DefaultRedirect` ke derived list (kafe **10.20 ⏸️**). Gerbang App
(`access_permission`) tetap layak sebagai sinyal jujur untuk App privat yang
tak memuat role pemakainya (kafe **10.22 ⏸️**), tetapi ia **tidak** akan
mengubah menu `kafe-qr` menjadi terisi — itu urusan kelengkapan manifest
(halaman depan + `chrome: {nav: menu}`), bukan otorisasi.

## Lampiran 2 — lapisan ROUTE klien buta permission (kafe 10.23 ⏸️, diukur 2026-09-24)

Koreksi atas Lampiran 1: benar bahwa menunya kosong bukan karena permission.
Tetapi ada cacat **kedua** yang lebih tajam, dan itu yang pemilik proyek tunjuk:
**surface publik tetap mendaftarkan seluruh CRUD turunan** untuk setiap entity
yang ikut bundle, tanpa melihat grant.

Bukti di browser (sesi `kafe-qr`, `kasir`, anonim-terautorisasi), dibaca dari
**DOM yang benar-benar dirender** — bukan status HTTP, sebab `/kafe/<apa saja>`
selalu 200 dari SPA catch-all (dibuktikan: `/kafe/route-acak-apapun` → 200):

| Route yang dikunjungi                         | Yang benar-benar dirender                                     |
| --------------------------------------------- | ------------------------------------------------------------- |
| `/kafe/cafe-master/menu-categories/new`       | **modal "Create Menu Category"** + tombol "New Entry"         |
| `/kafe/cafe-master/menu-categories/{id}`      | daftar + dua toast **"Failed to load record"** (404 dari API) |
| `/kafe/cafe-master/menu-categories/{id}/edit` | jatuh ke daftar (route tampak terdaftar, data 404)            |

Sementara operasi yang sama di API: `POST`/`PATCH`/`DELETE` → **401**.

**Akar di klien** — `shell/router.tsx` mendaftarkan empat route per entity
(list, `new`, `:id`, `:id/edit`) dengan komentar _"Derived CRUD routes per
entity"_, dan **tidak pernah menyebut** grant, lifecycle-hasCreate, maupun
permission; ia hanya menyalin `bundle.entities`. Sumber affordance-nya juga
buta permission: `getLifecycle(entity)` hanya membaca `characteristic` +
`lifecycle` — pemeriksaan `canDoEntityAction` baru muncul **di dalam** komponen
(TableRenderer/FormRenderer), jadi route & modalnya tetap dibuat.

**Kenapa klien tidak bisa memperbaikinya sendiri hari ini:** bundle **tidak
mengirim** grant-nya. `AppSummary` yang sampai ke klien berisi `access`,
`chrome`, `root_url`, … tetapi **tanpa `public_entities`**; entity
`menu-category` datang dengan `actions: null`, `exposed: false`,
`lifecycle: plain_crud` — cukup untuk memunculkan tombol, tidak cukup untuk
tahu bahwa hanya `list`+`find` yang boleh.

**Kenapa ini tetap bug meski server menolak:** penolakan sesungguhnya bekerja
(tidak ada data bocor), tetapi permukaan publik **menawarkan** kemampuan yang
tidak ada — dan itu tiga cacat sekaligus: (a) permukaan publik menyajikan
lajur tulis yang tak pernah diberikan (kafe **10.19 ⏸️** diperluas), (b)
rute/tombol yang selalu gagal dengan pesan tanpa sebab (kafe **10.19**), dan
(c) lajur `no-nav` publik — yang seharusnya murni baca — tampil seperti App
admin (kafe **10.20 ⏸️**).

**Arah perbaikan yang mungkin** (belum diputuskan):

1. **Bundle mengirim grant** untuk App publik (`app.public_entities` atau
   `entities[].granted_actions`), lalu `buildRoutes`/`getLifecycle` hanya
   mendaftarkan route & tombol yang di-grant. Menyelesaikan akar; menyentuh
   kontrak Spec Resolution API (§2 bentuk bundle) → perlu perubahan spec.
2. **Server menolak lebih awal**: `/_meta/ui?app=…` tidak mengirim entity yang
   tidak punya tindakan apa pun bagi pemanggil (sudah sebagian: entity tanpa
   `list`/`view` dibuang) — tetapi `menu-category` **punya** `list`, jadi ia
   sah ikut bundle; yang bermasalah adalah **route turunan**, bukan entity-nya.
   Jadi opsi ini tidak cukup sendirian.
3. **renderer tidak menurunkan CRUD** bila App `access: public` dan tidak ada
   grant tulis — varian paling kecil, tetapi menyembunyikan gejala, bukan
   menyatakan kontrak.

Catatan penting: opsi (1) **juga** menutup kafe **10.19 ⏸️** (tombol "New"
tanpa permission `create`) karena akarnya sama — affordance klien tidak
memiliki data izin. Karena itu 10.19 dan lampiran ini sebaiknya dikerjakan
sebagai **satu paket**.

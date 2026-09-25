# 2026-09-22-015 — Menu App difilter per-role; App publik tidak lagi membocorkan permukaan admin

**Konteks.** Verifikasi 10.9 (katalog kafe di browser sungguhan) membuka kelas bug
yang tidak terlihat pada item 10.1–10.8: semuanya diuji lewat HTTP endpoint, bukan
lewat UI yang benar-benar dijalankan. Membuka dev server di browser menemukan
**tujuh bug**, lima di antaranya menghasilkan halaman kosong atau 404 tanpa error.

## Yang diubah

### 1. Sesi login di-scope dengan NAMA App, bukan segmen URL

`src/shell/LoginPage.tsx` mengirim `app` dari segmen URL (`pos`), sementara role
dideklarasikan `app: kafe-pos`. `PermissionResolver` melewati setiap role yang
`app`-nya berbeda, jadi sesi autentikasi mendapat **0 permission** dan App privat
mengirim bundle **kosong** (menu tetap tampil karena dibangun dari manifest App,
bukan dari permission) — sidebar penuh item di atas halaman kosong. Terukur:
`app=kafe-pos` → 20 entity; `app=pos` → 0.

### 2. Redirect pasca-login tidak lagi menjatuhkan prefix App

`handleLogin` mundur ke `/{workspace}`, membuang `/app/pos`, sehingga bundle
diselesaikan untuk App pemilik root (kafe: `kafe-qr` yang **publik**) — pengguna
mendarat di App yang berbeda dari yang ia masuki. Kini mendarat di `root_url` App
yang diresolve (`surfacePath − mountPrefix`).

### 3. Catch-all berhenti mengalihkan secara diam-diam

Route yang tidak ada **dialihkan ke root surface**, sehingga link mati tampak
seperti link hidup: entri menu yang menunjuk entity tanpa grant merender daftar
entity PERTAMA, dan pengguna mengira halamannya terbuka. Kini 404 jujur.

### 4. Root App benar-benar cocok (App ber-`root_url` 404)

`index` hanya cocok bila sisa splat kosong — benar untuk `_admin`, salah untuk App
ber-`root_url` (sisanya `app/pos`). Root App kini route eksplisit dari
`surfacePath − mountPrefix`, jadi role seperti `dapur` tidak lagi mendarat di 404.

### 5. Menu App difilter terhadap isi bundle, bukan terhadap permission

Menu ditulis SEKALI untuk semua role sementara entity/form/report/dashboard
difilter per role, jadi entri terkurasi rutin menunjuk sesuatu yang tidak bisa
dibuka pemanggil. Aturan yang benar adalah **"apakah route ini ada di bundle ini"**,
bukan "apakah pemanggil punya permission" — keduanya berbeda tepat di kasus yang
penting: dashboard yang seluruh widget-nya terfilter tidak ada di bundle walau
pemanggil masih punya grant langsung atasnya. Terukur (manajer): link
"Pelanggan"/"Loyalitas" mati karena entity-nya tidak pernah ikut bundle;
terukur (dapur): link "Ringkasan Pemilik" tetap ada padahal dashboard-nya dibuang.

### 6. Dashboard hanya dikirim bila ada widget yang hidup

Dashboard dikirim murni karena `appCtx.allows(module)`, sementara widget
entity-backed difilter permission. Terukur (dapur): bundle memuat
`owner-overview` dengan daftar widget KOSONG → satu-satunya entri menu role itu
membuka empat placeholder "Widget definition not found". Placeholder-nya kini
jujur ("Not available for your role").

### 7. `entities: null` menghancurkan panel

`Bundle.Entities` tidak diinisialisasi, jadi caller tanpa entity menerima
`"entities": null`, dan `for (const e of bundle.entities)` melempar
`e.entities is not iterable` — ErrorBoundary mematikan panel yang tidak ada
hubungannya dengan entity (terukur: dapur membuka dashboard → crash di
`getWidget`). Server kini selalu mengirim `[]`, dan renderer tidak lagi bergantung
pada itu.

## Dua paparan (exposure) di App publik — ditemukan saat memverifikasi #1–#7

`AppContext.allows` **selalu** meloloskan `core` dan `formspec.core` (module
framework, tidak pernah dideklarasikan App). Benar untuk App privat, di mana
permission pemanggil tetap menjaga setiap entity. App publik berbeda:
`internal/api/meta.go` mengganti permission checker dengan **selalu-true** karena
tidak ada sesi untuk diperiksa, sehingga `allows` menjadi satu-satunya gerbang —
dan ia meloloskan segalanya.

- **Admin framework bocor ke pengunjung anonim.** `GET /_meta/ui?app=kafe-qr`
  (tanpa autentikasi) mengembalikan entity `formspec.core` (user, role, api-key,
  session) **dan** halaman `/access-management`; browser anonim merender tabel
  "Access Management" yang kolomnya termasuk `Password Hash`. Endpoint data tetap
  401, jadi tidak ada baris yang bocor — permukaan adminnya sendiri yang tidak
  boleh terjangkau. Kini hanya layar `/_auth/*` yang lolos (App publik
  membutuhkannya untuk menandatangani tamu).
- **`public_entities` diabaikan di bundle.** App publik mengirim SELURUH module
  yang di-mount: 13 entity termasuk `cafe-master.members` (nomor HP pelanggan),
  `employees`, `menu-item-prices`, `cafe-order.shifts`, `cash-movements`.
  Endpoint data tetap menegakkan allowlist, jadi tidak ada baris yang bocor —
  tetapi **skema** data privat dibagikan dan SPA membuat route untuknya.
  `BuildBundle` kini menurunkan checker dari `public_entities`: 13 → **4** entity,
  persis allowlist.

## Dampak & test pengunci

`internal/ui/meta.go` (filter menu, indeks entity, gating dashboard, gate
`publicScope` + `owns`, checker allowlist), `internal/api/meta.go` (checker
publik), `internal/auth/user.go` (sudah, changelog 013), `renderers/react-shadcn/`
(`App.tsx`, `stores/meta.ts`, `shell/LoginPage.tsx`, `kinds/dashboard/`),
`examples/kafe/spec/modules/formspec.core/seeds/roles.yaml` (grant pelanggan/
loyalitas/sesi-meja untuk kasir/pelayan/supervisor/manajer).

Test pengunci (semuanya dibuktikan **gagal** saat fix-nya dinonaktifkan):
`TestBuildBundle_MenuDropsUnreachableItems`, `TestBuildBundle_MenuDropsRoutesTheBundleRemoved`,
`TestBuildBundle_DashboardFollowsItsWidgets`, `TestBuildBundle_PublicAppHidesFrameworkAdmin`
(`internal/ui/menu_filter_test.go`), `meta.appname.test.ts`, `meta.test.ts`
(null entities), `App` root + catch-all.

**Verifikasi akhir di browser** (6 role × semua entri menu): **0 link mati, 0
placeholder**. Bundle anonim `kafe-qr`: 4 entity (persis allowlist), hanya
`/_auth/*` dari framework.

Referensi plan: `docs_internal/plan/todo.md`; item kafe `10.10`–`10.13`.

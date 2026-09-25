# 2026-09-23-003 — Grant publik sebagai floor; pemanggil login tidak lagi lebih buruk dari tamu

**Plan**: `docs_internal/plan/public-grant-signed-in-floor.md`
**Todo**: item baru `10.18` + `10.19` di `examples/kafe/gaps_found/TODO.md`.

**Konteks.** Laporan pemakaian nyata: login `kasir`/`kafe123`, buka `/kafe` →
dialihkan ke `/kafe/cafe-master/menu-categories` → halaman menampilkan
**"resource not found"** dan tidak bisa dipakai. Direproduksi: halaman katalog
publik `kafe-qr` yang terbuka untuk anonim (200, 4 kategori) menjawab **404**
untuk permintaan yang sama begitu membawa sesi.

## Yang diubah

`RequirePermissionOrAnonymous` menuntut permission penuh dari pemanggil
terautentikasi, sedangkan grant publik hanya mengotorisasi anonim — sehingga
sekadar login membuat pengguna kehilangan akses yang sedetik sebelumnya
tersedia. Terukur (sesi `app=kafe-qr`, 0 permission): `list menu-category`
**404**, `create order` **401**; sesi `app=kafe-pos` (33 permission) 200.

Grant kini adalah **floor**: `max(permission miliknya, grant publik)`.
Pemanggil yang tidak memegang permission jatuh ke grant dan ditandai
`withPublicGrantAuth`; scope grant (dan pelewatan `row_scope from: session`)
mengikutinya, sehingga fallback tidak pernah memberi baris lebih banyak daripada
yang diterima tamu. Pemegang permission tidak berubah jalurnya — surface POS yang
tidak membawa token tamu tetap tidak terfilter (#45 utuh).

**File**: `internal/api/middleware.go` (fallback + marker context),
`internal/api/scope.go` (`applyPublicScope`, `applyRowScope`),
`internal/api/public_entities_test.go` (+4 test), `docs/spec/frontend/05-app-kinds.md`
§1.1 (kontrak diselaraskan — sebelumnya menuliskan aturan lama).

## Bukti

| Kasus                                               | Sebelum        | Sesudah                            |
| --------------------------------------------------- | -------------- | ---------------------------------- |
| anonim `list menu-category`                         | 200            | 200                                |
| sesi 0-perm `list menu-category`                    | **404**        | **200**                            |
| sesi 0-perm `list menu-item`                        | 404            | **200**                            |
| sesi 0-perm `list order` tanpa token                | 404            | **403 fail-closed**                |
| sesi 0-perm `list order?guest_token=TOK-E2E-1`      | 404            | **total 1** (hanya baris itu)      |
| sesi 0-perm `list order?guest_token=NOPE`           | 404            | **total 0**                        |
| sesi 0-perm `list employee` (tanpa grant)           | 404            | **404** (tetap tertutup)           |
| sesi 0-perm `list dining-table` (grant `find` saja) | 404            | **404**                            |
| sesi ber-perm `list order` tanpa token              | 200 (21 baris) | **200 (21 baris)** — tidak berubah |
| anonim `list order` tanpa token                     | 403            | 403                                |

Diverifikasi juga di browser pada alur persis yang dilaporkan: login di
`/kafe/login` → katalog kafe terbuka dengan **4 kategori** (sebelumnya
"resource not found"); POS `/kafe/app/pos/login` → `/kafe/app/pos` tetap normal
(sidebar + 21 pesanan). `go test ./...` hijau.

## Sisa (→ todo kafe)

Saat memverifikasi ulang laporan pengguna ("mengapa kasir hanya menampilkan Menu
Category?"), tiga keluhan ternyata **bukan** bug izin dan dicatat terpisah:

1. **10.18 ⏸️** — App publik tanpa kontrol auth (`chrome.auth: none`, default
   `no-nav`) tidak menyediakan sign-out, jadi pengunjung yang sudah login tidak
   bisa keluar dari permukaan publik dari UI. (Di POS, `UserMenu` → Sign out
   memang ada.)
2. **10.19 ⏸️** — tombol "New" dirender dari lifecycle saja tanpa melihat
   permission `create`, sehingga pemanggil yang tidak boleh membuat row
   mendapat modal **tanpa tombol simpan**. Cakupannya bukan hanya surface
   publik: kasir juga melihatnya di POS.
3. **10.20 ⏸️** — App publik ber-`root_url: /` mendaratkan pengunjung di
   **daftar entity pertama** (`cafe-master/menu-category`), bukan halaman yang
   bermakna; `kafe-qr` tidak punya page ber-route `/` dan menunya kosong
   (`no-nav`). Inilah yang membuat `/kafe` tampak seperti App admin yang rusak —
   katalog pelanggan yang sebenarnya sehat ada di `/kafe/menu/{session_id}`.
4. **10.22 ⏸️** — App **privat** berisi 0 entity tidak bergerbang: `barista`
   (role `app: kafe-kds`) login ke `app=kafe-pos` → 200 + bundle 0 entity →
   SPA menampilkan "No entities found", padahal manifestnya sehat dan yang
   kurang hanya hak akses. Preseden benar sudah ada: `_admin` → 403.
5. **10.21 ⏸️** — login menerima `app` tak dikenal (`app-ngawur` → **200**).

**Catatan 2026-09-24 (temuan lanjutan, mengubah prioritas).** Penelusuran atas
keluhan "kasir hanya melihat Menu Category" menemukan akar struktural yang lebih
besar dari 10.18–10.20: **kontrol auth hidup hanya di chrome**. `AuthArea`
dirender di dalam header ketiga shell dan mengembalikan `null` untuk
`auth: none`; deklarasi auth non-chrome (`auth_action`) adalah closed set tanpa
`logout`; Page tidak punya blok/CTA auth; `useAutoLogout` tidak dipasang pada
permukaan publik; dan `private` + `no-nav` + `chrome: {auth: none}` **lolos
validasi**. Jadi "hak akses menu" bersifat _mandatory-by-omission_ — manifest
tetap hijau walau pengguna tidak punya jalan masuk/keluar. Tiga pertanyaan
keputusan dicatat di `docs_internal/plan/chrome-composition-spec.md`
§"Sisa yang belum diputuskan".

**Catatan 2026-09-24 (kedua): lapisan route klien buta permission.** Menjawab
pertanyaan pemilik proyek _"page diakses melalui kafe-qr, seharusnya 404"_:
benar bahwa penyebab awal bukan menu — tetapi ditemukan cacat **kedua** yang
lebih tajam. `shell/router.tsx` mendaftarkan empat route CRUD turunan per entity
(list, `new`, `:id`, `:id/edit`) tanpa melihat grant, dan `getLifecycle(entity)`
hanya membaca `characteristic`+`lifecycle`. Terukur di browser (sesi `kafe-qr`)
pada grant `[list, find]`: `/kafe/cafe-master/menu-categories/new`
**merender modal "Create Menu Category"**, sementara `POST`-nya **401**. Bundle
juga **tidak mengirim** grant-nya (`AppSummary` tanpa `public_entities`;
`entity.actions: null`), jadi klien tidak bisa memperbaikinya sendiri. Dicatat
sebagai kafe **10.23 ⏸️** (satu paket dengan **10.19 ⏸️**), dengan bukti DOM di
`docs_internal/plan/app-entry-gate.md` §Lampiran 2.

Catatan metode yang berlaku untuk penelusuran ini: **status HTTP bukan bukti**
route terdaftar di SPA — `/kafe/<apa pun>` selalu 200 dari catch-all (diuji
`/kafe/route-acak-apapun` → 200). Yang membuktikan adalah DOM yang dirender.

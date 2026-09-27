# Catatan: temuan intake anonim kafe (2026-09-26)

**Pemicu:** pemilik bertanya dua hal saat menyelidiki `/kafe`:

1. apa guna `public_entities` + `modules: [cafe-master, cafe-order]` di
   `kafe-qr`, dan apa kaitannya dengan landing page;
2. bagaimana bila pelanggan memilih meja yang **sedang dipakai** (bisa melihat
   pesanan pelanggan lain), dan bagaimana melawan orang iseng yang **membuat
   sesi di semua meja** sehingga meja tampak penuh.

Tidak ada kode yang diubah — ini pencatatan temuan + penunjukan item yang
menutupnya. Lihat `examples/kafe/gaps_found/TODO.md` **10.34 / 10.35 / 10.36**.

## Jawaban 1 — `public_entities` vs `modules`, dan kaitannya ke landing page

`modules:` = **seleksi manifest**. `BuildBundle` membuang seluruh manifest
(Entity/Page/Form/Table/…) yang module-nya tidak ada di daftar
(`internal/ui/meta.go` `appCtx.allows(e.Module)`). `cafe-master` + `cafe-order`
dipilih karena katalog menu ada di `cafe-master` dan pemesanan di `cafe-order`.

`public_entities:` = **gerbang auth anonim**. Tanpa field ini, `access: public`
memberi anonim `list`/`find`/`create` untuk **seluruh entity** di module yang
di-mount (`legacyPublicActions`, `internal/api/router.go`). Daftar ini
menyempitkannya menjadi pasangan entity+action tertentu, plus `scope` baris
per-permukaan.

**Bukti pengamanan ini bekerja** (tanpa header auth, dev server 2026-09-26):

| Permintaan                      | Hasil                          |
| ------------------------------- | ------------------------------ |
| `list cafe-master/dining-table` | **401** (list tidak diberikan) |
| `list cafe-order/table-session` | **401**                        |
| `list cafe-master/menu-item`    | 200 (granted)                  |

**Kaitan ke landing page: tidak ada.** `public_entities` mengatur _apa yang
boleh disentuh anonim_; `DefaultRedirect` mencari _route untuk dijadikan
landing_ (urutan: halaman `route: /` → `bundle.menu` pertama → entity pertama).
Keduanya lewat `bundle.entities` kebetulan, bukan sebagai kontrak.

Efek samping yang jujur dicatat: karena `DefaultRedirect` memilih entity
pertama yang **punya `list`**, dan di `kafe-qr` yang punya `list` adalah
`menu-category`/`menu-item`/`menu-item-price`/`order` (`dining-table` hanya
`find`), kafe mendarat di `cafe-master/dining-tables` karena **`bundle.entities`
urut alfabetis** (`cafe-master/dining-table` < `cafe-master/menu-category`) dan
`DefaultRedirect` **tidak** memeriksa `authorized_actions` — sehingga jatuh ke
catch-all "Page not found". Itu **kafe 10.20 ⏸️** (bagian kedua: fallback yang
sadar `public_entities`). Landing page `route: /` tetap jawaban yang benar
supaya (3) tidak pernah terpakai.

Jebakan yang terverifikasi hari ini: Page di-scope per **module**, bukan per
App. Terukur: bundle `?app=kafe-qr` ikut memuat `pos-workbench`
(`route: /pos`, module `cafe-order`). Jadi menaruh `route: /` di
`cafe-master`/`cafe-order` akan membuat landing pelanggan muncul juga di root
`/app/pos`. Bentuk penutupnya (field `home_page` per-App vs module khusus)
belum diputuskan — **tidak ditebak**.

## Jawaban 2 — meja "kosong", pesanan orang lain, dan pesan iseng

**Temuan terukur: `POST table-session` anonim dijawab 422, bukan 403** — jadi
request itu lolos otorisasi (`table-session` publik: `find`+`create`). Hanya
kegagalan validasi relasi yang menghentikannya. Artinya bukan otorisasi yang
menahan; isian yang benar akan lolos.

Tiga hal yang membedakan masalah nyata dari yang bukan:

- **Pada desain QR aslinya, tamu tidak pernah memilih meja.** QR memuat
  `qr_token`, jadi mengklaim meja = memegang tokennya. Tidak ada "meja kosong"
  untuk dipilih, dan tidak ada "meja sedang dipakai" untuk disalahpilih.
  Bahaya melihat pesanan orang lain lahir **hanya** dari usulan pemilih meja.
- **Kalau pemilih meja diambil sebagai keputusan produk**, jangan pakai token
  sebagai sumbu "terisi/kosong" (token itu statis dan selamanya). Sumbunya harus
  **sesi terbuka**: meja = terisi ⇔ ada `table-session` ber-`status: open`.
  Itu lebih baik juga secara privasi: tamu memilih **label meja**, bukan token —
  token tidak pernah bocor ke klien, jadi tidak ada yang bisa mengintip pesanan
  orang lain. Yang perlu ditambahkan hanyalah **proyeksi publik** (`kind: Api`,
  mis. `GET /api/v1/public/table-status`) yang mengembalikan `code` +
  `occupied` saja — bukan `SELECT` langsung atas `dining-table`.
  Catatan jujur: proyeksi ini **belum ada di kafe**.
- **Spam "semua meja terisi" bergantung pada siapa yang menentukan terisi.**
  Bila "kosong" diambil dari sesi terbuka, pembuat onar justru **mengisi** meja
  (persis serangan yang dikhawatirkan). Bila dari token, tidak ada jalan masuk
  untuk membukanya sama sekali.

**Yang menahan penyalahgunaan hari ini (terverifikasi):**
`order.guest_token` di-`default_from: "{session.guest_token}"`, dan
`public_entities[].scope` menuntut `guest_token` sebagai parameter permintaan —
tanpa itu permintaan **ditolak** (`list order` anonim tanpa token → **403**).
Selain itu pembuatan sesi per-meja tidak punya keunikan sama sekali
(`table-session` tidak punya `indexes:`), sehingga satu meja bisa menumpuk sesi.

**Yang belum dipakai padahal sudah ada:** rate limit per-resource/per-action
(`EntitySpec.RateLimit` / `Action.RateLimit`) — nol manifest kafe memakainya.
Ini jawaban langsung untuk "orang iseng memesan di semua meja" (10.36).

## Rekomendasi (urutan, bukan keputusan)

1. **10.35** (halaman masuk token → sesi) — menghapus seluruh kelas masalah ini
   untuk desain QR: tidak ada pemilih meja, tidak ada klaim meja, tidak ada
   kebocoran token.
2. **10.36** (`rate_limit` pada `order` + `table-session`) — harga rendah,
   berlaku apa pun bentuk entry-nya.
3. **10.34c** (unique parsial "satu sesi terbuka per meja", polanya sudah
   dipakai `shift`) — menutup penumpukan sesi.
4. Bila pemilih meja tetap dipilih sebagai produk: **proyeksi `table-status`**
   dulu (jangan ekspos token), baru pemilihnya.

## Bukti verifikasi

- Bundle anonim `?app=kafe-qr`: `dining-table [find]`,
  `table-session [find, create]`, `order [list, create]`,
  `menu-item [list, find]`; `menu: []`; tanpa page `route: /`.
- `list dining-table` anonim (tanpa grant) → 401; `list table-session` → 401.
- `POST table-session` anonim → **422** (lolos otorisasi, gagal validasi).
- `GET dining-table/JKT-A01-DEMO` anonim → **404** (find lewat token gagal).
- `POST order` anonim tanpa `guest_token` → 403 (`isPublicAction` + scope).
- `shift` punya `where: "status = 'open'"`; `table-session` tidak punya
  `indexes:`.
- `grep -rn "rate_limit" examples/kafe/spec` → 0 hasil.

# Plan — Public grant sebagai _floor_ untuk pemanggil terautentikasi

Sumber: laporan pemakaian nyata (`examples/kafe`, dev server :8099) — "login
`kasir`/`kafe123`, lalu `/kafe` → `/kafe/cafe-master/menu-categories` menampilkan
**resource not found**, tidak bisa melakukan apa pun".

## Masalah

Pada App `access: public`, **pemanggil yang sudah login mendapat akses LEBIH
SEDIKIT daripada anonim**. Itu inversi, bukan pembatasan.

Mekanismenya: `RequirePermissionOrAnonymous` (dibuat untuk #45) mengotorisasi
anonim lewat grant, tetapi menuntut permission penuh dari pemanggil
terautentikasi. Begitu ada sesi, grant publik berhenti berlaku untuknya —
padahal grant itulah yang membuat halamannya bisa dibaca.

**Bukti terukur** (dev server, spec kafe, `kafe-qr` allowlist
`menu-category: [list, find]`):

| Pemanggil                                  | `GET /_ui/entity/cafe-master/menu-category` |
| ------------------------------------------ | ------------------------------------------- |
| anonim                                     | **200**                                     |
| sesi `app=kafe-qr` (kasir, 0 permission)   | **404** `resource not found`                |
| sesi `app=kafe-pos` (kasir, 33 permission) | 200                                         |

Terukur juga pada aksi yang justru _ditawarkan_ halaman publik: `POST
/_ui/entity/cafe-order/order` anonim → **401** bila pemanggil membawa sesi,
padahal grant-nya `[create, list]` — artinya **pelanggan yang mendaftar lalu
login tidak bisa memesan**, dan katalognya 404.

Kelas bug ini bukan khas kafe: setiap App publik dengan pendaftaran
(`examples/storefront`) mengalaminya begitu pengunjung menekan "Sign up".

## Akar

`internal/api/middleware.go`:

```go
if IdentityFromContext(r.Context()) != nil {
    inner(next).ServeHTTP(w, r)   // ← sesi = wajib permission, grant diabaikan
    return
}
next.ServeHTTP(w, r)              // anonim = grant berlaku
```

Niat #45 benar dan tetap dipertahankan: grant publik **tidak boleh** menjadi
_bypass_ permission bagi pemanggil login yang sekadar menempuh URL yang sama.
Yang keliru adalah mengambil "wajib punya permission" alih-alih "tidak boleh
melebihi anonim".

## Konstruk

> **Grant publik adalah FLOOR, bukan jalur khusus anonim.**
> Efektivitas pemanggil = `max(permission miliknya, grant publik)`, dan bila ia
> berjalan di atas grant (karena tidak punya permission-nya), **scope grant ikut
> berlaku**.

Ini tidak pernah melebarkan akses:

- Grant publik **sudah publik** — dideklarasikan di manifest dan dikirim apa
  adanya ke pengunjung anonim lewat bundle.
- Jalur grant tetap dibatasi `scope`-nya (`guest_token`, `branch_id`), jadi
  pemanggil tanpa permission tidak pernah menerima baris lebih banyak daripada
  anonim.
- Aksi di luar grant (`update`/`delete` dan seluruh route non-publik) tetap
  digerbangi permission seperti sebelumnya — #45 utuh.

## Perubahan

### 1. `internal/api/middleware.go` — fallback grant

`RequirePermissionOrAnonymous`: identitas tanpa permission **tidak** ditolak,
melainkan ditandai "berjalan di atas grant publik" lewat context
(`withPublicGrantAuth`), lalu diteruskan. Identitas dengan permission berjalan
seperti biasa (governed permission + `row_scope` entity).

### 2. `internal/api/scope.go` — scope mengikuti otorisasinya

- `applyPublicScope`: berlaku untuk anonim **atau** pemanggil bertanda
  grant-authorised; tetap **dilewati** untuk pemanggil yang punya permission
  sendiri (kalau tidak, daftar POS kasir yang tidak membawa token tamu akan 403
  — alasan asli #45).
- `applyRowScope`: skip `row_scope from: session` diperluas ke pemanggil
  grant-authorised, karena bagi mereka grant-lah yang mengikat (sama seperti
  anonim). Tanpa ini, pelanggan tanpa atribut sesi akan 403 tepat pada alur
  pesan QR.

## Bukti yang harus ada setelah perbaikan

| Kasus                                                | Sebelum                         | Sesudah                        |
| ---------------------------------------------------- | ------------------------------- | ------------------------------ |
| anonim `list menu-category`                          | 200                             | 200                            |
| sesi 0-permission `list menu-category`               | 404                             | **200**                        |
| sesi 0-permission `create order`                     | 401                             | **201/422** (mencapai handler) |
| sesi 0-permission `list order` (tanpa `guest_token`) | 404                             | **403 fail-closed**            |
| sesi 0-permission `list order?guest_token=X`         | 404                             | **hanya baris X**              |
| sesi ber-permission `list order` tanpa token         | 200 (semua cabang sesuai scope) | **tidak berubah**              |
| anonim `list dining-table` (grant hanya `find`)      | 401                             | 401                            |

## Test pengunci

- `TestRequirePermissionOrAnonymous_SignedInFallsBackToGrant` — sesi tanpa
  permission → 200 (sebelumnya 404) _dan_ bertanda grant-authorised.
- `TestRequirePermissionOrAnonymous_SignedInWithPermissionUnaffected` — pemegang
  permission tidak ditandai (jalur lamanya tetap).
- `TestApplyPublicScope_GrantAuthorisedCallerIsScoped` — pemanggil login di atas
  grant tetap kena scope.
- `TestApplyPublicScope_PermissionedCallerNotScoped` — pemegang permission tidak
  difilter (regression #45).
- `TestApplyRowScope_GrantAuthorisedCallerSkipsSessionScope` — tidak fail-closed
  pada entity ber-`row_scope from: session`.

## Sisa (dicatat sebagai item todo)

1. **App publik tanpa kontrol auth** — `chrome.auth: none` (default `no-nav`)
   tidak menyediakan sign-out, sehingga pengunjung yang sudah login tidak bisa
   keluar dari permukaan publik (harus lewat App privat atau membersihkan
   penyimpanan sesi). Bukan lagi blocker setelah floor ini, tetapi tetap jebakan.
2. **Tombol "New" di permukaan publik** — dirender dari lifecycle saja, tanpa
   melihat grant, jadi muncul untuk entity yang anonim/grant-nya tidak memberi
   `create` (klik → 401). Perlu allowlist terbaca di klien agar tidak sekaligus
   menyembunyikan tombol yang sah (mis. `create order`).
3. **Sesi di-scope ke App publik dengan role 0** — sah secara teknis (role
   memang per-App), tetapi login di permukaan publik selalu menghasilkan sesi
   tanpa role; perlu keputusan apakah login di App publik sebaiknya
   workspace-level.

## Estimasi

Backend + test: **small–medium**. Sisa (1)–(3): medium (butuh keputusan).

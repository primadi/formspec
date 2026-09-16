# Plan — Public grant row scope (kafe TODO 2.2 / #45)

Sumber: `examples/kafe/gaps_found/TODO.md` 2.2, gap **#45** di
`12-hasil-verifikasi-runtime.md`, dan komentar GAP-06 di
`examples/kafe/spec/modules/cafe-order/pages/order-status-page.yaml`.

## Masalah yang ditutup

Setelah 1.2 (S3, allowlist per-entity), anonim hanya bisa menyentuh entity/aksi
yang dideklarasikan. Tapi "anonim boleh `list`" tetap berarti **anonim membaca
setiap baris** entity itu — jadi `list` pada `order` tidak bisa diberikan sama
sekali, dan pelanggan tidak bisa melihat pesanannya sendiri (halaman status hanya
menyaring di sisi klien, sementara API-nya terbuka). Itu sisa #45.

## Konstruk

`public_entities[].scope` — filter baris **per-permukaan** untuk pembacaan
anonim:

```yaml
- entity: cafe-order.order
  actions: [create, list]
  scope:
    - { field: guest_token, op: eq, from: route }
```

Grant mengatakan entity MANA; scope mengatakan BARIS mana. Nilainya dibaca server
dari parameter request, jadi klien tidak bisa melebarkannya; parameter yang tidak
ada **menolak** permintaan.

## Aturan normatif

| Aturan                         | Alasan                                                                                                                                                    |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| hanya `from: route`            | permukaan publik tidak punya identitas sesi; scope `from: session` tak akan pernah resolve → menolak semua bacaan, jadi ditolak saat validasi             |
| tidak boleh digabung `find`    | `find` me-resolve lewat id, scope tidak bisa menjaganya → akan terlihat terfilter padahal mengembalikan record apa pun                                    |
| nilai menimpa filter klien     | sama seperti `row_scope` (S2): bukan filter UI                                                                                                            |
| tidak ada parameter → 403      | fail closed, tidak pernah "tanpa filter"                                                                                                                  |
| berlaku **hanya** untuk anonim | route `/_ui/entity` dipakai bersama surface POS/KDS; kalau scope ikut berlaku untuk pemanggil terautentikasi, table POS yang tidak membawa token akan 403 |

## Ikut diperbaiki (jalur yang sama)

1. **Grant publik bukan bypass permission.** Sebelumnya grant menyetel
   `RequiredPermission = "public"`, jadi pemanggil yang sudah login melewati
   permission entity di route itu. Kini permintaan terautentikasi tetap wajib
   memegang `{module}.{plural}.{action}` (`RequirePermissionOrAnonymous`).
2. **Parameter scope grant tidak diparse sebagai filter field** (422 `unknown
field`) — perbaikan yang sama dengan yang dilakukan 1.1 untuk `row_scope`.
3. `order.guest_token` + `order-form-qr` menyalinnya dari sesi; komentar GAP-06
   di `order-status-page.yaml` dihapus karena engine sekarang menegakkannya.

## File

- `pkg/spec/resources.go` — `PublicEntityDecl.Scope` + validasi.
- `internal/api/scope.go` — `withPublicScope`, `publicScopeFromContext`,
  `applyPublicScope`.
- `internal/api/router.go` — `publicScope()`, `PublicScope` pada route,
  `RequirePermissionOrAnonymous`; `descriptor.go` — field baru.
- `internal/api/middleware.go` — middleware baru.
- `internal/api/handler.go` — panggilan + skip parameter di `parseListQuery`.
- `internal/genjsonschema` — `FilterSpec` sudah ada; schema diregenerasi.
- kafe: `apps/kafe-qr.yaml`, `order/entity.yaml`, `forms/order-form-qr.yaml`,
  `pages/order-status-page.yaml`.
- `docs/spec/frontend/05-app-kinds.md` §1.1 (baru).

## Bukti

Runtime (dev server + SQLite segar, 2 pesanan token berbeda): tanpa token →
403; dengan TOKENA → hanya TOKENA; dengan TOKENB → hanya TOKENB; TOKENA +
`guest_token[eq]=TOKENB` → tetap hanya TOKENA; token salah → 0 baris; `create`
anonim → 422 (mencapai handler). Unit: `pkg/spec` 3+4 case, `internal/api` 5 case.

## Sisa

Alur QR masih dua langkah (token → sesi → ID) karena `find` tidak bisa di-scope;
token sebagai kunci tunggal menuntut halaman yang men-resolve sesi dari token.

## Estimasi: **medium** (spec + router + middleware + handler + klien kafe)

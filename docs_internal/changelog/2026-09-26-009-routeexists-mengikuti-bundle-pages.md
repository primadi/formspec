# `routeExists` mengikuti `bundle.pages`, bukan registri Form/Table (todo 5.22.6)

## Apa yang diubah

`routeExists` (`internal/ui/meta.go`) memakai **dua sumber yang tidak sepakat**
untuk route bentuk `/M/form/<n>` dan `/M/table/<n>`:

|          | Sumber                          | Akibat                                              |
| -------- | ------------------------------- | --------------------------------------------------- |
| dulu     | registri `b.Forms` / `b.Tables` | Table yang **tidak punya route** tetap dianggap ada |
| sekarang | `b.Pages`                       | sama dengan apa yang di-iterasi `buildRoutes`       |

Keduanya berbeda justru pada kasus yang mudah terjadi: `BuildBundle` **tidak**
menurunkan `<name>-page` untuk Form/Table yang sudah direferensikan blok Page lain
(`covered[...]`), atau yang `public: false`. Jadi `routeExists` menjawab "route
ada" untuk route yang tidak pernah didaftarkan SPA — klik-nya mendarat di
catch-all surface, yaitu 404 yang **terlihat hidup**. Menu sudah dilaporkan benar
untuk entity dan navigasi-kind; hanya bentuk form/table ini yang bocor.

Memeriksa `b.Pages` juga menutup celah kedua tanpa kode tambahan: `b.Pages` sudah
disaring per pemanggil (`allowedPage`), jadi item menu yang menunjuk page yang
tidak boleh dibuka pemanggil itu ikut turun — dengan alasan yang sama dengan
entity.

## Dampak pada test lama (dan kenapa itu benar)

Dua assertion di `menu_filter_test.go` **mengharapkan `"Order table"` muncul** —
padahal `order-table` justru direferensikan Page `order-list` di fixture, jadi ia
adalah contoh bug-nya. Keduanya diperbarui: nilai yang diharapkan kini tidak
memuat `"Order table"`, dengan komentar yang menyebut alasannya. Test lama itu
mem-pin perilaku yang salah; membiarkannya berarti memperbaiki bug-nya nanti lagi.

`TestRouteExists_FormTableFollowBundlePages` (baru) memakai fixture sendiri dengan
**dua** Table untuk entity yang sama dan satu Page yang mereferensikan hanya salah
satunya — asimetri itulah yang membuat bug terlihat, dan fixture bersama tidak
punya pasangan seperti itu. Test memverifikasi prasyaratnya lebih dulu
(`covered-table` tidak punya derived page, `free-table` punya), lalu menegaskan
`routeExists` mengikuti fakta itu, lalu menegaskan item menu yang menunjuk route
mati itu **dibuang**.

## File terdampak

- `internal/ui/meta.go` — `routeExists` cabang 2 (form/table)
- `internal/ui/route_exists_test.go` — **baru**
- `internal/ui/menu_filter_test.go` — 2 ekspektasi diperbarui

## Bukti

- **Dibuktikan gagal** saat `routeExists` dikembalikan ke versi registri:
  `TestRouteExists_FormTableFollowBundlePages` gagal, dan kedua sub-test
  `TestBuildBundle_MenuDropsUnreachableItems` kembali memunculkan
  `"Order table"` di menu.
- `go build ./...` bersih; `go test ./...` **0 FAIL**; `go test ./internal/ui/
./internal/app/ ./internal/api/` hijau.
- `formspec validate --spec examples/kafe/spec --schema schemas` → 85 manifest
  0 problem; `formspec check -f examples/kafe/spec` → 0 error / 0 warning (tidak
  ada item menu kafe yang bergantung pada perilaku lama).

## Rujukan

Todo **5.22.6** (tertutup) · Fase 5.22, `docs/renderers/shadcn-shell/05-routing.md`
· `ResolveViewRoute` (`internal/ui/registry.go:193`) yang mendokumentasikan tabel
route yang sama.

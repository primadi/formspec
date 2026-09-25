# Routing: cabang `Listing` di `ResolveViewRoute` + testnya

**Tanggal**: 2026-09-25 · **Plan**: `docs_internal/plan/routing-docs-and-menu-visibility.md`
**Todo**: 5.22.1 · **Changelog terkait**: `2026-09-25-005` (dokumen routing)

## Apa yang diubah

`Registry.ResolveViewRoute` (`internal/ui/registry.go`) mendapat cabang untuk
kind `Listing` → `"/listing/" + name`, dan doc comment-nya kini menyebut bahwa
fungsi ini adalah **satu dari empat salinan** konvensi route yang sama.

File baru `internal/ui/registry_test.go` mem-pin seluruh tabel kind → route,
termasuk test paritas lintas-lapis.

## Kenapa

`Listing` sudah didukung di tiga tempat lain — `viewKinds`
(`cmd/formspec/validate_dangling.go`), `navigationPrefix`/`routeExists`
(`internal/ui/meta.go`), dan `buildRoutes` (router SPA) — tetapi **hilang** di
`ResolveViewRoute`. Akibatnya bukan error yang terlihat: `formspec validate`
**menerima** `view: product-catalog` (Listing ada di `viewKinds`), lalu
`app.Resolve` mengembalikan `view "…" not found` dan **App gagal resolve
seluruhnya**. Contoh `examples/storefront` lolos hanya karena memakai escape
hatch `route: /listing/product-catalog`, sehingga bug-nya tidak pernah terlihat
di contoh mana pun.

## Bukti

Empat test di `registry_test.go` **dibuktikan gagal** saat cabang `Listings`
dihapus sementara, semuanya dengan pesan yang menunjuk penyebab:

```
--- FAIL: TestResolveViewRoute
        registry_test.go:202: ResolveViewRoute("product-catalog") error: view "product-catalog" not found in module "catalog"
--- FAIL: TestResolveViewRoute_CoversEveryNavigableViewKind
        kind Listing is offered as a menu `view` target (viewKinds) but ResolveViewRoute cannot resolve…
--- FAIL: TestResolveViewRoute_ListingResolvesSoValidateAgrees
        Listing is accepted by validate's viewKinds but ResolveViewRoute failed…
--- FAIL: TestViewKindsMatchResolveViewRoute
        Listing is accepted as a menu `view` by formspec validate but ResolveViewRoute cannot resolve it…
```

Cabang dipulihkan → `go test ./internal/ui/` hijau (0.2s).
`go test ./...` hijau · `go build ./...` bersih.

Test paritas sengaja **menyalin** daftar `viewKinds` (tidak mengimpor), supaya
kegagalan berikutnya muncul sebagai test, bukan sebagai App yang tidak mount.

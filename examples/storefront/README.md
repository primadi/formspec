# Storefront — Public App (no-nav) + Backoffice

Contoh workspace **dua App** yang berbagi satu module `catalog` — demonstrasi
sumbu chrome (`app_renderer`) terpisah dari sumbu auth (`access`),
`docs/spec/frontend/05-app-kinds.md` §1:

| App          | app_renderer  | access    | root_url          | Surface                            |
| ------------ | ------------- | --------- | ----------------- | ---------------------------------- |
| `storefront` | `no-nav`      | `public`  | `/`               | Publik (anonim, tanpa nav standar) |
| `backoffice` | `sidebar-nav` | `private` | `/app/backoffice` | Autentikasi, sidebar admin         |

## Yang didemonstrasikan

- **`kind: Page` dengan blok `section:`** (`pages/home.yaml`) — `hero`,
  `feature_grid`, `carousel`, `cta` + blok `form` pendaftaran inline. Halaman
  ini dirender dalam chrome minimal (`NoNavShell`: brand bar + nav publik +
  footer), tanpa sidebar.
- **`kind: Listing`** (`listings/product-catalog.yaml`) — katalog produk publik
  read-only (search + filter kategori), tanpa row/bulk action.
- **Form pendaftaran publik** (`forms/register.yaml`) — create anonim ke entity
  `registration` (list/find/create publik di module App `access: public`).
- **App backoffice terpisah** — kelola `product` dan `registration` lewat UI
  derived (sidebar), di bawah root `/app/backoffice`, `access: private`.
- **Renderer & komponen SAMA** untuk kedua App — bedanya hanya wrapper chrome
  (`NoNavShell` vs `SideNavShell`) + pola auth (anonim vs autentikasi).
- **`stack_family: react-shadcn` + `persist_backend: jsonb-persist`** —
  deklarasi eksplisit implementasi shell & backend persist.

## Menjalankan

```bash
formspec dev
```

Buka:

- `http://localhost:8080/demo/` → storefront (tanpa login)
- `http://localhost:8080/demo/listing/product-catalog` → katalog publik
- `http://localhost:8080/demo/app/backoffice` → admin (login/izin)

## Struktur

```
spec/
  apps/
    storefront.yaml            # App no-nav + access: public, root "/"
    backoffice.yaml            # App sidebar-nav + access: private, root "/app/backoffice"
  modules/
    catalog/
      module.yaml
      master/product/entity.yaml        # Produk (katalog + admin)
      transaction/registration/entity.yaml # Pendaftaran publik (create anonim)
      pages/home.yaml                   # Home: hero + fitur + carousel + form + cta
      listings/product-catalog.yaml     # Katalog publik read-only
      forms/register.yaml               # Form pendaftaran (create)
```

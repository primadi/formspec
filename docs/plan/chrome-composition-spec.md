# Plan: Chrome Composition Spec — `App.spec.chrome`

**Status:** In Progress · **Tanggal:** 2026-08-29
**Referensi spec:** `docs/spec/frontend/05-app-kinds.md` §1, §4

## Latar Belakang

`no-nav` shell saat ini hardcode tiga hal di `NoNavShell.tsx`: brand bar, nav
link derived dari menu, dan auth controls (Sign in/Sign up saat anonim,
LogoutButton saat signed-in) + footer. Akibatnya `no-nav` tidak lagi berarti
"tanpa navigasi sama sekali" — App landing/marketing publik ikut menampilkan
Sign in/Sign up yang tidak diminta manifestnya.

## Tujuan

1. `no-nav` = konten murni: tanpa nav, tanpa auth controls secara default.
2. Komposisi chrome jadi deklaratif & general via sub-spec `App.spec.chrome`
   yang **ortogonal** terhadap `app_renderer` (archetype layout) dan `access`
   (sumbu auth).
3. Default di-resolve di backend meta (`internal/ui`) — single source of truth
   untuk semua `stack_family`; frontend membaca nilai efektif, tidak menebak.

## Desain Spec

```yaml
spec:
  app_renderer: no-nav # sidebar-nav | topnav | no-nav (default sidebar-nav)
  access: public # private | public (default private)
  chrome: # opsional — semua field default "auto"
    brand: auto # auto | show | hide
    nav: auto # auto | menu | none
    auth: auto # auto | links | button | none
    footer: auto # auto | show | hide
    breadcrumbs: auto # auto | show | hide
    theme_switcher: auto # auto | show | hide
```

Semantik:

- `auto` = default per archetype (matriks di bawah); nilai eksplisit override.
- `nav: menu` = render link dari menu resolved (leaf ber-route; entri
  `/login`|`/register` tetap dikecualikan); `nav: none` = tanpa nav.
- `auth`:
  - `links` — anonim: link "Sign in" + tombol "Sign up"; signed-in: logout.
  - `button` — anonim: satu tombol "Sign in"; signed-in: logout.
  - `none` — tidak pernah render auth UI. App `private` tetap di-guard
    surface boot (redirect ke login saat anonim); App `public` login hanya
    via URL langsung atau CTA page block.

Matriks default (`auto`) per archetype:

| Chrome elemen  | sidebar-nav | topnav | no-nav |
| -------------- | ----------- | ------ | ------ |
| brand          | show        | show   | show   |
| nav            | menu        | menu   | none   |
| auth           | links       | links  | none   |
| breadcrumbs    | show        | show   | hide   |
| theme_switcher | show        | show   | hide   |
| footer         | hide        | hide   | show   |

Skenario yang tercakup:

| Skenario                          | Spec                                                     |
| --------------------------------- | -------------------------------------------------------- |
| Landing/marketing publik          | `no-nav` + `public` + default (konten murni)             |
| Katalog publik + login (Registry) | `no-nav` + `public` + `chrome: {nav: menu, auth: links}` |
| Kiosk/POS private full-screen     | `no-nav` + `private` + default                           |
| Kiosk dengan logout               | `no-nav` + `private` + `chrome: {auth: button}`          |
| Admin standar                     | `sidebar-nav` + `private` + default                      |
| Internal tool top-nav             | `topnav` + `private` + default                           |

Escape hatch tetap: chrome ekstrem via `asset` custom component; archetype
benar-benar baru via `VisualSpecKind tier: app` (05-app-kinds.md §7).

## File yang Diubah

| File                                                | Perubahan                                                                               |
| --------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `pkg/spec/resources.go`                             | Struct `AppChrome` + field `Chrome` di `AppSpec` + konstanta nilai + validasi enum      |
| `internal/ui/meta.go`                               | `ChromeConfig` (resolved) di `AppSummary`; `resolveChrome()` menerapkan matriks default |
| `internal/api/meta.go`                              | Pass `Spec.Chrome` ke `AppContext`                                                      |
| `renderers/react-shadcn/src/types/manifest.ts`      | Tipe `ChromeConfig` + `AppSummary.chrome`                                               |
| `renderers/react-shadcn/src/shell/AuthArea.tsx`     | Komponen auth bersama (links/button/none × anonim/signed-in)                            |
| `renderers/react-shadcn/src/shell/NoNavShell.tsx`   | Baca chrome; hapus hardcode auth/nav/footer                                             |
| `renderers/react-shadcn/src/shell/SideNavShell.tsx` | `AuthArea` + override breadcrumbs/theme_switcher                                        |
| `renderers/react-shadcn/src/shell/TopNavShell.tsx`  | idem                                                                                    |
| `registry/spec/apps/registry.yaml`                  | `chrome: {nav: menu, auth: links}` — perilaku portal tidak berubah                      |
| `schemas/dist/latest/`                              | Regenerate via `make generate-schema`                                                   |
| `docs/spec/frontend/05-app-kinds.md`                | §4 no-nav + section Chrome Composition                                                  |
| `docs/renderers/shadcn-shell/03-kind-renderers.md`  | Perilaku shell terhadap chrome                                                          |
| `docs/reference/glossary.md`                        | Entri `chrome`                                                                          |

## Dependensi & Urutan

1. Spec Go (`pkg/spec`) → 2. resolusi meta (`internal/ui`, `internal/api`) →
2. tipe frontend → 4. shells → 5. registry.yaml + docs → 6. schemas + tests.

Level of effort: **medium** (spec+backend kecil, frontend sedang, docs).

## Keputusan

- Default `no-nav` benar-benar kosong (keputusan user 2026-08-29).
- Mekanisme: sub-spec `chrome:`, bukan archetype baru / asset-only.
- Resolusi default di backend meta, bukan frontend.
- Nilai chrome tidak dikenal fallback ke default archetype (lenient) —
  validasi ketat ditangani JSON Schema + `formspec validate`.

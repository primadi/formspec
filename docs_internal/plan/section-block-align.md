# Section Block: Align + Card Icon Fix

## Latar

Di `/default/portal/profile` (registry), section `card` tampak tidak konsisten:
section "Profil Saya" tampak left-align, "Roles"/"Permissions" tampak center.
Akar masalah (diukur via browser): `<section className="mx-auto max-w-6xl">`
adalah **grid item** dengan auto margin → auto margin menonaktifkan stretch
sizing di CSS Grid → section **shrink-to-fit** (lebar = konten + 32px padding)
lalu di-center. Section dengan konten panjang tampak "left", konten pendek
tampak "center". Teks di dalamnya selalu `text-align: start` — tidak ada
alignment yang benar-benar berubah.

Bug kedua: `items.icon` tidak muncul di section `card` — `CardBlock` tidak
merender `item.icon` (hanya `image/title/text/cta`), sedangkan
`FeatureGridBlock` merendernya.

## Keputusan desain

1. **Section box stretch penuh kolom grid-nya** (hapus `mx-auto` pada elemen
   `<section>` di `card`, `feature_grid`, `carousel`) — semua section sama
   lebar, tidak ada ilusi center/left yang bergantung panjang konten.
   `hero`/`cta` tidak terdampak (mx-auto mereka ada di inner div, section-nya
   sudah stretch).
2. **`align: left | center | right` (default `left`)** pada `SectionBlock` —
   opt-in untuk kebutuhan presentational. Konsisten dengan `TableColumn.align`
   yang sudah ada. `cta` tetap center by design (abaikan align).
3. **`CardBlock` merender `item.icon`** seperti `FeatureGridBlock`
   (`size-6 text-primary`).

## File yang diubah

| File                                                               | Perubahan                                                        |
| ------------------------------------------------------------------ | ---------------------------------------------------------------- |
| `pkg/spec/frontend.go`                                             | `SectionBlock.Align` + `@schema` enum                            |
| `renderers/react-shadcn/src/types/manifest.ts`                     | `SectionBlock.align`                                             |
| `renderers/react-shadcn/src/components/sections/SectionBlocks.tsx` | CardBlock: stretch + icon + align; FeatureGrid/Carousel: stretch |
| `schemas/`                                                         | regen via `make generate-schema`                                 |

## LOE

Small.

## Referensi

- `docs/spec/` frontend 06-page-kinds §1 (section blocks, closed set)
- Changelog: `2026-09-01-003-section-align-card-icon.md`

# 2026-09-01-003 — Section `align` + CardBlock stretch & icon fix

## Apa

1. `SectionBlock` dapat field baru `align: left | center | right` (default
   `left`) — Go spec (`pkg/spec/frontend.go`), TS type
   (`renderers/react-shadcn/src/types/manifest.ts`), JSON schema (regen).
2. `CardBlock` / `FeatureGridBlock` / `CarouselBlock`: hapus `mx-auto` pada
   elemen `<section>` — sebagai grid item, auto margin menonaktifkan stretch
   sizing → section shrink-to-fit mengikuti panjang konten → section pendek
   tampak "center", section panjang tampak "left" (ilusi inkonsisten di
   `/default/portal/profile`). Kini semua section stretch memenuhi kolom grid.
3. `CardBlock` kini merender `item.icon` (sebelumnya diabaikan — hanya
   `FeatureGridBlock` yang merender), sama seperti feature_grid:
   `size-6 text-primary`.
4. `align: center` pada card → item box `items-center` + `text-center`.

## Kenapa

Laporan user: di profile page, section "Profil Saya" tampak left sementara
"Roles"/"Permissions" tampak center; `items.icon` tidak muncul. Pengukuran
browser mengonfirmasi shrink-to-fit + auto-margin centering sebagai akar
masalah, dan `CardBlock` memang tidak merender icon.

## File terdampak

- `pkg/spec/frontend.go`
- `renderers/react-shadcn/src/types/manifest.ts`
- `renderers/react-shadcn/src/components/sections/SectionBlocks.tsx`
- `schemas/` (regen)
- `registry/web/dist/` (sync hasil build)

## Referensi

- Plan: `docs_internal/plan/section-block-align.md`
- Todo: 5.2.7 (section blocks) — entri baru 5.2.8

# Plan: Page kind — Master-detail split, Full-custom, Custom Page

**Status**: In progress (2026-08-24)
**Todo refs**: 5.2.3, 5.2.4, 5.2.5
**Spec refs**: `docs/spec/frontend/06-page-kinds.md` §1 (blocks), §1.1 (master-detail),
§13 (custom page), `docs/spec/frontend/07-component-kinds.md` §4 (asset contract)

## Ringkasan

Tiga item `kind: Page` yang belum selesai, semuanya di renderer `PageRenderer.tsx`
dan skema `PageSpec`/`BlockRef` di `pkg/spec/frontend.go` + `types/manifest.ts`:

1. **5.2.3 Master-detail split** — `layout.mode: split` + `binds: {source, param}`
   pada blok detail. Seleksi baris pada blok master (Table) menggerakkan blok
   detail (Form) tanpa navigasi route.
2. **5.2.4 Full-custom** — Page dengan satu entry `component:` tanpa blocks/tabs
   dirender full-bleed (tanpa wrapper border).
3. **5.2.5 Custom Page (`mode: custom`)** — Page menyerahkan seluruh render ke
   asset, dengan `binds` footprint (entities/actions/subscribe) yang di-enforce
   client-side (mekanisme sama dengan `needs:` component, todo 5.9.6).

## Perubahan

### Backend — `pkg/spec/frontend.go`

- `PageLayout`: tambah `Mode string` (`split` = master-detail).
- `BlockRef`: tambah `Binds *BlockBinds` (`{source, param}`).
- `PageSpec`: tambah `Mode string` (`custom`), `Asset string`, `Binds *PageBinds`.
- `PageBinds`: `{Entities, Actions, Subscribe []string}`.
- `ValidatePageSpec`: `mode: custom` → wajib `asset`, tidak boleh `blocks`/`tabs`.

### Frontend — `types/manifest.ts`

- Mirror `PageLayout.mode`, `BlockRef.binds`, `PageSpec.mode/asset/binds`.

### Frontend — `kinds/table/TableRenderer.tsx`

- Tambah prop `onSelect?: (record) => void` — dipanggil saat baris diklik
  (untuk master-detail). Highlight baris terpilih.

### Frontend — `kinds/page/PageRenderer.tsx`

- **Split mode**: deteksi `layout.mode === "split"`; cari blok master (table
  yang `ref`-nya cocok dengan `binds.source` blok detail); render master kiri
  (sempit) + detail kanan (lebar); state `selectedRecord`; detail form
  `id = selectedRecord[binds.param]`; tanpa seleksi → empty-state.
- **Full-custom**: satu blok `component:` → render full-bleed tanpa border.
- **Custom Page**: `mode === "custom"` → render `AssetRenderer` full-bleed
  dengan `needs` diturunkan dari `binds` (entities → `module.entity.*` actions).

## Verifikasi

- Backend: `go build ./...` + `go test ./...` + `make generate-schema`
- Frontend: `cd renderers/react-shadcn && npx vitest run` + `npx tsc --noEmit`

## Changelog

- `docs/changelog/2026-08-24-027-page-master-detail-full-custom-custom-page.md`

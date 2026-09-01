# 2026-08-31-006 — Fix chunk-splitting SPA: vendor-react mega-chunk >1 MB

## Apa

Perbaiki `manualChunks` di `renderers/react-shadcn/vite.config.ts`: aturan lama
`id.includes('react')` mencocokkan SEMUA library yang namanya mengandung
"react" (`@radix-ui/react-*`, `@base-ui/react`, `@tanstack/react-table`,
`react-hook-form`, `react-day-picker`) sehingga tergabung jadi satu chunk
`vendor-react` 1.316 kB → warning build "chunks larger than 1000 kB".

Sekarang pencocokan path-bounded (`node_modules/(react|react-dom|react-router|
react-router-dom|scheduler)/`) + split grup baru: `vendor-ui` (Radix/Base UI),
`vendor-forms` (react-hook-form/zod/@hookform), sisanya ke `vendor`.

## Hasil

| Chunk        | Sebelum    | Sesudah |
| ------------ | ---------- | ------- |
| vendor-react | 1.316 kB   | 222 kB  |
| vendor-ui    | —          | 110 kB  |
| vendor-forms | —          | 96 kB   |
| vendor       | (gabungan) | 256 kB  |
| vendor-icons | 631 kB     | 631 kB  |

Warning build hilang; semua chunk < 1 MB. Cache browser juga lebih efektif
(perubahan app code tidak lagi invalidate chunk vendor sebesar 1,3 MB).

## File terdampak

`renderers/react-shadcn/vite.config.ts`. Embedded SPA registry disink ulang ke
`registry/web/dist/` + `bin/formspec-registry` di-rebuild.

Referensi: `docs_internal/plan/registry-theme-switcher.md` (sesi build
`make build-registry`).

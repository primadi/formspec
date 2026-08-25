# 2026-08-24-033 — Fix files.ts duplicate identifier + FieldType gap

## Apa yang diubah

Dua perbaikan kecil yang ditemukan saat verifikasi build Fase 5:

1. **`renderers/react-shadcn/src/lib/files.ts`** — `let files` dan `export const files`
   bentrok (duplicate identifier) yang membuat `vite build` gagal. Internal state di-rename
   ke `fileList`; API publik (`files`, `addDownloadedFile`, dll) tidak berubah.

2. **`renderers/react-shadcn/src/types/manifest.ts`** — `FieldType` union kurang `text`,
   `richtext`, `file` padahal backend mendukungnya (`pkg/spec/entity.go`) dan widget-nya
   sudah ada (TextareaInput, RichText, FileInput — 5.10). Ditambahkan sehingga `formWidget()`
   di `engine/derive.ts` lolos type-check.

## File terdampak

- `renderers/react-shadcn/src/lib/files.ts`
- `renderers/react-shadcn/src/types/manifest.ts`

## Referensi

- Todo: `docs/plan/todo.md` §5.9.4, §5.10

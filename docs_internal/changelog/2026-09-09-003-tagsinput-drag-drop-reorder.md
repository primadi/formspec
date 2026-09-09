# 2026-09-09-003 — TagsInput: drag-drop reorder via dnd-kit

## Apa

`renderers/react-shadcn/src/widgets/TagsInput.tsx`: badge tag kini sortable —
bisa di-drag untuk mengubah urutan. Implementasi `@dnd-kit/core` +
`@dnd-kit/sortable` (`horizontalListSortingStrategy`, `PointerSensor`
activation distance 6px). Urutan baru di-emit ke `onChange` dengan shape value
yang sama (array untuk field json, comma-joined untuk string).

## Detail

- Seluruh badge = drag surface; tombol X `stopPropagation` pada
  `onPointerDown` agar klik hapus tidak memicu drag.
- `readonly` mode tetap tanpa dnd (badge statis).
- Urutan tersimpan: json → array berurutan; string → urutan comma-joined.

## File terdampak

- `renderers/react-shadcn/src/widgets/TagsInput.tsx`

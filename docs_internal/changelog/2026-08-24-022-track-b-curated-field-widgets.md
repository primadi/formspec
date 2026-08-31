# 2026-08-24-022 — Track B: Curated Field Widgets (5.10.10–5.10.14)

## Apa yang diubah

Implementasi Track B widget strategy (docs/plan/widget-strategy.md) — lima field widget kurasi,
semua **frontend-only, dependency-free** (mengikuti pola proyek: checkbox/select ditulis manual,
tanpa radix):

- **`5.10.10` RadioGroup** (`widgets/RadioGroup.tsx`) — single-choice enum alternatif `select`;
  button-based radio group.
- **`5.10.11` Combobox** (`widgets/Combobox.tsx`) — searchable select utk enum besar; custom
  dropdown (button + search input + list), close on click-outside.
- **`5.10.12` Password** (`widgets/PasswordInput.tsx`) — input masking + reveal toggle (Eye/EyeOff).
- **`5.10.13` Slider** (`widgets/SliderInput.tsx`) — native `<input type="range">`; min/max dari
  field rules, step dari `scale` (decimal).
- **`5.10.14` Tags** (`widgets/TagsInput.tsx`) — multi-select disimpan sebagai **comma-separated
  string** pada string field (frontend-only, tanpa backend change). Opsi representasi array
  (backend) ditunda.

Semua widget di-wire di router `FormFieldWidget` (`radio-group`, `combobox`, `password`,
`slider`, `tags`) + barrel `widgets/index.ts`. Ini opt-in via `widget:` di Form manifest —
derive tetap memakai default (select/input/number).

## Kenapa

Memberi developer pilihan widget input yang lebih kaya untuk enum (radio/combobox), field
sensitif (password), range numerik (slider), dan multi-tag — tanpa menambah dependensi eksternal
dan tanpa menyentuh backend.

## File terdampak

- `renderers/react-shadcn/src/widgets/RadioGroup.tsx`, `Combobox.tsx`, `PasswordInput.tsx`,
  `SliderInput.tsx`, `TagsInput.tsx` — baru
- `renderers/react-shadcn/src/widgets/index.ts` — export 5 widget
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — 5 case router baru
- `docs/plan/todo.md` — tandai 5.10.10–5.10.14 ✅

## Verifikasi

- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih
- `go test ./...` — tidak ada perubahan backend

# 2026-08-24-018 — Track A Widgets: Textarea + decimalinput/datetimeinput

## Apa yang diubah

Implementasi Track A widget strategy (docs/plan/widget-strategy.md) — tiga item frontend-only
dari set wajib `07-component-kinds.md` §1:

1. **`5.10.9` Textarea** — widget baru `TextareaInput` (`widgets/TextareaInput.tsx`, wrap
   `components/ui/textarea.tsx`). Router `FormFieldWidget` kini punya case `textarea` terpisah
   (sebelumnya numpang ke `TextInput`). `derive.formWidget()` memetakan field type `text` →
   `textarea`. `DetailPage` merender nilai field `text` dengan `whitespace-pre-wrap`.
2. **`5.10.6` DecimalInput** — nama manifest distinct `decimalinput` terdaftar di router
   (bersama `number`/`integer`/`decimal`) + `derive.formWidget()` kini memetakan `decimal` →
   `decimalinput` (integer tetap `number`).
3. **`5.10.7` DateTimeInput** — nama manifest distinct `datetimeinput` terdaftar di router
   (bersama `datepicker`/`date`/`datetime`) + `derive.formWidget()` kini memetakan `datetime` →
   `datetimeinput` (date tetap `datepicker`).

## Kenapa

Menutup gap set wajib widget input dan memberi nama manifest yang eksplisit untuk
`decimalinput`/`datetimeinput` (sebelumnya terlipat ke `number`/`datepicker`), sehingga
developer bisa menulis `widget: decimalinput` / `widget: datetimeinput` di Form manifest.

## File terdampak

- `renderers/react-shadcn/src/widgets/TextareaInput.tsx` — widget baru
- `renderers/react-shadcn/src/widgets/index.ts` — export `TextareaInput`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — case `textarea`, `decimalinput`, `datetimeinput`
- `renderers/react-shadcn/src/engine/derive.ts` — `formWidget()`: `text`→textarea, `decimal`→decimalinput, `datetime`→datetimeinput
- `renderers/react-shadcn/src/kinds/page/DetailPage.tsx` — render field `text` pre-wrap
- `docs/plan/todo.md` — tandai 5.10.6/5.10.7/5.10.9 ✅

## Verifikasi

- `npx vitest run` — 138 test lulus
- `npx tsc --noEmit` — bersih

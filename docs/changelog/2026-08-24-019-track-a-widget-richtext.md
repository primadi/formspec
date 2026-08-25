# 2026-08-24-019 — Track A Widget: RichText

## Apa yang diubah

Implementasi `5.10.4` RichText (set wajib `07-component-kinds.md` §1.2) — widget input untuk
field bertipe `richtext`:

- **`widgets/RichText.tsx`** (baru) — toolbar dasar (bold, italic, heading, bullet/numbered
  list, link/unlink) di atas area `contentEditable`, memakai `document.execCommand`. Menyimpan
  HTML. Mode readonly merender HTML tersanitasi. Bukan page builder.
- **`lib/sanitize.ts`** (baru) — sanitizer HTML client-side ringan yang **mirror sanitizer
  server** (`renderers/jsonb-persist/crud.go` `sanitizeHTML`): strip `script`/`style`, tag
  berbahaya (`iframe`/`object`/`embed`/`form`/`input`/`button`/`link`/`meta`), event-handler
  `on*`, dan `javascript:` URL. Defense-in-depth — server tetap otoritas saat tulis.
- **`lib/sanitize.test.ts`** (baru) — 6 kasus mirror `richtext_test.go` `TestSanitizeHTML`.
- **Router** `FormFieldWidget` — case `richtext` → `RichText`.
- **`derive.formWidget()`** — field type `richtext` → `richtext`.
- **`DetailPage`** — render nilai field `richtext` tersanitasi (`dangerouslySetInnerHTML`).

## Kenapa

Menutup gap set wajib widget input; developer kini bisa menulis field `type: richtext` dan
mendapat editor rich text dasar dengan HTML yang disanitasi server-side (sudah jalan, 6.9.1).

## File terdampak

- `renderers/react-shadcn/src/widgets/RichText.tsx` — widget baru
- `renderers/react-shadcn/src/lib/sanitize.ts` + `sanitize.test.ts` — sanitizer client + test
- `renderers/react-shadcn/src/widgets/index.ts` — export `RichText`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — case `richtext`
- `renderers/react-shadcn/src/engine/derive.ts` — `formWidget()`: `richtext`→richtext
- `renderers/react-shadcn/src/kinds/page/DetailPage.tsx` — render richtext sanitized
- `docs/plan/todo.md` — tandai 5.10.4 ✅

## Verifikasi

- `npx vitest run` — 144 test lulus (termasuk 6 test sanitize baru)
- `npx tsc --noEmit` — bersih

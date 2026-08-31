# 2026-08-24-025 — Headless Form + formspec.files Tray (5.9.5 + 5.9.4)

## Apa yang diubah

Melengkapi Track C — dua enhancement asset contract (07-component-kinds.md §4):

**`5.9.5` Headless form engine:**

- `lib/headless-form.ts` (baru) — `createHeadlessForm(entity, { mode, id? })` mengembalikan
  instance form headless: `values`/`setValue`/`getValue`, `isDirty`, `isReadonly`/`isVisible`/
  `isRequired` (FormSpecExpr), `compute`, `validate()` (zod dari field rules), `submit()`
  (POST/PATCH dengan CAS version), `reset`, `load`. Tanpa layout/widget — developer kuasai 100%
  markup (anak tangga tertinggi kontrol).
- `lib/zod-schema.ts` (baru) — `buildZodField` diekstrak dari `FormRenderer` (dipakai bersama,
  hindari duplikasi); FormRenderer kini mengimpor dari sini.

**`5.9.4` formspec.files tray:**

- `lib/files.ts` (baru) — store download tray (event-bus) + API `files` (`download`/`list`/
  `remove`/`clear`).
- `shell/DownloadTray.tsx` (baru) — panel floating yang menampilkan file ter-download;
  di-mount di `App.tsx`.

**Wiring:** `lib/formspec-client.ts` — `formspec.files` + `formspec.form` ditambahkan ke client
yang di-inject ke asset.

## Kenapa

Menuntaskan "tangga kontrol" asset: developer kini bisa memakai `formspec.form()` untuk form
headless penuh dan `formspec.files` untuk tray download — melengkapi `formspec.ui`/`api`/
`subscribe`/`navigate`/`theme`/`components`.

## File terdampak

- `renderers/react-shadcn/src/lib/headless-form.ts`, `lib/zod-schema.ts`, `lib/files.ts`,
  `shell/DownloadTray.tsx` — baru
- `renderers/react-shadcn/src/lib/formspec-client.ts` — `files` + `form`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — pakai `buildZodField` dari lib
- `renderers/react-shadcn/src/App.tsx` — mount `DownloadTray`
- `docs/plan/todo.md` — tandai 5.9.4, 5.9.5 ✅

## Verifikasi

- `npx vitest run` — 144 test lulus
- `npx tsc --noEmit` — bersih
- `go test ./...` — tidak ada perubahan backend

## Catatan

- Sisa Track C: `5.9.6` needs declaration, `5.9.7` CSP sandbox, `5.9.8` CSS scoped (hardening/
  keamanan — ditunda).

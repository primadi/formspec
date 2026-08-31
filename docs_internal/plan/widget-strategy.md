# Widget Strategy — shadcn → FormSpec

**Status:** Draft · **Tanggal:** 2026-08-24
**Referensi:** `docs/spec/frontend/07-component-kinds.md` §1 (katalog tier component), §2–§4
**Todo:** `docs/plan/todo.md` §5.2.7, §5.9, §5.10, §7.17
**Changelog:** `docs/changelog/2026-08-24-017-widget-strategy-docs.md`

## Keputusan

**TIDAK semua komponen shadcn di-mapping ke field widget.** Registry widget dasar adalah
**closed set yang dikurasi** (`07-component-kinds.md` §1). Widget = kontrak field-level input
(value/validation/permission per field). Mayoritas komponen shadcn (alert, alertDialog,
dropdown-menu, popover, accordion, carousel, dll) adalah **struktural/presentasi**, bukan input
field — tempatnya di jalur #2 dan #3 di bawah, bukan di katalog widget.

Alasan mapping massal ditolak:

- Melanggar prinsip closed set (`07-component-kinds.md` §1) — katalog widget tak terkendali.
- Merusak derived-by-default (setiap widget baru menambah permukaan derivasi/validasi).
- Membebani schema/validasi (`pkg/spec` + JSON Schema) tanpa nilai field-level yang jelas.
- Redundan dengan escape hatch `asset` (component custom) yang sudah di-design untuk UI tak berpola.

## Konteks (state 2026-08-24)

- Widget existing: `Badge`, `ChildTable`, `DateInput`, `GrantsEditor`, `JsonInput`, `NumberInput`,
  `RelationPicker`, `Select`, `Switch`, `TextInput`.
- Router: `kinds/form/FormRenderer.tsx` `FormFieldWidget` — switch atas `field.widget ?? entityField.type`.
- `components/ui` baru 15 komponen ter-install (textarea, skeleton, dialog, sheet, dsb) — alert,
  alert-dialog, radio-group, popover, dropdown-menu, dll belum ada.
- `formspec.ui` / `formspec.components` / `formspec.files` / asset loader belum diimplementasi (todo 5.9).
- Backend: `ctx.storage` ada (primitive Starlark upload/download); HTTP upload route belum ada (todo 7.17.1).

## Tiga jalur "UI rich"

1. **Field widget — set tertutup dikurasi** (todo 5.10): lengkapi set wajib §1 yang belum ada
   (textarea, richtext, fileinput, nama distinct `decimalinput`/`datetimeinput`) + tambah kurasi
   bernilai tinggi (5.10a: radio-group, combobox, password, slider, checkbox-group/tags).
2. **Chrome struktural untuk component `asset`** (todo 5.9.2/5.9.3/5.9.4): expose primitif UI
   shadcn lewat `formspec.ui` (toast/dialog/confirm/drawer), `formspec.components` (komposisi
   widget dasar), `formspec.files` (upload/download tray). Ini jalur utama biar asset custom
   dapat UI rich tanpa mengotori katalog widget.
3. **Block presentasi deklaratif di Page** (todo 5.2.7 + section blocks): banner/alert/notice,
   card, hero, dll — perluasan `SectionBlock`/`PageBlock` closed set (`06-page-kinds.md` §1).

## Aturan kurasi field widget

- Nama widget manifest = kebab-case, satu per widget.
- Wajib: implementasi di `src/widgets/`, case di router `FormFieldWidget`, mapping di
  `engine/derive.ts` `formWidget()`, barrel `widgets/index.ts`.
- Bila menyentuh field type baru → `pkg/spec` + `make generate-schema` + `formspec validate`.
- Komponen shadcn struktural di-install **on-demand** (bukan sekaligus) untuk menjaga bundle size.

## Track → todo.md

| Track                   | Todo                                                                                                |
| ----------------------- | --------------------------------------------------------------------------------------------------- |
| A — Set wajib §1        | 5.10.4 (richtext), 5.10.5 (fileinput, dep 7.17.1), 5.10.6/5.10.7 (nama distinct), 5.10.9 (textarea) |
| B — Kurasi field widget | 5.10.10–5.10.14 (opsional)                                                                          |
| C — Chrome struktural   | 5.9.2, 5.9.3, 5.9.4, 5.2.7, 5.10.8 (empty-state)                                                    |
| D — Spec/schema/docs    | update `07-component-kinds.md` §1 + schema regen                                                    |

## Relevant files

- `renderers/react-shadcn/src/widgets/` — `TextareaInput.tsx`, `FileInput.tsx`, `RichText.tsx`, item 5.10a; `index.ts`
- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx` — `FormFieldWidget` router + `buildZodField`
- `renderers/react-shadcn/src/engine/derive.ts` — `formWidget()`
- `renderers/react-shadcn/src/kinds/page/DetailPage.tsx` + `lib/renderCell.tsx` — render field baru
- `renderers/react-shadcn/src/shell/` — `formspec.ui` / `formspec.components` / asset loader (5.9)
- `internal/api/` — 7.17.1 upload route ke `ctx.storage` (backend, dep 5.10.5)
- `pkg/spec/entity.go` — `FieldType` + `StorageSpec`
- `docs/spec/frontend/07-component-kinds.md` — update §1
- `docs/plan/todo.md` — 5.2.7 / 5.9 / 5.10 / 7.17

## Verification

1. `cd renderers/react-shadcn && vitest run`
2. `npx tsc --noEmit` (rtk tsc) di `react-shadcn`
3. `rtk go test ./...` bila sentuh backend (7.17.1)
4. `make generate-schema` + `formspec validate --schema schemas`
5. Manual E2E: `go run cmd/formspec dev --dev-ui` di `examples/cafe` → form entity ber-field
   `text`/`richtext`/`file`; cek widget + upload + detail page. Dev login admin/admin.

## Further Considerations

1. Field types `money` (7.16) & `time` belum punya widget — usulkan track tambahan.
2. Enum multi-select: array vs delimiter string — rekomendasi array, menyentuh backend/migration.
3. Install shadcn batch vs on-demand — rekomendasi on-demand (jaga bundle size).

# 2026-09-09-001 — A11y: label form ter-associate dengan input (hapus warning "No Label associated with a form field")

**Tanggal**: 2026-09-09 · **Plan**: — · **Todo**: —

## Apa

8 warning a11y "No Label associated with a form field" di halaman Pengaturan
cafe (`/cafe/settings`) muncul karena `<label>` di `FormRenderer` tidak punya
`htmlFor` dan widget input tidak punya `id`.

Perbaikan:

- `kinds/form/FormRenderer.tsx` — generate id per field (`form-${useId()}-${field.name}`),
  set `htmlFor` pada label, dan teruskan `id` + `label` ke `FormFieldWidget` → semua widget.
- Widget `src/widgets/*` — tambah prop `id` (diteruskan ke input native):
  `TextInput`, `TextareaInput`, `NumberInput`, `PasswordInput`, `DateInput`,
  `JsonInput`, `SliderInput`, `TagsInput`. Widget button-based (`Switch`,
  `Combobox`, `RadioGroup`) menerima accessible name via `aria-label`/prop `label`.
- `Select` dan `RelationPicker` sudah mendukung `id` sebelumnya.
- Fix tambahan: case `default` di `FormFieldWidget` (tipe `string`) sekarang
  meneruskan `id` ke `TextInput` — sebelumnya hanya widget bernama eksplisit.

## Kenapa

Label yang tidak ter-associate dengan field membuat screen reader membacakan
field tanpa nama, dan Chrome DevTools menandai setiap field sebagai warning
a11y (terlihat 8x di halaman Pengaturan, satu per field).

## Verifikasi

- `/cafe/settings`: 8/8 label punya `for` yang cocok dengan `id` input
  (termasuk `Select` via `ThemedSelect` id, `NumberInput` native input id).
- Accessibility tree menampilkan accessible name untuk semua textbox/spinbutton/button.
- `tsc --noEmit` bersih.

## Rond 2 — form order (drawer, child-grid)

Dua warning lanjutan di form create order (`/cafe/cafe-order/orders?action=create`):

1. **"Incorrect use of `<label for=FORM_ELEMENT>`"** — label field `items`
   (child-grid) menunjuk id yang tidak ada (ChildTable bukan elemen labelable),
   dan label field `select` menunjuk `<button>` trigger (bukan form element).
   Fix: `FormRenderer` hanya merender `<label htmlFor>` untuk widget yang
   merender native labelable input (`LABELABLE_WIDGETS`); widget button-based
   (select/switch/combobox/radio-group) dan composite (child-grid, uuid,
   grants-editor) dirender sebagai `<span>` label + accessible name via
   `aria-label` (`ThemedSelect` dapat prop `ariaLabel`, diteruskan lewat
   wrapper `widgets/Select`).
2. **"A form field element should have an id or name attribute"** — hidden
   native date picker di `DateInput` tanpa id/name. Fix: id `${id}-picker`
   (fallback `name="date-picker-native"`), dan input per-sel di `ChildTable`
   diberi `name` (`child-{row}-{field}`) — `NumberInput` dapat prop `name`.

Verifikasi rond 2: drawer order (dengan 1 baris child) dan halaman Pengaturan —
0 bad label, 0 field tanpa id/name; `tsc --noEmit` bersih.

## Rond 3 — field readonly

Warning "Incorrect use of `<label for=FORM_ELEMENT>`" di form edit user
(`/cafe/formspec.core/users/{id}/edit`): field `username` readonly merender
display `<div>` (bukan input), tapi label tetap `<label for>`. Fix:
`isLabelable` di `FormRenderer` kini juga mengecualikan field readonly dan
mode view — label readonly dirender sebagai `<span>`.

Verifikasi rond 3: form edit user — 0 bad label, 6 label valid; `tsc --noEmit` bersih.

## File terkena dampak

- `renderers/react-shadcn/src/kinds/form/FormRenderer.tsx`
- `renderers/react-shadcn/src/widgets/{TextInput,TextareaInput,NumberInput,PasswordInput,DateInput,JsonInput,SliderInput,TagsInput,Switch,Combobox,RadioGroup,Select,ChildTable}.tsx`
- `renderers/react-shadcn/src/components/ui/select.tsx`

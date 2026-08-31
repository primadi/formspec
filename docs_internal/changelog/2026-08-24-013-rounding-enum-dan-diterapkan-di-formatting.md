# Rounding: Enum Dropdown + Diterapkan di Formatting

**Tanggal**: 2026-08-24 · **Plan**: `docs/plan/date-input-global-format-runtime-settings.md` (spec §10) · **Todo**: —

## Apa yang diubah

Dua perbaikan terkait setting `rounding` (mode pembulatan money/decimal, spec §10):

### (A) `rounding` jadi dropdown enum

- Sebelumnya field `rounding` di halaman Pengaturan dirender sebagai **textbox bebas**
  (`type: string` tanpa widget), padahal nilainya terbatas pada 5 mode.
- Kini dirender sebagai **dropdown (Select)**:
  - `examples/cafe/spec/modules/formspec.core/entities/app-setting.yaml`: field
    `rounding` tetap `type: string` (menghindari migrasi enum yang bermasalah)
    tapi diberi `enum_values: [half_even, half_up, half_down, up, down]`.
  - `examples/cafe/spec/modules/formspec.core/forms/settings-edit.yaml`: field
    `rounding` diberi `widget: select` → `FormRenderer` merender `<Select>`
    dengan opsi dari `entityField.enum_values`.
- Catatan: mengubah tipe field ke `enum` memicu error migrasi
  `duplicate column name: _name` (diff schema yang rapuh untuk perubahan tipe),
  jadi dipakai pendekatan `string` + `enum_values` + `widget: select`.

### (B) `rounding` benar-benar diterapkan di formatting

- Sebelumnya `rounding` **declared but unused** — `lib/format.ts` memakai
  `Intl.NumberFormat` yang tidak menerima mode rounding, jadi nilai `rounding`
  tidak memengaruhi output.
- Kini `lib/format.ts`:
  - Ekspor tipe `RoundingMode` + fungsi `roundTo(value, places, mode)` dengan
    semantik BigDecimal: `half_even` (banker's, default), `half_up`,
    `half_down`, `up`, `down`. Nilai di-"snap" ke presisi tinggi
    (`Math.round(v*factor*1e12)/1e12`) untuk membatalkan drift biner
    (mis. `1.005*100 = 100.49999…` → `100.5`) sehingga tie resolve ke digit
    yang benar.
  - `createFormatter` menerapkan `roundTo` sebelum `Intl.NumberFormat` di
    `money` dan `number`.
  - `types/manifest.ts`: `Settings.rounding` di-typing sebagai union
    `"half_even" | "half_up" | "half_down" | "up" | "down"`.
- Test: `src/lib/format.test.ts` — 12 test baru untuk `roundTo` (semua mode,
  tie, decimal places, non-finite) dan penerapan mode di `money`.

## File yang terkena dampak

- `examples/cafe/spec/modules/formspec.core/entities/app-setting.yaml`
- `examples/cafe/spec/modules/formspec.core/forms/settings-edit.yaml`
- `renderers/react-shadcn/src/lib/format.ts`
- `renderers/react-shadcn/src/types/manifest.ts`
- `renderers/react-shadcn/src/lib/format.test.ts`

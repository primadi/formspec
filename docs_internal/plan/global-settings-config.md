# Global Settings Config — Decimal Scale, Date Format, Money Format

**Status**: ✅ Complete (2026-08-24)
**Referensi spec**: `docs/spec/backend/01-core-basic.md` §10 (Config & Global Settings),
`docs/spec/backend/05-field-types.md` §2 (money), `docs/spec/frontend/06-page-kinds.md`
(format currency/date).

## Masalah

Saat ini format tampilan (money, date, decimal) **hard-code per komponen** di
frontend, dan tidak konsisten:

- `renderCell.tsx`, `ListingRenderer`, `DetailPage`, `ReportRenderer` → `en-US` + `USD`
- `DashboardRenderer` → `id-ID` + `IDR`
- `DateInput`, `DetailPage`, `TimelineRenderer` → `toLocaleString()`/`toLocaleDateString()`
  tanpa locale eksplisit
- `NumberInput` → `toLocaleString()` tanpa locale

Ini melanggar prinsip spec §10: **"jangan pernah menebak"** — setting yang
memengaruhi tampilan lintas-komponen harus hidup di satu tempat (global
`settings.*`), bukan ditebak per komponen.

## Solusi

Implementasikan namespace global `settings.*` yang sudah didefinisikan di spec
(Draft) sebagai kontrak yang berlaku:

1. **`pkg/spec`** — tambah struct `Settings` (typed) + `SettingsSpec` di
   `ConfigSpec` agar `settings.*` bisa dideklarasikan sebagai objek terstruktur
   (bukan string key acak). Field: `currency {code, decimal_places, symbol}`,
   `locale`, `timezone`, `date_format`, `decimal_scale`, `rounding`.
2. **Backend** — load `kind: Config` manifest, resolve nilai `settings.*` dengan
   default standar (ISO-8601 date, `en-US` locale, dst.), dan expose lewat
   bundle `/meta/ui` (`bundle.settings`).
3. **Frontend** — buat util format terpusat (`lib/format.ts`) yang membaca
   `bundle.settings` dari meta store, dan refactor semua hard-code
   `Intl.NumberFormat`/`toLocaleString`/`toLocaleDateString` untuk memakainya.

## File yang Diubah

| File                                                               | Perubahan                                                        |
| ------------------------------------------------------------------ | ---------------------------------------------------------------- |
| `pkg/spec/resources.go`                                            | Tambah `SettingsSpec` + `Settings` struct, `ConfigSpec.Settings` |
| `internal/manifest/loader.go`                                      | Parse `settings` dari Config spec                                |
| `internal/ui/meta.go`                                              | Tambah `Settings` ke `Bundle` + `AppContext`                     |
| `internal/api/meta.go`                                             | Wire settings ke `AppContext`                                    |
| `resource/formspec.go`                                             | Load settings dari Config manifests, pass ke router              |
| `renderers/.../types/manifest.ts`                                  | Tambah `Settings` type + `MetaBundle.settings`                   |
| `renderers/.../stores/meta.ts`                                     | Selector `getSettings`                                           |
| `renderers/.../lib/format.ts`                                      | **Baru** — util format terpusat                                  |
| `renderers/.../lib/renderCell.tsx`                                 | Pakai `format.ts`                                                |
| `renderers/.../kinds/{dashboard,listing,page,report,timeline}/...` | Pakai `format.ts`                                                |
| `renderers/.../widgets/{DateInput,NumberInput}.tsx`                | Pakai `format.ts`                                                |
| `examples/cafe/.../config/*.yaml`                                  | Contoh `settings.*`                                              |

## Level of Effort

- `pkg/spec` + backend wiring: **medium**
- Frontend format util + refactor: **medium**
- Contoh + test + changelog: **small**

## Dependensi

- Tidak ada dependensi eksternal; mengikuti kontrak spec yang sudah Draft.
- Setelah `pkg/spec` berubah → `make generate-schema` + `make generate` (TS types).

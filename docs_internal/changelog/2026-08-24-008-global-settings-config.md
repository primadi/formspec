# Global Settings Config — Decimal Scale, Date Format, Money Format

**Tanggal**: 2026-08-24 · **Plan**: `docs/plan/global-settings-config.md`

## Apa yang diubah

Implementasi namespace global `settings.*` (spec §10 — "jangan pernah
menebak") sebagai kontrak yang berlaku, sehingga format tampilan lintas
komponen (money, date, decimal) diatur dari satu tempat, bukan hard-code per
komponen.

**Backend:**

- `pkg/spec/resources.go`: tambah struct `Settings` + `CurrencySettings`
  (currency `{code, decimal_places, symbol}`, `locale`, `timezone`,
  `date_format`, `decimal_scale`, `rounding`) di `ConfigSpec`. `DecimalPlaces`
  pointer agar `0` (IDR tanpa minor unit) bisa dibedakan dari "unset".
  `ResolveSettings()` overlay deklarasi ke default standar (USD, en-US, UTC,
  YYYY-MM-DD, scale 2, half_even).
- `internal/ui/meta.go`: `Bundle.Settings` + `AppContext.Settings` — settings
  ter-resolve dikirim di setiap `/meta/ui` bundle.
- `internal/api/router.go` + `meta.go`: `SetSettings()` + wire ke
  `resolveAppContext`.
- `resource/formspec.go`: load `settings:` dari `kind: Config` manifest
  (yang pertama menang), resolve dengan default, pasang di router — termasuk
  jalur hot-reload.
- `internal/genjsonschema/generator.go`: `Settings`/`CurrencySettings` masuk
  `sharedTypes` agar schema ter-emit.

**Frontend:**

- `types/manifest.ts`: type `Settings` + `MetaBundle.settings`.
- `stores/meta.ts`: selector `getSettings()`.
- `lib/format.ts` (baru): `createFormatter(settings)` — util format terpusat
  (money/number/date/dateTime/relative) yang membaca settings global.
- Refactor semua hard-code format ke formatter: `renderCell.tsx`,
  `TableRenderer`, `ChildTable`, `ListingRenderer`, `DashboardRenderer`,
  `DetailPage`, `ReportRenderer`, `TimelineRenderer`, `DateInput`,
  `NumberInput`. Menghapus inkonsistensi `en-US`/`USD` vs `id-ID`/`IDR`.

**Contoh:**

- `examples/cafe/spec/modules/formspec.core/config.yaml` (baru): deklarasi
  `settings:` — IDR (0 desimal, simbol Rp), id-ID, Asia/Jakarta,
  DD/MM/YYYY, scale 2, half_even.

## Kenapa

Sebelumnya format tampilan hard-code per komponen dan tidak konsisten
(`renderCell.tsx` pakai en-US/USD, `DashboardRenderer` pakai id-ID/IDR).
Melanggar prinsip spec §10: setting yang memengaruhi tampilan lintas-komponen
harus hidup di satu tempat, bukan ditebak per komponen.

## File terdampak

- `pkg/spec/resources.go`, `internal/ui/meta.go`, `internal/api/{router,meta}.go`,
  `resource/formspec.go`, `internal/genjsonschema/generator.go`
- `renderers/react-shadcn/src/{types/manifest.ts, stores/meta.ts, lib/format.ts,
lib/format.test.ts, lib/renderCell.tsx}` + 8 renderer/widget files
- `examples/cafe/spec/modules/formspec.core/config.yaml`
- `schemas/` (regenerated)

## Test

- `go test ./...` — semua pass (termasuk `TestHandleMetaUI_SettingsInBundle`,
  `TestResolveSettings_Defaults`).
- `npx vitest run` — 113 pass (termasuk 10 test `format.test.ts`).
- `formspec validate --schema schemas` — 19 manifest cafe pass.
- Meta bundle `/meta/ui` mengembalikan settings ter-resolve (verified via curl).

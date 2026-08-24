# Seed `app-setting` Default dari Manifest Saat Find-or-Create

**Tanggal**: 2026-08-24 · **Plan**: `docs/plan/date-input-global-format-runtime-settings.md` (Configuration Page pattern, spec §10) · **Todo**: —

## Apa yang diubah

- **Bug**: halaman Pengaturan (`/settings`, Configuration Page atas entity
  `app-setting`) menampilkan form **kosong** pada akses pertama, padahal
  manifest `kind: Config` `spec.settings:` sudah mendeklarasikan default
  (IDR, id-ID, Asia/Jakarta, DD/MM/YYYY, dst).
- **Akar masalah**: find-or-create reference entity (`HandleFind`) hanya
  meng-seed **natural key** (`{name: "global"}`) — tidak menyalin nilai
  default dari manifest `settings:`. Record `app-setting` dibuat kosong,
  sehingga form edit menampilkan field kosong. Nilai default hanya diterapkan
  pada `bundle.settings` (formatting), bukan pada record/UI form.
- **Perbaikan** (`internal/api/handler.go`, `internal/api/router.go`):
  - `HandlerFactory` kini punya setter `SetSettings` (field `settings` sudah
    ada tapi belum di-wire) — `RouterBuilder.SetSettings` meneruskannya ke
    factory.
  - Saat find-or-create untuk `formspec.core/app-setting`, record di-seed
    dengan nilai default dari resolved settings via helper `seedSettingsData`
    (memetakan `Settings` → field `app-setting`: `currency_code`,
    `currency_decimal_places`, `currency_symbol`, `locale`, `timezone`,
    `date_format`, `decimal_scale`, `rounding`). Hanya nilai non-empty yang
    ditulis; entity reference lain tetap hanya meng-seed natural key.
- **Test**: `internal/api/handler_settings_seed_test.go` — verifikasi
  `app-setting` ter-seed dari settings, dan entity reference lain TIDAK
  ter-seed settings.

## File yang terkena dampak

- `internal/api/handler.go`
- `internal/api/router.go`
- `internal/api/handler_settings_seed_test.go` (baru)

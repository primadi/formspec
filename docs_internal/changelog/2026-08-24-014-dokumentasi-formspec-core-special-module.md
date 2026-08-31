# Dokumentasi `formspec.core` sebagai Special Module

**Tanggal**: 2026-08-24 · **Plan**: `docs/spec/platform/02-workspace-app-module.md` §9 · **Todo**: —

## Apa yang diubah

`docs/spec/platform/02-workspace-app-module.md` §9 "Resource Bawaan
`formspec.core`" diperkaya untuk menegaskan status `formspec.core` sebagai
**module khusus (special/reserved module)** framework, bukan sekadar daftar
resource bawaan:

- **Intro** kini menyatakan `formspec.core` adalah special/reserved module —
  selalu ada di setiap workspace, tidak perlu `depends_on`, dan **tidak boleh
  dideklarasikan oleh Module user**.
- **Subsection baru §9.1 "Karakteristik khusus"** mendokumentasikan:
  - **Reserved namespace** — user tidak bisa mendeklarasikan module bernama
    `formspec.core`.
  - **Bundled module (dogfooding)** — di-embed ke binary
    (`internal/auth/module/`, `//go:embed`), dimuat lewat manifest loader;
    `formspec generate auth` menyalin ke `external/auth` untuk dikustomisasi.
  - **Special-casing framework** — global settings (`app-setting`,
    Configuration Page pattern, `mergeRunningSettings` + `seedSettingsData`)
    dan auth core (`user`, `session`, `role`, `api-key`, `app-membership`,
    `workspace`).
  - **Selalu tersedia** — tanpa deklarasi `depends_on`.
- **Tabel resource** ditambah baris `app-setting` (record runtime
  global-settings, natural key `"global"`).
- **Subsection baru §9.2 "Route & Page yang disediakan"** — halaman eksplisit
  (`access-management` → `/access-management`, `settings` → `/settings`) dan
  derived CRUD route untuk entity `ui-exposed` (`user`, `role`, `api-key`,
  `app-membership`); `workspace`/`session` internal tanpa route UI.
- **Subsection baru §9.3 "Akses dari script"** — `resource.fetch("formspec.core.<entity>", id)`,
  `ctx.config().get("settings.*")`, dan `ctx.db()` (dengan deklarasi `uses`).
- **Subsection baru §9.4 "Override default value"** — runtime settings
  (`app-setting`), override module auth (`external/` via `formspec generate
auth`), shadow copy (`overrides/`), dan `auth_config_ref`.

## File yang terkena dampak

- `docs/spec/platform/02-workspace-app-module.md`

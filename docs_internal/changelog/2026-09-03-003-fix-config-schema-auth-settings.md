# 2026-09-03-003 — Fix Config schema: settings.auth "Property auth is not allowed"

## Apa yang diubah

- `internal/genjsonschema/generator.go`: tambahkan `AuthSettings`,
  `OAuthProviderSettings`, dan `RegistrationSettings` ke allowlist
  `sharedTypes` (di samping `Settings`/`CurrencySettings`).
- Regenerate JSON Schema via `make generate-schema` → `schemas/` (root +
  kinds), lalu stage ke `schemas/dist/` (v1 + alias latest) via
  `scripts/publish-schemas.sh`.

## Kenapa

Manifest `registry/spec/modules/portal/config.yaml` (Config kind, Fase 5 OAuth)
mendeklarasikan `spec.settings.auth.providers`, tapi schema yang dipakai VS Code
(`schemas/dist/latest/formspec.schema.json`) memvalidasi `Settings` tanpa properti
`auth` → error "Property auth is not allowed". Akar masalah: tipe `AuthSettings`
dkk sudah ada di `pkg/spec/resources.go` (Settings.Auth / Settings.Registration)
tapi belum masuk allowlist `sharedTypes` generator → `$defs` tidak di-emit,
`$ref: "#/$defs/AuthSettings"` jadi dangling, dan copy `dist/latest` bahkan tidak
memiliki properti `auth` sama sekali (stale).

## File terdampak

- `internal/genjsonschema/generator.go`
- `schemas/formspec.schema.json`, `schemas/kinds/*.schema.json`
- `schemas/dist/v1/*`, `schemas/dist/latest/*`

## Referensi

- `pkg/spec/resources.go` — `Settings.Auth` / `Settings.Registration`
- `registry/spec/modules/portal/config.yaml` — manifest yang memicu error
- Todo: Fase 5 auth (OAuth multi-provider)

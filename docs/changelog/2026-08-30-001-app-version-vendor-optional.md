# 2026-08-30 — 001 — `App.spec.version` & `App.spec.vendor` jadi optional

## Apa

`version` dan `vendor` pada `kind: App` tidak lagi wajib. Hanya `root_url`
yang tetap required (identitas routing App — unik per workspace; private App
harus `/app` atau prefix `/app/`, public App boleh `/`).

## Kenapa

Keduanya tidak dikonsumsi runtime sama sekali — hanya metadata publishing
marketplace. Mewajibkannya di setiap manifest App terlalu verbose untuk
kasus umum (dev/internal app). Sesuai prinsip _Convention over
Configuration_: syarat publishing seharusnya ditegakkan di pipeline
marketplace, bukan di booting App.

## File terkena dampak

- `pkg/spec/resources.go` — `omitempty` pada `AppSpec.Version`/`Vendor` + komentar penjelasan
- `schemas/formspec.schema.json`, `schemas/kinds/App.schema.json` — regenerate (`make generate-schema`), `required` kini hanya `root_url`
- `docs/kind/curation/App.md` — regenerate tabel atribut + update gotcha
- `docs/spec/platform/02-workspace-app-module.md` §4 — komentar optional di contoh
- `cmd/formspec/validate_test.go` — `TestValidateSchema_AppRequiresRootURL` kini assert App tanpa version/vendor diterima, tanpa root_url ditolak
- `ai_skills/formspec-kinds/SKILL.md`, `ai_skills/schema-validation/SKILL.md` + mirror `examples/{arisan,cafe,crc-management}/.agents/skills/`

## Referensi

- Diskusi: gotcha `docs/kind/curation/App.md` (root_url `/app/` prefix & kebutuhan optional)
- Changelog terkait: `2026-08-19-001-landing-page-app-renderer.md` (longgarkan pattern `root_url`)

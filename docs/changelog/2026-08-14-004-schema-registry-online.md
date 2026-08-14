# 2026-08-14-004 — Schema registry online + versi spec di apiVersion

## Perubahan

JSON Schema tidak lagi di-embed di binary (`embed_schemas.go` dihapus).
`formspec validate`, `formspec init`, dan subcommand baru `formspec schema`
memakai **schema registry** (`https://schemas.formspec.dev`, override
`FORMSPEC_SCHEMA_REGISTRY` atau `schema-registry:` di `formspec-app.yaml`) yang
di-cache lokal di `os.UserCacheDir()/formspec/schemas/<version>`.

Versi spec ditulis di `apiVersion` tiap manifest (`formspec.dev/v1`). Engine
(`internal/manifest`) melakukan version-routing via tabel `Versions` — v1, v2,
… dapat coexist tanpa install ulang CLI. `apiVersion` di-rename repo-wide dari
`formspec.dev/v1alpha1` → `formspec.dev/v1` (scope manifest; CRD operator K8s
`internal/operator/api/v1alpha1` + `deploy/operator/crds` tetap `v1alpha1`).

## Kenapa

Versi spec baru tidak boleh memaksa user install ulang CLI. Karena versi berada
di YAML dan schema diambil online, upgrade = ubah `apiVersion` + fetch schema
versi baru.

## File terdampak

- `internal/schemaregistry/` (baru) — client registry + cache: `ParseVersion`,
  `Ensure` (kinds spesifik), `EnsureFull` (via `index.json`), `List`, `Clear`.
- `cmd/formspec/validate.go` — schema layer version-routed; `--schema-refresh`;
  hapus fallback embedded (`resolveSchemaDir` dibuang).
- `cmd/formspec/init.go` — `extractSchemas` → `fetchSchemas` (registry); scaffold
  `schema-registry:` di `formspec-app.yaml`; offline → warn + skip.
- `cmd/formspec/schema.go` (baru) + `main.go` — subcommand
  `formspec schema fetch|update|list|clear`.
- `cmd/formspec/schema_registry.go` (baru) — resolusi registry URL
  (env > config > default).
- `internal/manifest/loader.go` — tabel `Versions`, `Validate` dispatch per versi.
- `embed_schemas.go` — dihapus.
- `scripts/publish-schemas.sh` — `index.json` kini memuat daftar `kinds`
  (registry self-describing).
- Rename `v1alpha1` → `v1`: `examples/`, `verticals/`, fixtures, test, `docs/`,
  `ai_skills/`.

## Referensi

`docs/plan/schema-registry-online.md`; todo 3.1.1 / 3.1.4.

# Plan — Schema Registry Online + Versi Spec di YAML

Status: **implemented** (2026-08-14). Changelog: `2026-08-14-004-schema-registry-online.md`.

## Masalah

- JSON Schema di-embed ke binary (`//go:embed`), jadi versi spec baru memaksa
  user install ulang CLI.
- Versi spec tidak punya tempat eksplisit: `apiVersion` hanya dicek non-kosong.
- Dua sumber schema (embed + remote) menambah duplikasi dan gesekan
  (`git add -f schemas/dist/`).

## Keputusan

1. **Single CLI `formspec`** — versi spec ditulis di `apiVersion` manifest
   (`formspec.dev/v1`, `formspec.dev/v2`, …); v1..n bisa coexist.
2. **Semua online** — schema diambil dari registry (`https://schemas.formspec.dev`,
   override `FORMSPEC_SCHEMA_REGISTRY` atau `schema-registry:` di
   `formspec-app.yaml`), di-cache lokal `os.UserCacheDir()/formspec/schemas/<version>`.
3. **Tanpa embed** — `embed_schemas.go` dihapus; offline = cache (belum ada
   cache = error jelas).
4. **Engine version-routing** — `internal/manifest` tabel `Versions`; hanya v1
   yang diimplementasikan sekarang; v2+ tinggal daftar di tabel + validator.
5. **Rename `v1alpha1` → `v1`** — manifest scope (bukan CRD operator K8s).

## Desain

```
apiVersion: formspec.dev/v1  (di tiap manifest)
        │  ParseVersion
        ▼
  version "v1"
        │  registry URL: <base>/v1/{formspec.schema.json, index.json, kinds/*.schema.json}
        ▼
  cache: ~/.cache/formspec/schemas/v1/   (Ensure / EnsureFull)
        │
        ├─ formspec validate  → kompilasi per kind per versi
        ├─ formspec init      → salin ke <project>/schemas/ + .vscode/settings.json
        └─ formspec schema fetch|update|list|clear
```

## Implementasi

- `internal/schemaregistry/` — `ParseVersion`, `Client` (BaseURL/CacheRoot/HTTP),
  `Ensure` (kinds dari manifest), `EnsureFull` (kinds dari `index.json`),
  `List`, `Clear`, `ResolveBaseURL`.
- `cmd/formspec/validate.go` — layer 2 version-routed; `--schema <dir>` override
  lokal; `--schema-refresh`.
- `cmd/formspec/init.go` — `fetchSchemas`; scaffold `schema-registry:`.
- `cmd/formspec/schema.go` — subcommand `formspec schema`.
- `internal/manifest/loader.go` — `Versions` table + `Validate` dispatch.
- `scripts/publish-schemas.sh` — `index.json` memuat `kinds` (self-describing).

## Verifikasi

- `go test ./...` — 542 pass, 9 gagal pre-existing (Clinic-UI-Showcase e2e).
- `formspec validate --spec examples/crc-management/spec` → `schema: v1
(registry …, cache …)`, 34 manifest, 0 problem.
- Offline tanpa cache → error jelas; offline dengan cache → sukses.
- `formspec schema fetch/list/clear` → berfungsi.
- `formspec init` (test) → `schemas/` terisi dari registry lokal.

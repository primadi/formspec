# 2026-09-11-004 — `formspec init` tidak lagi fetch schemas, pakai URL registry langsung

## Apa

`formspec init` tidak lagi mendownload schema set (root + kinds/\*) dari registry
ke `schemas/` project. Sebagai gantinya, `.vscode/settings.json` (yaml.schemas)
ditulis langsung menunjuk ke URL registry:

    "yaml.schemas": {
      "https://schemas.formspec.dev/v1/formspec.schema.json": ["spec/**/*.yaml", "spec/**/*.yml"]
    }

Fungsi `fetchSchemas` di `cmd/formspec/init.go` dihapus; `copySchemas` dipindah
ke `cmd/formspec/schema.go` karena masih dipakai `formspec schema fetch --out`.

## Kenapa

Init jadi cepat & bebas network — scaffold tetap jalan offline. Root schema
formspec self-contained (`$defs` internal), jadi YAML extension VS Code cukup
resolusi URL remote tanpa salinan lokal.

## File terkena dampak

- `cmd/formspec/init.go` — hapus `fetchSchemas`, pakai konstanta `schemaURL`
- `cmd/formspec/schema.go` — terima `copySchemas` dari init.go

## Referensi

- `docs_internal/plan/schema-registry-online.md`

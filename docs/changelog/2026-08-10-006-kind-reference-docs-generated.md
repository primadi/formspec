# 2026-08-10-006-kind-reference-docs-generated

**Lapis**: Docs infrastructure (baru) — `docs/kind/` + tooling
**Referensi**: `docs/plan/kind-reference-docs.md`

## Perubahan

Buat folder **`docs/kind/`** — referensi per-kind yang lengkap, **hybrid
generated + narrative**:

- **33 file kind** dipecah 4 grup: `curation/` (2), `data/` (11), `ui/` (15), `infra/` (5) + `README.md`
- Tiap file: `Kapan Memakai` + `Contoh Manifest` + `Atribut` (generated) + `Gotchas`
- **Zero drift**: tabel atribut di-generate dari `pkg/spec` (Go struct + godoc + `@schema` annotations) — sumber yang sama dengan `schemas/kinds/*.schema.json`
- **Protected region**: narasi manual di luar marker `<!-- generated:... -->` tidak pernah ditimpa saat regenerate (idempotent — diverifikasi `diff -r` kosong)

## Tooling baru

| Item | Detail |
|---|---|
| `internal/genkinddocs/markdown.go` | Markdown emitter + protected region merge |
| `cmd/formspec-gen-kind-docs/` | CLI: `formspec-gen-kind-docs [--out docs/kind]` |
| `Makefile` → `generate-kind-docs` | Regenerate 33 file; idempotent |

## Perbaikan parser

- `internal/genjsonschema/converter.go`: export `SchemaAnnotation`; `extractSchemaBody` baru — parsing `@schema {...}` kini **seimbang brace** (nilai dengan brace bertingkat seperti `"Order {order.number}"` tidak terpotong)
- `internal/genkinddocs`: `fieldText` strip baris `@schema` (bukan regex) — tidak bocor ke deskripsi

## File terdampak

- `docs/kind/` (34 file baru)
- `internal/genkinddocs/markdown.go` (baru), `cmd/formspec-gen-kind-docs/main.go` (baru)
- `internal/genjsonschema/converter.go` (export + parser), `generator.go` (tipe rename)
- `Makefile` (target `generate-kind-docs`)

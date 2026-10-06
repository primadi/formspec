# Plan — Schema `guard` shorthand (GuardDecl `oneOf: [string, object]`)

## Masalah

`GuardDecl` punya `UnmarshalYAML` yang menerima **dua bentuk**:

- inline string (kanonis, `02-core-extended.md` §14): `guard: "len(resource.items) > 0"`
- map eksplisit: `guard: {expression: "...", message: "..."}`

Tetapi JSON Schema hasil generate mendefinisikan `GuardDecl` sebagai **object saja**.
Akibatnya editor YAML menandai bentuk kanonis sebagai
`Incorrect type. Expected "GuardDecl"` — loader menerima manifest yang ditolak
schema. Ini kelas divergensi yang sama dengan item **5.4** (`FormRenderDecl`),
dan sisa yang sudah dicatat di catatan todo **3.1.1**.

Ditemukan live pada `verticals/billing/spec/modules/billing/entities/order.yaml`
baris 62.

## Perubahan

| File | Perubahan | Effort |
| --- | --- | --- |
| `internal/genjsonschema/generator.go` | special-case `GuardDecl` → `oneOf: [string, object]` (pola sama dengan `FormRenderDecl`) | small |
| `internal/genjsonschema/schema_refs_test.go` | guard `TestGuardDecl_AcceptsShorthandAndObject` | small |
| `schemas/formspec.schema.json`, `schemas/dist/**`, `schemas/kinds/*` | regen via `make generate-schema` | small |
| `docs_internal/plan/todo.md` | tutup sisa catatan 3.1.1 (`guard` bagian) + update header | small |

Dependensi: tidak ada. Referensi: `docs/spec/backend/02-core-extended.md` §
(state machine `guard`), `pkg/spec/entity.go` `GuardDecl.UnmarshalYAML`.

## Verifikasi

- `go test ./internal/genjsonschema/` — termasuk `TestGeneratedSchemas_MatchOnDisk`
  (menuntut schema di disk ikut ter-regenerate).
- `formspec validate --spec verticals/billing/spec --schema schemas` → 0 problem
  pada `order.yaml` (sebelumnya `guard` ditolak lapis schema).

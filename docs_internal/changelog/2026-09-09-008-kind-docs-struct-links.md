# 2026-09-09-008 — Link struct ke spec normatif di docs/kind

## Apa

Generator kind-docs (`internal/genkinddocs`) kini merender nama struct di kolom
Tipe tabel atribut sebagai link ke dokumen spec normatifnya (mis. `Field` →
`docs/spec/backend/05-field-types.md`). Mapping ada di `structDocLinks`
(`internal/genkinddocs/markdown.go`); struct tanpa mapping tetap plain code.

Ditambahkan juga section naratif "Referensi Struct" di `docs/kind/data/Entity.md`
yang memetakan struct → dokumen + nomor section (§).

## Kenapa

Tabel atribut di `docs/kind/*/*.md` sebelumnya hanya menyebut nama struct
(`Field`, `StateMachine`, dll.) tanpa dokumentasi atau link — pembaca tidak
tahu isinya apa saja.

## File terkena dampak

- `internal/genkinddocs/markdown.go` — map `structDocLinks` + `namedType`
- `docs/kind/**` — 11 file diregenerasi via `make generate-kind-docs`
- `docs/kind/data/Entity.md` — section "Referensi Struct" (naratif, preserved)

## Referensi

- `docs/spec/backend/05-field-types.md` (Field)
- Workflow: docs_internal/plan/todo.md

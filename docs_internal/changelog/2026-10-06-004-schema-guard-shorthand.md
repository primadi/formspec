# 2026-10-06-004 — Schema `guard` shorthand: `GuardDecl` → `oneOf [string, object]`

## Apa yang diubah

`internal/genjsonschema/generator.go` kini punya special-case `GuardDecl` yang
meng-emit `oneOf: [string, object]` (pola sama dengan `FormRenderDecl`),
sehingga bentuk kanonis `guard: "len(resource.items) > 0"` diterima schema di
samping bentuk map `guard: {expression, message}`. Test guard baru:
`TestGuardDecl_AcceptsShorthandAndObject` di
`internal/genjsonschema/schema_refs_test.go`. `schemas/formspec.schema.json`
diregenerasi (berlaku di `$defs` **dan** alias `definitions`).

Plan: `docs_internal/plan/schema-guard-shorthand.md`. Todo: catatan 3.1.1 +
cross-ref note 5.4.

## Kenapa

Ditemukan pada `verticals/billing/spec/modules/billing/entities/order.yaml`
baris 62: editor menandai `guard: "..."` sebagai
`Incorrect type. Expected "GuardDecl"`. Itu **bukan** kesalahan YAML —
`GuardDecl.UnmarshalYAML` menerima scalar string maupun map, dan string inline
adalah bentuk kanonis (`docs/spec/backend/02-core-extended.md` §14, dipakai
`examples/kafe`). Ini sisa terakhir dari divergensi loader↔schema yang sudah
dicatat di item **3.1.1** ("gunakan bentuk objek") — kelas yang sama dengan
`FormRenderDecl` (**5.4**), yang hanya menutup `render`.

## Bukti

- Sebelum: `formspec validate --spec verticals/billing/spec --schema schemas`
  → `entities/order.yaml#0` FAIL (`/spec/state_machine/transitions/0/guard`).
- Sesudah: `entities/order.yaml#0` → `[OK]`; problem `20 → 19` (sisanya
  pra-ada, nol baru).
- `go test ./internal/genjsonschema/` hijau, termasuk
  `TestGeneratedSchemas_MatchOnDisk` (schema di disk wajib ikut ter-regenerate).

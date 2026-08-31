# Fase 7 — Katalog Rule L1–L3 Lengkap (7.9.6)

**Tanggal:** 2026-08-25 · **Todo:** §7.9.6

## Apa yang diubah

Katalog rule field-level L1–L3 (`05-field-types.md` §3) kini lengkap
server-side. Rule yang sudah ada: `required`, `min_length`, `max_length`,
`pattern`, `email`, `url`, `min`, `max`, `positive`, `precision`, `future`,
`past`, `after:<field>`, `before:<field>`, `min_items`, `max_items`,
`exists:<resource>`. Yang **baru ditambahkan** di `validateSingleRule`
(`renderers/jsonb-persist/crud.go`):

- **`length`** — panjang string persis.
- **`in`** — nilai termasuk himpunan yang diberikan (enum/set).
- **`script`** — escape-hatch Starlark inline; ekspresi dijalankan dengan
  `value` = nilai field, harus truthy untuk lolos.
- **`unique`** — unik per tenant (cross-record). `validateFieldRules` kini
  menerima `database DB` (transaksi aktif saat Insert, `txReadDB` saat Update)
  dan `validateUnique` query langsung melalui DB tersebut — menghindari
  deadlock SQLite koneksi-tunggal (masalah yang sama dengan
  `ValidateRelationTargets`). `excludeID` mengecualikan record yang sedang
  di-update agar update yang mempertahankan nilainya sendiri tidak
  false-positive.

## File terdampak

- `renderers/jsonb-persist/crud.go`
- `renderers/jsonb-persist/crud_test.go` (test `LengthInScript` + `Unique`)

## Status

`go test ./...` hijau (0 fail). Todo 7.9.6 ditandai ✅ (himpunan tertutup ~20
rule lengkap). 7.9.1–7.9.5 (L4 `business_rules`, L5 `cross_validate`, L6
`consistency`) tetap di-defer — kontrak deklarasi L4–L6 belum dispesifikasikan
di `pkg/spec` (butuh design decision).

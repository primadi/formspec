# 4.6 — Satuan & konversi: `unit.factors` + `ctx.unit.convert()` (S12)

**Tanggal:** 2026-09-18 · **Plan:** `docs_internal/plan/fase-4-kafe-stok-hpp.md`
(4.6) · **TODO:** `examples/kafe/gaps_found/TODO.md` 4.6

## Apa yang diubah

Deklarasi `unit: {base, convertible}` (item 1.8) diperluas dengan **`factors`**,
dan konversi dihitung engine lewat `ctx.unit.convert(entity, field, value, from, to)`.

- `pkg/spec/entity.go` — `UnitDecl.Factors` + `UnitDecl.Convert` + validasi.
- `internal/starlark/context.go` — `ctx.unit.convert` builtin + `UnitConvert`.
- `internal/starlark/executor.go` — `UnitConvertHandler`.
- `internal/action/script.go` — `SetUnitConvertHandler`.
- `resource/formspec.go` — wiring (resolve entity/field → deklarasi unit).
- `docs/spec/backend/05-field-types.md` §1.6 — normatif.
- `examples/kafe/spec/.../ingredient/entity.yaml`, `.../recipe/entity.yaml` —
  `factors: {kg: 1000}`.

## Kenapa

Item 1.8 menambahkan deklarasi satuan, tetapi tanpa **faktor** deklarasinya
hanya mengatakan "satuan ini berhubungan" tanpa mengatakan **bagaimana** —
sehingga konversi tetap harus hidup di script, persis konvensi yang ingin
dihapus. `factors` menutupnya: satuan menjadi data, dan konversi (mis. ledakan
resep gram↔kg) dihitung engine.

## Bukti

| Perintah | Hasil |
| --- | --- |
| `go test ./pkg/spec/ -run 'TestValidateEntitySpec_Unit\|TestUnitDecl_Convert'` | PASS |
| `TestUnitDecl_Convert` | kg→gram 2000, gram→kg 2, identitas, satuan luar grup → **error** |
| `TestValidateEntitySpec_Unit` | 8 bentuk ditolak (tanpa faktor, faktor base, faktor luar convertible, ≤0) |
| `formspec validate` kafe | **0 problem** (69 manifest) |
| `go test ./...` | hijau |

## Sisa

Ledakan resep penuh (HPP dari resep × harga bahan) tetap item **4.1** — 4.6
menyediakan konversinya, bukan valuasinya.

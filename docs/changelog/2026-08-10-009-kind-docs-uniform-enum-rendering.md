# 2026-08-10-009-kind-docs-uniform-enum-rendering

**Lapis**: Tooling (`internal/genkinddocs`) + docs (`docs/kind/`)
**Referensi**: `docs/plan/kind-reference-docs.md`

## Perubahan

Keputusan: **tidak membedakan `enum` vs `string enum`** di kolom Tipe — bagi
author YAML keduanya berarti himpunan nilai tertutup yang divalidasi identik.

Sebelumnya rendering tidak konsisten:
- Named enum type → `enum (`Characteristic`: master · transaction · ...)` (ada nama tipe Go)
- `string` + `@schema {enum}` → `enum (two_step_autosave · ...)` (tanpa nama tipe)

Perbaikan di `internal/genkinddocs/markdown.go` → `namedType`: hapus prefix nama
tipe Go — nama tipe tidak pernah muncul di YAML, murni noise untuk author.

## Dampak

- Semua enum kini seragam: `enum (a · b · c)` (mis. `characteristic`,
  `lifecycle`, `driver`, `serves`, `trust_tier`, `mode`)
- `docs/kind/README.md`: tambah notasi tipe `enum (...)` di section Cara Baca
- Idempotent; narasi manual tidak berubah

## File terdampak

- `internal/genkinddocs/markdown.go` — `namedType`
- `docs/kind/**` — tabel atribut di-render ulang
- `docs/kind/README.md` — notasi tipe

# 2026-08-10-008-kind-docs-enum-render-fix

**Lapis**: Tooling (`internal/genkinddocs`) + docs (`docs/kind/`)
**Referensi**: `docs/plan/kind-reference-docs.md`

## Perubahan

Fix inkonsistensi: field `string` yang dikonstrain `@schema {enum: [...]}` (mis.
`EntitySpec.lifecycle`, `FormSpec.mode`, `WebhookSpec.method`) sebelumnya
ter-render sebagai `string` polos di tabel atribut, sementara gotchas menyebutnya
enum.

Perbaikan di `internal/genkinddocs/markdown.go` → `fieldType`: jika field punya
annotation `enum` dan tipe Go-nya bukan named enum type, tampilkan
`enum (a · b · c)` di kolom Tipe. Named enum type (mis. `Characteristic`) tetap
ditampilkan dengan nama tipenya.

## Dampak

- `docs/kind/`: 13 field kini ter-render sebagai enum (konsisten dengan gotchas)
- Idempotent — narasi manual tidak berubah
- `go build ./...` + regenerasi `diff -r` kosong

## File terdampak

- `internal/genkinddocs/markdown.go` — `fieldType`
- `docs/kind/**` — tabel atribut di-render ulang

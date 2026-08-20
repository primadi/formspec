# 2026-08-17-009 — `formspec generate` money/file field type mapping (3.3.3)

**Referensi**: `docs/plan/todo.md` (3.3.1–3.3.3), `docs/cli-tools/03-formspec-generate.md` §3,
`docs/spec/backend/05-field-types.md` §1.3/§2.

## Apa yang diubah

Melengkapi field type mapping `formspec generate` (todo 3.3.3) untuk dua tipe
yang sebelumnya jatuh ke `unknown`:

- **`money`** → `{ amount: string; currency: string }` — pasangan first-class
  `{amount, currency}` (`05-field-types.md` §2); `amount` adalah decimal
  arbitrary-precision → **string**, bukan `number`.
- **`file` / `attachment`** → `{ key: string; filename: string; content_type:
string; size: number; checksum: string }` — pointer ke objek `ctx.storage`
  dengan metadata kanonik (`05-field-types.md` §1.3).

## Kenapa

`money` dan `file` adalah tipe first-class di spec tapi belum ada di tabel
mapping docs §3 dan belum diimplementasikan di `tsFieldType` — menghasilkan
`unknown` yang kehilangan semua type-safety. Kini ditetapkan di spec + kode.

## File terdampak

- `cmd/formspec/generate.go` — `tsFieldType` tambah case `FieldMoney`,
  `FieldFile`, `FieldAttachment`.
- `cmd/formspec/generate_test.go` — `TestTsFieldType_MoneyAndFile`.
- `docs/cli-tools/03-formspec-generate.md` — tabel §3 tambah baris `money` dan
  `file`/`attachment`.
- `docs/plan/todo.md` — 3.3.1–3.3.3 ✅.

## Verifikasi

`go test ./cmd/formspec/...` hijau; `go test ./...` hijau (19 paket ok, 0 fail);
`make build` hijau.

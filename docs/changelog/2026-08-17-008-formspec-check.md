# 2026-08-17-008 — `formspec check [--fix]` (3.1.2)

**Referensi**: `docs/plan/todo.md` (3.1.2), `docs/plan/formspec-check.md`,
`docs/cli-tools/02-formspec-cli.md` §3, `docs/spec/frontend/08-formspec-expr.md` §4.

## Apa yang diubah

Menambahkan `formspec check` — analisis statis **lintas-file** (melampaui
`validate` yang per-manifest). Melaporkan minimal:

1. **Form field reference** — `FormField.Field` mereferensi field yang tidak
   ada di skema Entity target → **error**.
2. **FormSpecExpr field reference** — `visible_when`/`readonly_when`/
   `required_when`/`compute` mereferensi `fields.<name>` yang tidak ada →
   **error** (per `08-formspec-expr.md` §4: referensi field tak ada = error,
   bukan fail-safe).
3. **Cross-module resource existence** — `uses.resources` mereferensi
   `{module}.{entity}` yang tidak ada → **error**.

`--fix` menghapus deklarasi `uses.resources` yang broken (target entity tidak
ada) — aman, tidak mengubah footprint consent; blok `uses`/`resources` kosong
dibersihkan. Penambahan deklarasi (perluasan consent) TIDAK otomatis —
butuh konfirmasi interaktif, di-defer.

## Kenapa

`validate` hanya memvalidasi per-manifest. `check` menutup celah referensi
lintas-file: Form yang mereferensi field/entity yang tidak ada, dan
`uses.resources` yang menunjuk resource yang tidak ada — error kelas ini
menggagalkan `formspec apply` (per `08-formspec-expr.md` §4), jadi menangkapnya
di CLI mencegah error runtime.

## File terdampak

- `cmd/formspec/check.go` (baru), `cmd/formspec/check_test.go` (baru),
  `cmd/formspec/main.go`
- `docs/plan/formspec-check.md` (baru), `docs/plan/todo.md`

## Verifikasi

`go test ./cmd/formspec/...` hijau; `go test ./...` hijau (19 paket, 0 fail);
`make build` hijau. Manual: spec dengan field/expr/uses yang salah → 4 error;
`--fix` menghapus `uses.resources` broken dan membersihkan blok kosong.

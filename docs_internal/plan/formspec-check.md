# Plan — Fase 3.1.2: `formspec check [--fix]`

**Tanggal**: 2026-08-17 · **Status**: In progress
**Referensi**: `docs/plan/todo.md` (3.1.2), `docs/cli-tools/02-formspec-cli.md` §3,
`docs/spec/frontend/08-formspec-expr.md` §4, `docs/spec/platform/02-workspace-app-module.md` §7

## Tujuan

`formspec check` adalah analisis statis **lintas-file** (melampaui `validate`
yang per-manifest). Wajib melaporkan minimal (per `02-formspec-cli.md` §3):

1. **Form field reference** — `FormField.Field` mereferensi field yang tidak
   ada di skema Entity target → **error**.
2. **FormSpecExpr field reference** — `visible_when`/`readonly_when`/
   `required_when`/`compute` mereferensi `fields.<name>` yang tidak ada →
   **error** (per `08-formspec-expr.md` §4: referensi field tak ada = error,
   bukan fail-safe; menggagalkan apply).
3. **Cross-module resource existence** — `uses.resources` mereferensi
   `{module}.{entity}` yang tidak ada di registry → **error**.
4. **Unused cross-module declaration** — `uses.resources` yang tidak pernah
   dipakai → **warning**.

`--fix` memperbaiki yang bisa diperbaiki otomatis: menghapus deklarasi
`uses.resources` yang tidak terpakai. Penambahan deklarasi (perluasan footprint
consent) TIDAK otomatis — butuh konfirmasi interaktif (di luar scope v1;
dilaporkan sebagai saran).

## Perubahan

| File                                | Perubahan                                                                                                             |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `cmd/formspec/check.go` (baru)      | `runCheck` — load semua manifest, bangun registry entity + UI, jalankan 4 check, `--fix` hapus deklarasi tak terpakai |
| `cmd/formspec/main.go`              | Route `check` ke `runCheck` (ganti fallback "not implemented")                                                        |
| `cmd/formspec/check_test.go` (baru) | Test 4 check + `--fix`                                                                                                |
| `docs/plan/todo.md`                 | Tandai 3.1.2 ✅                                                                                                       |

## Keputusan

- Registry entity dibangun dari `manifest.LoadAll` + `RawSpecToEntitySpec`
  (bukan `entity.Registry` yang butuh DB) — `check` murni statis, tanpa DB.
- Registry UI dipakai `ui.NewRegistry().Load(manifests)` untuk Form/Table/dst.
- Ekstraksi referensi field dari FormSpecExpr: regex `fields\.([a-z_][a-z0-9_]*)`
  (grammar §2 — referensi field pakai prefix `fields.`).
- `--fix` hanya menghapus deklarasi tak terpakai (aman, tidak mengubah
  footprint consent). Penambahan deklarasi = perluasan consent → interaktif,
  di-defer.

## Verifikasi

- `go test ./cmd/formspec/...` hijau.
- `make build` hijau.
- Manual: `formspec check -f <spec>` pada contoh dengan field/expr/uses yang
  salah → error/warning sesuai; `--fix` menghapus deklarasi tak terpakai.

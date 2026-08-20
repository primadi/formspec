# Plan — Fase 3.6.1: `formspec migrate plan|apply`

**Tanggal**: 2026-08-17 · **Status**: In progress
**Referensi**: `docs/plan/todo.md` (3.6.1), `docs/cli-tools/02-formspec-cli.md` §3,
`renderers/jsonb-persist/migrate.go`

## Tujuan

`formspec migrate plan|apply` — memicu/inspeksi migrasi struktural otomatis
dari Entity diff (migrasi sendiri fully automatic, bukan hand-written).

- `formspec migrate plan` — tampilkan DDL yang akan dijalankan, tanpa eksekusi.
- `formspec migrate apply` — eksekusi (biasanya otomatis lewat `formspec apply`).

## Perubahan

| File                                  | Perubahan                                                                                                                                                              |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/formspec/migrate.go` (baru)      | `runMigrate` — load entity manifests → `[]db.EntityMigration`; buka DB dari `--dsn`; `plan` → `PlanMigrations` + cetak DDL; `apply` → `ApplyMigrations` + lapor jumlah |
| `cmd/formspec/main.go`                | Route `migrate` ke `runMigrate`; hapus dari "not implemented"; tambah usage                                                                                            |
| `cmd/formspec/migrate_test.go` (baru) | Test build entity migration list + plan terhadap SQLite in-memory                                                                                                      |
| `docs/plan/todo.md`                   | Tandai 3.6.1 ✅                                                                                                                                                        |

## Keputusan

- Daftar entity dibangun langsung dari `manifest.LoadAll` + `RawSpecToEntitySpec`
  (bukan `entity.Registry` yang butuh DB) — konsisten dengan `check`/`get`.
- `--dsn` default `sqlite:.formspec/data.db` (sama dengan `formspec dev`).
- `plan` mencetak satu blok DDL per entity (description + DDL); `apply` memakai
  `ApplyMigrations` yang sudah ada.

## Verifikasi

- `go test ./cmd/formspec/...` hijau.
- `make build` hijau.
- Manual: `migrate plan` pada spec contoh → DDL tampil; `migrate apply` → tabel
  dibuat.

# 2026-08-17-012 — `formspec migrate plan|apply` (3.6.1)

**Referensi**: `docs/plan/todo.md` (3.6.1), `docs/cli-tools/02-formspec-cli.md` §3,
`docs/plan/formspec-migrate.md`.

## Apa yang diubah

Menambah `formspec migrate plan|apply` — memicu/inspeksi migrasi struktural
otomatis dari Entity diff (migrasi sendiri fully automatic, bukan hand-written):

- `formspec migrate plan` — `PlanMigrations` + cetak DDL yang akan dijalankan,
  tanpa eksekusi.
- `formspec migrate apply` — `ApplyMigrations` (idempotent; plan ulang setelah
  apply → "No pending migrations").
- Daftar entity dibangun langsung dari manifest lokal
  (`loadEntityMigrations`), `--dsn` default `sqlite:.formspec/data.db`.

## Kenapa

`migrate` sebelumnya jatuh ke "not implemented". Memberi cara memicu/inspeksi
migrasi struktural dari CLI — melengkapi siklus deployment single-server.

## File terdampak

- `cmd/formspec/migrate.go` (baru), `cmd/formspec/migrate_test.go` (baru),
  `cmd/formspec/main.go`
- `docs/plan/formspec-migrate.md` (baru), `docs/plan/todo.md`

## Verifikasi

`go test ./cmd/formspec/...` hijau; `go test ./...` hijau (19 paket ok, 0 fail);
`make build` hijau; manual: `migrate plan` → DDL tampil; `migrate apply` → 6
migrasi; plan ulang → 0 pending (idempotent).

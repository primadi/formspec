# 2026-09-06-004 — DSN Relatif Di-anchor ke Lokasi Spec

## Apa

Path SQLite **relative** pada `--dsn` kini di-anchor ke project root yang
di-derive dari lokasi `--spec` (bukan working directory), sehingga file
database statis di mana pun perintah dijalankan:

- `formspec dev --spec examples/Clinic-UI-Showcase/spec --dsn
"sqlite:.formspec/clinic.db"` dijalankan dari root repo maupun dari
  folder example → db selalu
  `examples/Clinic-UI-Showcase/.formspec/clinic.db`
- DSN **absolute** dipakai apa adanya; DSN **postgres** tidak diubah;
  query param SQLite (`?_pragma=…`) dipertahankan; resolusi idempotent
- `--state-dir` default `formspec dev` ikut derive dari DSN ter-resolve

Implementasi: helper `resolveDSN(dsn, specPath)` di `cmd/formspec/dsn.go`
(+ unit test `dsn_test.go`), dipanggil di `runDev`, `runMigrate`,
`runBackupCreate`, `runRestore`, `runRepl`, `runArchiveRun`,
`runArchiveView`. `ParseDSN` (`renderers/jsonb-persist/config.go`) tidak
diubah — tetap pure parser. `projectRootOf` di `resource/formspec.go`
di-export menjadi `ProjectRootOf` untuk reuse CLI.

## Kenapa

Sebelumnya path relative diteruskan apa adanya ke SQLite yang menafsirkannya
relatif terhadap CWD proses — menjalankan `formspec dev` dari root repo vs
dari folder example menghasilkan dua database berbeda (keputusan user
2026-09-06: relative → anchor ke lokasi spec).

## File Terdampak

- `cmd/formspec/dsn.go`, `dsn_test.go` (baru)
- `cmd/formspec/dev.go`, `migrate.go`, `backup.go`, `repl.go`, `archive.go`
- `resource/formspec.go` (export `ProjectRootOf`)
- Docs: `docs/cli-tools/01-formspec-dev.md`,
  `examples/Clinic-UI-Showcase/how-to-run.md`,
  `examples/Clinic-UI-Showcase/docs/development.md`

## Referensi

- Plan: `docs_internal/plan/dsn-spec-anchored.md`
- Todo: 3.6.5

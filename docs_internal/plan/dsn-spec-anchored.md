# Plan — DSN Relatif Di-anchor ke Lokasi Spec

Status: ✅ Selesai (changelog 2026-09-06-004, todo 3.6.5)

## Masalah

Path SQLite pada DSN (`sqlite:.formspec/clinic.db`) tidak di-resolve —
diteruskan apa adanya ke SQLite yang menafsirkannya **relatif terhadap CWD
proses**. Akibatnya:

- Jalankan dari root repo → `.formspec/clinic.db` di root repo
- Jalankan dari `examples/Clinic-UI-Showcase/` → db lain di folder example

Dua lokasi menjalankan = dua database berbeda. Tidak deterministik.

## Keputusan (D: user, 2026-09-06)

- **Absolute DSN** → dipakai apa adanya.
- **Relative DSN** → di-anchor ke **lokasi spec** (project root yang
  di-derive dari spec dir, konvensi `08-project-layout.md`: spec tinggal di
  `<root>/spec`), sehingga file db statis di
  `<project-root>/.formspec/…` di mana pun perintah dijalankan.
- DSN non-SQLite (postgres) tidak diubah.

## File yang dibuat/diubah

| File                              | Perubahan                                                                                                                    |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `resource/formspec.go`            | Export `projectRootOf` → `ProjectRootOf` (dipakai ulang CLI)                                                                 |
| `cmd/formspec/dsn.go` (baru)      | Helper `resolveDSN(dsn, specPath)` — anchor relative sqlite path ke project root spec; idempotent; query param dipertahankan |
| `cmd/formspec/dsn_test.go` (baru) | Unit test: relative, absolute, postgres, no-scheme, query, idempotent                                                        |
| `cmd/formspec/dev.go`             | `runDev`: resolve DSN setelah config merge; `StateDir` default ikut derive dari DSN ter-resolve                              |
| `cmd/formspec/migrate.go`         | `runMigrate`: resolve DSN                                                                                                    |
| `cmd/formspec/backup.go`          | `runBackupCreate`, `runRestore`: resolve DSN                                                                                 |
| `cmd/formspec/repl.go`            | `runRepl`: resolve DSN                                                                                                       |
| `cmd/formspec/archive.go`         | `runArchiveRun`, `runArchiveView`: resolve DSN                                                                               |
| docs terkait                      | `how-to-run.md` (example), `docs/cli-tools/01-formspec-dev.md`, docs Clinic                                                  |

## Estimasi

Small.

## Referensi

- `renderers/jsonb-persist/config.go` — `ParseDSN` (tidak diubah: tetap
  pure parser, resolusi path terjadi di lapisan CLI)
- `resource/ctxresolver.go` — `StateDirFromDSN` (ikut DSN ter-resolve)
- `docs/spec/08-project-layout.md` — konvensi `<root>/spec`

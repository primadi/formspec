# 2026-08-12-001-rename-forma-to-formspec

Rename total: **forma → formspec**. Brand = **FormSpec**, domain = `formspec.dev`.

## Alasan

Nama "Forma" diambil produk AI code-gen lain (`getforma.dev`, YC S26, CLI `forma`,
roadmap Go Q3 2026). Benturan CLI + pencarian tidak bisa dihindari.

## Yang berubah

| Aspek        | Detail                                                                                                            |
| ------------ | ----------------------------------------------------------------------------------------------------------------- |
| Module path  | `github.com/primadi/forma` → `github.com/primadi/formspec`                                                        |
| apiVersion   | `forma.dev/v1alpha1` → `formspec.dev/v1alpha1`; `forma/v1` → `formspec/v1`                                        |
| Binary       | `forma`/`forma-ctl`/`forma-operator`/`forma-gen-schema`/`forma-gen-kind-docs` → `formspec*`                       |
| CLI          | `formspec dev/apply/validate/generate/init/check/consult`                                                         |
| Config file  | `forma-app.yaml` → `formspec-app.yaml`                                                                            |
| Brand text   | "Forma" → "FormSpec" di semua prose aktual                                                                        |
| Error codes  | `FORMA.*` → `FORMSPEC.*`                                                                                          |
| FormaExpr    | `FormSpecExpr` (folder `lib/formaexpr` → `lib/formspec-expr`, doc `08-formaexpr.md` → `08-formspec-expr.md`)      |
| FormaError   | `FormSpecError` (pkg/spec + semua SDK)                                                                            |
| Schema       | `schemas/forma.schema.json` → `schemas/formspec.schema.json`                                                      |
| System DB    | `forma_*` (outbox, audit*log, event_log, schema_migrations, control, shared, ops_backup, ops_ddl) → `formspec*\*` |
| SDK packages | Java `io/forma` → `io/formspec`, Python `lib_forma` → `lib_formspec`, Ruby `lib/forma` → `lib/formspec`           |
| Deploy       | CRD `forma.dev_*.yaml` → `formspec.dev_*.yaml`; image `registry.formspec.dev`; apiGroups K8s `formspec.dev`       |
| Docs         | `docs/cli-tools/*-forma-*.md` + `docs/runtimes/*-forma-*.md` di-rename; isi semua ditulis ulang                   |

## Scope exclusion

`docs_old/` dan `reff_docs/` (arsip read-only per `CLAUDE.md`) tidak diubah.

## Verifikasi

- `go build ./...` — clean
- `go test ./...` — 536 passed, 9 failed (e2e Clinic-UI-Showcase, pre-existing hook/state machine issues)
- `make generate-schema` — regen sukses, semua "FormSpecExpr" di deskripsi
- `make web-build` — frontend build sukses (vitest ran clean)
- `./bin/formspec validate --spec examples/cafe/spec` — 16/16 ✓
- `./bin/formspec validate --spec examples/arisan/spec` — 17/17 ✓
- Residual grep `\bforma\b` — 0 (semua hit di `.venv/` third-party atau false positive `format`/`performance`)

## Git repo

GitHub `primadi/forma` → `primadi/formspec` setelah commit ini dipush, lewat
Settings → General (redirect otomatis GitHub untuk clone/links/`go get`).
Remote lokal di `.git/config` bisa di-update ke `github.com/primadi/formspec.git`.

## Referensi

- Rencana: `docs/plan/rename-formspec.md`
- Master plan: `docs/plan/todo.md`

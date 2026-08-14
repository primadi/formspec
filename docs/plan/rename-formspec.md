# Plan — Rename "formspec" → "formspec"

**Tanggal**: 2026-08-12 · **Status**: In Progress
**Referensi**: `docs/spec/platform/01-overview.md` (visi), `CLAUDE.md` (konvensi repo),
`docs/plan/todo.md` (master plan)

## Latar Belakang

Nama produk **"FormSpec"** ternyata sudah dipakai produk AI code-generation lain
(`getformspec.dev`, Y Combinator S26, CLI `formspec`, roadmap Go Q3 2026). Untuk
menghindari kolisi merek/CLI/domain, produk di-rename total menjadi
**FormSpec** (`formspec.dev` — tersedia).

## Keputusan

1. Brand: **FormSpec**; domain: `formspec.dev`.
2. `FormSpecExpr` → **FormSpecExpr** (folder, ident, docs, schema description).
3. Rename **penuh** semua binary/CLI → `formspec*` (`formspec`, `formspec-ctl`,
   `formspec-operator`, `formspec-gen-schema`, `formspec-gen-kind-docs`).
4. GitHub repo `primadi/formspec` → `primadi/formspec` di-rename **setelah**
   migrasi kode selesai & test hijau (redirect otomatis GitHub menangani URL lama).
5. Scope exclusion: **`docs_old/` & `reff_docs/`** (arsip read-only) tidak diubah;
   `renderers/web/dist/` & `cmd/formspec/dist/` (build output) diregenerasi, tidak
   di-hand-edit.

## Mapping Nama

| Lama | Baru |
|---|---|
| `github.com/primadi/formspec` (go.mod + import) | `github.com/primadi/formspecspec` |
| `formspec.dev/v1` (apiVersion) | `formspec.dev/v1` |
| `formspec/v1` (VisualSpecKind) | `formspec/v1` |
| `formspec` CLI → `cmd/formspec/` | `formspec` → `cmd/formspec/` |
| `formspec-ctl` | `formspec-ctl` |
| `formspec-operator` | `formspec-operator` |
| `formspec-gen-schema` | `formspec-gen-schema` |
| `formspec-gen-kind-docs` | `formspec-gen-kind-docs` |
| `formspec-resource` / `formspec-control` / `formspec-sidecar` | `formspec-resource` / `formspec-control` / `formspec-sidecar` |
| `FormSpecError` | `FormSpecError` |
| `FormSpecExpr` / `formaexpr` | `FormSpecExpr` / `formspec-expr` |
| `FormSpecLib` / `FormSpecPage` / `FormaShell` / `FormaWidget` / `FormaContext` / `FormSpecElement` | `FormSpecLib` / `FormSpecPage` / `FormSpecShell` / `FormSpecWidget` / `FormSpecContext` / `FormSpecElement` |
| `formspec-app.yaml` | `formspec-app.yaml` |
| `schemas/formspec.schema.json` | `schemas/formspec.schema.json` |
| Error code prefix `FORMSPEC.` | `FORMSPEC.` |
| Java `io/formspec` | `io/formspec` |
| Python `lib_formspec` | `lib_formspec` |
| Ruby `lib/formspec` | `lib/formspec` |
| `registry.formspec.dev` (deploy) | `registry.formspec.dev` |
| API group K8s `formspec.dev` (CRD/rbac) | `formspec.dev` |

## Fase

0. Preparasi: dokumen ini + task di `docs/plan/todo.md` ✅ (file ini)
1. Backend Go core: `go.mod`, `pkg/spec/spec.go` (`APIVersion`),
   `pkg/spec/errors.go` (`FormSpecError`→`FormSpecError`), rename dir `cmd/formspec*`,
   `resource/formspec.go`→`resource/formspec.go`, semua import, fixtures.
2. Schema: `make generate-schema` → `schemas/formspec.schema.json`; update
   `.vscode/settings.json` (`yaml.schemas`); salinan schema di contoh & testdata.
3. apiVersion manifest: sed `formspec.dev/v1` → `formspec.dev/v1` di
   semua `*.yaml|yml` (contoh, testdata, CRD, rbac, image registry).
4. CLI & config: `formspec-app.yaml` di scaffold, help text, `.gitignore`.
5. Frontend: `src/lib/formspec-expr/` → `src/lib/formspec-expr/` + ident; logo
   `PrintRenderer`; `docs/spec/frontend/08-formaexpr.md` → `08-formspec-expr.md`.
6. SDK 9 bahasa: Go/Java/Python/Ruby/TS/Dotnet/Php/Browser.
7. Docs & instructions: `docs/cli-tools/`, `docs/runtimes/`, `CLAUDE.md`,
   `.github/`, `ai_skills/`, contoh.
8. Verifikasi: `go build` + `go test ./...`, `make generate-schema`,
   `make web-build`, `formspec validate` tiap contoh, grep residual.
9. Git: changelog + commit lokal; rename GitHub repo setelah fase 8 hijau.

## Catatan Implementasi

- Sed dilakukan case-sensitive & word-boundary-aware (`\bforma\b`) supaya tidak
  merusak kata Inggris seperti `platform`, `format`, `information`.
- `dist/` (build output) dikecualikan dari sed; diregenerasi via `make web-build`.
- `docs_old/` & `reff_docs/` dibiarkan (arsip); referensi historis tetap ada.

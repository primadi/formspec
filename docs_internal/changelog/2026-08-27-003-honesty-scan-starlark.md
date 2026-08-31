# 2026-08-27-003 — 3.1.1a: Honesty Scan Starlark di `formspec validate`

## Apa yang diubah

Implementasi honesty scan Starlark (todo 3.1.1a) — analisis statis setiap script
yang direferensikan action/hook (`impl: script_ref|script`) terhadap blok
`uses:` yang dideklarasikan:

- **Undeclared usage → ERROR**: script memakai `ctx.<primitive>` (closed set
  resolver-routed: db/cache/lock/queue/pubsub/storage/kvstore), target
  cross-module via `resource.call/fetch/create`, atau `ctx.secrets.get("key")`
  yang tidak dideklarasikan → gagal `USES_VIOLATION` di ProdMode/StrictMode.
- **Declared-but-unused → WARNING**: deklarasi `uses` yang tidak pernah
  dipakai script — consent footprint lebih besar dari realita.
- **`ctx.environment` branching → WARNING**: logika tergantung environment
  adalah konsep Control Plane; single-server tidak punya environment switch.
- **`--fix`**: menghapus declared-but-unused (prune list kosong + blok `uses`
  kosong) — TIDAK pernah menambah deklarasi (perluasan consent tetap manual,
  preseden 3.1.2).

**`cmd/formspec/honesty.go` (baru):**

- Parser AST `go.starlark.net/syntax` dengan walker manual (versi starlark-go
  ini tidak menyediakan visitor generik; `DotExpr.Name` bertipe `*Ident`,
  `AssignStmt` mencakup augmented assignment, `syntax.Parse(path, src, 0)`).
- Resolusi path script mengikuti `internal/action.resolveScript` (entity-dir
  dulu, lalu fallback spec-root module-scoped).
- Cakupan: Entity actions + Entity hooks + Service actions.
- Semantik deklarasi resource selaras runtime enforcement: hanya target
  cross-module (mengandung `.` atau `/`) yang wajib dideklarasikan; bare name
  = same-module implicit; wildcard `{m}.*`/`*` dihormati; `*` dianggap unused
  hanya bila tidak ada usage sama sekali.
- Wire ke `cmd/formspec/validate.go` sebagai Layer 1.6 + flag `--fix`;
  warning tidak membuat exit 1 (advisory).

**Test:** `cmd/formspec/honesty_test.go` (6 test): undeclared primitive error,
declared-but-unused warning, honest-uses clean, ctx.environment warning,
undeclared cross-module resource error, --fix removes + prune + re-scan clean.
Suite penuh: **878 pass, 0 fail**.

**Validasi dunia nyata:** `formspec validate` pada `examples/Clinic-UI-Showcase`
→ 0 false positive (akses bare `medicine` same-module tidak lagi ditandai) +
4 warning genuine: deklarasi mati `medicine.find`/`medicine.update` di action
`sell`/`dispatch` module pharmacy (entity `medicine` ada di module yang sama,
jadi deklarasi itu tidak pernah dicek runtime). Bisa dibersihkan dengan
`formspec validate --spec ... --fix`.

## Kenapa diubah

Menutup item 3.1.1a yang tertunda sejak 3.1.1 (2026-07-31) — kini analyzer
AST cukup matang dan semantik `uses` sudah stabil pasca-Fase 6/7.

## File terdampak

- `cmd/formspec/honesty.go` (baru), `honesty_test.go` (baru), `validate.go`
- `docs/plan/todo.md`

## Referensi

- Plan: `docs/plan/todo.md` §3.1.1a · Spec: `docs/spec/backend/01-core-basic.md` §5/§11.2

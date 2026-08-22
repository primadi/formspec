# Fase 6 Dogfooding — Consent Footprint + ABAC (Fase K)

**Tanggal**: 2026-08-21 · **Sequence**: 013
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase K)

## Apa yang diubah

Consent footprint (todo 6.2.5) + ABAC evaluator (todo 6.2.6).

### Fase K — selesai (K1 penuh; K2 mekanisme inti)

- **K1** (6.2.5) Consent footprint — `formspec check --footprint` (baru):
  membangun `permission.Registry` dari manifests dan mencetak footprint per
  module (required permissions + uses declarations + cross-module writes
  high-risk, D46) via `ModuleFootprint.String()`. Output untuk workspace owner
  saat install.
- **K2** (6.2.6) ABAC — `internal/auth/abac.go` (baru):
  `EvaluateGrantConditions(conditions, resourceData, params)` — mengevaluasi
  `ConditionGrant.Expr` (FormSpecExpr, bracket syntax `resource["branch"]`)
  terhadap `resource` + `params` via `starlark.EvalExpr`; gagal → error dengan
  `Message`. Ini primitif enforcement attribute-based authorization.

> K2 enforcement penuh (mengevaluasi kondisi role grant saat request custom
> action) adalah follow-up — evaluator inti sudah siap; wiring ke enforcement
> point butuh membawa conditions dari role grants ke request.

## Kenapa

Consent footprint memberi workspace owner visibilitas akses module sebelum
install; ABAC memungkinkan role grant action hanya di bawah constraint atribut
(mis. kode cabang).

## File yang terkena dampak

- `cmd/formspec/check.go` — flag `--footprint` + `printConsentFootprint`
- `internal/auth/abac.go` + `abac_test.go` (baru)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- `formspec check --footprint` pada `verticals/billing/spec` mencetak footprint
  billing (5 permissions, 2 uses).
- Test ABAC: pass, fail (dengan message), empty expr skipped, params — hijau.

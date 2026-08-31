# Fase 6 Dogfooding — `ctx.secrets` (Fase I)

**Tanggal**: 2026-08-21 · **Sequence**: 011
**Plan**: `docs/plan/fase6-dogfooding-auth-module.md` (Fase I)

## Apa yang diubah

Menghidupkan `ctx.secrets` — jalur satu-satunya untuk membaca Config key
`secret: true` (todo 6.8).

### Fase I — selesai (mekanisme; populasi store menunggu Config runtime 7.2)

- **I1** (6.8.1) `ctx.secrets().get(key)` — `secretsAPI` (baru) di
  `internal/starlark/context.go`; `ScriptExecutor.SecretsStore` +
  `SetSecretsStore` + `SetSecretsAudit`; di-wire di konstruksi CtxAPI.
- **I2** (6.8.2) `uses.secrets` enforcement — hanya key yang dideklarasikan di
  `uses.secrets` yang bisa dibaca; selain itu error `USES_VIOLATION`.
- **I3** (6.8.3) Secret tak pernah di log — `secretsAPI` tidak pernah menulis
  nilai secret ke log.
- **I4** (6.8.4) Audit tiap secret read — hook `SecretsAudit(key)` dipanggil
  pada setiap read sukses.

> Populasi `SecretsStore` dari Config keys `secret: true` menunggu Config
> runtime (Fase 7.2) — mekanisme sudah siap, store diisi saat Config runtime
> landing.

## Kenapa

Secret Config keys tidak boleh dibaca via `ctx.config` (yang non-secret);
`ctx.secrets` adalah jalur eksplisit yang di-gate oleh `uses.secrets`, tidak
pernah di-log, dan diaudit.

## File yang terkena dampak

- `internal/starlark/context.go` — `secretsAPI` + `Secrets` field di CtxAPI
- `internal/starlark/executor.go` — `SecretsStore`/`SecretsAudit` + wiring +
  `declaredUsesSecrets`
- `internal/starlark/secrets_test.go` (baru)

## Verifikasi

- `go build ./...` + `go test ./...` hijau.
- Test `secretsAPI`: get declared (value + audit), undeclared blocked, missing
  key → None — hijau.
